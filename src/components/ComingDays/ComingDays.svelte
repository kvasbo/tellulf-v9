<script lang="ts">
	import { onMount } from 'svelte';
	import Day from './Day.svelte';

	let days = [];
	// For now this gets pretty much everything.
	async function updateCalendar() {
		console.log('Updating calendar');
		const data = await fetch('/api/data');
		const json = await data.json();
		days = json.days;
	}

	onMount(() => {
		updateCalendar();
		const interval = setInterval(updateCalendar, 15000);
		return () => clearInterval(interval);
	});
</script>

<calendar>
	{#each days as day}
		<Day {day} />
	{/each}
</calendar>
