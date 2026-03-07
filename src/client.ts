// Clock
function updateClock() {
	const now = new Date();
	document.getElementById('now_time')!.textContent = now.toLocaleTimeString(
		'nb-NO',
		{ hour: 'numeric', minute: '2-digit' },
	);
	document.getElementById('now_date')!.textContent = now.toLocaleDateString(
		'nb-NO',
		{ weekday: 'long', day: 'numeric' },
	);
	const week = Temporal.Now.plainDateISO().weekOfYear;
	document.getElementById('now_week')!.textContent = `Uke ${week}`;
}

updateClock();
setInterval(updateClock, 1000);

// Reload page when clicking the clock
document
	.getElementById('now_time')!
	.addEventListener('click', () => window.location.reload());

// Calendar visibility - hide days that overflow
function checkCalendarVisibility() {
	const calendarEl = document.querySelector('calendar');
	if (!calendarEl) return;

	const containerHeight = calendarEl.clientHeight;
	const dayElements = Array.from(
		calendarEl.querySelectorAll('day'),
	) as HTMLElement[];

	let cumulativeHeight = 0;

	for (const el of dayElements) {
		cumulativeHeight += el.offsetHeight;
		el.classList.toggle('day-hidden', cumulativeHeight > containerHeight);
	}
}

// Handle SSE messages from HTMX
const initialVersion =
	document.getElementById('server-version')?.dataset?.version;

document.body.addEventListener('htmx:sseMessage', ((evt: CustomEvent) => {
	if (evt.detail.type === 'calendar') {
		requestAnimationFrame(checkCalendarVisibility);
	}
	if (evt.detail.type === 'version') {
		const el = document.getElementById('server-version');
		if (el && initialVersion && el.dataset.version !== initialVersion) {
			window.location.reload();
		}
	}
}) as EventListener);

// Run visibility check on resize
const calendarEl = document.querySelector('calendar');
if (calendarEl) {
	new ResizeObserver(() =>
		requestAnimationFrame(checkCalendarVisibility),
	).observe(calendarEl);
}

// Initial visibility check
requestAnimationFrame(checkCalendarVisibility);

// Reload page on the hour
const now = new Date();
const startOfNextHour = new Date();
startOfNextHour.setUTCHours(now.getUTCHours() + 1, 0, 1, 0);
setTimeout(
	() => window.location.reload(),
	startOfNextHour.getTime() - now.getTime(),
);
