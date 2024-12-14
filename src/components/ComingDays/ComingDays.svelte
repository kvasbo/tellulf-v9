<script lang="ts">
	import { onMount } from 'svelte';
	import Day from './Day.svelte';

	let days = [];
	async function updateCalendar() {
		console.log('Updating calendar');
		const data = await fetch("/api/calendar");
		days = await data.json();
	}

	onMount(() => {
		updateCalendar();
		const interval = setInterval(updateCalendar, 15000);
		return () => clearInterval(interval);
	});

</script>

<calendar>
	{#each days as day}
		<Day day={day} />
	{/each}
</calendar>
