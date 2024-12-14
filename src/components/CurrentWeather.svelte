<script lang="ts">
	import { getSunrise, getSunset } from 'sunrise-sunset-js';
	const jsDate = new Date();
	const sunRiseDate = getSunrise(59.9139, 10.7522, jsDate);
	const sunSetDate = getSunset(59.9139, 10.7522, jsDate);
	$: sunrise = sunRiseDate.toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' });
	$: sunset = sunSetDate.toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' });
</script>

<currentweather>
	<div id="current_temperature">&deg;</div>
	<div id="aux-weather-info">
		<div id="day_info_sun">
			<img alt="" src="/sunrise.png" width="22" />{sunrise}<img
				alt=""
				src="/sunset.png"
				width="22"
			/>{sunset}
		</div>
		<div id="current_pressure"></div>
		<div id="current_humidity"></div>
	</div>
</currentweather>
