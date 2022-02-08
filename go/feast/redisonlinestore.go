package feast

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/feast-dev/feast/go/protos/feast/types"
	"github.com/feast-dev/feast/go/protos/feast/serving"
	"github.com/golang/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"github.com/go-redis/redis/v8"
	"github.com/spaolacci/murmur3"
	"sort"
	"strings"
)

type redisType int

const (
	redisNode    redisType = 0
	redisCluster redisType = 1
)

type RedisOnlineStore struct {

	// Feast project name
	// TODO (woop): Should we remove project as state that is tracked at the store level?
	project string

	// Redis database type, either a single node server (RedisType.Redis) or a cluster (RedisType.RedisCluster)
	t redisType

	// Redis client connector
	client *redis.Client
}

func NewRedisOnlineStore(project string, onlineStoreConfig map[string]interface{}) (*RedisOnlineStore, error) {
	store := RedisOnlineStore{project: project}

	var address []string
	var password string

	// Parse redis_type and write it into conf.t
	t, err := getRedisType(onlineStoreConfig)
	if err != nil {
		return nil, err
	}

	// Parse connection_string and write it into conf.address, conf.password, and conf.ssl
	redisConnJson, ok := onlineStoreConfig["connection_string"]
	if !ok {
		// default to "localhost:6379"
		redisConnJson = "localhost:6379"
	}
	if redisConnStr, ok := redisConnJson.(string); !ok {
		return nil, errors.New(fmt.Sprintf("Failed to convert connection_string to string: %+v", redisConnJson))
	} else {
		parts := strings.Split(redisConnStr, ",")
		for _, part := range parts {
			if strings.Contains(part, ":") {
				address = append(address, part)
			} else if strings.Contains(part, "=") {
				kv := strings.SplitN(part, "=", 2)
				if kv[0] == "password" {
					password = kv[1]
				} else if kv[0] == "ssl" {
					// TODO (woop): Add support for TLS/SSL
					//ssl = kv[1] == "true"
				} else {
					return nil, errors.New(fmt.Sprintf("Unrecognized option in connection_string: %s. Must be one of 'password', 'ssl'", kv[0]))
				}
			} else {
				return nil, errors.New(fmt.Sprintf("Unable to parse a part of connection_string: %s. Must contain either ':' (addresses) or '=' (options", part))
			}
		}
	}

	if t == redisNode {
		store.client = redis.NewClient(&redis.Options{
			Addr:     address[0],
			Password: password, // no password set
			DB:       0,        // use default DB

		})
	} else {
		return nil, errors.New("only single node Redis is supported at this time")
	}

	return &store, nil
}

func getRedisType(onlineStoreConfig map[string]interface{}) (redisType, error) {
	var t redisType

	redisTypeJson, ok := onlineStoreConfig["redis_type"]
	if !ok {
		// default to "redis"
		redisTypeJson = "redis"
	} else if redisTypeStr, ok := redisTypeJson.(string); !ok {
		return -1, errors.New(fmt.Sprintf("Failed to convert redis_type to string: %+v", redisTypeJson))
	} else {
		if redisTypeStr == "redis" {
			t = redisNode
		} else if redisTypeStr == "redis_cluster" {
			t = redisCluster
		} else {
			return -1, errors.New(fmt.Sprintf("Failed to convert redis_type to enum: %s. Must be one of 'redis', 'redis_cluster'", redisTypeStr))
		}
	}
	return t, nil
}

func (r *RedisOnlineStore) OnlineRead(entityKeys []types.EntityKey, view string, features []string) ([][]Feature, error) {
	featureCount := len(features)
	var hsetKeys = make([]string, featureCount+1)
	h := murmur3.New32()
	intBuffer := h.Sum32()
	byteBuffer := make([]byte, 4)

	for i := 0; i < featureCount; i++ {
		h.Write([]byte(view + ":" + features[i]))
		intBuffer = h.Sum32()
		binary.LittleEndian.PutUint32(byteBuffer, intBuffer)
		hsetKeys[i] = string(byteBuffer)
		h.Reset()
	}

	tsKey := fmt.Sprintf("_ts:%s", view)
	hsetKeys[featureCount] = tsKey
	features = append(features, tsKey)

	redisKeys := make([]*[]byte, len(entityKeys))
	for i := 0; i < len(entityKeys); i++ {

		var key, err = BuildRedisKey(r.project, entityKeys[i])
		if err != nil {
			return nil, err
		}
		redisKeys[i] = key
	}

	// Retrieve features from Redis
	// TODO: Move context object out
	ctx := context.Background()

	results := make([][]Feature, len(entityKeys))

	for entityIndex, redisKey := range redisKeys {
		results[entityIndex] = make([]Feature, len(features))
		keyString := string(*redisKey)
		// TODO: Add pipelining (without transactions)
		res, err := r.client.HMGet(ctx, keyString, hsetKeys...).Result()
		if err != nil {
			return nil, err
		}
		var timeStamp timestamppb.Timestamp
		timeStampInterface := res[len(res)-1]
		if timeStampString, ok := timeStampInterface.(string); !ok {
			return nil, errors.New("Error parsing value from redis")
		} else {
			if err := proto.Unmarshal([]byte(timeStampString), &timeStamp); err != nil {
				return nil, errors.New("Error converting parsed redis value to timestamppb.Timestamp")
			}
		}
		res = res[:len(res)-1]
		
		for featureIndex,resString := range res {
			if valueString, ok := resString.(string); !ok {
				return nil, errors.New("Error parsing value from redis")
			} else {
				var value types.Value
				if err := proto.Unmarshal([]byte(valueString), &value); err != nil {
					return nil, errors.New("Error converting parsed redis value to types.Value")
				} else {
					featureName := features[ featureIndex ]
					ref := serving.FeatureReferenceV2{ 	FeatureViewName: view,
						FeatureName: featureName}
					feature := Feature { 	reference: ref,
											timestamp: timeStamp,
											value: value }
					results[entityIndex][featureIndex] = feature
				}
			}
		}
		
	}

	return results, nil
}

