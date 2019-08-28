package ote

import (
	"fmt"
	"os"
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

type OrdererTrafficEngine struct {
	ordConf    *ordererConf.TopLevel
	genConf    *genesisconfig.Profile
	tlsEnabled bool
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

	// notice: lower case
	genConf := genesisconfig.Load("twoorgsorderergenesis")
	return &OrdererTrafficEngine{
		ordConf,
		genConf,
		true,
	}
}

func (e *OrdererTrafficEngine) StartProducer(serverAddr string, chanID string, payload int) {
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
			err = mspmgmt.LoadLocalMsp(fmt.Sprintf("%s/msp", matches[0]), e.ordConf.General.BCCSP, "OrdererMSP")
			if err != nil { // Handle errors reading the config file
				panic(fmt.Sprintf("Failed to initialize local MSP on chan %s: %s", chanID, err))
			}

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
		panic(fmt.Sprintf("Error creating Producer for ord[%s] , err: %v", ordererName, err))
	}
	time.Sleep(3 * time.Second)

	logger.Info("Starting Producer to send TXs to ord[%s] srvr=%s chID=%s, %v", ordererName, serverAddr, chanID, time.Now())

	b := NewBroadcastClient(client, chanID, signer)
	time.Sleep(2 * time.Second)

	_ = b.Broadcast([]byte("100"))
	err = b.GetAck()
	if err == nil {
		logger.Info("successfully broadcast TX %v", time.Now())
	} else {
		logger.Error("failed to broadcast TX %v, err: %v", time.Now(), err)
	}
}

func (e *OrdererTrafficEngine) StartConsumer(serverAddr string, chanID string, seek int) {
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
	if e.tlsEnabled {
		if len(matches) > 0 {
			e.ordConf.General.BCCSP.SwOpts.FileKeystore.KeyStorePath = fmt.Sprintf("%s/msp/keystore", matches[0])
			err = mspmgmt.LoadLocalMsp(fmt.Sprintf("%s/msp", matches[0]), e.ordConf.General.BCCSP, "OrdererMSP")
			if err != nil { // Handle errors reading the config file
				panic(fmt.Sprintf("Failed to initialize local MSP on chan %s: %s", chanID, err))
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

	consumerConnP, err := grpc.DialContext(ctx, serverAddr, dialOpts...)
	if err != nil {
		panic(fmt.Sprintf("Error connecting (grpc) to %s, err: %v", serverAddr, err))
	}
	client, err := ab.NewAtomicBroadcastClient(consumerConnP).Deliver(context.TODO())
	if err != nil {
		panic(fmt.Sprintf("Error invoking Deliver() on grpc connection to %s, err: %v", serverAddr, err))
	}
	s := NewDeliverClient(client, chanID, signer)
	if err = s.seekOldest(); err != nil {
		panic(fmt.Sprintf("ERROR starting srvr=%s chID=%s; err: %v", serverAddr, chanID, err))
	}

	logger.Info(fmt.Sprintf("Started to recv delivered batches srvr=%s chID=%s", serverAddr, chanID))
	s.readUntilClose()
}
