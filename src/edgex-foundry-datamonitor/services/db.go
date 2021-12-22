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
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/deblasis/edgex-foundry-datamonitor/config"
	appTypes "github.com/deblasis/edgex-foundry-datamonitor/types"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/dtos"
	"github.com/kelindar/column"
)

type DB struct {
	events         *column.Collection
	eventsBackup   *column.Collection
	readings       *column.Collection
	readingsBackup *column.Collection

	matchedEventIds   *matched
	matchedReadingIds *matched
	BackupLock        *sync.RWMutex

	eventSerial   int64
	readingSerial int64

	eventSerialBackup   int64
	readingSerialBackup int64

	filterString string
	bufferSize   int64

	sync.RWMutex
}

type matched struct {
	Serials map[int64]struct{}
	sync.RWMutex
}

func NewDB(filterCadenceMs int64) *DB {
	db := &DB{
		eventSerial:   math.MinInt64 + config.MaxBufferSize,
		readingSerial: math.MinInt64 + config.MaxBufferSize,

		bufferSize: config.DefaultBufferSizeInDataPage,

		matchedEventIds: &matched{
			Serials: map[int64]struct{}{},
		},
		matchedReadingIds: &matched{
			Serials: map[int64]struct{}{},
		},
		BackupLock: &sync.RWMutex{},
	}
	db.initCollections()
	return db
}

func (db *DB) UpdateBufferSize(newBufferSize int64) {
	db.Lock()
	defer db.Unlock()

	db.bufferSize = newBufferSize
	db.evictOldEvents()
	db.evictOldReadings()
}

func (db *DB) UpdateFilter(filter string) {
	db.Lock()
	defer db.Unlock()
	db.filterString = filter

	db.refreshIndex(db.eventsBackup, "event_topic", stringType)
	db.refreshIndex(db.eventsBackup, "event_id", stringType)
	db.refreshIndex(db.eventsBackup, "event_deviceName", stringType)
	db.refreshIndex(db.eventsBackup, "event_profileName", stringType)
	db.refreshIndex(db.eventsBackup, "event_created", intType)
	db.refreshIndex(db.eventsBackup, "event_origin", intType)
	db.refreshIndex(db.eventsBackup, "event_tags", stringType)

	db.refreshIndex(db.readingsBackup, "event_topic", stringType)
	db.refreshIndex(db.readingsBackup, "event_id", stringType)
	db.refreshIndex(db.readingsBackup, "event_deviceName", stringType)
	db.refreshIndex(db.readingsBackup, "event_profileName", stringType)
	db.refreshIndex(db.readingsBackup, "event_created", intType)
	db.refreshIndex(db.readingsBackup, "event_origin", intType)
	db.refreshIndex(db.readingsBackup, "event_tags", stringType)

	db.refreshIndex(db.readingsBackup, "reading_id", stringType)
	db.refreshIndex(db.readingsBackup, "reading_created", intType)
	db.refreshIndex(db.readingsBackup, "reading_origin", intType)
	db.refreshIndex(db.readingsBackup, "reading_deviceName", stringType)
	db.refreshIndex(db.readingsBackup, "reading_profileName", stringType)
	db.refreshIndex(db.readingsBackup, "reading_valueType", stringType)
	db.refreshIndex(db.readingsBackup, "reading_binaryValue", byteArrType)
	db.refreshIndex(db.readingsBackup, "reading_mediaType", stringType)
	db.refreshIndex(db.readingsBackup, "reading_value", stringType)

	db.filter()

}

func (db *DB) GetEventsCount(filter bool) int64 {
	var count int64
	var colls *column.Collection
	if filter {
		colls = db.eventsBackup
	} else {
		colls = db.events
	}
	colls.Query(func(txn *column.Txn) error {
		if db.filterString == "" {
			count = int64(txn.Count())
		} else {
			count = int64(txn.With("matching_serial_idx").Count())
		}
		return nil
	})
	return count
}

func (db *DB) GetTotalEventsCount(filter bool) int64 {
	var count int64
	var colls *column.Collection
	if filter {
		colls = db.eventsBackup
	} else {
		colls = db.events
	}
	colls.Query(func(txn *column.Txn) error {
		count = int64(txn.Count())
		return nil
	})
	return count
}

