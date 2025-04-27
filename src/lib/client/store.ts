import { writable } from 'svelte/store';

type WeatherState = {
	temperature: number;
	humidity: number;
	pressure: number;
};

export type HourlyForecastStore = HourlyForecast[];

// TODO: THERE ARE MORE FIELDS IN THE FORECAST OBJECT
export type HourlyForecast = {
	hour: number;
	symbol: string;
	details: {
		probability_of_precipitation: number;
		precipitation_amount: number;
	};
	instant: {
		air_temperature: number;
	};
};

export const hourlyForecastStore = writable<HourlyForecastStore>([]);

export const weatherStore = writable<WeatherState>({
	temperature: 9999,
	humidity: 9999,
	pressure: 9999
});
