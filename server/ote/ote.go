package ote

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"fabric-orderer-benchmark/server/helpers"
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
	genesisConfigLocation = "CONFIGTX_ORDERER_"
	ordererConfigLocation = "ORDERER_GENERAL_"
	batchSizeParamStr     = genesisConfigLocation + "BATCHSIZE_MAXMESSAGECOUNT"
	batchTimeoutParamStr  = genesisConfigLocation + "BATCHTIMEOUT"
	ordererTypeParamStr   = genesisConfigLocation + "ORDERERTYPE"
)

const randomString = "abcdef1234567890"

func extraDataSend(size int) string {
	extraTxData := make([]byte, size*1024) //size in kb
	for i := range extraTxData {
		extraTxData[i] = randomString[rand.Intn(len(randomString))]
	}
	return string(extraTxData)
}

type OrdererTrafficEngine struct {
	ordConf    *ordererConf.TopLevel
	genConf    *genesisconfig.Profile
	tlsEnabled bool
}

func NewOTE() *OrdererTrafficEngine {
	// Establish the default configuration from yaml files - and this also
	// picks up any variables overridden on command line or in environment
	ordConf, err := ordererConf.Load()
	if err != nil {
		panic(" Cannot Load orderer config data: " + err.Error())
	}
	genConf := genesisconfig.Load("TwoOrgsOrdererGenesis")
	return &OrdererTrafficEngine{
		ordConf,
		genConf,
		true,
	}
}

func (e *OrdererTrafficEngine) StartProducer(serverAddr string, chanID string, ordererIndex int, channelIndex int, payload int) {
	signer := localmsp.NewSigner()
	ordererName := "orderer.example.com"
	fpath := helpers.GetCryptoConfigPath(fmt.Sprintf("ordererOrganizations/example.com/orderers/%s"+"*", ordererName))
	matches, err := filepath.Glob(fpath)

	logger.Info("startProducer ordererName=%s, fpath=%s, len(matches)=%d", ordererName, fpath, len(matches))

	if err != nil {
		panic(fmt.Sprintf("startProducer: cannot find filepath %s ; err: %v\n", fpath, err))
	}

	var conn *grpc.ClientConn
	if e.tlsEnabled {
		if len(matches) > 0 {
			e.ordConf.General.BCCSP.SwOpts.FileKeystore.KeyStorePath = fmt.Sprintf("%s/msp/keystore", matches[0])
			creds, err1 := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/tls/ca.crt", matches[0]), fmt.Sprintf("%s", ordererName))
			conn, err1 = grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
			if err1 != nil {
				panic(fmt.Sprintf("Error connecting (grpcs) to %s, err: %v\n", serverAddr, err))
			}
		} else {
			panic(fmt.Sprintf("startProducer: no msp directory filepath name matches: %s\n", fpath))
		}
	} else {
		conn, err = grpc.Dial(serverAddr, grpc.WithInsecure())
		if err != nil {
			panic(fmt.Sprintf("Error connecting (grpc) to %s, err: %v", serverAddr, err))
		}
	}
	defer func() {
		_ = conn.Close()
	}()

	client, err := ab.NewAtomicBroadcastClient(conn).Broadcast(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Error creating Producer for ord[%d] ch[%d], err: %v", ordererIndex, channelIndex, err))
	}
	time.Sleep(3 * time.Second)

	logger.Info("Starting Producer to send TXs to ord[%d] ch[%d] srvr=%s chID=%s, %v", ordererIndex, channelIndex, serverAddr, chanID, time.Now())

	b := NewBroadcastClient(client, chanID, signer)
	time.Sleep(2 * time.Second)

	txData := extraDataSend(payload) //create the extra payload

	_ = b.Broadcast([]byte(fmt.Sprintf("Testing  %v %s", time.Now(), txData)))
	err = b.GetAck()
	if err == nil {
		logger.Info("successfully broadcast TX %v", time.Now())
	} else {
		logger.Error("failed to broadcast TX %v, err: %v", time.Now(), err)
	}
}

func (e *OrdererTrafficEngine) StartConsumer(serverAddr string, chanID string, ordererIndex int, channelIndex int, txRecvCntrP *int64, blockRecvCntrP *int64, consumerConnP **grpc.ClientConn, seek int, quiet bool, tlsEnabled bool, orgMSPID string) {
	signer := localmsp.NewSigner()
	ordererName := "orderer.example.com"
	fpath := helpers.GetCryptoConfigPath(fmt.Sprintf("ordererOrganizations/example.com/orderers/%s"+"*", ordererName))
	matches, err := filepath.Glob(fpath)
	if err != nil {
		panic(fmt.Sprintf("startConsumer: cannot find filepath %s ; err: %v", fpath, err))
	}
	if seek < -2 {
		fmt.Println("Wrong seek value.")
	}
	var maxGrpcMsgSize int = 1000 * 1024 * 1024
	var dialOpts []grpc.DialOption
	var conntimeout time.Duration = 30 * time.Second
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxGrpcMsgSize),
		grpc.MaxCallRecvMsgSize(maxGrpcMsgSize)))
	if tlsEnabled {
		if len(matches) > 0 {
			e.ordConf.General.BCCSP.SwOpts.FileKeystore.KeyStorePath = fmt.Sprintf("%s/msp/keystore", matches[0])
			if ordererIndex == 0 { // Loading the msp's of orderer0 for every channel is enough to create the deliver client
				err = mspmgmt.LoadLocalMsp(fmt.Sprintf("%s/msp", matches[0]), e.ordConf.General.BCCSP, orgMSPID)
				if err != nil { // Handle errors reading the config file
					panic(fmt.Sprintf("Failed to initialize local MSP on chan %s: %s", chanID, err))
				}
			}
			creds, err := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/tls/ca.crt", matches[0]), fmt.Sprintf("%s", ordererName))
			if err != nil {
				panic(fmt.Sprintf("Error creating grpc tls client creds, serverAddr %s, err: %v", serverAddr, err))
			}
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			panic(fmt.Sprintf("startConsumer: no msp directory filepath name matches: %s\n", fpath))
		}
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	//(*consumerConnP), err = grpc.Dial(serverAddr, dialOpts...)
	ctx, cancel := context.WithTimeout(context.Background(), conntimeout)
	defer cancel()

	*consumerConnP, err = grpc.DialContext(ctx, serverAddr, dialOpts...)
	if err != nil {
		panic(fmt.Sprintf("Error connecting (grpc) to %s, err: %v", serverAddr, err))
	}
	client, err := ab.NewAtomicBroadcastClient(*consumerConnP).Deliver(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Error invoking Deliver() on grpc connection to %s, err: %v", serverAddr, err))
	}
	s := NewDeliverClient(client, chanID, signer)
	if err = s.seekOldest(); err != nil {
		panic(fmt.Sprintf("ERROR starting srvr=%s chID=%s; err: %v", serverAddr, chanID, err))
	}

	logger.Info(fmt.Sprintf("Started to recv delivered batches srvr=%s chID=%s", serverAddr, chanID))
	s.readUntilClose(ordererIndex, channelIndex, txRecvCntrP, blockRecvCntrP)
}
