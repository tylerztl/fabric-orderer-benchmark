package ote

import (
	"errors"
	"fabric-orderer-benchmark/server/helpers"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/localmsp"
	genesisconfig "github.com/hyperledger/fabric/common/tools/configtxgen/localconfig" // config for genesis.yaml
	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	ordererConf "github.com/hyperledger/fabric/orderer/common/localconfig" // config, for the orderer.yaml
	ab "github.com/hyperledger/fabric/protos/orderer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	MaxGrpcMsgSize = 1000 * 1024 * 1024
	ConnTimeout    = 30 * time.Second
	Logger         = helpers.GetLogger()
	AppConf        = helpers.GetAppConf().Conf
)

var reqChans = make(map[uint64]chan uint64)

type OrdererTrafficEngine struct {
	ordConf        *ordererConf.TopLevel
	genConf        *genesisconfig.Profile
	signer         crypto.LocalSigner
	ordererClients map[string][]*BroadcastClient
	txId           uint64
	txIdMutex      *sync.Mutex
}

func NewOTE() *OrdererTrafficEngine {
	_ = os.Setenv("FABRIC_CFG_PATH", helpers.GetSampleConfigPath())
	defer func() {
		_ = os.Unsetenv("FABRIC_CFG_PATH")
	}()

	// Establish the default configuration from yaml files - and this also
	// picks up any variables overridden on command line or in environment
	ordConf, err := ordererConf.Load()
	if err != nil {
		panic(" Cannot Load orderer config data: " + err.Error())
	}

	genConf := genesisconfig.Load(strings.ToLower(AppConf.Profile))

	if len(AppConf.ConnOrderers) == 0 {
		panic(" Cannot find connect orderers config")
	}
	fpath := helpers.GetCryptoConfigPath(fmt.Sprintf("ordererOrganizations/example.com/orderers/%s"+"*", AppConf.ConnOrderers[0].Host))
	matches, err := filepath.Glob(fpath)
	if err != nil {
		panic(fmt.Sprintf("Cannot find filepath %s ; err: %v\n", fpath, err))
	} else if len(matches) != 1 {
		panic(fmt.Sprintf("No msp directory filepath name matches: %s\n", fpath))
	}
	ordConf.General.BCCSP.SwOpts.FileKeystore.KeyStorePath = fmt.Sprintf("%s/msp/keystore", matches[0])
	err = mspmgmt.LoadLocalMsp(fmt.Sprintf("%s/msp", matches[0]), ordConf.General.BCCSP, AppConf.OrdererMsp)
	if err != nil { // Handle errors reading the config file
		panic(fmt.Sprintf("Failed to initialize local MSP: %v", err))
	}
	signer := localmsp.NewSigner()

	engine := &OrdererTrafficEngine{
		ordConf,
		genConf,
		signer,
		initOrdererClients(signer),
		0,
		new(sync.Mutex),
	}

	var serverAddr string
	if AppConf.Local {
		serverAddr = fmt.Sprintf("localhost:%d", AppConf.ConnOrderers[0].Port)
	} else {
		serverAddr = fmt.Sprintf("%s:%d", AppConf.ConnOrderers[0].Host, AppConf.ConnOrderers[0].Port)
	}
	if len(AppConf.Channels) == 0 {
		panic(" Cannot find any channel, please create channel firstly!")
	}
	for _, channel := range AppConf.Channels {
		go engine.StartConsumer(serverAddr, AppConf.ConnOrderers[0].Host, channel, matches[0])
	}

	return engine
}

