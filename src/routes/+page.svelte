<script lang="ts">
	import '../styles/tellulf.scss';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { Updater } from '$lib/client/updater';
	import Clock from '../components/Clock.svelte';
	import CurrentWeather from '../components/CurrentWeather.svelte';

	let updater;

	onMount(() => {
		if (browser) {
			updater = new Updater();
		}
	});

</script>

<Clock />
<CurrentWeather />