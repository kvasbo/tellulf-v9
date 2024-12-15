<script lang="ts">
	import { hourlyForecastStore } from '$lib/client/store';

	$: hourlyForecastStore;

	function calculateMinMax() {
		let min = 100;
		let max = -100;
		$hourlyForecastStore.forEach((forecast) => {
			if (forecast.instant.air_temperature < min) {
				min = forecast.instant.air_temperature;
			}
			if (forecast.instant.air_temperature > max) {
				max = forecast.instant.air_temperature;
			}
		});

		// Function to map a value from one range to another
		const mapToRange = (value: number) => {
			return ((value - min) / (max - min)) * 80;
		};

		return {
			min,
			max,
			mapToRange
		};
	}

	let min: number, max: number, mapToRange: Function;

	$: if ($hourlyForecastStore) {
		const temp = calculateMinMax();
		min = temp.min;
		max = temp.max;
		mapToRange = temp.mapToRange;
	}
</script>

<nowcast>
	<div class="weather">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<span
					class="forecastMovablePart"
					style="margin-bottom: {15 + mapToRange(forecast.instant.air_temperature) * 7}%;"
				>
					<img class="weather_icon" alt="symbol" src="weathericon/png/{forecast.symbol}.png" />
					<div class="temperature">
						{forecast.instant.air_temperature}°
					</div>
					<div class="forecastProb">
						{#if forecast.details.probability_of_precipitation != 0}
							{forecast.details.probability_of_precipitation}%
							<br />
							{forecast.details.precipitation_amount}mm
						{:else}
							&nbsp;<br />
							&nbsp;
						{/if}
					</div>
				</span>
				<div class="time">
					{forecast.hour}
				</div>
			</forecast>
		{/each}
	</div>
</nowcast>
