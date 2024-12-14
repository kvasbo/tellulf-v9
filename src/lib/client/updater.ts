import { weatherStore, powerStoreCabin, powerStoreHome } from './store';

export class Updater {
	constructor() {
		console.log('Updater instantiated');
		setInterval(this.update, 5000);
	}

	async update() {
		//console.log('Updating data');
		// Fetch data from the server
		const response = await fetch("/api/calendar");
		const data = await response.json();
		console.log(data);
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

	}
}