<script lang="ts">
	let { day } = $props();
</script>

<!-- Define filter for icons -->
<svg width="0" height="0">
	<defs>
		<filter id="shape-shadow">
			<feDropShadow dx="0" dy="0" stdDeviation="4" flood-color="black" flood-opacity="0.4" />
		</filter>
	</defs>
</svg>

<day>
	<div class="dateHeader">
		{day.weekday}
		<div class="headerWeather">
			{#if day.daily_forecast}
				<div class="headerWeatherPrepArea">
					{#if day.daily_forecast.lightRainProbability > 10}
						<div class="weatherPrep light">
							<img
								alt="light rain"
								class="headerWeatherIconPrep light"
								style="filter: url(#shape-shadow);"
								src="weather-icons-animated/rain.svg"
							/>
							<span class="rainProb">{day.daily_forecast.lightRainProbability}%</span>
						</div>
					{/if}
					{#if day.daily_forecast.heavyRainProbability > 10}
						<div class="weatherPrep heavy">
							<img
								alt="heavy rain"
								class="headerWeatherIconPrep heavy"
								style="filter: url(#shape-shadow);"
								src="weather-icons-animated/extreme-rain.svg"
							/>
							<span class="rainProb">{day.daily_forecast.heavyRainProbability}%</span>
						</div>
					{/if}
				</div>
				<img alt="cold" class="headerWeatherTempIcon" src="low-temperature.png" />
				{Math.round(day.daily_forecast.minTemp)}&deg;
				<img alt="warm" class="headerWeatherTempIcon" src="high-temperature.png" />
				{Math.round(day.daily_forecast.maxTemp)}&deg;
			{/if}
		</div>
	</div>
	<div class="dinner">
		{#each day.dinner as dinner}
			<div class="dinner">
				<img alt="dinner" class="calendaricon birthday" src="dinner.svg" />{dinner.displayTitle}
			</div>
		{/each}
	</div>
	<div class="birthdays">
		{#each day.birthdays as birthday}
			<div class="birthday">
				<img alt="cake" class="calendaricon birthday" src="birthday.svg" />{birthday.displayTitle}
			</div>
		{/each}
	</div>
	<div class="events">
		{#each day.events as event}
			{#if event.fullDay === true}
				<div class="event">
					{event.displayTitle}
					{#if event.hyttaWeather}
						<span class="hytta-weather">
							({event.hyttaWeather.temperature}°, {event.hyttaWeather.rainProbability}% regn)
						</span>
					{/if}
				</div>
			{:else}
				<div class="event">
					<span class="event-time timespan">
						{event.displayTime.start}
						{event.displayTime.spacer}
						{event.displayTime.end}
					</span>
					{event.displayTitle}
				</div>
			{/if}
		{/each}
	</div>
</day>
