package grpchandler

import (
	"fabric-orderer-benchmark/server/sdkprovider"
	"golang.org/x/net/context"

	pb "fabric-orderer-benchmark/protos"
)

type ChannelService struct {
	provider sdkprovider.SdkProvider
}

func NewChannelService() *ChannelService {
	return &ChannelService{
		provider: GetSdkProvider(),
	}
}

func (c *ChannelService) SendTransaction(ctx context.Context, r *pb.SendTransactionRequest) (*pb.ServerStatus, error) {
	code, err := c.provider.SendTransaction()
	return &pb.ServerStatus{Status: code}, err
}
