<script lang="js">
	import { Entur } from '$lib/Entur.mjs';
	import { onMount } from 'svelte';

	let entur;
	let trains = [];

	function updateTrains() {
		if (entur) {
			trains = entur.getTrains();
		}
	}

	onMount(() => {
		entur = new Entur();
		// Initial load
		// updateTrains();

		// Set up interval for updates
		const interval = setInterval(updateTrains, 5000);

		// Cleanup on component destroy
		return () => clearInterval(interval);
	});
</script>

<div class="footerBox">
	<strong>Neste baner</strong>
	{#each trains.slice(0, 4) as train}
		<div>
			<span style="width: 50px; display: inline-block;">{new Date(train.time).toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' })}</span> {train.destination}
		</div>
	{/each}
</div>