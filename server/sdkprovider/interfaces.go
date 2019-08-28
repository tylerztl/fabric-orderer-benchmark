package sdkprovider

import (
	pb "fabric-orderer-benchmark/protos"
	. "fabric-orderer-benchmark/server/helpers"
)

type SdkProvider interface {
	CreateChannel(channelID string) (TransactionID, pb.StatusCode, error)
	SendTransaction() (pb.StatusCode, error)
}
