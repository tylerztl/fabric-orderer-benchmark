package server

import (
	pb "fabric-orderer-benchmark/protos"
	"fabric-orderer-benchmark/server/grpchandler"
	"fabric-orderer-benchmark/server/helpers"
	"net"

	"google.golang.org/grpc"
)

var (
	ServerPort string
	EndPoint   string
)

var logger = helpers.GetLogger()

func Run() (err error) {
	EndPoint = ":" + ServerPort
	conn, err := net.Listen("tcp", EndPoint)
	if err != nil {
		logger.Error("TCP Listen err:%s", err)
	}

	srv := newGrpc()
	logger.Info("gRPC and https listen on: %s", ServerPort)

	if err = srv.Serve(conn); err != nil {
		logger.Error("ListenAndServe: %s", err)
	}

	return err
}

func newGrpc() *grpc.Server {
	server := grpc.NewServer()
	// TODO
	pb.RegisterChannelServer(server, grpchandler.NewChannelService())

	return server
}
