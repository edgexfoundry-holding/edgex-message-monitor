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
package config

const (
	PrefProcotol = "_PrefProcotol"

	PrefRedisHost     = "_RedisHost"
	PrefRedisPort     = "_RedisPort"
	PrefRedisPassword = "_RedisPassword"
	PrefMQTTHost      = "_MQTTHost"
	PrefMQTTPort      = "_MQTTPort"
	PrefMQTTClientId  = "_MQTTClientId"
	PrefMQTTPassword  = "_MQTTPassword"
	PrefMQTTUsername  = "_MQTTUsername"

	PrefShouldConnectAtStartup        = "_ShouldConnectAtStartup"
	PrefEventsTableSortOrderAscending = "_EventsTableSortOrderAscending"
	PrefBufferSizeInDataPage          = "_BufferSizeInDataPage"
	PrefEventsTopic                   = "_EventsTopic"

	SessionDataPageDataType   = "Session_DataPageDataType"
	SessionDataPageBufferSize = "Session_DataPage_BufferSize"
	SessionDataPageSearch     = "Session_DataPage_Search"
)

const (
	ProtocolRedis = "Redis"
	ProtocolMQTT  = "MQTT"
)

const (
	DefaultProcotol = ProtocolRedis

	RedisDefaultHost     = "localhost"
	RedisDefaultPort     = 6379
	RedisDefaultPassword = ""
	MQTTDefaultHost      = "localhost"
	MQTTDefaultPort      = 1883
	MQTTDefaultClientId  = ""
	MQTTDefaultPassword  = ""
	MQTTDefaultUsername  = ""

	DefaultEventsTopic = "edgex/events/#"

	DefaultShouldConnectAtStartup        = false
	DefaultEventsTableSortOrderAscending = false
	DefaultBufferSizeInDataPage          = 100
	DefaultFilteringUpdateCadenceMs      = 1000
)

const (
	MinBufferSize = 1
	MaxBufferSize = 100000
)

const (
	DataTypeEvents   = "Events"
	DataTypeReadings = "Readings"
)
