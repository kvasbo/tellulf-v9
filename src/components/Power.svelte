<script lang="ts">
	import { powerStoreHome, powerStoreCabin } from '$lib/client/store';

	let { where } = $props();

	const powerHome = $derived($powerStoreHome.power);
	const todayHome = $derived($powerStoreHome.powerToday);
	const priceHome = $derived($powerStoreHome.price);
	const powerCabin = $derived($powerStoreCabin.power);
	const todayCabin = $derived($powerStoreCabin.powerToday);
	const priceCabin = $derived($powerStoreCabin.price);

	const currentPower = $derived(where === 'home' ? powerHome : powerCabin);
	const currentToday = $derived(where === 'home' ? todayHome : todayCabin);
	const currentPrice = $derived(where === 'home' ? priceHome : priceCabin);
	const place = where === 'home' ? 'Hjemme' : 'Hytta';

</script>

<style>
	.powerLabel {
		width: 60px;
			display: inline-block;
	}
</style>

<div class="footerBox">
	<div>
		<strong>{place}</strong><br />
		<span class="powerLabel">I dag</span>{Math.round(currentToday)} kWh<br />
		<span class="powerLabel">Nå</span>{Math.round(currentPower / 1000)} kW<br />
		<span class="powerLabel">Pris</span>{currentPrice.toFixed(2)} kr
	</div>
</div>