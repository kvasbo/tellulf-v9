<script lang="ts">
	import '../styles/tellulf.scss';
	import { Updater } from '$lib/client/updater';
	import { onMount } from 'svelte';
	import Clock from '../components/Clock.svelte';
	import CurrentWeather from '../components/CurrentWeather.svelte';
	import Entur from '../components/Entur.svelte';
	import ComingDays from '../components/ComingDays/ComingDays.svelte';
	import Power from '../components/Power.svelte';

	let updater;
	onMount(() => {
		updater = new Updater();
		const interval = setInterval(() => updater.update(), 15000);
		return () => clearInterval(interval);
	});
</script>

<Clock />
<CurrentWeather />
<ComingDays />

<footer>
	<Power where="home" />
	<Power where="cabin" />
	<Entur />
</footer>
