// Pre-create Intl formatters (expensive to construct, cheap to call)
const timeFmt = new Intl.DateTimeFormat('nb-NO', {
	hour: 'numeric',
	minute: '2-digit',
});
const dateFmt = new Intl.DateTimeFormat('nb-NO', {
	weekday: 'long',
	day: 'numeric',
});

// Cache DOM refs
const elTime = document.getElementById('now_time')!;
const elDate = document.getElementById('now_date')!;
const elWeek = document.getElementById('now_week')!;

// Clock - update on the minute, then schedule next update
let lastDay = -1;

function updateClock() {
	const now = new Date();
	elTime.textContent = timeFmt.format(now);

	const day = now.getDate();
	if (day !== lastDay) {
		lastDay = day;
		elDate.textContent = dateFmt.format(now);
		elWeek.textContent = `Uke ${Temporal.Now.plainDateISO().weekOfYear}`;
	}

	// Schedule next update at the start of the next minute
	const msUntilNextMinute =
		(60 - now.getSeconds()) * 1000 - now.getMilliseconds();
	setTimeout(updateClock, msUntilNextMinute);
}
updateClock();

// Reload page when clicking the clock
elTime.addEventListener('click', () => window.location.reload());

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

// Reload page once per day (midnight) - robust against NTP clock adjustments
let lastDayForReload = new Date().getDate();
setInterval(() => {
	if (new Date().getDate() !== lastDayForReload) {
		window.location.reload();
	}
}, 30000);
