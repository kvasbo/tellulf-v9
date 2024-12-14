import { Calendar } from '$lib/server/Calendar.mjs';
import { Entur } from '$lib/server/Entur.mjs';

const calendar = new Calendar();
const entur = new Entur();

export async function GET(): Promise<Response> {
	try {
		const out = { success: true };
		return new Response(JSON.stringify(out), {
			headers: {
				'Content-Type': 'application/json'
			}
		});
	} catch (error: unknown) {
		const err = error as Error;
		console.error(err.message);
		return new Response(
			JSON.stringify({
				success: false,
				error: err.message
			}),
			{
				status: 500,
				headers: {
					'Content-Type': 'application/json'
				}
			}
		);
	}
}