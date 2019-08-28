package test

import (
	pb "fabric-orderer-benchmark/protos"
	"testing"
)

func TestCreateChannel(t *testing.T) {
	status, err := CreateChannel("mychannel")
	if status != pb.StatusCode_SUCCESS || err != nil {
		t.Error("Create channel failed")
	}
}
