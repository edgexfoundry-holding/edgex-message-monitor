// Copyright 2021 Alessandro De Blasis <alex@deblasis.net>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package messaging

import (
	"errors"
	"fmt"
	"sync"

	"github.com/deblasis/edgex-foundry-datamonitor/config"
	edgexM "github.com/edgexfoundry/go-mod-messaging/v2/messaging"
	"github.com/edgexfoundry/go-mod-messaging/v2/pkg/types"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	sync.Mutex
	edgeXClient edgexM.MessageClient
	cfg         *config.Config

	IsConnected  bool
	IsConnecting bool

	OnConnect func() bool
}

func NewClient(cfg *config.Config) (*Client, error) {

	c := &Client{
		Mutex:        sync.Mutex{},
		cfg:          cfg,
		IsConnected:  false,
		IsConnecting: false,
		OnConnect:    func() bool { return true },
	}

	return c, nil
}

func (c *Client) Connect() error {
	c.Lock()
	defer c.Unlock()
	c.IsConnected = false

	currentProtocol := c.cfg.GetCurrentProcotol()
	var messageBusConfig types.MessageBusConfig
	if currentProtocol == config.ProtocolRedis {
		messageBusConfig = types.MessageBusConfig{
			SubscribeHost: types.HostInfo{
				Host:     c.cfg.GetRedisHost(),
				Port:     c.cfg.GetRedisPort(),
				Protocol: edgexM.Redis,
			},
			Type: edgexM.Redis,
			Optional: map[string]string{
				"Password": c.cfg.GetRedisPassowrd(),
			},
		}
		log.Infof("connecting to %v:%v\n", c.cfg.GetRedisHost(), c.cfg.GetRedisPort())

	} else {
		messageBusConfig = types.MessageBusConfig{
			SubscribeHost: types.HostInfo{
				Host:     c.cfg.GetMQTTHost(),
				Port:     c.cfg.GetMQTTPort(),
				Protocol: "tcp",
			},
			Type: edgexM.MQTT,
			Optional: map[string]string{
				"ClientId": c.cfg.GetMQTTClientId(),
				"Username": c.cfg.GetMQTTUsername(),
				"Password": c.cfg.GetMQTTPassword(),
			},
		}
		log.Infof("connecting to %v:%v\n", c.cfg.GetMQTTHost(), c.cfg.GetMQTTPort())

	}

	c.IsConnecting = true
	defer func() {
		c.IsConnecting = false
	}()

	log.Info("Message bus config optional parameters:")
	for k, v := range messageBusConfig.Optional {
		log.Info(fmt.Sprintf("Optional %v: %v", k, v))
	}

	messageBus, err := edgexM.NewMessageClient(messageBusConfig)

	if err != nil {
		log.Error(err)
		return err
	}

	c.edgeXClient = messageBus

	err = c.edgeXClient.Connect()
	if err != nil {
		log.Error(err)
		return err
	}

	// edgex doesn't return error on connect... but only on Suscribe / Publish
	// that's why we have to do something like this, which is not ideal
	c.IsConnected = c.OnConnect()

	return nil
}

func (c *Client) Disconnect() error {
	c.Lock()
	defer c.Unlock()
	c.edgeXClient.Disconnect()
	c.IsConnected = false
	return nil
}

func (c *Client) Subscribe(topic string) (chan types.MessageEnvelope, chan error) {

	errorChannel := make(chan error)
	if c.edgeXClient == nil {
		errorChannel <- errors.New("client not initialized")
		return nil, errorChannel
	}

	messages := make(chan types.MessageEnvelope)

	err := c.edgeXClient.Subscribe([]types.TopicChannel{
		{
			Topic:    topic,
			Messages: messages,
		},
	}, errorChannel)

	if err != nil {
		log.Error(err)
		go (func() {
			errorChannel <- err
		})()
	}

	return messages, errorChannel
}