func (db *DB) GetReadingsCount(filter bool) int64 {
	var count int64
	var colls *column.Collection
	if filter {
		colls = db.readingsBackup
	} else {
		colls = db.readings
	}
	colls.Query(func(txn *column.Txn) error {
		if db.filterString == "" {
			count = int64(txn.Count())
		} else {
			count = int64(txn.With("matching_serial_idx").Count())
		}
		return nil
	})
	return count
}

func (db *DB) GetTotalReadingsCount(filter bool) int64 {
	var count int64
	var colls *column.Collection
	if filter {
		colls = db.readingsBackup
	} else {
		colls = db.readings
	}
	colls.Query(func(txn *column.Txn) error {
		count = int64(txn.Count())
		return nil
	})
	return count
}

func (db *DB) GetEvents(filter bool) []appTypes.MessageEvent {
	events := make([]appTypes.MessageEvent, 0)

	mapFunc := func(v column.Selector) {
		events = append(events, columnToEvent(v))
	}

	var colls *column.Collection
	if filter {
		colls = db.eventsBackup
	} else {
		colls = db.events
	}
	colls.Query(func(txn *column.Txn) error {

		// FILTER only supports pause mode (use eventsBackup)
		if !filter {
			txn.Select(mapFunc)
			return nil
		}

		if db.filterString == "" {
			txn.Select(mapFunc)
		} else {
			txn.With("matching_serial_idx").Select(mapFunc)
		}
		return nil
	})
	return events
}

func (db *DB) GetReadings(filter bool) []appTypes.MessageReading {
	readings := make([]appTypes.MessageReading, 0)

	mapFunc := func(v column.Selector) {

		readings = append(readings, appTypes.MessageReading{
			Topic: v.StringAt("event_topic"),

			BaseReading: dtos.BaseReading{
				Id:           v.StringAt("reading_id"),
				Created:      v.IntAt("reading_created"),
				Origin:       v.IntAt("reading_origin"),
				DeviceName:   v.StringAt("reading_deviceName"),
				ResourceName: v.StringAt("reading_resourceName"),
				ProfileName:  v.StringAt("reading_profileName"),
				ValueType:    v.StringAt("reading_valueType"),
				BinaryReading: dtos.BinaryReading{
					BinaryValue: []byte(v.StringAt("reading_binaryValue")),
					MediaType:   v.StringAt("reading_mediaType"),
				},
				SimpleReading: dtos.SimpleReading{
					Value: v.StringAt("reading_value"),
				},
			},
		})

	}

	var colls *column.Collection
	if filter {
		colls = db.readingsBackup
	} else {
		colls = db.readings
	}

	colls.Query(func(txn *column.Txn) error {

		if db.filterString == "" {
			txn.Select(mapFunc)
		} else {
			txn.With("matching_serial_idx").Select(mapFunc)
		}
		return nil
	})
	return readings
}

func filterMatchesIndex(field string) string {
	return fmt.Sprintf("%v_idx", field)
}

func (db *DB) refreshIndex(c *column.Collection, fieldName string, t indexType) {
	c.DropIndex(filterMatchesIndex(fieldName))
	c.CreateIndex(filterMatchesIndex(fieldName), fieldName, func(r column.Reader) bool {

		//Loose search, assuming it's case insensitive...

		switch t {
		case stringType, byteArrType:
			return strings.Contains(strings.ToLower(r.String()), strings.ToLower(db.filterString))
		case intType:
			return strings.Contains(strings.ToLower(fmt.Sprintf("%v", r.Int())), strings.ToLower(db.filterString))
		default:
			log.Fatalf("unhandled type %v in refreshIndex", t)
		}
		return false
	})
}

func (db *DB) refreshMatchingIndex(c *column.Collection, fieldName string, t indexType) {
	c.DropIndex("matching_" + filterMatchesIndex(fieldName))
	c.CreateIndex("matching_"+filterMatchesIndex(fieldName), fieldName, func(r column.Reader) bool {

		switch t {
		case isMatchingEventType:
			db.matchedEventIds.RLock()
			defer db.matchedEventIds.RUnlock()

			serial, _ := strconv.Atoi(r.String())
			_, matching := db.matchedEventIds.Serials[int64(serial)]
			return matching
		case isMatchingReadingType:
			db.matchedReadingIds.RLock()
			defer db.matchedReadingIds.RUnlock()

			serial, _ := strconv.Atoi(r.String())
			_, matching := db.matchedReadingIds.Serials[int64(serial)]
			return matching
		default:
			log.Fatalf("unhandled type %v in refreshMatchingIndex", t)
		}
		return false
	})
}

