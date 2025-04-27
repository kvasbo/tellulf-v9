import type { HourlyForecast } from '$lib/client/store';

export function calculateMinMaxTemps(data: HourlyForecast[]): { min: number; max: number } {
	let min = 100;
	let max = -100;

	data.forEach((forecast) => {
		if (forecast.instant.air_temperature < min) {
			min = forecast.instant.air_temperature;
		}
		if (forecast.instant.air_temperature > max) {
			max = forecast.instant.air_temperature;
		}
		console.log(min, max);
	});

	max = Math.max(Math.ceil((max + 5) / 5) * 5, 0);
	min = Math.min(Math.floor((min - 5) / 5) * 5, 0);

	if (max - min < 20) {
		max = min + 20;
	}

	return {
		min,
		max
	};
}
