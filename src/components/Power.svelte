<script lang="ts">
	import type { PowerData } from '$lib/server/Tibber';
	import { onMount, onDestroy } from 'svelte';

	let powerData: PowerData | null = $state(null);

	let { where } = $props();

	let intervalId: NodeJS.Timeout; // Store the interval ID

	const maxPower: number = 30000 / 100;

	onMount(() => {
		// Start the interval
		intervalId = setInterval(async () => {
			const d = await fetch('/api/power?where=' + where);
			powerData = await d.json();
		}, 1000);
	});

	onDestroy(() => {
		// Clear the interval when the component is destroyed
		clearInterval(intervalId);
	});

	function getPowerDisplay(power: number): string {
		return `${(power / 1000).toFixed(1)}`;
	}

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

	function getBGColor(power: number) {
		return power < 0 ? '#3ef0a4' : '#3ea4f0';
	}

	function getHeader(where: string) {
		return where === 'home' ? 'Hjemme' : 'Hytta';
	}

	function getMonthlyStatus(): string {
		if (!powerData || powerData.monthlyConsumption === undefined || !powerData.cap) return '';
		let status = `${powerData.monthlyConsumption.toFixed(1)}/${powerData.cap}`;

		if (isNorgesprisActive()) {
			const now = new Date();
			const currentDay = now.getDate();
			const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
			const expectedUsage = (currentDay / daysInMonth) * powerData.cap;
			const difference = powerData.monthlyConsumption - expectedUsage;
			const percentDiff = (difference / expectedUsage) * 100;
			const sign = percentDiff > 0 ? '+' : '';
			status += ` (${sign}${percentDiff.toFixed(0)}%)`;
		}

		return status;
	}

	function isNorgesprisActive(): boolean {
		return Date.now() >= new Date('2025-10-01').getTime();
	}

	function getPriceStatus(): string {
		if (!powerData || powerData.monthlyConsumption === undefined || !powerData.cap) return '';
		if (!isNorgesprisActive()) return 'Spotpris';
		const isSubsidized = powerData.monthlyConsumption < powerData.cap;
		return isSubsidized ? 'Norgespris' : 'Spotpris';
	}

	function getPriceColor(): string {
		if (!powerData) return '#3ea4f0';
		if (!isNorgesprisActive()) return '#f0a43e';
		const isSubsidized = powerData.monthlyConsumption < powerData.cap;
		return isSubsidized ? '#3ef0a4' : '#f0a43e';
	}

	// If we have production, return that as min
	function getMinPower(): number {
		if (!powerData) {
			return 0;
		}
		let minPower = powerData.minPower;
		if (powerData.maxPowerProduction > 0) {
			// If we have production!
			minPower = Math.min(powerData.minPower, powerData.maxPowerProduction * -1);
		}
		return minPower;
	}
</script>

<div class="footerBox">
	{#if powerData}
		<strong>{getHeader(where)}</strong>
		<table class="footerTable">
			<tbody>
				<tr>
					<td>Forbruk</td>
					<td>Kost</td>
					<td>Pris</td>
				</tr>
				<tr>
					<td>{getUsage(powerData.accumulatedConsumption)}</td>
					<td>{powerData.accumulatedCost.toFixed(2)} kr</td>
					<td>{powerData.effectivePrice ? powerData.effectivePrice.toFixed(2) : powerData.currentPrice.toFixed(2)} kr</td>
				</tr>
				{#if powerData.monthlyConsumption !== undefined && powerData.cap}
				<tr class="monthlyStatus">
					<td colspan="2">{getMonthlyStatus()}</td>
					<td style="color: {getPriceColor()}">{getPriceStatus()}</td>
				</tr>
				{/if}
			</tbody>
		</table>
		<div class="powerShow">
			<div
				class="powerBar"
				style="background-color: {getBGColor(powerData.currentPower)}; width: {Math.abs(
					powerData.currentPower / maxPower
				)}%"
			>
				{getPowerDisplay(powerData.currentPower)}&nbsp;kW
			</div>
			<div class="maxBar" style="width: {powerData.maxPower / maxPower}%"></div>
			<div class="avgBar" style="width: {powerData.averagePower / maxPower}%"></div>
			<div
				class="minBar"
				style="background-color: {getBGColor(getMinPower())}; width: {Math.abs(
					getMinPower() / maxPower
				)}%"
			></div>
		</div>
	{/if}
</div>

<style>
	.powerShow {
		width: 100%;
		height: 50px;
		position: relative;
		display: flex;
		flex-direction: column;
		justify-content: flex-start;
		align-items: flex-start;
		margin-right: 25px;
		box-sizing: border-box;
	}
	.powerBar {
		height: 40px;
		background-color: #3ea4f0;
		transition: width 0.5s linear;
		display: flex;
		justify-content: flex-start;
		align-items: center;
		padding-left: 10px;
		margin-top: 10px;
		overflow: visible;
		break-inside: avoid;
		font-variant-numeric: tabular-nums;
	}
	.minBar {
		height: 5px;
		background-color: #3ea4f033;
		transition: width 0.5s linear;
	}
	.maxBar {
		height: 5px;
		background-color: #3ea4f055;
		transition: width 0.5s linear;
	}
	.avgBar {
		height: 5px;
		background-color: #3ea4f077;
		transition: width 0.5s linear;
	}
	.monthlyStatus {
		font-size: 0.85em;
		opacity: 0.7;
	}
	.monthlyStatus td {
		padding-top: 2px;
		white-space: nowrap;
	}
</style>
