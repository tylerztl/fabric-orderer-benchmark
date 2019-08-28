package sdkprovider

import (
	pb "fabric-orderer-benchmark/protos"
)

type SdkProvider interface {
	SendTransaction() (pb.StatusCode, error)
}
