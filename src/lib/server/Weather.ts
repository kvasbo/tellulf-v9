import { DateTime, Settings } from 'luxon';
import {
	YrCompleteResponseSchema,
	LongTermForecastSchema,
	type TimeSeries,
	type LongTermForecastDay
} from './types.met';

interface Sted {
	lat: number;
	lon: number;
}

type StedNavn = 'oslo' | 'hytta';

type Steder = {
	[key in StedNavn]: Sted;
};

const steder: Steder = {
	oslo: {
		lat: 59.9508,
		lon: 10.6848
	},
	hytta: {
		lat: 59.1347,
		lon: 10.3246
	}
};

// Configure the time zone
Settings.defaultZone = 'Europe/Oslo';

// Helper functions to build URLs
function buildForecastUrl(sted: Sted): string {
	return `https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=${sted.lat}&lon=${sted.lon}`;
}

function buildLongTermForecastUrl(sted: Sted): string {
	return `https://api.met.no/weatherapi/subseasonal/1.0/complete?lat=${sted.lat}&lon=${sted.lon}`;
}

function buildDangerUrl(sted: Sted): string {
	return `https://api.met.no/weatherapi/metalerts/2.0/current.json?lat=${sted.lat}&lon=${sted.lon}`;
}

interface CurrentWeather {
	temperature: number;
	symbol: string;
}

interface DailyForecast {
	minTemp: number;
	maxTemp: number;
	meanTemp: number;
	lightRainProbability: number;
	heavyRainProbability: number;
	symbol: string;
}

interface HourlyForecast {
	symbol: string | undefined;
	details: {
		precipitation_amount: number;
		precipitation_amount_max: number;
		precipitation_amount_min: number;
		probability_of_precipitation: number;
		probability_of_thunder: number;
	};
	instant: {
		air_pressure_at_sea_level?: number;
		air_temperature?: number;
		air_temperature_percentile_10?: number;
		air_temperature_percentile_90?: number;
		cloud_area_fraction?: number;
		cloud_area_fraction_high?: number;
		cloud_area_fraction_low?: number;
		cloud_area_fraction_medium?: number;
		dew_point_temperature?: number;
		fog_area_fraction?: number;
		relative_humidity?: number;
		ultraviolet_index_clear_sky?: number;
		wind_from_direction?: number;
		wind_speed?: number;
		wind_speed_of_gust?: number;
		wind_speed_percentile_10?: number;
		wind_speed_percentile_90?: number;
	};
	hour: string;
}

interface DangerData {
	response: string;
	severity: string;
	description: string;
	headline: string;
	instruction: string;
}

type LocationData = {
	forecast: TimeSeries[];
	longTermForecast: LongTermForecastDay[];
	dangerData: DangerData[];
};

/**
 * Weather data from Yr
 */
export class Weather {
	private weatherData: Record<StedNavn, LocationData> = {
		oslo: {
			forecast: [],
			longTermForecast: [],
			dangerData: []
		},
		hytta: {
			forecast: [],
			longTermForecast: [],
			dangerData: []
		}
	};

	// Common options for fetch
	private readonly fetchOptions: RequestInit = {
		method: 'GET',
		headers: {
			'User-Agent': 'tellulf v6: audun@kvasbo.no'
		}
	};

	constructor() {
		setTimeout(() => {
			this.updateForecasts();
		}, 1000);
		setInterval(
			() => {
				this.updateForecasts();
			},
			30 * 60 * 1000
		); // Every 30 minutes
	}

	private updateForecasts(): void {
		// Fetch forecast for all locations
		const locations: StedNavn[] = ['oslo', 'hytta'];
		locations.forEach(location => {
			this.fetchForecastData(location);
			this.fetchLongTermForecast(location);
			this.fetchDanger(location);
		});
	}

	getCurrentWeather(location: StedNavn = 'oslo'): CurrentWeather {
		const out: CurrentWeather = {
			temperature: 999,
			symbol: 'blank'
		};

		const forecast = this.weatherData[location].forecast;
		if (forecast[0]) {
			out.temperature = forecast[0]?.data?.instant?.details?.air_temperature ?? 999;
			out.symbol = forecast[0]?.data?.next_1_hours?.summary?.symbol_code ?? 'blank';
		}

		return out;
	}

