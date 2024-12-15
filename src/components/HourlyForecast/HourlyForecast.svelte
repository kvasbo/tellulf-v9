<script lang="ts">
	import { hourlyForecastStore } from '$lib/client/store';
</script>

<nowcast>
	<div class="weather">
		{#each $hourlyForecastStore.slice(1, 19) as forecast}
			<forecast>
				<div class="time">
					{forecast.hour}
				</div>
				<div class="weather_symbol">
					<img class="weather_icon" alt="symbol" src="weathericon/png/{forecast.symbol}.png" />
				</div>
				<div class="temperature">
					{forecast.instant.air_temperature}°
				</div>
				<div class="forecast_prob">
					<div class="forecast_prob">
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
			</forecast>
		{/each}
	</div>
</nowcast>
