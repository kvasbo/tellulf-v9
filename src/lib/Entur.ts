import { XMLParser } from 'fast-xml-parser';
import { z } from 'zod';

const enturSchema = z.object({
	Siri: z.object({
		ServiceDelivery: z.object({
			EstimatedTimetableDelivery: z.object({
				EstimatedJourneyVersionFrame: z.object({
					EstimatedVehicleJourney: z.array(
						z.object({
							EstimatedCalls: z
								.object({
									EstimatedCall: z.union([
										z.array(
											z.object({
												StopPointName: z.string().optional(),
												ExpectedDepartureTime: z.string().optional(),
												DestinationDisplay: z.string()
											})
										),
										z.object({
											StopPointName: z.string().optional(),
											ExpectedDepartureTime: z.string().optional(),
											DestinationDisplay: z.string()
										})
									])
								})
								.optional(),
							LineRef: z.string(),
							DirectionRef: z.number()
						})
					)
				})
			})
		})
	})
});

interface Train {
	time: string;
	destination: string;
}

export class Entur {
	trains: Train[] = [];

	constructor() {
		setTimeout(() => this.Update(), 5000);
		setInterval(() => {
			this.Update();
		}, 60000);
	}

	getTrains(): Train[] {
		return this.trains;
	}

	async Update() {
		try {
			const parser = new XMLParser();
			const xmlData = await fetch('https://api.entur.io/realtime/v1/rest/et?datasetId=RUT');
			const text = await xmlData.text();
			const parsed = parser.parse(text);
			const valid = enturSchema.safeParse(parsed);
			if (!valid.success) {
				console.error('Invalid data from Entur');
				this.trains = [];
				return;
			}
			const trips =
				valid.data.Siri.ServiceDelivery.EstimatedTimetableDelivery.EstimatedJourneyVersionFrame
					.EstimatedVehicleJourney;
			const filteredTrips = trips.filter((trip) => {
				// Check if the trip has any EstimatedCalls
				if (
					!trip.EstimatedCalls ||
					!trip.EstimatedCalls.EstimatedCall ||
					!Array.isArray(trip.EstimatedCalls.EstimatedCall) ||
					trip.EstimatedCalls.EstimatedCall.length === 0
				) {
					return false;
				}
				// Check if the trip is on the correct line and direction
				if (trip.LineRef !== 'RUT:Line:1' || trip.DirectionRef !== 1) {
					return false;
				}
				if (
					!Array.isArray(trip.EstimatedCalls.EstimatedCall) ||
					trip.EstimatedCalls.EstimatedCall.filter((call) => call.StopPointName === 'Slemdal')
						.length === 0
				) {
					return false;
				}
				return true;
			});
			const filteredTrains = filteredTrips.map((trip): { time: string; destination: string } => {
				// @ts-expect-error - We have already checked that this is an array
				const found = trip.EstimatedCalls?.EstimatedCall.find(
					(stop: { StopPointName: string }) => stop.StopPointName === 'Slemdal'
				);
				return {
					time: found.ExpectedDepartureTime,
					destination: found.DestinationDisplay
				};
			});

			this.trains = filteredTrains.sort((a, b) => {
				return new Date(a.time).getTime() - new Date(b.time).getTime();
			});

			console.log(`Entur updated with ${this.trains.length} trains`);
		} catch (error) {
			console.error('Error: ', error);
			this.trains = [];
		}
	}
}