func (db *DB) cleanMatches() {
	db.matchedEventIds.Lock()
	defer db.matchedEventIds.Unlock()
	db.matchedEventIds.Serials = map[int64]struct{}{}

	db.matchedReadingIds.Lock()
	defer db.matchedReadingIds.Unlock()
	db.matchedReadingIds.Serials = map[int64]struct{}{}
}

func (db *DB) filter() {
	db.cleanMatches()
	if db.filterString == "" {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// db.events.Query(func(txn *column.Txn) error {
		db.eventsBackup.Query(func(txn *column.Txn) error {
			txn.With(filterMatchesIndex("event_id")).
				Union(filterMatchesIndex("event_deviceName")).
				Union(filterMatchesIndex("event_profileName")).
				Union(filterMatchesIndex("event_created")).
				Union(filterMatchesIndex("event_origin")).
				Union(filterMatchesIndex("event_tags")).
				Union(filterMatchesIndex("event_topic")).
				Select(func(v column.Selector) {
					serial, err := strconv.Atoi(v.ValueAt("serial").(string))
					if err != nil {
						return
					}
					db.matchedEventIds.Lock()
					defer db.matchedEventIds.Unlock()

					db.matchedEventIds.Serials[int64(serial)] = struct{}{}

				})

			return nil
		})
	}()

	go func() {
		defer wg.Done()
		db.readingsBackup.Query(func(txn *column.Txn) error {
			txn.With(filterMatchesIndex("event_id")).
				Union(filterMatchesIndex("event_deviceName")).
				Union(filterMatchesIndex("event_profileName")).
				Union(filterMatchesIndex("event_created")).
				Union(filterMatchesIndex("event_origin")).
				Union(filterMatchesIndex("event_tags")).
				Union(filterMatchesIndex("event_topic")).
				Union(filterMatchesIndex("reading_id")).
				Union(filterMatchesIndex("reading_created")).
				Union(filterMatchesIndex("reading_origin")).
				Union(filterMatchesIndex("reading_deviceName")).
				Union(filterMatchesIndex("reading_resourceName")).
				Union(filterMatchesIndex("reading_profileName")).
				Union(filterMatchesIndex("reading_valueType")).
				Union(filterMatchesIndex("reading_binaryValue")).
				Union(filterMatchesIndex("reading_mediaType")).
				Union(filterMatchesIndex("reading_value")).
				Select(func(v column.Selector) {
					serial, err := strconv.Atoi(v.ValueAt("serial").(string))
					if err != nil {
						return
					}
					db.matchedReadingIds.Lock()
					defer db.matchedReadingIds.Unlock()

					db.matchedReadingIds.Serials[int64(serial)] = struct{}{}
				})

			return nil
		})
	}()

	wg.Wait()

	db.refreshMatchingIndex(db.eventsBackup, "serial", isMatchingEventType)
	db.refreshMatchingIndex(db.readingsBackup, "serial", isMatchingReadingType)

}

func (db *DB) BackupCurrentData() {
	db.BackupLock.Lock()
	defer db.BackupLock.Unlock()

	db.initBackupCollections()

	db.events.Query(func(txn *column.Txn) error {
		txn.Select(func(v column.Selector) {
			event := columnToEvent(v)
			serial, _ := strconv.Atoi(v.ValueAt("serial").(string))
			db.eventsBackup.InsertObject(eventToMap(event, int64(serial)))
		})
		return nil
	})

	db.readings.Query(func(txn *column.Txn) error {
		txn.Select(func(v column.Selector) {
			event := columnToEvent(v)
			reading := columnToReading(v)
			serial, _ := strconv.Atoi(v.ValueAt("serial").(string))
			db.readingsBackup.InsertObject(readingToMap(event, reading, int64(serial)))
		})
		return nil
	})

	db.eventSerialBackup = db.eventSerial
	db.readingSerialBackup = db.readingSerial

}