func initOrdererClients(signer crypto.LocalSigner) (ordererClients map[string][]*BroadcastClient) {
	ordererClients = make(map[string][]*BroadcastClient)
	var clients []*BroadcastClient
	for _, orderer := range AppConf.ConnOrderers {
		var serverAddr string
		if AppConf.Local {
			serverAddr = fmt.Sprintf("localhost:%d", orderer.Port)
		} else {
			serverAddr = fmt.Sprintf("%s:%d", orderer.Host, orderer.Port)
		}

		fpath := helpers.GetCryptoConfigPath(fmt.Sprintf("ordererOrganizations/example.com/orderers/%s"+"*", orderer.Host))
		matches, err := filepath.Glob(fpath)
		if err != nil {
			panic(fmt.Sprintf("Cannot find filepath %s ; err: %v\n", fpath, err))
		} else if len(matches) != 1 {
			panic(fmt.Sprintf("No msp directory filepath name matches: %s\n", fpath))
		}
		var dialOpts []grpc.DialOption
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxGrpcMsgSize),
			grpc.MaxCallRecvMsgSize(MaxGrpcMsgSize)))
		if AppConf.TlsEnabled {
			creds, err := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/tls/ca.crt", matches[0]), orderer.Host)
			if err != nil {
				panic(fmt.Sprintf("Error creating grpc tls client creds, serverAddr %s, err: %v", serverAddr, err))
			}
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			dialOpts = append(dialOpts, grpc.WithInsecure())
		}

		ctx, _ := context.WithTimeout(context.Background(), ConnTimeout)
		//defer cancel()

		ordererConn, err := grpc.DialContext(ctx, serverAddr, dialOpts...)
		if err != nil {
			panic(fmt.Sprintf("Error connecting (grpc) to %s, err: %v", serverAddr, err))
		}

		client, err := ab.NewAtomicBroadcastClient(ordererConn).Broadcast(context.TODO())
		if err != nil {
			panic(fmt.Sprintf("Error creating broadcast client for orderer[%s] , err: %v", serverAddr, err))
		}
		for _, channel := range AppConf.Channels {
			ordererClients[channel] = append(clients, newBroadcastClient(client, channel, signer))
		}
		Logger.Info("Starting Producer for orderer[%s], %v", serverAddr, time.Now())
	}

	return
}

func (e *OrdererTrafficEngine) GetAndAddTxId() (id uint64) {
	e.txIdMutex.Lock()
	defer e.txIdMutex.Unlock()
	id = e.txId
	e.txId++
	return
}

func (e *OrdererTrafficEngine) TransactionProducer() error {
	txId := e.GetAndAddTxId()

	channelId := AppConf.Channels[txId%uint64(len(AppConf.Channels))]
	client := e.ordererClients[channelId][txId%uint64(len(AppConf.ConnOrderers))]
	if err := client.broadcast([]byte(strconv.FormatUint(txId, 10))); err != nil {
		Logger.Error("failed to broadcast TxId [%d], err: %v", txId, err)
		return err
	}
	if err := client.getAck(); err != nil {
		Logger.Error("failed to broadcast TxId [%d], getAck err: %v", txId, err)
		return err
	}
	Logger.Info("successfully broadcast TxId [%d] to channel [%s] orderer [%s]", txId, channelId,
		AppConf.ConnOrderers[txId%uint64(len(AppConf.ConnOrderers))].Host)

	txChan := make(chan uint64)
	reqChans[txId] = txChan

	select {
	case blockNum := <-txChan:
		delete(reqChans, txId)
		Logger.Info("TxId [%d] successful write to block [%d] on channel [%s]", txId, blockNum, channelId)
		return nil
	case <-time.After(time.Second * time.Duration(AppConf.ReqTimeout)):
		errStr := fmt.Sprintf("Timeout to broadcast TxId [%d] to channel [%s] orderer [%s]", txId, channelId,
			AppConf.ConnOrderers[txId%uint64(len(AppConf.ConnOrderers))].Host)
		Logger.Error(errStr)
		return errors.New(errStr)
	}
}

func (e *OrdererTrafficEngine) StartConsumer(serverAddr, serverHost, channelId, cryptoPath string) {
	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxGrpcMsgSize),
		grpc.MaxCallRecvMsgSize(MaxGrpcMsgSize)))
	if AppConf.TlsEnabled {
		creds, err := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/tls/ca.crt", cryptoPath), serverHost)
		if err != nil {
			panic(fmt.Sprintf("startConsumer: error creating grpc tls client creds, serverAddr %s, err: %v", serverAddr, err))
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), ConnTimeout)
	defer cancel()

	consumerConnP, err := grpc.DialContext(ctx, serverAddr, dialOpts...)
	if err != nil {
		panic(fmt.Sprintf("Error connecting (grpc) to %s, err: %v", serverAddr, err))
	}
	client, err := ab.NewAtomicBroadcastClient(consumerConnP).Deliver(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Error create deliver client on grpc connection to %s, err: %v", serverAddr, err))
	}
	s := newDeliverClient(client, channelId, e.signer)
	if err = s.seekNewest(); err != nil {
		panic(fmt.Sprintf("Error seek newest block from serverAddr=%s channelID=%s; err: %v", serverAddr, channelId, err))
	}

	Logger.Info("Start to recv delivered batches from serverAddr=%s channelID=%s", serverAddr, channelId)
	s.readUntilClose()
}
