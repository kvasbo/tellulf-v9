import { connect } from './live.js';
import { report } from './report.js';
import { initSky } from './sky.js';

report(
	`page: loaded ${window.innerWidth}x${window.innerHeight} ${document.visibilityState} ${navigator.userAgent}`,
);
window.addEventListener('error', (e) =>
	report(`error: ${e.message} (${e.filename}:${e.lineno})`),
);
window.addEventListener('unhandledrejection', (e) =>
	report(`error: ${e.reason}`),
);
document.addEventListener('visibilitychange', () =>
	report(`page: ${document.visibilityState}`),
);

// Living weather-reactive sky background
initSky();

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
function mustGet(id) {
	const el = document.getElementById(id);
	if (!el) throw new Error(`Missing element #${id}`);
	return el;
}
const elTime = mustGet('now_time');
const elDate = mustGet('now_date');
const elWeek = mustGet('now_week');

// Clock - update on the minute, then schedule next update
let lastDay = -1;

function updateClock() {
	const now = new Date();
	elTime.textContent = timeFmt.format(now);

	const day = now.getDate();
	if (day !== lastDay) {
		lastDay = day;
		elDate.textContent = dateFmt.format(now);
		elWeek.textContent = `uke ${Temporal.Now.plainDateISO().weekOfYear}`;
	}

	// Schedule next update at the start of the next minute
	const msUntilNextMinute =
		(60 - now.getSeconds()) * 1000 - now.getMilliseconds();
	setTimeout(updateClock, msUntilNextMinute);
}
updateClock();

// Reload page when clicking the clock
elTime.addEventListener('click', () => window.location.reload());

// Corner stamps: when this page loaded, and when data last arrived
const stampFmt = new Intl.DateTimeFormat('nb-NO', {
	hour: '2-digit',
	minute: '2-digit',
	second: '2-digit',
	timeZone: 'Europe/Oslo',
});
const loadedAt = stampFmt.format(new Date());
let dataAt = '–';
// The calendar swap replaces these spans, so look them up every time.
function fillStamps() {
	mustGet('stamp-loaded').textContent = loadedAt;
	mustGet('stamp-data').textContent = dataAt;
}
fillStamps();

// Calendar visibility - hide days that overflow
function checkCalendarVisibility() {
	const calendarEl = document.querySelector('tl-calendar');
	if (!calendarEl) return;

	const containerHeight = calendarEl.clientHeight;
	const dayElements = calendarEl.querySelectorAll('tl-day');

	let cumulativeHeight = 0;

	for (const el of dayElements) {
		cumulativeHeight += el.offsetHeight;
		el.classList.toggle('day-hidden', cumulativeHeight > containerHeight);
	}
}

// Every fragment counts as fresh data; a new calendar may overflow.
document.addEventListener('fragment', (e) => {
	dataAt = stampFmt.format(new Date());
	fillStamps();
	if (e.detail === 'calendar') {
		requestAnimationFrame(checkCalendarVisibility);
	}
});

// The server sends its version after each snapshot and every minute; a new
// one means it restarted, maybe with new code, so reload.
connect('/sse', {
	version: (v) => {
		if (v !== document.body.dataset.version) window.location.reload();
	},
});

// Run visibility check on resize
const calendarEl = document.querySelector('tl-calendar');
if (calendarEl) {
	new ResizeObserver(() =>
		requestAnimationFrame(checkCalendarVisibility),
	).observe(calendarEl);
}

// Initial visibility check
requestAnimationFrame(checkCalendarVisibility);

// Reload page once per day (midnight) - robust against NTP clock adjustments
const lastDayForReload = new Date().getDate();
setInterval(() => {
	if (new Date().getDate() !== lastDayForReload) {
		window.location.reload();
	}
}, 30000);
