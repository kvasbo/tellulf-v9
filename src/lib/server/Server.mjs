import express from "express";
import path from "path";
import WebSocket from "ws";
import http from "http";
import Twig from "twig";
import * as sass from "sass";
import { readFileSync } from "fs";
import { fileURLToPath } from "url";

import { Days } from "./Days.mjs";
import { Entur } from "./Entur.mjs";
import { Weather } from "./Weather.mjs";
import { Calendar } from "./Calendar.mjs";
import { Settings } from "luxon";
import { Smarthouse } from "./Smarthouse.mjs";
import { MqttClient } from "./MQTT.mjs";
import { PowerPrice } from "./PowerPrice.mjs";

// Get the filename and directory name
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Import the src/version.json file
import { version } from "./version.json";

// Load the twig template
const templateData = readFileSync("./views/index.twig", "utf8");
const template = Twig.twig({ id: "index", data: templateData });


// Configure the time zone
Settings.defaultZone = "Europe/Oslo";

const app = express();

const mqttClient = new MqttClient();

// Create smarthouse connector
const smart = new Smarthouse(mqttClient);
smart.startMqtt();

const powerPriceGetter = new PowerPrice();

// Express settings
app.use("/assets", express.static(path.join(__dirname, "../assets"))); // Static routes

const port = 3000;

const calendar = new Calendar();
const days = new Days(calendar);
const entur = new Entur();
const weather = new Weather();

// Return stylesheet
app.get("/client.css", (req, res) => {
  // Load and compile Sass and JS
  const stylesheet = sass.compile("./sass/tellulf.scss", { style: "compressed" });
  res.setHeader("Content-Type", "text/css");
  res.send(stylesheet.css.toString());
});

// Return javascript
app.get("/client.js", (req, res) => {
  res.setHeader("Content-Type", "text/javascript");
  const clientJs = readFileSync("./src/Client.js", "utf8");
  res.send(clientJs);
});

// Return manifest
app.get("/manifest.json", (req, res) => {
  res.setHeader("Content-Type", "text/json");
  const clientJs = readFileSync("./assets/manifest.json", "utf8");
  res.send(clientJs);
});

app.get("/", (req, res) => {
  const data = {
    current_temperature: days.weather.getCurrentWeather().temperature,
    current_weather_icon: days.weather.getCurrentWeather().symbol,
    days: days.generateComingDays(),
    today: days.GenerateToday(),
    hourly_weather: days.weather.getHourlyForecasts(),
    danger_data: days.weather.getDangerData(),
  };

  const rendered = template.render(data);
  res.send(rendered);
  console.log("Rendered index");
});

const server = http.createServer(app);

server.listen(port, () => {
  console.log(
    `Tellulf version ${version} listening on port ${port} for both WS and HTTP`,
  );
});

/**
 * Push data to all connected clients
 */
function pushDataToClients() {

  const homey = smart.getData();
  const enturData = entur.Get();
  const currentWeather = weather.getCurrentWeather();
  const longTermForecast = weather.getDailyForecasts();
  const powerPrice = powerPriceGetter.getPowerPrice();

}

/**
 * Run the update loop for the clock once per minute
 * @param queueNext
 * @returns {void}
 */
function updateClocks(queueNext = true) {
  console.log(
    `Sending time (${new Date().toLocaleTimeString()}) to ${
      clients.length
    } clients.`,
  );

  // Recalculate the delay until the start of the next minute
  const now = new Date();
  const delay = 60000 - (now.getSeconds() * 1000 + now.getMilliseconds());

  const timePayload = JSON.stringify({
    time: Clock.getTime(),
  });

  clients.forEach((client) => {
    client.send(timePayload);
  });

  // Set a timeout for the next execution
  if (queueNext) {
    setTimeout(updateClocks, delay);
  }
}

updateClocks();
pushDataToClients();
setInterval(pushDataToClients, 10000);

process.on("uncaughtException", function (err) {
  console.log("Caught exception: " + err);
  process.exit(1);
});
