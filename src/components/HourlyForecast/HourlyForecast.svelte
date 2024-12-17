<script lang="ts">
	import { hourlyForecastStore } from '$lib/client/store';
	import { weatherIconMapping } from '$lib/client/weatherSymbolMapping';

	$: hourlyForecastStore;

	// Function to get the height of the rain bar
	// 3mm of rain will be 50% of the height
	function getRainHeight(rain: number) {
		return Math.min(100, rain * 17);
	}

	function calculateMinMax() {
		let min = 100;
		let max = -100;
		let displayZeroLine = 'none';
		$hourlyForecastStore.slice(1, 19).forEach((forecast) => {
			if (forecast.instant.air_temperature < min) {
				min = forecast.instant.air_temperature;
			}
			if (forecast.instant.air_temperature > max) {
				max = forecast.instant.air_temperature;
			}
		});

		// Fix the ranges
		if (min > 0 && max > 10) {
			min = 0;
			max = Math.max(max, 25);
		} else if (min > -10 && max <= 10) {
			max = Math.max(max, 10);
			min = Math.min(min, -10);
			displayZeroLine = 'block';
		} else if (max < 0 && min < -10) {
			max = 0;
			min = Math.min(min, -25);
		}

		// Function to map a value from one range to another
		const mapToRange = (value: number) => {
			return ((value - min) / (max - min)) * 100;
		};

		return {
			min,
			max,
			displayZeroLine,
			mapToRange
		};
	}

	let min: number, max: number, displayZeroLine: string, mapToRange: Function;

	$: if ($hourlyForecastStore) {
		const temp = calculateMinMax();
		min = temp.min;
		max = temp.max;
		displayZeroLine = temp.displayZeroLine;
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
				<feDropShadow dx="0" dy="0" stdDeviation="4" flood-color="#333333" flood-opacity="0.2" />
			</filter>
		</defs>
	</svg>
	<div class="weather">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<div class="zeroLine" style="bottom: {mapToRange(0)}%; display: {displayZeroLine};"></div>
				<div
					class="rain"
					style="height: {getRainHeight(forecast.details.precipitation_amount)}%;"
				></div>
				<img
					class="rainWave"
					src="/wave.svg"
					style="display: {forecast.details.precipitation_amount > 0
						? 'block'
						: 'none'}; bottom: {getRainHeight(forecast.details.precipitation_amount)}%;"
				/>
				<span
					class="forecastMovablePart"
					style="bottom: {mapToRange(forecast.instant.air_temperature)}%;"
				>
					<img
						class="weather_icon"
						alt="symbol"
						src={getWeatherIcon(forecast.symbol)}
						style="filter: url(#shape-shadow);"
					/>
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

<style>
	.rainWave {
		position: absolute;
		width: 100%;
		display: none;
	}
</style>
