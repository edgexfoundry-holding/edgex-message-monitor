function getRandomFloat(min, max) {
  return Math.random() * (max - min) + min;
}
function getRandomInt(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
}

const deviceName = "my-custom-device";
let message = "test-message";

// DataSender sends async value to MQTT broker every 15 seconds
schedule("*/15 * * * * *", () => {
  let body = {
    name: deviceName,
    cmd: "randnum",
    randnum: getRandomFloat(25, 29).toFixed(1),
  };
  publish("DataTopic", JSON.stringify(body));
});

// DataSender sends async value to MQTT broker every 5 seconds
schedule("*/5 * * * * *", () => {
  let topic = "edgex/events/mqtt-test";
  const uuid = require("uuid")
  let id = uuid()
  let id1 = uuid()

  let payload = {
    apiVersion: "v2",
    id: id,
    deviceName: "my-custom-device",
    profileName: "my-custom-device-profile",
    sourceName: "randnum",
    origin: 1638896535003543739,
    readings: [
      {
        id: id1,
        origin: 1638896535003499457,
        deviceName: "my-custom-device",
        resourceName: "randnum",
        profileName: "my-custom-device-profile",
        valueType: "Float32",
        binaryValue: null,
        mediaType: "",
        value: ""+getRandomInt(1000, -1000),
      },
    ],
  };
  let body = {
    ReceivedTopic: topic,
    CorrelationID: "X-Correlation-ID",
    Payload: Buffer.from(JSON.stringify({ event: payload })).toString("base64"),
    ContentType: "application/json",
  };
  publish(topic, JSON.stringify(body));
});

// DataSender sends async value to MQTT broker every 6 seconds
schedule("*/6 * * * * *", () => {
  let topic = "edgex/events/mqtt-test2";

  const uuid = require("uuid")
  let id = uuid()
  let id1 = uuid()
  let id2 = uuid()
  // let id = "ecd9c2d6-0479-4197-82e4-ec61aeb89e1b""

  let payload = {
    apiVersion: "v2",
    id: id,
    deviceName: "Random-Integer-Device",
    profileName: "Random-Integer-Device",
    sourceName: "Int16",
    origin: 1638896531678878727,
    readings: [
      {
        id: id1,
        origin: 1638896531678878727,
        deviceName: "Random-Integer-Device",
        resourceName: "Int16",
        profileName: "Random-Integer-Device",
        valueType: "Int16",
        binaryValue: null,
        mediaType: "",
        value: ""+getRandomInt(1000, -1000),
      },
      {
        id: id2,
        origin: 1638896531678878727,
        deviceName: "Random-Integer-Device",
        resourceName: "Int16",
        profileName: "Random-Integer-Device",
        valueType: "Int16",
        binaryValue: null,
        mediaType: "",
        value: ""+getRandomInt(1000, -1000),
      },
    ],
  };
  let body = {
    ReceivedTopic: topic,
    CorrelationID: "X-Correlation-ID",
    Payload: Buffer.from(JSON.stringify({ event: payload })).toString("base64"),
    ContentType: "application/json",
  };
  publish(topic, JSON.stringify(body));
});

// CommandHandler receives commands and sends response to MQTT broker
// 1. Receive the reading request, then return the response
// 2. Receive the set request, then change the device value
subscribe("CommandTopic", (topic, val) => {
  var data = val;
  if (data.method == "set") {
    message = data[data.cmd];
  } else {
    switch (data.cmd) {
      case "ping":
        data.ping = "pong";
        break;
      case "message":
        data.message = message;
        break;
      case "randnum":
        data.randnum = 12.123;
        break;
    }
  }
  publish("ResponseTopic", JSON.stringify(data));
});
