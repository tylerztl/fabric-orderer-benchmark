package test

import (
	pb "fabric-orderer-benchmark/protos"
	"fabric-orderer-benchmark/server/ote"
	"fmt"
	"golang.org/x/net/context"
	"testing"
)

func TestProducer(t *testing.T) {
	o := ote.NewOTE()
	o.StartProducer("localhost:7050", "mychannel1", 0)

}

func sendTransaction() (pb.StatusCode, error) {
	conn := NewConn()
	defer conn.Close()

	c := pb.NewChannelClient(conn)
	context := context.Background()
	body := &pb.SendTransactionRequest{}

	r, err := c.SendTransaction(context, body)
	fmt.Printf("StatusCode: %s, err: %v\n", r.Status, err)
	return r.Status, err
}

func TestSendTransaction(t *testing.T) {
	status, err := sendTransaction()
	if status != pb.StatusCode_SUCCESS || err != nil {
		t.Error("Send transaction failed")
	}
}
