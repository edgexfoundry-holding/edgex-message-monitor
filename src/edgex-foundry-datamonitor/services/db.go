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

	"fyne.io/fyne/v2"
	log "github.com/sirupsen/logrus"

	"github.com/deblasis/edgex-foundry-datamonitor/config"
	"github.com/deblasis/edgex-foundry-datamonitor/data"
	"github.com/deblasis/edgex-foundry-datamonitor/messaging"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/dtos"
	"github.com/kelindar/column"
)

type DB struct {
	events   *column.Collection
	readings *column.Collection

	eventsSnapshot   *column.Collection
	readingsSnapshot *column.Collection

	envelopesMap map[string]messaging.Envelope
	eventMapLock sync.RWMutex

	matchedEventIds   *matched
	matchedReadingIds *matched

	eventSerial   int64
	readingSerial int64

	filterString       string
	topicFilter        string
	defaultTopicFilter string

	bufferSize int64

	State DBState

	preferences fyne.Preferences

	sync.RWMutex
}

func NewDB(preferences fyne.Preferences) *DB {

	topicFilterPrefix := strings.TrimRight(preferences.StringWithFallback(config.PrefEventsTopic, config.DefaultEventsTopic), "#")
	defaultTopicFilter := fmt.Sprintf("%s#", topicFilterPrefix)

	db := &DB{
		eventSerial:   math.MinInt64 + config.MaxBufferSize,
		readingSerial: math.MinInt64 + config.MaxBufferSize,

		envelopesMap: make(map[string]messaging.Envelope),

		bufferSize: config.DefaultBufferSizeInDataPage,

		matchedEventIds: &matched{
			Serials: map[int64]struct{}{},
		},
		matchedReadingIds: &matched{
			Serials: map[int64]struct{}{},
		},
		defaultTopicFilter: defaultTopicFilter,
		topicFilter:        defaultTopicFilter,

		preferences: preferences,
		State:       DBLive,
	}
	db.initCollections()

	return db
}

func (db *DB) getTrackedEvent(eventId string) *messaging.Envelope {
	db.eventMapLock.RLock()
	defer db.eventMapLock.RUnlock()
	event, ok := db.envelopesMap[eventId]
	if !ok {
		log.Debugf("getTrackedEvent(%v) not ok", eventId)
		return nil
	}
	return &event
}

func (db *DB) Pause() {
	log.Infof("PAUSE")

	db.Lock()
	defer db.Unlock()

	go func() {
		db.cleanSnapshottedEvents()
		db.cleanSnapshottedReadings()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		liveEvents := db.getEventsWithSerial(DBLive)
		for eSerial, eventEnvelope := range liveEvents {

			eMap := eventToMap(eventEnvelope, int64(eSerial))
			db.eventsSnapshot.InsertObject(eMap)
		}
	}()

	go func() {
		defer wg.Done()
		liveReadings := db.getReadingsWithSerial(DBLive)
		for rSerial, readingEnvelope := range liveReadings {
			reading := readingEnvelope.Content.(dtos.BaseReading)
			event := db.getTrackedEvent(readingEnvelope.EventId)
			rMap := readingToMap(*event, reading, int64(rSerial))
			db.readingsSnapshot.InsertObject(rMap)
		}
	}()
	wg.Wait()

	db.State = DBPaused
	db.updateFilter(db.filterString)

}

func (db *DB) Resume() {
	log.Infof("RESUME")

	db.Lock()
	defer db.Unlock()

	db.cleanSnapshottedEvents()
	db.cleanSnapshottedReadings()

	db.State = DBLive
	db.updateFilter(db.filterString)

}

func (db *DB) GetState() DBState {
	db.RLock()
	defer db.RUnlock()
	return db.State
}

func (db *DB) UpdateBufferSize(newBufferSize int64) {
	db.Lock()
	defer db.Unlock()

	db.bufferSize = newBufferSize
	db.evictOldEvents()
	db.evictOldReadings()
}

