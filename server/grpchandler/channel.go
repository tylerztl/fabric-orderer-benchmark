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

func (c *ChannelService) CreateChannel(ctx context.Context, r *pb.CreateChannelRequest) (*pb.CreateChannelResponse, error) {
	transactionID, code, err := c.provider.CreateChannel(r.ChannelId)
	return &pb.CreateChannelResponse{Status: code, TransactionId: string(transactionID)}, err
}

func (c *ChannelService) SendTransaction(ctx context.Context, r *pb.SendTransactionRequest) (*pb.ServerStatus, error) {
	code, err := c.provider.SendTransaction()
	return &pb.ServerStatus{Status: code}, err
}
