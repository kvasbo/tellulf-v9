import { XMLParser } from 'fast-xml-parser';

// TODO: Zod or similar for type checking

export class Entur {
	trains = [];

	constructor() {
		setTimeout(() => this.Update(), 5000);
		setInterval(() => {
			this.Update();
		}, 60000);
	}

	getTrains() {
		return this.trains;
	}

	async Update() {
		try {
			const parser = new XMLParser();
			const xmlData = await fetch('https://api.entur.io/realtime/v1/rest/et?datasetId=RUT');
			const text = await xmlData.text();
			const parsed = parser.parse(text);
			const trips =
				parsed.Siri.ServiceDelivery.EstimatedTimetableDelivery.EstimatedJourneyVersionFrame
					.EstimatedVehicleJourney;
			const filteredTrips = trips.filter(
				(trip: {
					EstimatedCalls: {
						EstimatedCall: {
							length: number;
							filter: (arg0: (call: unknown) => boolean) => {
								(): unknown;
								new (): unknown;
								length: number;
							};
						};
					};
					LineRef: string;
					DirectionRef: number;
				}) => {
					if (
						!trip.EstimatedCalls?.EstimatedCall ||
						trip.EstimatedCalls.EstimatedCall.length === 0
					) {
						return false;
					}
					if (trip.LineRef !== 'RUT:Line:1' || trip.DirectionRef !== 1) {
						return false;
					}
					// @ts-expect-error - This is a hack to get around the fact that the type is unknown
					if (
						!Array.isArray(trip.EstimatedCalls.EstimatedCall) ||
						trip.EstimatedCalls.EstimatedCall.filter((call) => call.StopPointName === 'Slemdal')
							.length === 0
					) {
						return false;
					}
					return true;
				}
			);
			const filteredTrains = filteredTrips.map(
				(trip: { EstimatedCalls: { EstimatedCall: any[] } }) => {
					const found = trip.EstimatedCalls.EstimatedCall.find(
						(stop) => stop.StopPointName === 'Slemdal'
					);
					return {
						time: found.ExpectedDepartureTime,
						destination: found.DestinationDisplay
					};
				}
			);

			this.trains = filteredTrains.sort((a: { time: number }, b: { time: number }) => {
				return new Date(a.time).getTime() - new Date(b.time).getTime();
			});

			console.log(`Entur updated with ${this.trains.length} trains`);
		} catch (error) {
			console.error('Error: ', error);
			this.trains = [];
		}
	}
}
