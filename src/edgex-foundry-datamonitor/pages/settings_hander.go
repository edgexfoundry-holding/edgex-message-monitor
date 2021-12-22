// Copyright 2021 painterner <i5stiyuki@icloud.com>
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
package pages

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/widget"
	"github.com/deblasis/edgex-foundry-datamonitor/config"
	"github.com/deblasis/edgex-foundry-datamonitor/data"
	"github.com/deblasis/edgex-foundry-datamonitor/services"
	log "github.com/sirupsen/logrus"
)

type configOption struct {
	hostname string
	port     int
	clientId string
	username string
	password string
}
type configPrefOption struct {
	hostname string
	port     string
	clientId string
	username string
	password string
}

type settingsPageItems struct {
	hostname                   *widget.Entry
	port                       *widget.Entry
	shouldConnectAutomatically *widget.Check
	eventsSortedAscendingly    *widget.Check
	dataPageBufferSize         *widget.Entry
	protocolSelectEntry        *widget.Select
	protocolClientIdEntry      *widget.Entry
	protocolUsernameEntry      *widget.Entry
	protocolPasswordEntry      *widget.Entry
	topicEntry                 *widget.Entry
	form                       *widget.Form
}

type settingConfigOption struct {
	dfaut configOption
	pref  configPrefOption
	items *settingsPageItems
}

type settingsPageHandler struct {
	appState *services.AppManager

	Key widget.TreeNodeID

	selectedProtocol string

	formContainer fyne.CanvasObject

	allProtocol map[string]*settingConfigOption

	prepared bool
}

func NewSettingsPageHandler(appState *services.AppManager) *settingsPageHandler {
	p := &settingsPageHandler{
		Key:      SettingsPageKey,
		appState: appState,
		allProtocol: map[string]*settingConfigOption{
			config.ProtocolRedis: {
				dfaut: configOption{
					hostname: config.RedisDefaultHost,
					port:     config.RedisDefaultPort,
					password: config.RedisDefaultPassword,
				},
				pref: configPrefOption{
					hostname: config.PrefRedisHost,
					port:     config.PrefRedisPort,
					password: config.PrefRedisPassword,
				},
			},
			config.ProtocolMQTT: {
				dfaut: configOption{
					hostname: config.MQTTDefaultHost,
					port:     config.MQTTDefaultPort,
					password: config.MQTTDefaultPassword,
					clientId: config.MQTTDefaultClientId,
					username: config.MQTTDefaultUsername,
				},
				pref: configPrefOption{
					hostname: config.PrefMQTTHost,
					port:     config.PrefMQTTPort,
					password: config.PrefMQTTPassword,
					clientId: config.PrefMQTTClientId,
					username: config.PrefMQTTUsername,
				},
			},
		},
	}

	return p
}

