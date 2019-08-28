package sdkprovider

import (
	pb "fabric-orderer-benchmark/protos"
	"fabric-orderer-benchmark/server/helpers"
	"fabric-orderer-benchmark/server/ote"
	"fmt"

	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
)

var logger = helpers.GetLogger()
var appConf = helpers.GetAppConf().Conf

type OrgInstance struct {
	Admin       string
	User        string
	AdminClient *resmgmt.Client
}

type FabSdkProvider struct {
	Sdk           *fabsdk.FabricSDK
	Org           map[string]*OrgInstance
	DefaultOrg    string
	OrdererEngine *ote.OrdererTrafficEngine
}

func NewFabSdkProvider() (*FabSdkProvider, error) {
	configOpt := config.FromFile(helpers.GetConfigPath("config.yaml"))
	sdk, err := fabsdk.New(configOpt)
	if err != nil {
		logger.Error("Failed to create new SDK: %s", err)
		return nil, err
	}

	provider := &FabSdkProvider{
		Sdk:           sdk,
		Org:           make(map[string]*OrgInstance),
		OrdererEngine: ote.NewOTE(),
	}
	for _, org := range appConf.OrgInfo {
		//clientContext allows creation of transactions using the supplied identity as the credential.
		adminContext := sdk.Context(fabsdk.WithUser(org.Admin), fabsdk.WithOrg(org.Name))

		// Resource management client is responsible for managing channels (create/update channel)
		// Supply user that has privileges to create channel (in this case orderer admin)
		adminClient, err := resmgmt.New(adminContext)
		if err != nil {
			logger.Error("Failed to new resource management client: %s", err)
			return nil, err
		}
		provider.Org[org.Name] = &OrgInstance{org.Admin, org.User, adminClient}
		if org.Default {
			provider.DefaultOrg = org.Name
		}
	}

	go provider.OrdererEngine.StartConsumer("localhost:7050", "mychannel1", 0)

	return provider, nil
}

func (f *FabSdkProvider) CreateChannel(channelID string) (helpers.TransactionID, pb.StatusCode, error) {
	orgName := f.DefaultOrg
	orgInstance, ok := f.Org[orgName]
	if !ok {
		logger.Error("Not found resource management client for org: %s", orgName)
		return "", pb.StatusCode_INVALID_ADMIN_CLIENT, fmt.Errorf("Not found admin client for org:  %v", orgName)
	}
	mspClient, err := mspclient.New(f.Sdk.Context(), mspclient.WithOrg(orgName))
	if err != nil {
		logger.Error("New mspclient err: %s", err)
		return "", pb.StatusCode_FAILED_NEW_MSP_CLIENT, err
	}
	adminIdentity, err := mspClient.GetSigningIdentity(orgInstance.Admin)
	if err != nil {
		logger.Error("MspClient getSigningIdentity err: %s", err)
		return "", pb.StatusCode_FAILED_GET_SIGNING_IDENTITY, err
	}
	req := resmgmt.SaveChannelRequest{ChannelID: channelID,
		ChannelConfigPath: helpers.GetChannelConfigPath(channelID + ".tx"),
		SigningIdentities: []msp.SigningIdentity{adminIdentity}}
	txID, err := orgInstance.AdminClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts),
		resmgmt.WithOrdererEndpoint(appConf.OrdererEndpoint))
	if err != nil {
		logger.Error("Failed SaveChannel: %s", err)
		return "", pb.StatusCode_FAILED_CREATE_CHANNEL, err
	}
	logger.Debug("Successfully created channel: %s", channelID)
	return helpers.TransactionID(txID.TransactionID), pb.StatusCode_SUCCESS, nil
}

func (f *FabSdkProvider) SendTransaction() (pb.StatusCode, error) {
	return pb.StatusCode_SUCCESS, nil
}