func (db *DB) updateFilter(filter string) {
	db.filterString = filter

	dbState := db.State
	eventsSource := db.events
	readingsSource := db.readings
	if dbState == DBPaused {
		log.Debugf("DB: UpdateFilter -> reading from snapshot")

		eventsSource = db.eventsSnapshot
		readingsSource = db.readingsSnapshot
	}

	db.refreshIndex(eventsSource, "event_id", stringType)
	db.refreshIndex(eventsSource, "event_deviceName", stringType)
	db.refreshIndex(eventsSource, "event_profileName", stringType)
	db.refreshIndex(eventsSource, "event_created", intType)
	db.refreshIndex(eventsSource, "event_origin", intType)
	db.refreshIndex(eventsSource, "event_tags", stringType)

	db.refreshIndex(readingsSource, "event_id", stringType)
	db.refreshIndex(readingsSource, "event_deviceName", stringType)
	db.refreshIndex(readingsSource, "event_profileName", stringType)
	db.refreshIndex(readingsSource, "event_created", intType)
	db.refreshIndex(readingsSource, "event_origin", intType)
	db.refreshIndex(readingsSource, "event_tags", stringType)

	db.refreshIndex(readingsSource, "reading_id", stringType)
	db.refreshIndex(readingsSource, "reading_created", intType)
	db.refreshIndex(readingsSource, "reading_origin", intType)
	db.refreshIndex(readingsSource, "reading_deviceName", stringType)
	db.refreshIndex(readingsSource, "reading_profileName", stringType)
	db.refreshIndex(readingsSource, "reading_valueType", stringType)
	db.refreshIndex(readingsSource, "reading_binaryValue", byteArrType)
	db.refreshIndex(readingsSource, "reading_mediaType", stringType)
	db.refreshIndex(readingsSource, "reading_value", stringType)

	db.filter()
}

func (db *DB) updateTopicFilter(topicFilter string) {

	prefix := strings.TrimRight(db.preferences.StringWithFallback(config.PrefEventsTopic, config.DefaultEventsTopic), "#")

	db.topicFilter = fmt.Sprintf("%s%s", prefix, topicFilter)

	dbState := db.State
	eventsSource := db.events
	readingsSource := db.readings
	if dbState == DBPaused {
		log.Debugf("DB: UpdateFilter -> reading from snapshot")

		eventsSource = db.eventsSnapshot
		readingsSource = db.readingsSnapshot
	}

	db.refreshTopicIndex(eventsSource)
	db.refreshTopicIndex(readingsSource)

	db.filter()
}

func (db *DB) UpdateFilter(filter string) {
	log.Debugf("DB: UpdateFilter %v", filter)
	db.Lock()
	defer db.Unlock()
	db.updateFilter(filter)
}

func (db *DB) GetEventsCount() int64 {
	var count int64

	var source = db.events
	if db.State == DBPaused {
		source = db.eventsSnapshot
	}

	source.Query(func(txn *column.Txn) error {
		if db.filterString == "" && strings.EqualFold(db.topicFilter, db.defaultTopicFilter) {
			count = int64(txn.Count())
		} else {
			count = int64(txn.With("matching_serial_idx").Count())
		}
		return nil
	})
	return count
}

func (db *DB) GetTotalEventsCount() int64 {
	var count int64
	db.events.Query(func(txn *column.Txn) error {
		count = int64(txn.Count())
		return nil
	})
	return count
}

func (db *DB) GetReadingsCount() int64 {
	var count int64

	var source = db.readings
	if db.State == DBPaused {
		source = db.readingsSnapshot
	}

	source.Query(func(txn *column.Txn) error {
		if db.filterString == "" && strings.EqualFold(db.topicFilter, db.defaultTopicFilter) {
			count = int64(txn.Count())
		} else {
			count = int64(txn.With("matching_serial_idx").Count())
		}
		return nil
	})
	return count
}

func (db *DB) GetTotalReadingsCount() int64 {
	var count int64
	db.readings.Query(func(txn *column.Txn) error {
		count = int64(txn.Count())
		return nil
	})
	return count
}

func (db *DB) getEventsWithSerial(state DBState) []messaging.Envelope {

	events := make([]messaging.Envelope, 0)

	mapFunc := func(v column.Selector) {
		var (
			readings []dtos.BaseReading
			tags     map[string]string
		)

		json.Unmarshal([]byte(v.StringAt("event_readings")), &readings)
		json.Unmarshal([]byte(v.StringAt("event_tags")), &tags)

		serial, err := strconv.Atoi(v.ValueAt("serial").(string))
		if err != nil {
			return
		}
		eventId := v.StringAt("event_id")
		events = append(events,
			messaging.Envelope{
				Serial:  int64(serial),
				Topic:   v.StringAt("event_topic"),
				EventId: eventId,
				Content: dtos.Event{
					Id:          eventId,
					DeviceName:  v.StringAt("event_deviceName"),
					ProfileName: v.StringAt("event_profileName"),
					Created:     v.IntAt("event_created"),
					Origin:      v.IntAt("event_origin"),
					Readings:    readings,
					Tags:        tags,
				},
			})

	}

	var source = db.events
	if state == DBPaused {
		log.Infof("reading from snapshot")
		source = db.eventsSnapshot
	}

	source.Query(func(txn *column.Txn) error {
		if db.filterString == "" && strings.EqualFold(db.topicFilter, db.defaultTopicFilter) {
			txn.Select(mapFunc)
		} else {
			txn.With("matching_serial_idx").Select(mapFunc)
		}
		return nil
	})
	return events

}