func (db *DB) OnEventReceived(event appTypes.MessageEvent) {
	db.BackupLock.Lock()
	defer db.BackupLock.Unlock()

	eSerial := db.nextEventSerial()

	mapped := eventToMap(event, eSerial)

	db.events.InsertObject(mapped)
	for _, reading := range event.Readings {
		rSerial := db.nextReadingSerial()
		db.readings.InsertObject(readingToMap(event, appTypes.MessageReading{BaseReading: reading, Topic: event.Topic}, rSerial))
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		db.evictOldEvents()
	}()

	go func() {
		defer wg.Done()
		db.evictOldReadings()
	}()

	go func() {
		defer wg.Done()
		db.filter()
	}()

	wg.Wait()

}

func (db *DB) evictOldEvents() {
	db.events.Query(func(txn *column.Txn) error {
		txn.WithValue("serial", func(v interface{}) bool {
			serial, _ := strconv.Atoi(v.(string))
			return int64(serial) <= db.lastEventSerial()-db.bufferSize
		}).Range("serial", func(v column.Cursor) {
			v.Delete()
		})

		return nil
	})
	db.eventsBackup.Query(func(txn *column.Txn) error {
		txn.WithValue("serial", func(v interface{}) bool {
			serial, _ := strconv.Atoi(v.(string))
			return int64(serial) <= db.eventSerialBackup-db.bufferSize
		}).Range("serial", func(v column.Cursor) {
			serial, _ := strconv.Atoi(v.Selector.ValueAt("serial").(string))
			db.matchedEventIds.Lock()
			defer db.matchedEventIds.Unlock()
			delete(db.matchedEventIds.Serials, int64(serial))

			v.Delete()
		})

		return nil
	})
}

func (db *DB) evictOldReadings() {
	db.readings.Query(func(txn *column.Txn) error {
		txn.WithValue("serial", func(v interface{}) bool {
			serial, _ := strconv.Atoi(v.(string))
			return int64(serial) <= db.lastReadingSerial()-db.bufferSize
		}).Range("serial", func(v column.Cursor) {
			v.Delete()
		})
		return nil
	})
	db.readingsBackup.Query(func(txn *column.Txn) error {
		txn.WithValue("serial", func(v interface{}) bool {
			serial, _ := strconv.Atoi(v.(string))
			return int64(serial) <= db.readingSerialBackup-db.bufferSize
		}).Range("serial", func(v column.Cursor) {
			serial, _ := strconv.Atoi(v.Selector.ValueAt("serial").(string))
			db.matchedReadingIds.Lock()
			defer db.matchedReadingIds.Unlock()
			delete(db.matchedReadingIds.Serials, int64(serial))

			v.Delete()
		})
		return nil
	})
}

func eventToMap(event appTypes.MessageEvent, serial int64) map[string]interface{} {

	tagsJson, _ := json.Marshal(event.Tags)
	readingsJson, _ := json.Marshal(event.Readings)

	m := map[string]interface{}{
		"serial": fmt.Sprintf("%v", serial),

		"event_topic":         event.Topic,
		"event_id":            event.Id,
		"event_deviceName":    event.DeviceName,
		"event_profileName":   event.ProfileName,
		"event_created":       event.Created,
		"event_origin":        event.Origin,
		"event_readingsCount": len(event.Readings),
		"event_readings":      readingsJson,
		"event_tags":          string(tagsJson),
	}

	return m
}

func columnToEvent(v column.Selector) appTypes.MessageEvent {
	var (
		readings []dtos.BaseReading
		tags     map[string]string
	)
	json.Unmarshal([]byte(v.StringAt("event_readings")), &readings)
	json.Unmarshal([]byte(v.StringAt("event_tags")), &tags)

	return appTypes.MessageEvent{
		Event: dtos.Event{
			Id:          v.StringAt("event_id"),
			DeviceName:  v.StringAt("event_deviceName"),
			ProfileName: v.StringAt("event_profileName"),
			Created:     v.IntAt("event_created"),
			Origin:      v.IntAt("event_origin"),
			Readings:    readings,
			Tags:        tags,
		},
		Topic: v.StringAt("event_topic"),
	}
}

