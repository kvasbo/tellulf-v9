import { writable } from 'svelte/store';

export type WeatherState = {
	temperature: number;
	humidity: number;
	pressure: number;
};

export const weatherStore = writable<WeatherState>({
	temperature: 9999,
	humidity: 9999,
	pressure: 9999
});