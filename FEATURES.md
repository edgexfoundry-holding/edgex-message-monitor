# [EdgeX Foundry(TM) - Build Simple Data Monitor UI #03 - Features](https://www.topcoder.com/challenges/a3e6a611-0bea-4bb7-b77d-0c371c166136?tab=details)


## Disconnected state
<img src="./assets/disconnected.png" alt="connected" />
added topic and details about the messaging protocol

## Pause feature
<img src="./assets/pausebutton.png" alt="pause">
Pause/Unpause button and visual indication of the Data page status.

When paused, the user will filter on the paused state, the buffer will continue filling up in the background.
Unpausing resumes normal operation.

All the active filters are preserved.

## New Data Page
<img src="./assets/newDataPage.png" alt="connected" />
Added topic filtering.
The application shows the root of the events stream it's subscribed to and allows the user to refine down.
For example by replacing the wildcard `#` with `#integer#` you would only see events from `Random-Unsigned-Integer-Device` and `Random-Integer-Device`.
If you need more granularity. You can try with something like `#Int16` and you'll filter down to events from the topic `Random-Integer-Device/Random-Integer-Device/Int16`.
Precise inputs without wildcard work as well.

## New Settings Page
<img src="./assets/newSettingsPage.png" alt="new settings page" />
<img src="./assets/newSettingsPage2.png" alt="new settings page" />

The new settings page now handles the 2 messaging brokers (Redis and MQTT) with added the optional parameters.

It also supports the Topic option. Please note that if the topic doesn't specify a wildcard, it would mean that it's already filtering down to one single topic and it would disable topic filtering in the Data page.

This is because if the user subscribes to `Random-Integer-Device/Random-Integer-Device/Int16`, they will only see these events, nothing to filter down.

---
Challenge #2 & #1 legacy features

## Data page
The data page allows the user to view Events
<img src="./assets/dataPageEvents.png" alt="data events" />

and Readings
<img src="./assets/dataPageReadings.png" alt="data readings" />

as they are ingested.

### Buffer size
It has a configurable "Buffer size" that indicates the number of events/readings that are gonna be kept in memory for further inspection. When the buffer is full, the oldest event/reading is dropped.
The initial value can be changed in the Settings page:
<img src="./assets/settingsPageBufferSize.png" alt="settings buffer size">

### Filter
The filter effectively starts a "live query" on the data.
It means that it will match events/readings matching in a case-insensitive way their properties with the filter.
The query is run on the buffer and it means that the results will change as the events/readings are dropped/received.
<img src="./assets/dataPageFiltered.png" alt="data filtered" />

If the user is viewing readings, the query will match also the parent event properties as per requirements
<img src="./assets/dataPageReadingsReq.png" alt="readings req">

Clicking on an event/reading allows the user to inspect the JSON.
<img src="./assets/dataPageEventsDetail.png" alt="event detail" />
<img src="./assets/dataPageReadingsDetail.png" alt="reading detail" />

