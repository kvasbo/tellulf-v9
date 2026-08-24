import { z } from 'zod';

const enturSchema = z.object({
	data: z.object({
		stopPlace: z.object({
			estimatedCalls: z.array(
				z.object({
					expectedDepartureTime: z.string(),
					destinationDisplay: z.object({ frontText: z.string() }),
					serviceJourney: z.object({ directionType: z.string() }),
				}),
			),
		}),
	}),
});

// Slemdal station, metro line 1 only. "inbound" = towards the city centre.
const query = `{
	stopPlace(id: "NSR:StopPlace:58268") {
		estimatedCalls(numberOfDepartures: 20, filters: [{select: [{lines: ["RUT:Line:1"]}]}]) {
			expectedDepartureTime
			destinationDisplay { frontText }
			serviceJourney { directionType }
		}
	}
}`;

interface Train {
	time: string;
	destination: string;
}

export class Entur {
	trains: Train[] = [];

	constructor() {
		setTimeout(() => this.Update(), 1000);
		setInterval(() => {
			this.Update();
		}, 60000);
	}

	getTrains(): Train[] {
		return this.trains;
	}

	async Update() {
		try {
			const response = await fetch(
				'https://api.entur.io/journey-planner/v3/graphql',
				{
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'ET-Client-Name': 'kvasbo-tellulf',
					},
					body: JSON.stringify({ query }),
				},
			);
			const valid = enturSchema.safeParse(await response.json());
			if (!valid.success) {
				console.error('Invalid data from Entur; keeping previous departures');
				return;
			}
			this.trains = valid.data.data.stopPlace.estimatedCalls
				.filter((call) => call.serviceJourney.directionType === 'inbound')
				.map((call) => ({
					time: call.expectedDepartureTime,
					destination: call.destinationDisplay.frontText,
				}))
				.sort(
					(a, b) => new Date(a.time).getTime() - new Date(b.time).getTime(),
				);

			console.log(`Entur updated with ${this.trains.length} trains`);
		} catch (error) {
			console.error(
				'Error updating Entur; keeping previous departures:',
				error,
			);
		}
	}
}
