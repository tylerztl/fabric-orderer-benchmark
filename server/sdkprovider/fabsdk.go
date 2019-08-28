package sdkprovider

import (
	pb "fabric-orderer-benchmark/protos"
	"fabric-orderer-benchmark/server/helpers"
	"fabric-orderer-benchmark/server/ote"
)

var logger = helpers.GetLogger()

type FabSdkProvider struct {
	OrdererEngine *ote.OrdererTrafficEngine
}

func NewFabSdkProvider() (*FabSdkProvider, error) {
	provider := &FabSdkProvider{
		OrdererEngine: ote.NewOTE(),
	}

	go provider.OrdererEngine.StartConsumer("localhost:7050", "mychannel1", 0)

	return provider, nil
}

func (f *FabSdkProvider) SendTransaction() (pb.StatusCode, error) {
	f.OrdererEngine.StartProducer("localhost:7050", "mychannel1", 0)
	return pb.StatusCode_SUCCESS, nil
}
