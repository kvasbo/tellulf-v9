import { Tibber } from '$lib/server/Tibber';
import type { Places } from '$lib/server/Tibber';
const tibber = new Tibber();

export async function GET({ request }): Promise<Response> {
	// Get params
	const params = new URL(request.url).searchParams;
	return new Response(JSON.stringify(tibber.getPowerData(params.get('where') as Places)), {
		headers: {
			'Content-Type': 'application/json'
		}
	});
}
