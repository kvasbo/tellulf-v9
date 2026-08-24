import { afterEach, describe, expect, test } from 'bun:test';
import { Entur } from './Entur.js';

const realFetch = globalThis.fetch;

function mockFetch(body: unknown) {
	globalThis.fetch = (async () =>
		new Response(JSON.stringify(body), {
			headers: { 'Content-Type': 'application/json' },
		})) as unknown as typeof fetch;
}

function call(
	time: string,
	destination: string,
	directionType: 'inbound' | 'outbound',
) {
	return {
		expectedDepartureTime: time,
		destinationDisplay: { frontText: destination },
		serviceJourney: { directionType },
	};
}

const graphqlResponse = {
	data: {
		stopPlace: {
			estimatedCalls: [
				call('2026-08-24T11:47:00+02:00', 'Bergkrystallen', 'inbound'),
				call('2026-08-24T11:37:00+02:00', 'Frognerseteren', 'outbound'),
				call('2026-08-24T11:32:00+02:00', 'Bergkrystallen', 'inbound'),
			],
		},
	},
};

afterEach(() => {
	globalThis.fetch = realFetch;
});

describe('Entur.Update', () => {
	test('keeps only inbound departures, sorted by time', async () => {
		mockFetch(graphqlResponse);
		const entur = new Entur();
		await entur.Update();
		expect(entur.getTrains()).toEqual([
			{ time: '2026-08-24T11:32:00+02:00', destination: 'Bergkrystallen' },
			{ time: '2026-08-24T11:47:00+02:00', destination: 'Bergkrystallen' },
		]);
	});

	test('keeps previous departures on invalid response', async () => {
		mockFetch(graphqlResponse);
		const entur = new Entur();
		await entur.Update();
		mockFetch({ errors: [{ message: 'boom' }] });
		await entur.Update();
		expect(entur.getTrains()).toHaveLength(2);
	});
});
