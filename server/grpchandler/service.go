package grpchandler

import (
	pb "fabric-orderer-benchmark/protos"
	"golang.org/x/net/context"
)

type OteService struct {
}

func NewOteService() *OteService {
	return &OteService{}
}

func (c *OteService) SendTransaction(ctx context.Context, r *pb.SendTransactionRequest) (*pb.ServerStatus, error) {
	getEngine().StartProducer("localhost:7050", "mychannel1")
	return &pb.ServerStatus{Status: pb.StatusCode_SUCCESS}, nil
}
