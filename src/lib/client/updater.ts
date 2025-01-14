import { weatherStore, powerStoreCabin, powerStoreHome, hourlyForecastStore, type HourlyForecastStore } from './store';

let version: string | null = null;

export class Updater {
	constructor() {
		console.log('Updater instantiated');
		setTimeout(this.update, 1000);
		setInterval(this.update, 15000);
	}

	async update() {

		// Fetch data from the server
		const response = await fetch("/api/calendar");
		const data = await response.json();

		// Version check
		if (data.version !== version && version !== null) {
			console.log('Version mismatch');
			location.reload();
		}

		// console.log(data);

		version = data.version;

		if (data.homey) {
			weatherStore.update((state) => {
				state.temperature = data.homey.tempOut;
				state.humidity = data.homey.humOut;
				state.pressure = data.homey.pressure;
				return state;
			});
			powerStoreCabin.update((state) => {
				state.price = data.powerPrice;
				state.power = data.homey.powerCabin;
				state.powerToday = data.homey.powerUsedTodayCabin;
				return state;
			});
			powerStoreHome.update((state) => {
				state.price = data.powerPrice;
				state.power = data.homey.power;
				state.powerToday = data.homey.powerUsedToday;
				return state;
			});
		}
		if (data.hourlyWeather) {
			hourlyForecastStore.update((state: HourlyForecastStore) => {
				state = data.hourlyWeather;
				return state;
			});
		}
	}
}