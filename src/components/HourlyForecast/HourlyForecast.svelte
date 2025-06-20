<script lang="ts">
	import { hourlyForecastStore } from '$lib/client/store';
	import { weatherIconMapping } from '$lib/client/weatherSymbolMapping';
	import { calculateMinMaxTemps } from '$lib/weatherCalculations';

	$: hourlyForecastStore;

	const waves = ['wave1.svg', 'wave2.svg', 'wave3.svg'];

	// Function to get the height of the rain bar
	// 3mm of rain will be 50% of the height
	function getRainHeight(rain: number) {
		return Math.min(100, rain * 17);
	}

	function calculateMinMax() {
		const forecasts = $hourlyForecastStore.slice(1, 19);
		const { min, max } = calculateMinMaxTemps(forecasts);

		const displayZeroLine = max > 0 && min < 0 ? 'block' : 'none';

		// Function to map a value from one range to another
		const mapToRange = (value: number) => {
			return ((value - min) / (max - min)) * 100;
		};

		const getBackground = () => {
			if (max <= 0) {
				return `linear-gradient(180deg, #e3f2fd00	0%, #e3f2fdFF 100%)`;
			} else if (min >= 0) {
				// If the temperature is above 0, use a red gradient
				return `linear-gradient(180deg, #ff8a8022 0%, #ff8a8000 100%)`;
			} else {
				// If the temperature is between -10 and 10, use a white gradient
				return `linear-gradient(180deg, #ff8a8022 0%, #e3f2fd99 100%)`;
			}
		};

		return {
			min,
			max,
			displayZeroLine,
			mapToRange,
			getBackground
		};
	}

	let min: number, max: number, displayZeroLine: string, mapToRange: Function, background: Function;

	$: if ($hourlyForecastStore) {
		const temp = calculateMinMax();
		min = temp.min;
		max = temp.max;
		displayZeroLine = temp.displayZeroLine;
		mapToRange = temp.mapToRange;
		background = temp.getBackground;
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
	<div class="weather" style="background: {background()}">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<div class="zeroLine" style="bottom: {mapToRange(0)}%; display: {displayZeroLine};"></div>
				<div
					class="rain"
					style="height: {getRainHeight(forecast.details.precipitation_amount)}%;"
				></div>
				{#if false && forecast.details.precipitation_amount > 0}
					<img
						alt="rain"
						class="rainWave"
						src={waves[Math.floor(Math.random() * waves.length)]}
						style="bottom: {getRainHeight(forecast.details.precipitation_amount)}%;"
					/>
				{/if}
				<div
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
				</div>
				<div class="time">
					{forecast.hour}
				</div>
			</forecast>
		{/each}
		<div class="minTemp">{min}</div>
		<div class="maxTemp">{max}</div>
	</div>
</nowcast>

<style>
	.rain {
		background-color: #3ea4f0;
		position: absolute;
		bottom: 0;
		color: white;
		display: flex;
		width: 100%;
	}
	.rainWave {
		position: absolute;
		width: 100%;
		display: block;
	}
	.maxTemp,
	.minTemp {
		width: calc(100% / 18);
		font-size: 30px;
		opacity: 0.1;
		position: absolute;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.maxTemp {
		top: 10px;
		left: 0;
	}
	.minTemp {
		bottom: 50px;
		left: 0;
	}
</style>
