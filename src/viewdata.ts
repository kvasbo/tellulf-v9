import { getSunrise, getSunset } from 'sunrise-sunset-js';
import type { Entur } from './lib/Entur.js';
import type { Smarthouse } from './lib/server/Smarthouse.js';
import type { PowerData } from './lib/server/Tibber.js';
import type { HourlyForecast } from './lib/server/Weather.js';
import { calculateMinMaxTemps } from './lib/weatherCalculations.js';
import { weatherIconMapping } from './lib/weatherSymbolMapping.js';

// --- Current Weather ---

// Cache sunrise/sunset per day (only changes daily, involves trig calculations)
const sunTimeFmt = new Intl.DateTimeFormat('nb-NO', {
	hour: '2-digit',
	minute: '2-digit',
});
let sunCache: { date: string; rise: string; set: string } | null = null;

function getCachedSunTimes(): { rise: string; set: string } {
	const today = new Date().toDateString();
	if (!sunCache || sunCache.date !== today) {
		const d = new Date();
		const rise = getSunrise(59.9508, 10.6847, d);
		const set = getSunset(59.9508, 10.6847, d);
		sunCache = {
			date: today,
			rise: rise ? sunTimeFmt.format(rise) : '',
			set: set ? sunTimeFmt.format(set) : '',
		};
	}
	return sunCache;
}

export function buildCurrentWeatherData(
	homey: ReturnType<Smarthouse['getData']>,
) {
	const sun = getCachedSunTimes();
	const tempTime = homey.lastTempTime;

	return {
		temperature: homey.tempOut,
		sunrise: sun.rise,
		sunset: sun.set,
		pressure: Math.round(homey.pressure),
		humidity: Math.round(homey.humOut),
		showTempTime: tempTime > 0 && Date.now() - tempTime > 20 * 60 * 1000,
		tempTimeStr: tempTime > 0 ? new Date(tempTime).toLocaleTimeString() : '',
	};
}

// --- Hourly Forecast ---

function getWeatherIcon(symbol: string): string {
	if (!weatherIconMapping[symbol]) return '';
	const folder = ['clearsky_night', 'partly-cloudy-night'].includes(symbol)
		? 'static'
		: 'animated';
	return `/weather-icons-${folder}/${weatherIconMapping[symbol]}.svg`;
}

function getRainHeight(rain: number): number {
	return Math.min(100, rain * 17);
}

export function buildHourlyForecastData(hourlyForecasts: HourlyForecast[]) {
	const forecasts = hourlyForecasts.slice(1, 19);
	if (forecasts.length === 0) {
		return {
			forecasts: [],
			min: 0,
			max: 0,
			background: '',
			displayZeroLine: 'none',
			mapToRange: () => 0,
			getRainHeight,
		};
	}

	const { min, max } = calculateMinMaxTemps(forecasts);
	const mapToRange = (value: number) => ((value - min) / (max - min)) * 100;
	const displayZeroLine = max > 0 && min < 0 ? 'block' : 'none';

	let background: string;
	if (max <= 0) {
		background = 'linear-gradient(180deg, #e3f2fd00 0%, #e3f2fdFF 100%)';
	} else if (min >= 0) {
		background = 'linear-gradient(180deg, #ff8a8022 0%, #ff8a8000 100%)';
	} else {
		background = 'linear-gradient(180deg, #ff8a8022 0%, #e3f2fd99 100%)';
	}

	// Add icon src to each forecast
	const enrichedForecasts = forecasts.map((f) => ({
		...f,
		iconSrc: getWeatherIcon(f.symbol ?? ''),
	}));

	return {
		forecasts: enrichedForecasts,
		min,
		max,
		background,
		displayZeroLine,
		mapToRange,
		getRainHeight,
	};
}

// --- Power ---

// Norgespris activated 2025-10-01, permanently true now — no need to recompute
const NORGESPRIS_ACTIVE = Date.now() >= new Date('2025-10-01').getTime();

function getUsage(used: number): string {
	if (new Date().getDate() === 30 && new Date().getMonth() === 10) {
		return `${Math.round(used * 3.6)} MJ`;
	}
	return `${used.toFixed(2)} kWh`;
}

function getMonthlyStatus(powerData: PowerData): string {
	if (powerData.monthlyConsumption === undefined) return '';
	let status = `${powerData.monthlyConsumption.toFixed(1)} kWh`;

	if (NORGESPRIS_ACTIVE && powerData.cap) {
		const now = new Date();
		const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
		const monthEnd = new Date(now.getFullYear(), now.getMonth() + 1, 1);
		const fractionOfMonth =
			(now.getTime() - monthStart.getTime()) /
			(monthEnd.getTime() - monthStart.getTime());
		const expectedUsage = Math.max(fractionOfMonth * powerData.cap, 1);
		const percentDiff =
			((powerData.monthlyConsumption - expectedUsage) / expectedUsage) * 100;
		const sign = percentDiff > 0 ? '+' : '';
		status += ` (${sign}${percentDiff.toFixed(0)}%)`;
	}
	return status;
}

function getMonthlyCost(powerData: PowerData): string {
	if (powerData.monthlyCost === undefined) return '';
	return `${powerData.monthlyCost.toFixed(0)} kr`;
}

function getPriceStatus(powerData: PowerData): string {
	if (powerData.monthlyConsumption === undefined) return '';
	if (!NORGESPRIS_ACTIVE || !powerData.cap) return 'Spotpris';
	return powerData.monthlyConsumption < powerData.cap
		? 'Norgespris'
		: 'Spotpris';
}

function getPriceColor(powerData: PowerData): string {
	if (!NORGESPRIS_ACTIVE || !powerData.cap) return '#f0a43e';
	return powerData.monthlyConsumption < powerData.cap ? '#3ef0a4' : '#f0a43e';
}

function getMinPower(powerData: PowerData): number {
	let minPower = powerData.minPower;
	if (powerData.maxPowerProduction > 0) {
		minPower = Math.min(powerData.minPower, powerData.maxPowerProduction * -1);
	}
	return minPower;
}

export function buildPowerData(powerData: PowerData, where: string) {
	const maxPower = 30000 / 100;
	const minP = getMinPower(powerData);

	return {
		powerData,
		header: where === 'home' ? 'Hjemme' : 'Hytta',
		usage: getUsage(powerData.accumulatedConsumption),
		monthlyStatus: getMonthlyStatus(powerData),
		monthlyCost: getMonthlyCost(powerData),
		priceStatus: getPriceStatus(powerData),
		priceColor: getPriceColor(powerData),
		bgColor: powerData.currentPower < 0 ? '#3ef0a4' : '#3ea4f0',
		powerDisplay: (powerData.currentPower / 1000).toFixed(1),
		powerWidth: Math.abs(powerData.currentPower / maxPower),
		maxWidth: powerData.maxPower / maxPower,
		avgWidth: powerData.averagePower / maxPower,
		minWidth: Math.abs(minP / maxPower),
		minBgColor: minP < 0 ? '#3ef0a4' : '#3ea4f033',
	};
}

// --- Entur ---

export function buildEnturData(entur: Entur) {
	const trains = entur
		.getTrains()
		.slice(0, 3)
		.map((train) => ({
			time: new Date(train.time).toLocaleTimeString('nb-NO', {
				hour: '2-digit',
				minute: '2-digit',
			}),
			destination: train.destination,
		}));
	return { trains };
}