func BuildRedisKey(project string, entityKey types.EntityKey) (*[]byte, error) {
	serKey, err := SerializeEntityKey(entityKey)
	if err != nil {
		return nil, err
	}

	fullKey := append(*serKey, []byte(project)...)
	return &fullKey, nil
}

func SerializeEntityKey(entityKey types.EntityKey) (*[]byte, error) {
	//    Serialize entity key to a bytestring so that it can be used as a lookup key in a hash table.

	// Ensure that we have the right amount of join keys and entity values
	if len(entityKey.JoinKeys) != len(entityKey.EntityValues) {
		return nil, errors.New(fmt.Sprintf("The amount of join key names and entity values don't match: %s vs %s", entityKey.JoinKeys, entityKey.EntityValues))
	}

	// Make sure that join keys are sorted so that we have consistent key building
	m := make(map[string]*types.Value)

	for i := 0; i < len(entityKey.JoinKeys); i++ {
		m[entityKey.JoinKeys[i]] = entityKey.EntityValues[i]
	}

	keys := make([]string, 0, len(m))
	for k := range entityKey.JoinKeys {
		keys = append(keys, entityKey.JoinKeys[k])
	}
	sort.Strings(keys)

	// Build the key
	length := 5 * len(keys)
	bufferList := make([][]byte, length)

	for i := 0; i < len(keys); i++ {
		offset := i * 2
		byteBuffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(byteBuffer, uint32(types.ValueType_Enum_value["STRING"]))
		bufferList[offset] = byteBuffer
		bufferList[offset+1] = []byte(keys[i])
	}

	for i := 0; i < len(keys); i++ {
		offset := (2 * len(keys)) + (i * 3)
		value := m[keys[i]].GetVal()

		valueBytes, valueTypeBytes, err := SerializeValue(value)
		if err != nil {
			return valueBytes, err
		}

		typeBuffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(typeBuffer, uint32(valueTypeBytes))

		lenBuffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(lenBuffer, uint32(len(*valueBytes)))

		bufferList[offset+0] = typeBuffer
		bufferList[offset+1] = lenBuffer
		bufferList[offset+2] = *valueBytes
	}

	// Convert from an array of byte arrays to a single byte array
	var entityKeyBuffer []byte
	for i := 0; i < len(bufferList); i++ {
		entityKeyBuffer = append(entityKeyBuffer, bufferList[i]...)
	}

	return &entityKeyBuffer, nil
}

func SerializeValue(value interface{}) (*[]byte, types.ValueType_Enum, error) {
	// TODO: Implement support for other types (at least the major types like ints, strings, bytes)
	switch x := (value).(type) {
	case *types.Value_StringVal:
		valueString := []byte(x.StringVal)
		return &valueString, types.ValueType_STRING, nil
	case *types.Value_BytesVal:
		return &x.BytesVal, types.ValueType_BYTES, nil
	case *types.Value_Int32Val:
		valueBuffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(valueBuffer, uint32(x.Int32Val))
		return &valueBuffer, types.ValueType_INT32, nil
	case *types.Value_Int64Val:
		// TODO (woop): We unfortunately have to use 32 bit here for backward compatibility :(
		valueBuffer := make([]byte, 4)
		binary.LittleEndian.PutUint32(valueBuffer, uint32(x.Int64Val))
		return &valueBuffer, types.ValueType_INT64, nil
	case nil:
		return nil, types.ValueType_INVALID, fmt.Errorf("could not detect type for %v", x)
	default:
		return nil, types.ValueType_INVALID, fmt.Errorf("could not detect type for %v", x)
	}
}
