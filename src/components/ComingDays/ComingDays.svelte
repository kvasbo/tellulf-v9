<script lang="ts">
	import { onMount, afterUpdate } from 'svelte';
	import Day from './Day.svelte'; // Your Day component

	let days = [];
	let calendarEl: HTMLElement; // Reference to the <calendar> element
	let resizeObserver: ResizeObserver;

	// --- Visibility Check Logic ---
	function checkVisibility() {
		// Ensure the container element reference exists
		if (!calendarEl) return;

		const containerHeight = calendarEl.clientHeight;
		// Get all direct children of the <calendar> element.
		// Assumes these are the rendered <Day> components.
		const dayElements = Array.from(calendarEl.children).filter(
			(el): el is HTMLElement => el instanceof HTMLElement // Basic type guard
		);

		// Reset visibility for all items before measuring by removing the hidden class
		dayElements.forEach((el) => el.classList.remove('day-hidden'));

		let cumulativeHeight = 0;
		let firstHiddenIndex = -1; // Track the index of the first item to hide

		for (let i = 0; i < dayElements.length; i++) {
			const item = dayElements[i];
			// Ensure the element is actually rendered and has dimensions
			if (!item.offsetHeight) continue;

			const itemHeight = item.offsetHeight;

			// Check if adding this item would exceed the container height.
			// We ensure at least the first item (i=0) is shown, even if it overflows alone.
			if (cumulativeHeight + itemHeight > containerHeight && i > 0) {
				firstHiddenIndex = i; // Mark this one to be hidden
				break; // No need to check further items
			}

			// If it fits, add its height
			cumulativeHeight += itemHeight;
		}

		// Apply the 'day-hidden' class to the items that should be hidden
		dayElements.forEach((el, i) => {
			if (firstHiddenIndex !== -1 && i >= firstHiddenIndex) {
				el.classList.add('day-hidden');
			} else {
				// Ensure class is removed if it was previously hidden but now fits
				el.classList.remove('day-hidden');
			}
		});
	}
	// --- End Visibility Check Logic ---

	// Your existing function to fetch and update data
	async function updateCalendar() {
		console.log('Updating calendar');
		try {
			const data = await fetch('/api/data');
			if (!data.ok) {
				throw new Error(`HTTP error! status: ${data.status}`);
			}
			const json = await data.json();
			days = json.days || []; // Update the days array, Svelte will re-render
			// checkVisibility() will be called via afterUpdate hook
		} catch (error) {
			console.error('Failed to fetch calendar data:', error);
			days = []; // Clear days on error perhaps?
		}
	}

	onMount(() => {
		// Setup the ResizeObserver to call checkVisibility when container size changes
		resizeObserver = new ResizeObserver(() => {
			// Use requestAnimationFrame to avoid layout thrashing and check after paint
			requestAnimationFrame(checkVisibility);
		});

		if (calendarEl) {
			resizeObserver.observe(calendarEl);
		}

		// Initial data fetch and set interval
		updateCalendar();
		const interval = setInterval(updateCalendar, 15000);

		// Initial visibility check after first mount and render
		// Use requestAnimationFrame to make sure layout is calculated
		requestAnimationFrame(checkVisibility);

		// Cleanup function
		return () => {
			clearInterval(interval);
			// Disconnect observer when component is destroyed
			if (resizeObserver && calendarEl) {
				resizeObserver.unobserve(calendarEl);
			}
			if (resizeObserver) {
				resizeObserver.disconnect();
			}
		};
	});

	// Run checkVisibility after Svelte has updated the DOM
	// This handles cases where the 'days' array changes or content within Day updates height
	afterUpdate(() => {
		// Use requestAnimationFrame to ensure check runs after browser layout/paint
		requestAnimationFrame(checkVisibility);
	});
</script>

<calendar bind:this={calendarEl}>
	{#each days as day, i (day.id || i)}
		<Day {day} />
	{/each}
</calendar>
