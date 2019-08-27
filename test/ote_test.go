package test

import (
	"fabric-orderer-benchmark/server/ote"
	"testing"
)

func TestProducer(t *testing.T) {
	o := ote.NewOTE()
	o.StartProducer("localhost:7050", "mychannel1", 0)
}
