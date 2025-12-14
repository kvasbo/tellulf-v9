<script lang="ts">
	import { getSunrise, getSunset } from 'sunrise-sunset-js';
	import { weatherStore } from '$lib/client/store';
	const jsDate = new Date();
	const sunRiseDate = getSunrise(59.9508, 10.6847, jsDate);
	const sunSetDate = getSunset(59.9508, 10.6847, jsDate);
	$: sunrise = sunRiseDate?.toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' });
	$: sunset = sunSetDate?.toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' });
	$: temperature = $weatherStore.temperature;
	$: humidity = Math.round($weatherStore.humidity);
	$: pressure = Math.round($weatherStore.pressure);
  $: tempTime = $weatherStore.lastTempTime;
	$: showTempTime = tempTime > 0 && Date.now() - tempTime > 20 * 60 * 1000;
</script>

<currentweather>
	<div id="current_temperature">{temperature}&deg;</div>
	<div id="aux-weather-info">
		<div id="day_info_sun">
			<img alt="" src="/sunrise.png" width="22" />&nbsp;{sunrise}&nbsp;&nbsp;<img
				alt=""
				src="/sunset.png"
				width="22"
			/>&nbsp;{sunset}
		</div>
		<div id="current_pressure">{pressure} hPa</div>
		<div id="current_humidity">{humidity}% hum</div>
		{#if showTempTime}
			<div id="current_temperature_time">{new Date(tempTime).toLocaleTimeString()}</div>
		{/if}
	</div>
</currentweather>

<style>
  #current_temperature_time {
    color: rgba(255, 0, 0, 0.35)
  }
</style>
