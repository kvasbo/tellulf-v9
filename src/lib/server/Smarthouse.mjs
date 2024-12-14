import { Database } from "./Database.mjs";

export class Smarthouse {
  constructor(mqttClient) {
    this.mqttClient = mqttClient;
    this.database = new Database();
  }

  coolerRoomTemp = -9999;
  coolerRoomHumidity = -9999;
  coolerRoomBattery = -9999;

  status = {
    home: structuredClone(statusInitValues),
    cabin: structuredClone(statusInitValues),
  };

  temp = -9999;
  hum = -9999;
  pressure = 0;

  getData() {
    const costHome = this.status.home.currentPrice.totalAfterSupport;
    const costCabin = this.status.cabin.currentPrice.totalAfterSupport;
    const output = {
      tempOut: this.temp,
      humOut: this.hum,
      pressure: this.pressure,
      powerCostNowCabin: costCabin,
      powerCostNowHome: costHome,
      powerUsedToday: this.status.home.day.accumulatedConsumption,
      power: this.status.home.power,
      costToday: this.status.home.day.accumulatedCost,
      powerCabin: this.status.cabin.power,
      powerUsedTodayCabin: this.status.cabin.day.accumulatedConsumption,
      costTodayCabin: this.status.cabin.day.accumulatedCost,
      coolerRoomHumidity: this.coolerRoomHumidity,
      coolerRoomTemp: this.coolerRoomTemp,
      coolerRoomBattery: this.coolerRoomBattery,
    };
    return output;
  }

  parseTibberData(message) {
    // Convert to object
    const data = JSON.parse(message);
    // Set internal status.
    this.status = data;
  }

  startMqtt() {
    this.mqttClient.client.on("message", (topic, message) => {
      const msg = message.toString();
      switch (topic) {
        // The big new one!
        case "tibber/power":
          this.parseTibberData(msg);
          break;
        case "tellulf/weather/tempOut":
          this.temp = parseFloat(message.toString());
          this.mqttClient.log("Temperature set to:", this.temp);
          this.database.putTemperaturePoint(this.temp);
          break;
        case "tellulf/weather/humidity":
          this.hum = parseFloat(message.toString());
          this.mqttClient.log("Humidity set to:", this.hum);
          break;
        case "tellulf/weather/pressure":
          this.pressure = parseFloat(message.toString());
          this.mqttClient.log("Pressure set to:", this.pressure);
          break;
        case "kjølerom/temperature":
          this.coolerRoomTemp = parseFloat(message.toString());
          break;
        case "kjølerom/humidity":
          this.coolerRoomHumidity = parseFloat(message.toString());
          break;
        case "kjølerom/battery":
          this.coolerRoomBattery = parseFloat(message.toString());
          break;
        default:
          break;
      }
    });
  }
}

const statusInitValues = {
  power: 0,
  day: {
    accumulatedConsumption: 0,
    accumulatedProduction: 0,
    accumulatedCost: 0,
  },
  month: {
    accumulatedConsumption: 0,
    accumulatedProduction: 0,
    accumulatedCost: 0,
  },
  minPower: 0,
  averagePower: 0,
  maxPower: 0,
  accumulatedReward: 0,
  powerProduction: 0,
  minPowerProduction: 0,
  maxPowerProduction: 0,
  usageForDay: {},
  usageForTodayLastHourSeen: 0,
  usageForTodayUpToThisHour: 0,
  prices: {},
  currentPrice: {
    energy: 0,
    tax: 0,
    transportCost: 0,
    energyAfterSupport: 0,
    totalAfterSupport: 0,
  },
};