func (db *DB) GetEvents() []dtos.Event {
	log.Debugf("DB: GetEvents")

	events := make([]dtos.Event, 0)

	envelopes := db.getEventsWithSerial(db.State)
	for _, e := range envelopes {
		events = append(events, e.Content.(dtos.Event))
	}
	return events
}

func (db *DB) getReadingsWithSerial(state DBState) []messaging.Envelope {
	readings := make([]messaging.Envelope, 0)

	mapFunc := func(v column.Selector) {

		serial, err := strconv.Atoi(v.ValueAt("serial").(string))
		if err != nil {
			return
		}
		eventId := v.StringAt("event_id")
		readings = append(readings,
			messaging.Envelope{
				Serial:  int64(serial),
				EventId: eventId,
				Topic:   v.StringAt("event_topic"),
				Content: dtos.BaseReading{
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

	var source = db.readings
	if state == DBPaused {
		source = db.readingsSnapshot
	}

	source.Query(func(txn *column.Txn) error {

		if db.filterString == "" && strings.EqualFold(db.topicFilter, db.defaultTopicFilter) {
			txn.Select(mapFunc)
		} else {
			txn.With("matching_serial_idx").Select(mapFunc)
		}
		return nil
	})
	return readings
}

func (db *DB) GetReadings() []dtos.BaseReading {
	readings := make([]dtos.BaseReading, 0)

	envelopes := db.getReadingsWithSerial(db.State)
	for _, e := range envelopes {
		readings = append(readings, e.Content.(dtos.BaseReading))
	}

	return readings
}

func filterMatchesIndex(field string) string {
	return fmt.Sprintf("%v_idx", field)
}

func (db *DB) refreshTopicIndex(c *column.Collection) {
	fieldName := "event_topic"

	regex := data.MustCompileTopicFilterRegex(db.topicFilter)

	c.DropIndex(filterMatchesIndex(fieldName))
	c.CreateIndex(filterMatchesIndex(fieldName), fieldName, func(r column.Reader) bool {

		log.Debugf("matching %v with %v => %v", r.String(), regex, regex.Match([]byte(r.String())))

		return regex.Match([]byte(r.String()))
	})
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
	log.Debugf("DB: filter()")

	db.cleanMatches()
	if db.filterString == "" && strings.EqualFold(db.topicFilter, db.defaultTopicFilter) {
		return
	}

	dbState := db.State
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		var source = db.events
		if dbState == DBPaused {
			log.Infof("filtering from snapshot")
			source = db.eventsSnapshot
		}

		source.Query(func(txn *column.Txn) error {
			txn.
				With(filterMatchesIndex("event_id")).
				Union(filterMatchesIndex("event_deviceName")).
				Union(filterMatchesIndex("event_profileName")).
				Union(filterMatchesIndex("event_created")).
				Union(filterMatchesIndex("event_origin")).
				Union(filterMatchesIndex("event_tags")).
				With(filterMatchesIndex("event_topic")).
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
		var source = db.readings
		if dbState == DBPaused {
			log.Infof("filtering from snapshot")
			source = db.readingsSnapshot
		}
		source.Query(func(txn *column.Txn) error {
			txn.
				With(filterMatchesIndex("event_id")).
				Union(filterMatchesIndex("event_deviceName")).
				Union(filterMatchesIndex("event_profileName")).
				Union(filterMatchesIndex("event_created")).
				Union(filterMatchesIndex("event_origin")).
				Union(filterMatchesIndex("event_tags")).
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
				With(filterMatchesIndex("event_topic")).
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

	if dbState == DBLive {
		db.refreshMatchingIndex(db.events, "serial", isMatchingEventType)
		db.refreshMatchingIndex(db.readings, "serial", isMatchingReadingType)
	} else {
		db.refreshMatchingIndex(db.eventsSnapshot, "serial", isMatchingEventType)
		db.refreshMatchingIndex(db.readingsSnapshot, "serial", isMatchingReadingType)
	}

}

func (db *DB) trackEventEnvelope(envelope messaging.Envelope) {
	db.eventMapLock.Lock()
	defer db.eventMapLock.Unlock()
	log.Debugf("trackEventEnvelope(%v)", envelope.EventId)
	db.envelopesMap[envelope.EventId] = envelope
}

func (db *DB) OnEventReceived(envelope messaging.Envelope) {

	event := envelope.Content.(dtos.Event)
	db.trackEventEnvelope(envelope)

	eSerial := db.nextEventSerial()
	eMap := eventToMap(envelope, eSerial)
	db.events.InsertObject(eMap)

	for _, reading := range event.Readings {
		rSerial := db.nextReadingSerial()
		rMap := readingToMap(envelope, reading, rSerial)
		db.readings.InsertObject(rMap)
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
			serial, _ := strconv.Atoi(v.Selector.ValueAt("serial").(string))
			db.matchedReadingIds.Lock()
			defer db.matchedReadingIds.Unlock()
			delete(db.matchedReadingIds.Serials, int64(serial))

			v.Delete()
		})
		return nil
	})
}

func (db *DB) cleanSnapshottedEvents() {
	db.eventsSnapshot.Query(func(txn *column.Txn) error {
		txn.DeleteAll()
		return nil
	})
}

func (db *DB) cleanSnapshottedReadings() {
	db.readingsSnapshot.Query(func(txn *column.Txn) error {
		txn.DeleteAll()
		return nil
	})
}

func eventToMap(envelope messaging.Envelope, serial int64) map[string]interface{} {

	event := envelope.Content.(dtos.Event)

	tagsJson, _ := json.Marshal(event.Tags)
	readingsJson, _ := json.Marshal(event.Readings)

	m := map[string]interface{}{
		"serial": fmt.Sprintf("%v", serial),

		"event_id":            event.Id,
		"event_deviceName":    event.DeviceName,
		"event_profileName":   event.ProfileName,
		"event_created":       event.Created,
		"event_origin":        event.Origin,
		"event_readingsCount": len(event.Readings),
		"event_readings":      readingsJson,
		"event_tags":          string(tagsJson),

		"event_topic": envelope.Topic,
	}

	return m
}

func readingToMap(envelope messaging.Envelope, reading dtos.BaseReading, serial int64) map[string]interface{} {

	event := envelope.Content.(dtos.Event)

	tags, _ := json.Marshal(event.Tags)

	m := map[string]interface{}{
		"serial": fmt.Sprintf("%v", serial),

		"event_id":          event.Id,
		"event_deviceName":  event.DeviceName,
		"event_profileName": event.ProfileName,
		"event_created":     event.Created,
		"event_origin":      event.Origin,
		"event_tags":        string(tags),

		"event_topic": envelope.Topic,

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

func (db *DB) initCollections() {
	eventsSnapshotCollection := column.NewCollection()
	eventsSnapshotCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(eventsSnapshotCollection)
	db.eventsSnapshot = eventsSnapshotCollection

	readingsSnapshotCollection := column.NewCollection()
	readingsSnapshotCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(readingsSnapshotCollection)
	setupReadingFields(readingsSnapshotCollection)
	db.readingsSnapshot = readingsSnapshotCollection

	eventsCollection := column.NewCollection()

	eventsCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(eventsCollection)
	db.events = eventsCollection

	readingsCollection := column.NewCollection()
	readingsCollection.CreateColumn("serial", column.ForKey())
	setupEventFields(readingsCollection)
	setupReadingFields(readingsCollection)
	db.readings = readingsCollection

}

func setupEventFields(c *column.Collection) {
	c.CreateColumn("event_id", column.ForString())
	c.CreateColumn("event_deviceName", column.ForString())
	c.CreateColumn("event_profileName", column.ForString())
	c.CreateColumn("event_created", column.ForInt64())
	c.CreateColumn("event_origin", column.ForInt64())
	c.CreateColumn("event_readings", column.ForString())
	c.CreateColumn("event_readingCount", column.ForInt64())
	c.CreateColumn("event_tags", column.ForString())

	c.CreateColumn("event_topic", column.ForString())
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

type matched struct {
	Serials map[int64]struct{}
	sync.RWMutex
}

type indexType int

const (
	stringType indexType = iota
	intType
	byteArrType
	isMatchingEventType
	isMatchingReadingType
)
