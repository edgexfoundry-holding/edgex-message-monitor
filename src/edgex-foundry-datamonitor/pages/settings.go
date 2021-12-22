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
package pages

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/deblasis/edgex-foundry-datamonitor/config"
	"github.com/deblasis/edgex-foundry-datamonitor/data"
	"github.com/deblasis/edgex-foundry-datamonitor/services"
)

func settingsScreen(win fyne.Window, appState *services.AppManager) fyne.CanvasObject {
	a := fyne.CurrentApp()
	preferences := a.Preferences()

	redisHostname := widget.NewEntry()
	redisHostname.SetPlaceHolder(fmt.Sprintf("Insert Redis host (default: %v)", config.RedisDefaultHost))
	redisHostname.Validator = data.StringNotEmptyValidator
	redisPort := widget.NewEntry()
	redisPort.SetPlaceHolder(fmt.Sprintf("Insert Redis port (default: %v)", config.RedisDefaultPort))
	redisPort.Validator = validation.NewRegexp(`\d`, "Must contain a number")
	redisPassword := widget.NewEntry()
	redisPassword.SetPlaceHolder("optional")
	redisPassword.Validator = data.MustBeEmptyOrMatchLengthValidator(config.RedisPasswordLength, data.ErrRedisPasswordLength)

	redisFields := []*widget.FormItem{
		{Text: "Hostname", Widget: redisHostname, HintText: "EdgeX Redis Pub/Sub hostname"},
		{Text: "Port", Widget: redisPort, HintText: "EdgeX Redis Pub/Sub port"},
		{Text: "Password", Widget: redisPassword, HintText: "EdgeX Redis password"},
	}

	mqttHostname := widget.NewEntry()
	mqttHostname.SetPlaceHolder(fmt.Sprintf("Insert MQTT host (default: %v)", config.MQTTDefaultHost))
	mqttHostname.Validator = data.StringNotEmptyValidator
	mqttPort := widget.NewEntry()
	mqttPort.SetPlaceHolder(fmt.Sprintf("Insert MQTT port (default: %v)", config.MQTTDefaultPort))
	mqttPort.Validator = validation.NewRegexp(`\d`, "Must contain a number")

	mqttClientId := widget.NewEntry()
	mqttClientId.SetPlaceHolder("optional")
	mqttUsername := widget.NewEntry()
	mqttUsername.SetPlaceHolder("optional")
	mqttPassword := widget.NewEntry()
	mqttPassword.SetPlaceHolder("optional")
	mqttFields := []*widget.FormItem{
		{Text: "Hostname", Widget: mqttHostname, HintText: "EdgeX MQTT Pub/Sub hostname"},
		{Text: "Port", Widget: mqttPort, HintText: "EdgeX MQTT Pub/Sub port"},
		{Text: "ClientId", Widget: mqttClientId, HintText: "EdgeX MQTT clientId"},
		{Text: "Username", Widget: mqttUsername, HintText: "EdgeX MQTT username"},
		{Text: "Password", Widget: mqttPassword, HintText: "EdgeX MQTT password"},
	}

	topic := widget.NewEntry()
	topic.SetPlaceHolder(fmt.Sprintf("Insert topic (default: %v)", config.DefaultEventsTopic))
	topic.Validator = data.StringNotEmptyValidator

	shouldConnectAutomatically := widget.NewCheckWithData("Connect at startup", binding.NewBool())
	eventsSortedAscendingly := widget.NewCheckWithData("Sort events ascendingly", binding.NewBool())

	dataPageBufferSize := widget.NewEntry()
	dataPageBufferSize.SetPlaceHolder("* required")
	dataPageBufferSize.Validator = data.MinMaxValidator(config.MinBufferSize, config.MaxBufferSize, data.ErrInvalidBufferSize)

	commonFields := []*widget.FormItem{
		{Text: "Topic", Widget: topic, HintText: "It must terminate with # to enable filtering in the Data page"},
		{
			Text:     "",
			Widget:   shouldConnectAutomatically,
			HintText: "",
		},
		{
			Text:     "",
			Widget:   eventsSortedAscendingly,
			HintText: "",
		},
		{Text: "Initial buffer size in Data page", Widget: dataPageBufferSize},
	}

	//read from settings
	redisHostname.SetText(preferences.StringWithFallback(config.PrefRedisHost, config.RedisDefaultHost))
	redisPort.SetText(fmt.Sprintf("%d", preferences.IntWithFallback(config.PrefRedisPort, config.RedisDefaultPort)))
	redisPassword.SetText(preferences.StringWithFallback(config.PrefRedisPassword, ""))

	mqttHostname.SetText(preferences.StringWithFallback(config.PrefMQTTHost, config.MQTTDefaultHost))
	mqttPort.SetText(fmt.Sprintf("%d", preferences.IntWithFallback(config.PrefMQTTPort, config.MQTTDefaultPort)))
	mqttClientId.SetText(preferences.StringWithFallback(config.PrefMQTTClientId, ""))
	mqttUsername.SetText(preferences.StringWithFallback(config.PrefMQTTUsername, ""))
	mqttPassword.SetText(preferences.StringWithFallback(config.PrefMQTTPassword, ""))

	topic.SetText(preferences.StringWithFallback(config.PrefEventsTopic, config.DefaultEventsTopic))

	shouldConnectAutomatically.SetChecked(preferences.BoolWithFallback(config.PrefShouldConnectAtStartup, config.DefaultShouldConnectAtStartup))
	eventsSortedAscendingly.SetChecked(preferences.BoolWithFallback(config.PrefEventsTableSortOrderAscending, config.DefaultEventsTableSortOrderAscending))
	dataPageBufferSize.SetText(fmt.Sprintf("%d", preferences.IntWithFallback(config.PrefBufferSizeInDataPage, config.DefaultBufferSizeInDataPage)))

	redisForm := widget.NewForm(redisFields...)
	mqttForm := widget.NewForm(mqttFields...)

	connectedWarning := canvas.NewText("* since you are currently connected, some settings require you to disconnect and reconnect to be applied", theme.ErrorColor())
	connectedWarning.TextSize = 11

	if appState.GetConnectionState() == services.ClientConnected {
		connectedWarning.Show()
	} else {
		connectedWarning.Hide()
	}
	commonForm := widget.NewForm(commonFields...)
	commonForm.CancelText = "Reset defaults"

	messagingSelector := widget.NewSelect([]string{
		string(config.RedisMessagingType),
		string(config.MQTTMessagingType),
	}, func(s string) {
		if config.MessagingType(s) == config.RedisMessagingType {
			redisForm.Show()
			mqttForm.Hide()
		} else {
			redisForm.Hide()
			mqttForm.Show()
		}
	})

	messagingSelectorContainer := container.NewBorder(nil, widget.NewSeparator(), widget.NewLabelWithStyle("Messaging", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, messagingSelector)

	messagingSelector.SetSelected(preferences.StringWithFallback(config.PrefSelectedMessaging, string(config.RedisMessagingType)))

	commonForm.OnSubmit = func() {
		log.Info("Settings form submitted")

		preferences.SetString(config.PrefSelectedMessaging, messagingSelector.Selected)
		if config.MessagingType(messagingSelector.Selected) == config.RedisMessagingType {
			preferences.SetString(config.PrefRedisHost, strings.TrimSpace(redisHostname.Text))
			p, _ := strconv.Atoi(redisPort.Text)
			preferences.SetInt(config.PrefRedisPort, p)
			preferences.SetString(config.PrefRedisPassword, strings.TrimSpace(redisPassword.Text))
		} else {
			preferences.SetString(config.PrefMQTTHost, strings.TrimSpace(mqttHostname.Text))
			p, _ := strconv.Atoi(mqttPort.Text)
			preferences.SetInt(config.PrefMQTTPort, p)
			preferences.SetString(config.PrefMQTTClientId, strings.TrimSpace(mqttClientId.Text))
			preferences.SetString(config.PrefMQTTUsername, strings.TrimSpace(mqttUsername.Text))
			preferences.SetString(config.PrefMQTTPassword, strings.TrimSpace(mqttPassword.Text))
		}

		preferences.SetString(config.PrefEventsTopic, strings.TrimSpace(topic.Text))

		preferences.SetBool(config.PrefShouldConnectAtStartup, shouldConnectAutomatically.Checked)
		preferences.SetBool(config.PrefEventsTableSortOrderAscending, eventsSortedAscendingly.Checked)

		buffSize, _ := strconv.Atoi(dataPageBufferSize.Text)
		preferences.SetInt(config.PrefBufferSizeInDataPage, buffSize)

		a.SendNotification(&fyne.Notification{
			Title:   "EdgeX Redis Pub/Sub Connection Settings",
			Content: fmt.Sprintf("%v:%v", redisHostname.Text, redisPort.Text),
		})
	}

	commonForm.OnCancel = func() {
		if config.MessagingType(messagingSelector.Selected) == config.RedisMessagingType {
			redisHostname.Text = config.RedisDefaultHost
			redisPort.Text = fmt.Sprintf("%d", config.RedisDefaultPort)
			redisHostname.Validate()
			redisPort.Validate()
			redisForm.Refresh()
		} else {
			mqttHostname.Text = config.MQTTDefaultHost
			mqttPort.Text = fmt.Sprintf("%d", config.MQTTDefaultPort)
			mqttHostname.Validate()
			mqttPort.Validate()
			mqttForm.Refresh()
		}

		topic.Text = config.DefaultEventsTopic

		shouldConnectAutomatically.SetChecked(config.DefaultShouldConnectAtStartup)
		eventsSortedAscendingly.SetChecked(config.DefaultEventsTableSortOrderAscending)

		log.Info("Settings reset to default")
	}

	return container.NewVBox(messagingSelectorContainer, redisForm, mqttForm, commonForm, connectedWarning)

}
