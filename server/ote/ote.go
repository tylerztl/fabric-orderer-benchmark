package ote

import (
	"fabric-orderer-benchmark/server/helpers"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

type OrdererTrafficEngine struct {
	ordConf *ordererConf.TopLevel
	genConf *genesisconfig.Profile
	signer  crypto.LocalSigner
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

func (e *OrdererTrafficEngine) StartProducer(serverAddr string, chanID string) {
	ordererName := "orderer.example.com"
	fpath := helpers.GetCryptoConfigPath(fmt.Sprintf("ordererOrganizations/example.com/orderers/%s"+"*", ordererName))
	matches, err := filepath.Glob(fpath)

	Logger.Info("startProducer ordererName=%s, fpath=%s, len(matches)=%d", ordererName, fpath, len(matches))

	if err != nil {
		panic(fmt.Sprintf("startProducer: cannot find filepath %s ; err: %v\n", fpath, err))
	}

	var conn *grpc.ClientConn
	if AppConf.TlsEnabled {
		creds, err1 := credentials.NewClientTLSFromFile(fmt.Sprintf("%s/tls/ca.crt", matches[0]), fmt.Sprintf("%s", ordererName))
		conn, err1 = grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
		if err1 != nil {
			panic(fmt.Sprintf("Error connecting (grpcs) to %s, err: %v\n", serverAddr, err))
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
		panic(fmt.Sprintf("Error creating Producer for ord[%s] , err: %v", ordererName, err))
	}
	time.Sleep(3 * time.Second)

	Logger.Info("Starting Producer to send TXs to ord[%s] srvr=%s chID=%s, %v", ordererName, serverAddr, chanID, time.Now())

	b := newBroadcastClient(client, chanID, e.signer)
	time.Sleep(2 * time.Second)

	_ = b.broadcast([]byte("100"))
	err = b.getAck()
	if err == nil {
		Logger.Info("successfully broadcast TX %v", time.Now())
	} else {
		Logger.Error("failed to broadcast TX %v, err: %v", time.Now(), err)
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
