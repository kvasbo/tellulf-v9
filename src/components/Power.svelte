<script lang="ts">
	import { powerStoreHome, powerStoreCabin } from '$lib/client/store';

	let { where } = $props();

	const powerHome = $derived($powerStoreHome.power);
	const todayHome = $derived($powerStoreHome.powerToday);
	const priceHome = $derived($powerStoreHome.price);
	const costHome = $derived($powerStoreHome.costToday);
	const powerCabin = $derived($powerStoreCabin.power);
	const todayCabin = $derived($powerStoreCabin.powerToday);
	const priceCabin = $derived($powerStoreCabin.price);
	const costCabin = $derived($powerStoreCabin.costToday);

	const currentPower = $derived(where === 'home' ? powerHome : powerCabin);
	const currentToday = $derived(where === 'home' ? todayHome : todayCabin);
	const currentPrice = $derived(where === 'home' ? priceHome : priceCabin);
	const currentCost = $derived(where === 'home' ? costHome : costCabin);
	const place = where === 'home' ? 'Hjemme' : 'Hytta';

	function getUsage() {
		try {
			// If it's november 30th, use joules instead of kwh
			if (new Date().getDate() === 30 && new Date().getMonth() === 10) {
				return `${Math.round(currentToday * 3.6)} MJ`;
			}
			return `${currentToday.toFixed(2)} kWh`;
		} catch (e) {
			console.error(e);
			return '0 kWh';
		}
	}
</script>

<div class="footerBox">
	<table class="footerTable">
		<tbody>
			<tr>
				<td colspan="2"><strong>{place}</strong></td>
			</tr>
			<tr>
				<td>I dag</td>
				<td>{getUsage()}</td>
			</tr>
			<tr>
				<td>Nå</td>
				<td>{Math.round(currentPower / 1000)} kW</td>
			</tr>
			<tr>
				<td>Pris</td>
				<td>{currentPrice.toFixed(2)} kr</td>
			</tr>
			<tr>
				<td>Kost</td>
				<td>{currentCost.toFixed(2)} kr</td>
			</tr>
		</tbody>
	</table>
</div>

<style>
	.powerLabel {
		width: 60px;
		display: inline-block;
	}
</style>
