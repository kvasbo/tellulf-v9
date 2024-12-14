export class Updater {
	constructor() {
		console.log('Updater instantiated');
		setInterval(this.update, 5000);
	}

	async update() {
		//console.log('Updating data');
		// Fetch data from the server
		const response = await fetch('/api/legacy');
		const data = await response.json();
		console.log(data);
	}
}