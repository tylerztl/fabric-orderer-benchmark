package ote

import (
	"fabric-orderer-benchmark/server/helpers"
	"fmt"
	"math"
	"strings"

	"github.com/hyperledger/fabric/common/crypto"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
)

var logger = helpers.GetLogger()

var (
	oldest  = &ab.SeekPosition{Type: &ab.SeekPosition_Oldest{Oldest: &ab.SeekOldest{}}}
	newest  = &ab.SeekPosition{Type: &ab.SeekPosition_Newest{Newest: &ab.SeekNewest{}}}
	maxStop = &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: math.MaxUint64}}}
)

type DeliverClient struct {
	client ab.AtomicBroadcast_DeliverClient
	chanID string
	signer crypto.LocalSigner
}

type BroadcastClient struct {
	client ab.AtomicBroadcast_BroadcastClient
	chanID string
	signer crypto.LocalSigner
}

func NewDeliverClient(client ab.AtomicBroadcast_DeliverClient, chanID string, signer crypto.LocalSigner) *DeliverClient {
	return &DeliverClient{
		client: client,
		chanID: chanID,
		signer: signer,
	}
}

func NewBroadcastClient(client ab.AtomicBroadcast_BroadcastClient, chanID string, signer crypto.LocalSigner) *BroadcastClient {
	return &BroadcastClient{
		client: client,
		chanID: chanID,
		signer: signer,
	}
}

func (d *DeliverClient) seekHelper(chanID string, start *ab.SeekPosition, stop *ab.SeekPosition) *cb.Envelope {
	env, err := utils.CreateSignedEnvelope(cb.HeaderType_DELIVER_SEEK_INFO, d.chanID, d.signer, &ab.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: ab.SeekInfo_BLOCK_UNTIL_READY,
	}, 0, 0)
	if err != nil {
		panic(err)
	}
	return env
}

func (d *DeliverClient) seekOldest() error {
	return d.client.Send(d.seekHelper(d.chanID, oldest, maxStop))
}

func (d *DeliverClient) seekNewest() error {
	return d.client.Send(d.seekHelper(d.chanID, newest, maxStop))
}

func (d *DeliverClient) seekSingle(blockNumber uint64) error {
	specific := &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: blockNumber}}}
	return d.client.Send(d.seekHelper(d.chanID, specific, specific))
}

func (d *DeliverClient) readUntilClose(ordererIndex int, channelIndex int, txRecvCntrP *int64, blockRecvCntrP *int64) {
	for {
		msg, err := d.client.Recv()
		if err != nil {
			if !strings.Contains(err.Error(), "is closing") {
				// print if we do not see the msg indicating graceful closing of the connection
				logger.Error(fmt.Sprintf("Consumer for orderer %d channel %d readUntilClose() Recv error: %v", ordererIndex, channelIndex, err))
			}
			return
		}
		switch t := msg.Type.(type) {
		case *ab.DeliverResponse_Status:
			logger.Info(fmt.Sprintf("Got DeliverResponse_Status: %v", t))
			return
		case *ab.DeliverResponse_Block:
			*txRecvCntrP += int64(len(t.Block.Data.Data))
			*blockRecvCntrP++
			logger.Info(fmt.Sprintf("Block.Data: %v", t.Block.Data))
		}
	}
}

func (b *BroadcastClient) Broadcast(transaction []byte) error {
	env, err := utils.CreateSignedEnvelope(cb.HeaderType_MESSAGE, b.chanID, b.signer, &cb.ConfigValue{Value: transaction}, 0, 0)
	if err != nil {
		panic(err)
	}
	return b.client.Send(env)
}

func (b *BroadcastClient) GetAck() error {
	msg, err := b.client.Recv()
	if err != nil {
		return err
	}
	if msg.Status != cb.Status_SUCCESS {
		return fmt.Errorf("catch unexpected status: %v", msg.Status)
	}
	return nil
}
