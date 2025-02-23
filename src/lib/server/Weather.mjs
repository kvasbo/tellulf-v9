import { DateTime, Settings } from 'luxon';
import { YrCompleteResponseSchema, LongTermForecastSchema } from './types.met.mjs';

const YR_URL_FORECAST =
	'https://api.met.no/weatherapi/locationforecast/2.0/complete?lat=59.9508&lon=10.6848';
const YR_URL_FORECAST_LONG =
	'https://api.met.no/weatherapi/subseasonal/1.0/complete?lat=59.9508&lon=10.6848';
const YR_URL_DANGER =
	'https://api.met.no/weatherapi/metalerts/2.0/current.json?lat=59.9508&lon=10.6848';

// Configure the time zone
Settings.defaultZone = 'Europe/Oslo';

/**
 * Weather data from Yr
 */
export class Weather {
	forecast = [];
	longTermForecast = [];
	dangerData = [];

	// Common options for fetch
	fetchOptions = {
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

	updateForecasts() {
		// Fetch forecast
		this.fetchForecastData();
		this.fetchLongTermForecast();
		this.fetchDanger();
	}

	getCurrentWeather() {
		const out = {
			temperature: 999,
			symbol: 'blank'
		};

		if (this.forecast[0]) {
			out.temperature = this.forecast[0]?.data?.instant?.details?.air_temperature
				? this.forecast[0]?.data.instant.details.air_temperature
				: 999;
			out.symbol = this.forecast[0]?.data?.next_1_hours?.summary?.symbol_code
				? this.forecast[0].data.next_1_hours.summary.symbol_code
				: 'blank';
		}

		return out;
	}

	/**
	 * Get the daily forecasts
	 * @returns DailyForecasts
	 */
	getDailyForecasts() {
		const dayForecasts = {};
		for (const series of this.longTermForecast) {
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
	getDangerData() {
		return this.dangerData;
	}

	/**
	 * Get the hourly forecasts
	 * @returns HourlyForecast[]
	 */
	getHourlyForecasts() {
		const out = [];

		this.forecast.forEach((series) => {
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
	async fetchDanger() {
		const data = await fetch(YR_URL_DANGER, this.fetchOptions);
		const danger = await data.json();
		this.dangerData = danger.features.map((feature) => {
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
	async fetchLongTermForecast() {
		const forecast = await fetch(YR_URL_FORECAST_LONG, this.fetchOptions);

		if (forecast.ok) {
			const forecastJson = await forecast.json();
			const forecastValidated = LongTermForecastSchema.safeParse(forecastJson);
			if (forecastValidated.success) {
				console.log("Long term forecast validated, let's go!");
				console.log(
					'Number of long term days',
					forecastValidated.data.properties.timeseries.length
				);
				this.longTermForecast = forecastValidated.data.properties.timeseries;
			} else {
				console.log('Could not validate long term forecast');
			}
		}
	}

	/**
	 * Do the fetching from Met api
	 * @returns
	 */
	async fetchForecastData() {
		try {
			// Fetch and decode JSON
			console.log('Fetching forecast from yr.no', YR_URL_FORECAST);
			const fetchResponse = await fetch(YR_URL_FORECAST, this.fetchOptions);

			if (!fetchResponse.ok) {
				// Log the fetch error message
				console.log(fetchResponse.statusText);
				console.error('Could not fetch forecast from yr.no');
				setTimeout(() => {
					this.fetchForecastData();
				}, 1000 * 10);
				return;
			}

			const forecast = await fetchResponse.json();

			// Validate the response
			const forecastValidated = YrCompleteResponseSchema.safeParse(forecast);

			if (forecastValidated.success) {
				console.log("Forecast validated, let's go!");
				console.log('Number of forecasts', forecastValidated.data.properties.timeseries.length);
				this.forecast = forecastValidated.data.properties.timeseries;
			} else {
				console.log('Could not validate forecast');
				console.log(forecastValidated);
				setTimeout(() => {
					this.fetchForecastData();
				}, 1000 * 10);
			}
		} catch (error) {
			console.error(error);
			setTimeout(() => {
				this.fetchForecastData();
			}, 1000 * 10);
		}
	}
}
