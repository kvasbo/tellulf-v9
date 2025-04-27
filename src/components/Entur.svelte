<script lang="ts">
	import { Entur } from '$lib/Entur.js';
	import FooterTable from './Footer/FooterTable.svelte';
	import { onMount } from 'svelte';

	let entur: Entur;
	let tableData: string[][] = [];

	function updateTrains() {
		if (entur) {
			let trains = entur.getTrains();
			tableData = trains.map((train) => [
				new Date(train.time).toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' }),
				train.destination
			]);
		}
	}

	onMount(() => {
		entur = new Entur();
		// Initial load
		updateTrains();

		// Set up interval for updates
		const interval = setInterval(updateTrains, 5000);

		// Cleanup on component destroy
		return () => clearInterval(interval);
	});
</script>

<FooterTable header="Neste baner" {tableData} />
