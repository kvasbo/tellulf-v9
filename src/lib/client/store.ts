import { writable } from 'svelte/store';

type WeatherState = {
	temperature: number;
	humidity: number;
	pressure: number;
};

type PowerState = {
	price: number;
	power: number;
	powerToday: number;
}

export const weatherStore = writable<WeatherState>({
	temperature: 9999,
	humidity: 9999,
	pressure: 9999
});

export const powerStoreHome = writable<PowerState>({
	price: 9999,
	power: 9999,
	powerToday: 9999
});

export const powerStoreCabin = writable<PowerState>({
	price: 9999,
	power: 9999,
	powerToday: 9999
});
