<script lang="ts">
	import '../styles/tellulf.scss';
	import Clock from '../components/Clock.svelte';
	import CurrentWeather from '../components/CurrentWeather.svelte';
	import Entur from '../components/Entur.svelte';
	import ComingDays from '../components/ComingDays/ComingDays.svelte';
</script>

<Clock />
<CurrentWeather />
<ComingDays />

<footer>
	<Entur />
</footer>
