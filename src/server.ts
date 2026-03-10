import path from 'node:path';
import { Eta } from 'eta';
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

const port = Number(process.env.EXPOSE_PORT) || 3000;
const projectRoot = path.resolve(import.meta.dir, '..');

// Eta setup
const eta = new Eta({
	views: path.resolve(projectRoot, 'views'),
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

// Vendor file mapping
const vendorFiles: Record<string, string> = {
	'/vendor/htmx.min.js': path.resolve(
		projectRoot,
		'node_modules/htmx.org/dist/htmx.min.js',
	),
	'/vendor/sse.js': path.resolve(
		projectRoot,
		'node_modules/htmx-ext-sse/sse.js',
	),
	'/vendor/idiomorph-ext.min.js': path.resolve(
		projectRoot,
		'node_modules/idiomorph/dist/idiomorph-ext.min.js',
	),
};

// Static directories to search (in order)
const staticDirs = [
	path.resolve(projectRoot, 'static'),
	path.resolve(projectRoot, 'public'),
];

// Render a partial to string
function renderPartial(name: string, data: Record<string, unknown>): string {
	return eta.render(`partials/${name}`, data);
}

// SSE helpers
const encoder = new TextEncoder();
const lastSent: Record<string, string> = {};

function formatSSE(event: string, data: string): Uint8Array {
	const lines = data.replace(/\n/g, '\ndata: ');
	return encoder.encode(`event: ${event}\ndata: ${lines}\n\n`);
}

// Main page handler
function handleIndex(): Response {
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
	return new Response(html, {
		headers: { 'Content-Type': 'text/html; charset=utf-8' },
	});
}

// SSE handler
function handleSSE(req: Request): Response {
	const stream = new ReadableStream({
		start(controller) {
			function sendEvent(event: string, data: string) {
				controller.enqueue(formatSSE(event, data));
			}

			function sendEventIfChanged(event: string, data: string) {
				if (lastSent[event] === data) return;
				lastSent[event] = data;
				sendEvent(event, data);
			}

			// Send initial data
			sendEvent(
				'current-weather',
				renderPartial(
					'currentWeather',
					buildCurrentWeatherData(smart.getData()),
				),
			);
			sendEvent(
				'hourly-forecast',
				renderPartial(
					'hourlyForecast',
					buildHourlyForecastData(weather.getHourlyForecasts()),
				),
			);
			sendEvent(
				'calendar',
				renderPartial('calendar', { days: days.generateComingDays() }),
			);
			sendEvent(
				'power-home',
				renderPartial(
					'power',
					buildPowerData(tibber.getPowerData(Places.Home), 'home'),
				),
			);
			sendEvent(
				'power-cabin',
				renderPartial(
					'power',
					buildPowerData(tibber.getPowerData(Places.Cabin), 'cabin'),
				),
			);
			sendEvent('entur', renderPartial('entur', buildEnturData(entur)));

			// Power updates every 1s - skip if unchanged
			const powerInterval = setInterval(() => {
				sendEventIfChanged(
					'power-home',
					renderPartial(
						'power',
						buildPowerData(tibber.getPowerData(Places.Home), 'home'),
					),
				);
				sendEventIfChanged(
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
					'current-weather',
					renderPartial(
						'currentWeather',
						buildCurrentWeatherData(smart.getData()),
					),
				);
				sendEventIfChanged(
					'hourly-forecast',
					renderPartial(
						'hourlyForecast',
						buildHourlyForecastData(weather.getHourlyForecasts()),
					),
				);
				sendEventIfChanged(
					'calendar',
					renderPartial('calendar', { days: days.generateComingDays() }),
				);
			}, 15000);

			// Entur updates every 30s - skip if unchanged
			const enturInterval = setInterval(() => {
				sendEventIfChanged(
					'entur',
					renderPartial('entur', buildEnturData(entur)),
				);
			}, 30000);

			// Version check every 60s
			const versionInterval = setInterval(() => {
				sendEvent('version', VERSION);
			}, 60000);

			// Cleanup on client disconnect
			req.signal.addEventListener('abort', () => {
				clearInterval(powerInterval);
				clearInterval(dataInterval);
				clearInterval(enturInterval);
				clearInterval(versionInterval);
				controller.close();
			});
		},
	});

	return new Response(stream, {
		headers: {
			'Content-Type': 'text/event-stream',
			'Cache-Control': 'no-cache',
			Connection: 'keep-alive',
		},
	});
}

// Start server
Bun.serve({
	port,
	async fetch(req) {
		const url = new URL(req.url);
		const pathname = url.pathname;

		// Main page
		if (pathname === '/') {
			return handleIndex();
		}

		// SSE endpoint
		if (pathname === '/sse') {
			return handleSSE(req);
		}

		// Vendor JS from node_modules
		if (vendorFiles[pathname]) {
			return new Response(Bun.file(vendorFiles[pathname]));
		}

		// Static files: try static/ then public/
		const safePath = path.normalize(pathname).replace(/^(\.\.[/\\])+/, '');
		for (const dir of staticDirs) {
			const filePath = path.join(dir, safePath);
			const file = Bun.file(filePath);
			if (await file.exists()) {
				return new Response(file);
			}
		}

		return new Response('Not Found', { status: 404 });
	},
});

console.log(`Tellulf running on port ${port}`);
