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
package services

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/deblasis/edgex-foundry-datamonitor/config"
	"github.com/deblasis/edgex-foundry-datamonitor/messaging"
)

type AppManager struct {
	sync.RWMutex
	client           *messaging.Client
	config           *config.Config
	currentContainer *fyne.Container

	CurrentPage widget.TreeNodeID

	navBar *fyne.Container

	db *DB
	ep *EventProcessor

	pageHandlers map[widget.TreeNodeID]PageHandler

	drawFn func(*fyne.Container)

	sessionState *SessionState
}

func NewAppManager(client *messaging.Client, cfg *config.Config, ep *EventProcessor, db *DB) *AppManager {

	return &AppManager{
		RWMutex: sync.RWMutex{},
		client:  client,
		config:  cfg,
		db:      db,
		ep:      ep,

		pageHandlers: make(map[widget.TreeNodeID]PageHandler),

		sessionState: &SessionState{},
	}
}

func (a *AppManager) SetPageHandler(page widget.TreeNodeID, handler PageHandler) {
	a.pageHandlers[page] = handler
}

func (a *AppManager) GetPageHandler(page widget.TreeNodeID) PageHandler {
	return a.pageHandlers[page]
}

func (a *AppManager) GetEventProcessor() *EventProcessor {
	return a.ep
}

func (a *AppManager) GetDB() *DB {
	return a.db
}

func (a *AppManager) SetCurrentContainer(container *fyne.Container, drawFn func(*fyne.Container)) {
	a.Lock()
	defer a.Unlock()
	a.currentContainer = container
	a.drawFn = drawFn
}

func (a *AppManager) SetNav(nav *fyne.Container) {
	a.Lock()
	defer a.Unlock()
	a.navBar = nav
}

func (a *AppManager) Refresh() {
	log.Debugf("AppManager: refresh")
	refreshNavBar := func() {
		if a.navBar == nil {
			return
		}
		bnts := a.navBar.Objects[1].(*fyne.Container)

		var playPauseIcon fyne.Resource
		switch a.GetProcessorState() {
		case Running:
			playPauseIcon = theme.MediaPauseIcon()
			bnts.Objects[0].(*widget.Label).SetText("RUNNING")
			bnts.Objects[1].Show()
		default:
			playPauseIcon = theme.MediaPlayIcon()
			bnts.Objects[0].(*widget.Label).SetText("PAUSED")
			bnts.Objects[1].Hide()
		}

		bnts.Objects[2].(*widget.Button).SetIcon(playPauseIcon)

		if a.GetConnectionState() == ClientConnected {
			bnts.Objects[0].Show()
			bnts.Objects[2].Show()
			bnts.Objects[3].Show()
		} else {
			bnts.Objects[0].Hide()
			bnts.Objects[2].Hide()
			bnts.Objects[3].Hide()
		}

		a.navBar.Refresh()
	}
	refreshContent := func() {
		if a.drawFn != nil && a.currentContainer != nil {
			a.drawFn(a.currentContainer)
		}
	}

	refreshNavBar()
	refreshContent()

	dataPageH := a.GetPageHandler(a.CurrentPage)
	if dataPageH != nil {
		dataPageH.Refresh()
	}

}

func (a *AppManager) GetCurrentContainer() (*fyne.Container, func(*fyne.Container)) {
	a.RLock()
	defer a.RUnlock()
	return a.currentContainer, a.drawFn
}

func (a *AppManager) GetConnectionState() ConnectionState {
	a.RLock()
	defer a.RUnlock()
	if a.client.IsConnected {
		return ClientConnected
	}
	if a.client.IsConnecting {
		return ClientConnecting
	}
	return ClientDisconnected
}

func (a *AppManager) GetProcessorState() ProcessorState {
	a.RLock()
	defer a.RUnlock()
	return a.ep.CurrentState
}

type ConnectionInfo struct {
	Host  string
	Port  int
	Type  config.MessagingType
	Topic string
}

func (a *AppManager) GetConnectionInfo() ConnectionInfo {

	t := a.config.GetSelectedMessaging()
	if t == config.RedisMessagingType {
		return ConnectionInfo{
			Host:  a.config.GetRedisHost(),
			Port:  a.config.GetRedisPort(),
			Topic: a.config.GetTopic(),
			Type:  t,
		}
	}

	return ConnectionInfo{
		Host:  a.config.GetMQTTHost(),
		Port:  a.config.GetMQTTPort(),
		Topic: a.config.GetTopic(),
		Type:  t,
	}

}

func (a *AppManager) Connect() error {
	a.Lock()
	defer a.Unlock()
	a.ep.Activate()
	return a.client.Connect()
}

func (a *AppManager) Disconnect() error {
	a.Lock()
	defer a.Unlock()
	a.ep.Deactivate()
	return a.client.Disconnect()
}

type SessionState struct {
	DataPage_SelectedTopic    *string
	DataPage_SelectedDataType *string
	DataPage_Search           *string
	DataPage_BufferSize       *int
}

func (a *AppManager) SetDataPageSelectedTopic(dt string) {
	a.Lock()
	defer a.Unlock()
	a.sessionState.DataPage_SelectedTopic = config.String(dt)

	a.db.updateTopicFilter(config.StringVal(a.sessionState.DataPage_SelectedTopic))
	a.db.UpdateFilter(config.StringVal(a.sessionState.DataPage_Search))

}

func (a *AppManager) SetDataPageSelectedDataType(dt string) {
	a.Lock()
	defer a.Unlock()
	a.sessionState.DataPage_SelectedDataType = config.String(dt)
}

func (a *AppManager) SetDataPageSearch(search string) {
	log.Debugf("AppManager: SetDataPageSearch %v", search)
	a.Lock()
	defer a.Unlock()
	a.sessionState.DataPage_Search = config.String(search)
	a.db.UpdateFilter(search)
}

func (a *AppManager) SetDataPageBufferSize(bs int) {
	a.Lock()
	defer a.Unlock()
	a.sessionState.DataPage_BufferSize = config.Int(bs)
	a.db.UpdateBufferSize(int64(bs))
}

func (a *AppManager) GetDataPageSelectedTopic() *string {
	a.RLock()
	defer a.RUnlock()
	return a.sessionState.DataPage_SelectedTopic
}

func (a *AppManager) GetDataPageSelectedDataType() *string {
	a.RLock()
	defer a.RUnlock()
	return a.sessionState.DataPage_SelectedDataType
}

func (a *AppManager) GetDataPageSearch() *string {
	a.RLock()
	defer a.RUnlock()
	return a.sessionState.DataPage_Search
}

func (a *AppManager) GetDataPageBufferSize() *int {
	a.RLock()
	defer a.RUnlock()
	return a.sessionState.DataPage_BufferSize
}
