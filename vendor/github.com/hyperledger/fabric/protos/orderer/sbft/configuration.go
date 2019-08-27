/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sbft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/orderer/consensus/sbft/persist"
	"github.com/hyperledger/fabric/protos/orderer"
)

// TypeKey is the string with which this consensus implementation is identified across Fabric.
const TypeKey = "sbft"

func init() {
	orderer.ConsensusTypeMetadataMap[TypeKey] = ConsensusTypeMetadataFactory{}
}

// ConsensusTypeMetadataFactory allows this implementation's proto messages to register
// their type with the orderer's proto messages. This is needed for protolator to work.
type ConsensusTypeMetadataFactory struct{}

// NewMessage implements the Orderer.ConsensusTypeMetadataFactory interface.
func (dogf ConsensusTypeMetadataFactory) NewMessage() proto.Message {
	return &ConfigMetadata{}
}

// Marshal serializes this implementation's proto messages. It is called by the encoder package
// during the creation of the Orderer ConfigGroup.
func Marshal(md *ConfigMetadata) ([]byte, error) {
	copyMd := proto.Clone(md).(*ConfigMetadata)
	for _, c := range copyMd.Consenters {
		// Expect the user to set the config value for client/server certs to the
		// path where they are persisted locally, then load these files to memory.
		clientCert, err := ioutil.ReadFile(string(c.GetClientSignCert()))
		if err != nil {
			return nil, fmt.Errorf("cannot load client cert for consenter %s:%d: %s", c.GetHost(), c.GetPort(), err)
		}
		c.ClientSignCert = clientCert

		serverCert, err := ioutil.ReadFile(string(c.GetServerTlsCert()))
		if err != nil {
			return nil, fmt.Errorf("cannot load server cert for consenter %s:%d: %s", c.GetHost(), c.GetPort(), err)
		}
		c.ServerTlsCert = serverCert
	}
	return proto.Marshal(copyMd)
}

func ReadJsonConfig(file string) (*ConsensusConfig, error) {
	configData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	jconfig := &JsonConfig{}
	err = json.Unmarshal(configData, jconfig)
	if err != nil {
		return nil, err
	}

	config := &ConsensusConfig{}
	config.Consensus = jconfig.Consensus
	config.Peers = make(map[string]*Consenter)
	for n, p := range jconfig.Peers {
		if p.Address == "" {
			return nil, fmt.Errorf("The required peer address is missing (for peer %d)", n)
		}
		//cert, err := crypto.ParseCertPEM(p.Cert)
		//if err != nil {
		//	fmt.Println("exiting")
		//	return nil, err
		//}
		config.Peers[p.Address] = p.Cert
	}

	// XXX check for duplicate cert
	if config.Consensus.N != 0 && int(config.Consensus.N) != len(config.Peers) {
		return nil, fmt.Errorf("peer config does not match pbft N")
	}
	config.Consensus.N = uint64(len(config.Peers))

	return config, nil
}

func SaveConfig(p *persist.Persist, c *ConsensusConfig) error {
	craw, err := proto.Marshal(c)
	if err != nil {
		return err
	}
	err = p.StoreState("config", craw)
	return err
}

func RestoreConfig(p *persist.Persist) (*ConsensusConfig, error) {
	raw, err := p.ReadState("config")
	if err != nil {
		return nil, err
	}
	config := &ConsensusConfig{}
	err = proto.Unmarshal(raw, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