func (p *settingsPageHandler) makeSettingScreenContent(appState *services.AppManager, protocolName string) (fyne.CanvasObject, *widget.Select) {
	a := fyne.CurrentApp()
	preferences := a.Preferences()

	protocols := p.allProtocol
	protocol := protocols[protocolName]

	createFormItems := func(protocol *settingConfigOption) *settingsPageItems {
		hostname := widget.NewEntry()
		hostname.SetPlaceHolder(fmt.Sprintf("Insert host (default: %v)", protocol.dfaut.hostname))
		hostname.Validator = data.StringNotEmptyValidator

		port := widget.NewEntry()
		port.SetPlaceHolder(fmt.Sprintf("Insert port (default: %v)", protocol.dfaut.port))
		port.Validator = validation.NewRegexp(`\d`, "Must contain a number")

		shouldConnectAutomatically := widget.NewCheckWithData("Connect at startup", binding.NewBool())
		eventsSortedAscendingly := widget.NewCheckWithData("Sort events ascendingly", binding.NewBool())

		dataPageBufferSize := widget.NewEntry()
		dataPageBufferSize.SetPlaceHolder("* required")
		dataPageBufferSize.Validator = data.MinMaxValidator(config.MinBufferSize, config.MaxBufferSize, data.ErrInvalidBufferSize)

		protocolSelectEntry := widget.NewSelect([]string{config.ProtocolRedis, config.ProtocolMQTT}, func(s string) {})
		protocolSelectEntry.SetSelected(protocolName)

		// protocol options
		// protocolUsernameLabel := widget.NewLabel("Username")
		protocolUsernameEntry := widget.NewEntry()
		protocolUsernameEntry.SetPlaceHolder(fmt.Sprintf("Set username"))
		// protocolUsernameContainer := container.NewHBox(protocolUsernameLabel, protocolUsernameEntry)

		protocolClientIdEntry := widget.NewEntry()
		protocolClientIdEntry.SetPlaceHolder(fmt.Sprintf("Set client id"))

		protocolPasswordEntry := widget.NewEntry()
		protocolPasswordEntry.SetPlaceHolder(fmt.Sprintf("Set password"))

		// // protocol options container
		// protocolOptionsContainer := container.NewVBox(protocolUsernameContainer, protocolClientIdEntry, protocolPasswordEntry)

		// protocolOptionsAccordion := widget.NewAccordion(
		// 	widget.NewAccordionItem("Protocol Options", protocolOptionsContainer),
		// )

		topicEntry := widget.NewEntry()
		topicEntry.SetPlaceHolder(fmt.Sprintf("Message topic"))

		//read from settings
		hostname.SetText(preferences.StringWithFallback(protocol.pref.hostname, protocol.dfaut.hostname))
		port.SetText(fmt.Sprintf("%d", preferences.IntWithFallback(protocol.pref.port, protocol.dfaut.port)))
		protocolUsernameEntry.SetText(preferences.StringWithFallback(protocol.pref.username, protocol.dfaut.username))
		protocolPasswordEntry.SetText(preferences.StringWithFallback(protocol.pref.password, protocol.dfaut.password))
		protocolClientIdEntry.SetText(preferences.StringWithFallback(protocol.pref.clientId, protocol.dfaut.clientId))

		topicEntry.SetText(preferences.StringWithFallback(config.PrefEventsTopic, config.DefaultEventsTopic))
		shouldConnectAutomatically.SetChecked(preferences.BoolWithFallback(config.PrefShouldConnectAtStartup, config.DefaultShouldConnectAtStartup))
		eventsSortedAscendingly.SetChecked(preferences.BoolWithFallback(config.PrefEventsTableSortOrderAscending, config.DefaultEventsTableSortOrderAscending))
		dataPageBufferSize.SetText(fmt.Sprintf("%d", preferences.IntWithFallback(config.PrefBufferSizeInDataPage, config.DefaultBufferSizeInDataPage)))

		return &settingsPageItems{
			hostname:                   hostname,
			port:                       port,
			shouldConnectAutomatically: shouldConnectAutomatically,
			eventsSortedAscendingly:    eventsSortedAscendingly,
			dataPageBufferSize:         dataPageBufferSize,
			protocolClientIdEntry:      protocolClientIdEntry,
			protocolSelectEntry:        protocolSelectEntry,
			protocolUsernameEntry:      protocolUsernameEntry,
			protocolPasswordEntry:      protocolPasswordEntry,
			topicEntry:                 topicEntry,
		}
	}

	if protocols[protocolName].items == nil {
		protocols[protocolName].items = createFormItems(protocol)
	}
	// restore shared elements
	for key, _ := range protocols {
		if key != protocolName && protocols[key].items != nil {
			protocols[protocolName].items.protocolSelectEntry.OnChanged = func(s string) {}
			protocols[protocolName].items.protocolSelectEntry.SetSelected(protocols[key].items.protocolSelectEntry.Selected)
			protocols[protocolName].items.topicEntry.SetText(protocols[key].items.topicEntry.Text)
			protocols[protocolName].items.shouldConnectAutomatically.SetChecked(protocols[key].items.shouldConnectAutomatically.Checked)
			protocols[protocolName].items.eventsSortedAscendingly.SetChecked(protocols[key].items.eventsSortedAscendingly.Checked)
			protocols[protocolName].items.dataPageBufferSize.SetText(protocols[key].items.dataPageBufferSize.Text)
			break
		}
	}

	makeForm := func() *widget.Form {
		optionArray := []*widget.FormItem{
			{
				Text:   "*Password",
				Widget: protocol.items.protocolPasswordEntry,
			},
		}
		if protocolName == config.ProtocolMQTT {
			optionArray = []*widget.FormItem{
				{
					Text:   "*User Name",
					Widget: protocol.items.protocolUsernameEntry,
				},
				{
					Text:   "*Password",
					Widget: protocol.items.protocolPasswordEntry,
				},
				{
					Text:   "*Client Id",
					Widget: protocol.items.protocolClientIdEntry,
				},
			}
		}

		items := []*widget.FormItem{
			{Text: "Protocol", Widget: protocol.items.protocolSelectEntry, HintText: "Message Procotol"},
			{Text: "Hostname", Widget: protocol.items.hostname, HintText: "EdgeX Redis Pub/Sub hostname"},
			{Text: "Port", Widget: protocol.items.port, HintText: "EdgeX Redis Pub/Sub port"},
			{
				Text:     "Topic",
				Widget:   protocol.items.topicEntry,
				HintText: fmt.Sprintf("Edgex Events Topic"),
			},
			{
				Text:     "",
				Widget:   protocol.items.shouldConnectAutomatically,
				HintText: "",
			},
			{
				Text:     "",
				Widget:   protocol.items.eventsSortedAscendingly,
				HintText: "",
			},
			{Text: "Initial buffer size in Data page", Widget: protocol.items.dataPageBufferSize},
			{Text: "", Widget: widget.NewLabelWithStyle("Protocol optional configuration:", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})},
		}

		items = append(items, optionArray...)

		form := &widget.Form{
			Items: items,
			OnSubmit: func() {
				log.Info("Settings form submitted")
				for _, protocol := range protocols {
					if protocol.items == nil {
						continue
					}
					preferences.SetString(protocol.pref.hostname, strings.TrimSpace(protocol.items.hostname.Text))
					preferences.SetString(protocol.pref.port, strings.TrimSpace(protocol.items.hostname.Text))
					preferences.SetString(protocol.pref.username, strings.TrimSpace(protocol.items.protocolUsernameEntry.Text))
					preferences.SetString(protocol.pref.password, strings.TrimSpace(protocol.items.protocolPasswordEntry.Text))
					preferences.SetString(protocol.pref.clientId, strings.TrimSpace(protocol.items.protocolClientIdEntry.Text))

					p, _ := strconv.Atoi(protocol.items.port.Text)
					preferences.SetInt(protocol.pref.port, p)
				}

				preferences.SetBool(config.PrefShouldConnectAtStartup, protocol.items.shouldConnectAutomatically.Checked)
				preferences.SetBool(config.PrefEventsTableSortOrderAscending, protocol.items.eventsSortedAscendingly.Checked)

				bsize, _ := strconv.Atoi(protocol.items.dataPageBufferSize.Text)
				preferences.SetInt(config.PrefBufferSizeInDataPage, bsize)

				preferences.SetString(config.PrefProcotol, protocol.items.protocolSelectEntry.Selected)

				preferences.SetString(config.PrefEventsTopic, protocol.items.topicEntry.Text)

				a.SendNotification(&fyne.Notification{
					Title:   "EdgeX Redis Pub/Sub Connection Settings",
					Content: "Settings has saved",
				})
			},

			CancelText: "Reset defaults",
		}

		form.OnCancel = func() {
			for _, protocol := range protocols {
				if protocol.items == nil {
					continue
				}
				protocol.items.hostname.Text = protocol.dfaut.hostname
				protocol.items.port.Text = fmt.Sprintf("%d", protocol.dfaut.port)
			}

			protocol.items.shouldConnectAutomatically.SetChecked(config.DefaultShouldConnectAtStartup)
			protocol.items.eventsSortedAscendingly.SetChecked(config.DefaultEventsTableSortOrderAscending)
			protocol.items.hostname.Validate()
			protocol.items.port.Validate()

			form.Refresh()
			log.Info("Settings reset to default")
		}

		return form
	}

	if protocols[protocolName].items.form == nil {
		protocols[protocolName].items.form = makeForm()
	}

	return protocols[protocolName].items.form, protocol.items.protocolSelectEntry
}

func (p *settingsPageHandler) SetInitialState() {
	for _, items := range p.allProtocol {
		items.items = nil
	}
	form, protocolSelectEntry := p.makeSettingScreenContent(p.appState, p.appState.GetConfig().GetCurrentProcotol())
	formContainer := container.NewHBox(
		form,
	)

	var onProtocolChange func(s string)

	onProtocolChange = func(s string) {
		newform, newProtocolSelectEntry := p.makeSettingScreenContent(p.appState, s)
		formContainer.Remove(form)
		form = newform
		protocolSelectEntry = newProtocolSelectEntry
		protocolSelectEntry.OnChanged = onProtocolChange
		formContainer.Add(form)
		formContainer.Refresh()
	}
	protocolSelectEntry.OnChanged = onProtocolChange

	p.formContainer = formContainer

}

func (p *settingsPageHandler) RehydrateSession() {
}

func (p *settingsPageHandler) SetupBindings() {

}
