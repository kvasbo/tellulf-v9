<script lang="js">
	import { Entur } from '$lib/Entur.mjs';
	import { onMount } from 'svelte';

	let entur;
	$: trains = [];

	onMount(() => {
		entur = new Entur();
		setInterval(() => {
			trains = entur.getTrains();
		}, 5000);
	});

</script>

<div id="bane">
	<strong>Neste baner</strong>
	{#each trains.slice(0,4) as train}
		<div>
			{new Date(train.time).toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' })} {train.destination}
		</div>
	{/each}
</div>
