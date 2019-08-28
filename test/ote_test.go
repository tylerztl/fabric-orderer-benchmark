package test

import (
	pb "fabric-orderer-benchmark/protos"
	"fmt"
	"testing"

	"golang.org/x/net/context"
)

func sendTransaction() (pb.StatusCode, error) {
	conn := NewConn()
	defer conn.Close()

	c := pb.NewOteClient(conn)
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