	/**
	 * Get the daily forecasts
	 * @returns DailyForecasts
	 */
	getDailyForecasts(location: StedNavn = 'oslo'): Record<string, DailyForecast> {
		const dayForecasts: Record<string, DailyForecast> = {};
		const longTermForecast = this.weatherData[location].longTermForecast;
		for (const series of longTermForecast) {
			const time = new Date(series.time);
			const date = time.toISOString().slice(0, 10);
			let symbol = 'blank';
			// Define the symbol based on the data for precipitation
			if (series.data.next_24_hours.details.probability_of_heavy_precipitation > 50) {
				symbol =
					series.data.next_24_hours.details.probability_of_frost > 50 ? 'heavysnow' : 'heavyrain';
			} else if (series.data.next_24_hours.details.probability_of_precipitation > 50) {
				symbol = series.data.next_24_hours.details.probability_of_frost > 50 ? 'snow' : 'rain';
			}

			dayForecasts[date] = {
				minTemp: series.data.next_24_hours.details.air_temperature_min,
				maxTemp: series.data.next_24_hours.details.air_temperature_max,
				meanTemp: series.data.next_24_hours.details.air_temperature_mean,
				lightRainProbability:
					Math.round(series.data.next_24_hours.details.probability_of_precipitation / 10) * 10,
				heavyRainProbability:
					Math.round(series.data.next_24_hours.details.probability_of_heavy_precipitation / 10) *
					10,
				symbol
			};
		}

		return dayForecasts;
	}

	/**
	 * Return the danger data
	 */
	getDangerData(location: StedNavn = 'oslo'): DangerData[] {
		return this.weatherData[location].dangerData;
	}

	/**
	 * Get the hourly forecasts
	 * @returns HourlyForecast[]
	 */
	getHourlyForecasts(location: StedNavn = 'oslo'): HourlyForecast[] {
		const out: HourlyForecast[] = [];
		const forecast = this.weatherData[location].forecast;

		forecast.forEach((series) => {
			if (series.data?.next_1_hours?.details) {
				const dt = DateTime.fromISO(series.time);

				out.push({
					symbol: series.data.next_1_hours.summary?.symbol_code,
					details: series.data.next_1_hours.details,
					instant: series.data.instant.details,
					hour: dt.hour.toString()
				});
			}
		});
		return out;
	}

	/**
	 * Fetch the dangerous weather report.
	 * @returns
	 */
	private async fetchDanger(location: StedNavn): Promise<void> {
		const url = buildDangerUrl(steder[location]);
		const data = await fetch(url, this.fetchOptions);
		const danger = await data.json();
		this.weatherData[location].dangerData = danger.features.map((feature: any) => {
			return {
				response: feature.properties.awarenessResponse,
				severity: feature.properties.severity,
				description: feature.properties.description,
				headline: feature.properties.title,
				instruction: feature.properties.instruction
			};
		});
	}

	/**
	 * Fetch the long term forecast
	 * @returns LongTermForecast
	 */
	private async fetchLongTermForecast(location: StedNavn): Promise<void> {
		const url = buildLongTermForecastUrl(steder[location]);
		const forecast = await fetch(url, this.fetchOptions);

		if (forecast.ok) {
			const forecastJson = await forecast.json();
			const forecastValidated = LongTermForecastSchema.safeParse(forecastJson);
			if (forecastValidated.success) {
				console.log(`Long term forecast for ${location} validated, let's go!`);
				console.log(
					`Number of long term days for ${location}`,
					forecastValidated.data.properties.timeseries.length
				);
				this.weatherData[location].longTermForecast = forecastValidated.data.properties.timeseries;
			} else {
				console.log(`Could not validate long term forecast for ${location}`);
			}
		}
	}

	/**
	 * Do the fetching from Met api
	 * @returns
	 */
	private async fetchForecastData(location: StedNavn): Promise<void> {
		try {
			const url = buildForecastUrl(steder[location]);
			// Fetch and decode JSON
			console.log(`Fetching forecast for ${location} from yr.no`, url);
			const fetchResponse = await fetch(url, this.fetchOptions);

			if (!fetchResponse.ok) {
				// Log the fetch error message
				console.log(fetchResponse.statusText);
				console.error(`Could not fetch forecast for ${location} from yr.no`);
				setTimeout(() => {
					this.fetchForecastData(location);
				}, 1000 * 10);
				return;
			}

			const forecast = await fetchResponse.json();

			// Validate the response
			const forecastValidated = YrCompleteResponseSchema.safeParse(forecast);

			if (forecastValidated.success) {
				console.log(`Forecast for ${location} validated, let's go!`);
				console.log(`Number of forecasts for ${location}`, forecastValidated.data.properties.timeseries.length);
				this.weatherData[location].forecast = forecastValidated.data.properties.timeseries;
			} else {
				console.log(`Could not validate forecast for ${location}`);
				console.log(forecastValidated);
				setTimeout(() => {
					this.fetchForecastData(location);
				}, 1000 * 10);
			}
		} catch (error) {
			console.error(error);
			setTimeout(() => {
				this.fetchForecastData(location);
			}, 1000 * 10);
		}
	}
}
