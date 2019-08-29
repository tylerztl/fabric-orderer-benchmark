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
	err := getEngine().TransactionProducer()
	if err != nil {
		return &pb.ServerStatus{Status: pb.StatusCode_FAILED}, err
	}
	return &pb.ServerStatus{Status: pb.StatusCode_SUCCESS}, nil
}
