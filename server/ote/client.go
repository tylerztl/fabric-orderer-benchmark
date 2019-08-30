package ote

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/crypto"
	cb "github.com/hyperledger/fabric/protos/common"
	ab "github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/pkg/errors"
)

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
	client   ab.AtomicBroadcast_BroadcastClient
	clientId uint32
	mutex    sync.Mutex
	chanID   string
	signer   crypto.LocalSigner
}

func newDeliverClient(client ab.AtomicBroadcast_DeliverClient, chanID string, signer crypto.LocalSigner) *DeliverClient {
	return &DeliverClient{
		client: client,
		chanID: chanID,
		signer: signer,
	}
}

func newBroadcastClient(client ab.AtomicBroadcast_BroadcastClient, clientId uint32, chanID string, signer crypto.LocalSigner) *BroadcastClient {
	return &BroadcastClient{
		client:   client,
		clientId: clientId,
		chanID:   chanID,
		signer:   signer,
	}
}

func (d *DeliverClient) seekHelper(chanID string, start *ab.SeekPosition, stop *ab.SeekPosition) *cb.Envelope {
	seekInfo := &ab.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: ab.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := utils.CreateSignedEnvelope(cb.HeaderType_DELIVER_SEEK_INFO, d.chanID, d.signer, seekInfo, int32(0), uint64(0))
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

func (d *DeliverClient) seekSpecified(blockNumber uint64) error {
	specific := &ab.SeekPosition{Type: &ab.SeekPosition_Specified{Specified: &ab.SeekSpecified{Number: blockNumber}}}
	return d.client.Send(d.seekHelper(d.chanID, specific, specific))
}

func (d *DeliverClient) readUntilClose() {
	for {
		msg, err := d.client.Recv()
		if err != nil {
			panic(fmt.Sprintf("Consumer recv error: %v", err))
		}
		switch t := msg.Type.(type) {
		case *ab.DeliverResponse_Status:
			Logger.Info(fmt.Sprintf("Got DeliverResponse_Status: %v", t))
		case *ab.DeliverResponse_Block:
			go transactionResponse(t.Block)
		}
	}
}

func transactionResponse(block *cb.Block) {
	if block.Header.Number == 0 {
		return
	}
	for _, envBytes := range block.Data.Data {
		envelope, err := utils.GetEnvelopeFromBlock(envBytes)
		if err != nil {
			Logger.Error("Error GetEnvelopeFromBlock:", err)
		}
		payload, err := utils.GetPayload(envelope)
		if err != nil {
			Logger.Error("Error GetPayload:", err)
		}
		msg := cb.ConfigValue{}
		if err := proto.Unmarshal(payload.Data, &msg); err != nil {
			Logger.Error("Error proto unmarshal", err)
		}
		txId, err := strconv.ParseUint(string(msg.Value), 10, 64)
		if err != nil {
			Logger.Error("Error ParseUint:", err)
		}

		Logger.Info("Seek block number:%d, payload:%d", block.Header.Number, txId)
		//if txChan := txPool.getChanByTxId(txId); txChan != nil {
		//	txChan <- block.Header.Number
		//}
		if txChan, ok := ReqChans.Load(txId); txChan != nil && ok {
			txChan.(chan uint64) <- block.Header.Number
		}
	}
}

func (b *BroadcastClient) broadcast(transaction []byte) error {
	env, err := utils.CreateSignedEnvelope(cb.HeaderType_MESSAGE, b.chanID, b.signer, &cb.ConfigValue{Value: transaction}, 0, 0)
	if err != nil {
		return err
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	if err := b.client.Send(env); err != nil {
		return errors.WithMessage(err, "could not send")
	}
	return b.getAck()
}

func (b *BroadcastClient) getAck() error {
	msg, err := b.client.Recv()
	if err != nil {
		return err
	}
	if msg.Status != cb.Status_SUCCESS {
		return fmt.Errorf("catch unexpected status: %v", msg.Status)
	}
	return nil
}
