interface HourlyForecastInput {
	instant: { air_temperature?: number };
}

export function calculateMinMaxTemps(data: HourlyForecastInput[]): {
	min: number;
	max: number;
} {
	let min = 100;
	let max = -100;

	data.forEach((forecast) => {
		const temp = forecast.instant.air_temperature ?? 0;
		if (temp < min) {
			min = temp;
		}
		if (temp > max) {
			max = temp;
		}
	});

	max = Math.max(Math.ceil((max + 5) / 5) * 5, 0);
	min = Math.min(Math.floor((min - 5) / 5) * 5, 0);

	if (max - min < 20) {
		max = min + 20;
	}

	return {
		min,
		max,
	};
}
