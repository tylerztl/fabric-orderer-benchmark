package ote

import (
	"errors"
	"fabric-orderer-benchmark/server/helpers"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/crypto"
	"github.com/hyperledger/fabric/common/localmsp"
	"go.uber.org/atomic"
	//genesisconfig "github.com/hyperledger/fabric/common/tools/configtxgen/localconfig" // config for genesis.yaml
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
	ReqChans       = new(sync.Map)
)

type TxPool struct {
	reqChans map[uint64]chan uint64
	mutex    *sync.RWMutex
}

func newTxPool() *TxPool {
	return &TxPool{
		make(map[uint64]chan uint64),
		new(sync.RWMutex),
	}
}

var txPool = newTxPool()

func (t *TxPool) getChanByTxId(txId uint64) chan uint64 {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.reqChans[txId]
}

func (t *TxPool) deleteChan(txId uint64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.reqChans, txId)
}

func (t *TxPool) addChan(txId uint64, c chan uint64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.reqChans[txId] = c
}

type OrdererTrafficEngine struct {
	ordConf *ordererConf.TopLevel
	//genConf        *genesisconfig.Profile
	signer  crypto.LocalSigner
	clients map[string][][]*BroadcastClient // channels~orderers~clients
	txId    *atomic.Uint64
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

	//genConf := genesisconfig.Load(strings.ToLower(AppConf.Profile))

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
		//genConf,
		signer,
		initOrdererClients(signer),
		atomic.NewUint64(0),
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

func initOrdererClients(signer crypto.LocalSigner) (channelClients map[string][][]*BroadcastClient) {
	if AppConf.Clients < 1 {
		panic("Invalid parameter for clients number!")
	}

	channelClients = make(map[string][][]*BroadcastClient)
	for _, channel := range AppConf.Channels {
		ordererClients := make([][]*BroadcastClient, len(AppConf.ConnOrderers))
		for i, orderer := range AppConf.ConnOrderers {
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
			clients := make([]*BroadcastClient, AppConf.Clients)
			for i := uint32(0); i < AppConf.Clients; i++ {
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

				clients[i] = newBroadcastClient(client, i, channel, signer)
				Logger.Info("Starting Producer client [%d] for orderer [%s] channel [%s], %v", i, serverAddr, channel, time.Now())
			}
			ordererClients[i] = clients
		}
		channelClients[channel] = ordererClients
	}

	return
}

func (e *OrdererTrafficEngine) TransactionProducer() error {
	txId := e.txId.Inc()
	channelId := AppConf.Channels[txId%uint64(len(AppConf.Channels))]
	client := e.clients[channelId][txId%uint64(len(AppConf.ConnOrderers))][txId%uint64(AppConf.Clients)]

	if err := client.broadcast([]byte(strconv.FormatUint(txId, 10))); err != nil {
		Logger.Error("failed to broadcast TxId [%d], err: %v", txId, err)
		return err
	}
	Logger.Info("client [%d] successfully broadcast TxId [%d] to channel [%s] orderer [%s]", client.clientId,
		txId, channelId, AppConf.ConnOrderers[txId%uint64(len(AppConf.ConnOrderers))].Host)

	txChan := make(chan uint64)
	//txPool.addChan(txId, txChan)
	ReqChans.Store(txId, txChan)

	select {
	case blockNum := <-txChan:
		//txPool.deleteChan(txId)
		ReqChans.Delete(txId)
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
