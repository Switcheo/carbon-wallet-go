package api

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// GetGRPCConnection Obtains a gRPC connection
// CONTRACT: should always close the connection after using: defer grpcConn.Close()
// Example:
// grpcConn, _ := getGRPCConnection("127.0.0.1:9090")
// defer grpcConn.Close()
func GetGRPCConnection(targetGRPCAddress string, clientCtx client.Context) (*grpc.ClientConn, error) {
	// log.Info("Obtaining gRPC connection from: ", targetGRPCAddress)
	// Create a connection to the gRPC server.
	grpcConn, err := grpc.Dial(
		targetGRPCAddress,   // your gRPC server address.
		grpc.WithInsecure(), // The SDK doesn't support any transport security mechanism.
		// if the request/response types contain interface instead of 'nil' you should pass the application specific codec.
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.NewProtoCodec(clientCtx.InterfaceRegistry).GRPCCodec())),
	)
	if err != nil {
		log.Error("Failed to obtain gRPC connection from: ", targetGRPCAddress)
		return nil, err
	}
	return grpcConn, nil
}
