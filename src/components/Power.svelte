<script lang="ts">
	import type { PowerData } from '$lib/server/Tibber';
	import { onMount, onDestroy } from 'svelte';

	let powerData: PowerData | null = $state(null);

	let { where } = $props();

	let intervalId: number; // Store the interval ID

	onMount(() => {
		// Start the interval
		intervalId = setInterval(async () => {
			const d = await fetch('/api/power?where=' + where);
			const data = await d.json();
			powerData = data;
		}, 1000);
	});

	onDestroy(() => {
		// Clear the interval when the component is destroyed
		clearInterval(intervalId);
	});

	function getUsage(used: number): string {
		try {
			// If it's november 30th, use joules instead of kWh
			if (new Date().getDate() === 30 && new Date().getMonth() === 10) {
				return `${Math.round(used * 3.6)} MJ`;
			}
			return `${used.toFixed(2)} kWh`;
		} catch (e) {
			console.error(e);
			return '0 kWh';
		}
	}
</script>

<div class="footerBox">
	{#if powerData}
		<table class="footerTable">
			<tbody>
				<tr>
					<td colspan="2"><strong>{where}</strong></td>
				</tr>
				<tr>
					<td>I dag</td>
					<td>{getUsage(powerData.accumulatedConsumption)}</td>
				</tr>
				<tr>
					<td>Nå</td>
					<td>{Math.round(powerData.currentPower / 1000)} kW</td>
				</tr>
				<tr>
					<td>Pris</td>
					<td>{powerData.currentPrice.toFixed(2)} kr</td>
				</tr>
				<tr>
					<td>Kost</td>
					<td>{powerData.accumulatedCost.toFixed(2)} kr</td>
				</tr>
			</tbody>
		</table>
	{/if}
</div>
