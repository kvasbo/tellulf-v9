import 'dotenv/config';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { Eta } from 'eta';
import express from 'express';
import { Entur } from './lib/Entur.js';
import { Places } from './lib/Enums.js';
import { Calendar } from './lib/server/Calendar.js';
import { Days } from './lib/server/Days.js';
import { MqttClient } from './lib/server/MQTT.js';
import { Smarthouse } from './lib/server/Smarthouse.js';
import { Tibber } from './lib/server/Tibber.js';
import { VERSION } from './lib/server/version.js';
import { Weather } from './lib/server/Weather.js';
import {
	buildCurrentWeatherData,
	buildEnturData,
	buildHourlyForecastData,
	buildPowerData,
} from './viewdata.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const app = express();
const port = process.env.EXPOSE_PORT || 3000;

// Eta setup
const eta = new Eta({
	views: path.resolve(__dirname, '../views'),
	cache: true,
});

// Initialize server-side services
const weather = new Weather();
const calendar = Calendar.getInstance();
const days = new Days(calendar, weather);
const mqttClient = MqttClient.getInstance();
const smart = new Smarthouse(mqttClient);
smart.startMqtt();
const tibber = new Tibber();
const entur = new Entur();

// Serve static files
app.use(express.static(path.resolve(__dirname, '../static')));
app.use(express.static(path.resolve(__dirname, '../public')));

// Serve vendor JS from node_modules
app.get('/vendor/htmx.min.js', (_req, res) => {
	res.sendFile(
		path.resolve(__dirname, '../node_modules/htmx.org/dist/htmx.min.js'),
	);
});
app.get('/vendor/sse.js', (_req, res) => {
	res.sendFile(path.resolve(__dirname, '../node_modules/htmx-ext-sse/sse.js'));
});
app.get('/vendor/idiomorph-ext.min.js', (_req, res) => {
	res.sendFile(
		path.resolve(
			__dirname,
			'../node_modules/idiomorph/dist/idiomorph-ext.min.js',
		),
	);
});

// Render a partial to string
function renderPartial(name: string, data: Record<string, unknown>): string {
	return eta.render(`partials/${name}`, data);
}

// Main page
app.get('/', (_req, res) => {
	const homey = smart.getData();
	const hourly = weather.getHourlyForecasts();
	const comingDays = days.generateComingDays();

	const html = eta.render('layout', {
		version: VERSION,
		currentWeatherHtml: renderPartial(
			'currentWeather',
			buildCurrentWeatherData(homey),
		),
		hourlyForecastHtml: renderPartial(
			'hourlyForecast',
			buildHourlyForecastData(hourly),
		),
		calendarHtml: renderPartial('calendar', { days: comingDays }),
		powerHomeHtml: renderPartial(
			'power',
			buildPowerData(tibber.getPowerData(Places.Home), 'home'),
		),
		powerCabinHtml: renderPartial(
			'power',
			buildPowerData(tibber.getPowerData(Places.Cabin), 'cabin'),
		),
		enturHtml: renderPartial('entur', buildEnturData(entur)),
	});
	res.type('html').send(html);
});

// SSE endpoint
app.get('/sse', (req, res) => {
	res.writeHead(200, {
		'Content-Type': 'text/event-stream',
		'Cache-Control': 'no-cache',
		Connection: 'keep-alive',
	});
	res.flushHeaders();

	sendAllData(res);

	// Power updates every 1s - skip if unchanged
	const powerInterval = setInterval(() => {
		sendEventIfChanged(
			res,
			'power-home',
			renderPartial(
				'power',
				buildPowerData(tibber.getPowerData(Places.Home), 'home'),
			),
		);
		sendEventIfChanged(
			res,
			'power-cabin',
			renderPartial(
				'power',
				buildPowerData(tibber.getPowerData(Places.Cabin), 'cabin'),
			),
		);
	}, 1000);

	// General data updates every 15s - skip if unchanged
	const dataInterval = setInterval(() => {
		sendEventIfChanged(
			res,
			'current-weather',
			renderPartial('currentWeather', buildCurrentWeatherData(smart.getData())),
		);
		sendEventIfChanged(
			res,
			'hourly-forecast',
			renderPartial(
				'hourlyForecast',
				buildHourlyForecastData(weather.getHourlyForecasts()),
			),
		);
		sendEventIfChanged(
			res,
			'calendar',
			renderPartial('calendar', { days: days.generateComingDays() }),
		);
	}, 15000);

	// Entur updates every 30s - skip if unchanged
	const enturInterval = setInterval(() => {
		sendEventIfChanged(
			res,
			'entur',
			renderPartial('entur', buildEnturData(entur)),
		);
	}, 30000);

	// Version check every 60s
	const versionInterval = setInterval(() => {
		sendEvent(res, 'version', VERSION);
	}, 60000);

	req.on('close', () => {
		clearInterval(powerInterval);
		clearInterval(dataInterval);
		clearInterval(enturInterval);
		clearInterval(versionInterval);
	});
});

function sendEvent(res: express.Response, event: string, data: string) {
	const lines = data.replace(/\n/g, '\ndata: ');
	res.write(`event: ${event}\ndata: ${lines}\n\n`);
}

// Skip sending if data hasn't changed since last send
const lastSent: Record<string, string> = {};
function sendEventIfChanged(
	res: express.Response,
	event: string,
	data: string,
) {
	if (lastSent[event] === data) return;
	lastSent[event] = data;
	sendEvent(res, event, data);
}

function sendAllData(res: express.Response) {
	sendEvent(
		res,
		'current-weather',
		renderPartial('currentWeather', buildCurrentWeatherData(smart.getData())),
	);
	sendEvent(
		res,
		'hourly-forecast',
		renderPartial(
			'hourlyForecast',
			buildHourlyForecastData(weather.getHourlyForecasts()),
		),
	);
	sendEvent(
		res,
		'calendar',
		renderPartial('calendar', { days: days.generateComingDays() }),
	);
	sendEvent(
		res,
		'power-home',
		renderPartial(
			'power',
			buildPowerData(tibber.getPowerData(Places.Home), 'home'),
		),
	);
	sendEvent(
		res,
		'power-cabin',
		renderPartial(
			'power',
			buildPowerData(tibber.getPowerData(Places.Cabin), 'cabin'),
		),
	);
	sendEvent(res, 'entur', renderPartial('entur', buildEnturData(entur)));
}

app.listen(port, () => {
	console.log(`Tellulf running on port ${port}`);
});
