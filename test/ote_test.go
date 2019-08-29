package test

import (
	pb "fabric-orderer-benchmark/protos"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/net/context"
)

var producersWG sync.WaitGroup

func sendTransaction() (pb.StatusCode, error) {
	conn := NewConn()
	defer conn.Close()

	c := pb.NewOteClient(conn)
	context := context.Background()
	body := &pb.SendTransactionRequest{}

	if r, err := c.SendTransaction(context, body); err != nil {
		fmt.Println("err: ", err.Error())
		return pb.StatusCode_FAILED, err
	} else {
		return r.Status, nil
	}
}

func syncSendTransaction(c pb.OteClient) (pb.StatusCode, error) {
	context := context.Background()
	body := &pb.SendTransactionRequest{}

	if r, err := c.SendTransaction(context, body); err != nil {
		producersWG.Done()
		fmt.Println("err: ", err.Error())
		return pb.StatusCode_FAILED, err
	} else {
		producersWG.Done()
		fmt.Println("Status: ", r.Status)
		return r.Status, nil
	}
}

func TestSendTransaction(t *testing.T) {
	status, err := sendTransaction()
	if status != pb.StatusCode_SUCCESS || err != nil {
		t.Error("Send transaction failed")
	}
}

func TestSyncSendTransaction(t *testing.T) {
	conn := NewConn()
	defer conn.Close()
	c := pb.NewOteClient(conn)

	txNums := 10000
	producersWG.Add(txNums)
	for i := 0; i < txNums; i++ {
		go syncSendTransaction(c)
	}
	producersWG.Wait()
}
