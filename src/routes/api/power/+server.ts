import { Tibber } from '$lib/server/Tibber';
import type { Places } from '$lib/Enums';

let tibber: Tibber | null = null;

function getTibberInstance() {
	if (!tibber) {
		tibber = new Tibber();
	}
	return tibber;
}

export async function GET({ request }): Promise<Response> {
	// Get params
	const params = new URL(request.url).searchParams;
	const tibberInstance = getTibberInstance();
	return new Response(JSON.stringify(tibberInstance.getPowerData(params.get('where') as Places)), {
		headers: {
			'Content-Type': 'application/json'
		}
	});
}
