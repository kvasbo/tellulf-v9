import {
	weatherStore,
	hourlyForecastStore,
	type HourlyForecastStore
} from './store';

let version: string | null = null;

export class Updater {
	constructor() {
		console.log('Updater instantiated');
		setTimeout(this.update, 1000);
		setInterval(this.update, 15000);
	}

	async update() {
		// Fetch data from the server
		const response = await fetch('/api/data');
		const data = await response.json();

		// Version check
		if (data.version !== version && version !== null) {
			console.log('Version mismatch');
			location.reload();
		}

		version = data.version;

		if (data.homey) {
			weatherStore.update((state) => {
				state.temperature = data.homey.tempOut;
        state.lastTempTime = data.homey.lastTempTime;
				state.humidity = data.homey.humOut;
				state.pressure = data.homey.pressure;
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
