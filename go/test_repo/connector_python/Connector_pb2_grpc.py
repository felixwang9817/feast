# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import grpc

import Connector_pb2 as feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2


class OnlineStoreStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.OnlineRead = channel.unary_unary(
                '/grpc.connector.OnlineStore/OnlineRead',
                request_serializer=feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadRequest.SerializeToString,
                response_deserializer=feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadResponse.FromString,
                )


class OnlineStoreServicer(object):
    """Missing associated documentation comment in .proto file."""

    def OnlineRead(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')


def add_OnlineStoreServicer_to_server(servicer, server):
    rpc_method_handlers = {
            'OnlineRead': grpc.unary_unary_rpc_method_handler(
                    servicer.OnlineRead,
                    request_deserializer=feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadRequest.FromString,
                    response_serializer=feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadResponse.SerializeToString,
            ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
            'grpc.connector.OnlineStore', rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))


 # This class is part of an EXPERIMENTAL API.
class OnlineStore(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def OnlineRead(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/grpc.connector.OnlineStore/OnlineRead',
            feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadRequest.SerializeToString,
            feast_dot_third__party_dot_grpc_dot_connector_dot_Connector__pb2.OnlineReadResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)
