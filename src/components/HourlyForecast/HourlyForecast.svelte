<script lang="ts">
	import { hourlyForecastStore } from '$lib/client/store';
	import { weatherIconMapping } from '$lib/client/weatherSymbolMapping';

	$: hourlyForecastStore;

	function calculateMinMax() {
		let min = 100;
		let max = -100;
		$hourlyForecastStore.slice(1,19).forEach((forecast) => {
			if (forecast.instant.air_temperature < min) {
				min = forecast.instant.air_temperature;
			}
			if (forecast.instant.air_temperature > max) {
				max = forecast.instant.air_temperature;
			}
		});

		console.log('pre', min, max);

		// Fix the ranges
		if (min > 0) {
			min = 0;
			max = Math.max(max, 20);
		} else if (max < 0) {
			max = 0;
			min = Math.min(min, -25);
		}


		console.log('post', min, max);

		// Make sure the range touches 0
		// min = Math.min(min, 0);
		//max = Math.max(max, 0);

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

	function getWeatherIcon(symbol: string) {
		if (!weatherIconMapping[symbol]) {
			return '';
		}
		return `/weather-icons-${isItStatic(symbol)}/${weatherIconMapping[symbol]}.svg`;
	}

	function isItStatic(symbol: string) {
		return ['clearsky_night', 'partly-cloudy-night'].includes(symbol) ? 'static' : 'animated';
	}
</script>

<nowcast>
	<!-- Define filter for icons -->
	<svg width="0" height="0">
		<defs>
			<filter id="shape-shadow">
				<feDropShadow dx="0" dy="0" stdDeviation="4"
											flood-color="#333333" flood-opacity="0.2"/>
			</filter>
		</defs>
	</svg>
	<div class="weather">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<span
					class="forecastMovablePart"
					style="margin-bottom: {10 + mapToRange(forecast.instant.air_temperature) * 7}%;"
				>
					<img class="weather_icon" alt="symbol" src={getWeatherIcon(forecast.symbol)} style="filter: url(#shape-shadow);"/>
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
