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
		// Round to nearest five up and down
		min = Math.floor(min / 5) * 5;
		max = Math.ceil(max / 5) * 5;

		// Function to map a value from one range to another
		const mapToRange = (value) => {
			return ((value - min) / (max - min)) * 80;
		};

		return {
			min,
			max,
			mapToRange
		};
	}
	const { min, max, mapToRange } = calculateMinMax();
</script>

<nowcast>
	<div class="weather">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<span class="forecastMovablePart" style="margin-bottom: {20 + (mapToRange(forecast.instant.air_temperature) * 10)}%;">
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