func columnToReading(v column.Selector) appTypes.MessageReading {
	var (
		readings []appTypes.MessageReading
		tags     map[string]string
	)
	json.Unmarshal([]byte(v.StringAt("event_readings")), &readings)
	json.Unmarshal([]byte(v.StringAt("event_tags")), &tags)

	return appTypes.MessageReading{
		BaseReading: dtos.BaseReading{
			Id:           v.StringAt("reading_id"),
			DeviceName:   v.StringAt("reading_deviceName"),
			Created:      v.IntAt("reading_created"),
			Origin:       v.IntAt("reading_origin"),
			ResourceName: v.StringAt("reading_resourceName"),
			ProfileName:  v.StringAt("reading_profileName"),
			ValueType:    v.StringAt("reading_valueType"),
			BinaryReading: dtos.BinaryReading{
				BinaryValue: []byte(v.StringAt("reading_binaryValue")),
				MediaType:   v.StringAt("reading_mediaType"),
			},
			SimpleReading: dtos.SimpleReading{
				Value: v.StringAt("reading_value"),
			},
		},
		Topic: v.StringAt("event_topic"),
	}
}

func readingToMap(event appTypes.MessageEvent, reading appTypes.MessageReading, serial int64) map[string]interface{} {

	tags, _ := json.Marshal(event.Tags)

	m := map[string]interface{}{
		"serial": fmt.Sprintf("%v", serial),

		"event_id":          event.Id,
		"event_deviceName":  event.DeviceName,
		"event_profileName": event.ProfileName,
		"event_created":     event.Created,
		"event_origin":      event.Origin,
		"event_tags":        string(tags),
		"event_topic":       event.Topic,

		"reading_id":           reading.Id,
		"reading_created":      reading.Created,
		"reading_origin":       reading.Origin,
		"reading_deviceName":   reading.DeviceName,
		"reading_resourceName": reading.ResourceName,
		"reading_profileName":  reading.ProfileName,
		"reading_valueType":    reading.ValueType,
		"reading_binaryValue":  reading.BinaryValue,
		"reading_mediaType":    reading.MediaType,
		"reading_value":        reading.Value,
	}

	return m
}

func (db *DB) Reset() {
	log.Warn("reset DB")
	db.initCollections()
}

func (db *DB) initCollections() {
	eventsCollection := column.NewCollection()

	eventsCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(eventsCollection)
	db.events = eventsCollection

	readingsCollection := column.NewCollection()
	readingsCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(readingsCollection)
	setupReadingFields(readingsCollection)
	db.readings = readingsCollection

	db.initBackupCollections()
}

func (db *DB) initBackupCollections() {
	eventsBackCollection := column.NewCollection()
	eventsBackCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(eventsBackCollection)
	db.eventsBackup = eventsBackCollection

	readingsCollection := column.NewCollection()
	readingsCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(readingsCollection)
	setupReadingFields(readingsCollection)
	db.readingsBackup = readingsCollection
}

func setupEventFields(c *column.Collection) {
	c.CreateColumn("event_topic", column.ForString())
	c.CreateColumn("event_id", column.ForString())
	c.CreateColumn("event_deviceName", column.ForString())
	c.CreateColumn("event_profileName", column.ForString())
	c.CreateColumn("event_created", column.ForInt64())
	c.CreateColumn("event_origin", column.ForInt64())
	c.CreateColumn("event_readings", column.ForString())
	c.CreateColumn("event_readingCount", column.ForInt64())
	c.CreateColumn("event_tags", column.ForString())
}

func setupReadingFields(c *column.Collection) {
	// BaseReading
	c.CreateColumn("reading_id", column.ForString())
	c.CreateColumn("reading_created", column.ForInt64())
	c.CreateColumn("reading_origin", column.ForInt64())
	c.CreateColumn("reading_deviceName", column.ForString())
	c.CreateColumn("reading_resourceName", column.ForString())
	c.CreateColumn("reading_profileName", column.ForString())
	c.CreateColumn("reading_valueType", column.ForString())

	// BaseReading.BinaryReading
	c.CreateColumn("reading_binaryValue", column.ForString())
	c.CreateColumn("reading_mediaType", column.ForString())
	// BaseReading.SimpleReading
	c.CreateColumn("reading_value", column.ForString())
}

func (db *DB) lastEventSerial() int64 {
	return db.eventSerial
}
func (db *DB) nextEventSerial() int64 {
	db.Lock()
	defer db.Unlock()
	next := db.eventSerial + 1
	db.eventSerial = next
	return next
}

func (db *DB) lastReadingSerial() int64 {
	return db.readingSerial
}
func (db *DB) nextReadingSerial() int64 {
	db.Lock()
	defer db.Unlock()
	next := db.readingSerial + 1
	db.readingSerial = next
	return next
}

type indexType int

const (
	stringType indexType = iota
	intType
	byteArrType
	isMatchingEventType
	isMatchingReadingType
)
