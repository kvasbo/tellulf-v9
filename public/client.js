import { connect } from './live.js';
import { report } from './report.js';

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

// Pre-create Intl formatters (expensive to construct, cheap to call)
const timeFmt = new Intl.DateTimeFormat('nb-NO', {
	hour: 'numeric',
	minute: '2-digit',
});
const dateFmt = new Intl.DateTimeFormat('nb-NO', {
	weekday: 'long',
	day: 'numeric',
	month: 'long',
});

// Cache DOM refs. Every page has the time and date; only the overview has
// the week. The server renders all three, so they are right before this runs.
function mustGet(id) {
	const el = document.getElementById(id);
	if (!el) throw new Error(`Missing element #${id}`);
	return el;
}
const elTime = mustGet('now_time');
const elDate = mustGet('now_date');
const elWeek = document.getElementById('now_week');

// Clock - update on the minute, then schedule next update
let lastDay = -1;

function updateClock() {
	const now = new Date();
	elTime.textContent = timeFmt.format(now);

	const day = now.getDate();
	if (day !== lastDay) {
		lastDay = day;
		// "mandag 26. oktober" → "Mandag 26. oktober"
		const date = dateFmt.format(now);
		elDate.textContent = date.charAt(0).toUpperCase() + date.slice(1);
		if (elWeek) {
			elWeek.textContent = `uke ${Temporal.Now.plainDateISO().weekOfYear}`;
		}
	}

	// Schedule next update at the start of the next minute
	const msUntilNextMinute =
		(60 - now.getSeconds()) * 1000 - now.getMilliseconds();
	setTimeout(updateClock, msUntilNextMinute);
}
updateClock();

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

// Calendar overflow: hide the coming days that don't fit, from the bottom.
// Today and the corner line have their own space and are never hidden.
function checkCalendarVisibility() {
	const days = document.querySelector('.calendar .days');
	if (!days) return;

	// clientHeight includes the padding; the days stack from the content edge.
	const limit =
		days.clientHeight - Number.parseFloat(getComputedStyle(days).paddingTop);
	let cumulativeHeight = 0;
	for (const el of days.querySelectorAll('.day')) {
		cumulativeHeight += el.offsetHeight;
		el.classList.toggle('day-hidden', cumulativeHeight > limit);
	}
}

// Every fragment counts as fresh data; a new calendar may overflow. The
// check runs right away, not in requestAnimationFrame: a hidden page (the
// display asleep over the midnight reload) never gets an animation frame,
// and would show an overflowing calendar until the next calendar change.
document.addEventListener('fragment', (e) => {
	dataAt = stampFmt.format(new Date());
	fillStamps();
	if (e.detail === 'calendar') {
		checkCalendarVisibility();
	}
});

// The mode is the path of the page the screens should show, set over MQTT
// and back to the overview after data-mode-timeout seconds (0: for good).
// A page follows every change of it. The first value a page gets is the
// mode as it stands: the overview goes there at once, and so does a page
// that already is the mode's. Any other page was opened directly (or left
// there by a restart): it stays, and goes home after the same timeout
// unless the mode changes first.
const modeTimeoutMs = Number(document.body.dataset.modeTimeout) * 1000;
let lastMode = null;
let homeTimer;

function go(path) {
	report(`mode: ${window.location.pathname} → ${path}`);
	window.location.assign(path);
}

function onMode(path) {
	if (path === lastMode) return; // a reconnect's snapshot, nothing new
	const first = lastMode === null;
	lastMode = path;
	const here = window.location.pathname;
	if (path === here) {
		clearTimeout(homeTimer); // the mode's page now: the server says when to leave
		return;
	}
	if (first && here !== '/') {
		report(`mode: ${here} opened directly, the mode is ${path}`);
		if (modeTimeoutMs > 0) homeTimer = setTimeout(() => go('/'), modeTimeoutMs);
		return;
	}
	go(path);
}

// The server sends its version after each snapshot and every minute; a new
// one means it restarted, maybe with new code, so reload. Going to another
// page is a same-origin navigation, which styles.css turns into a page
// transition.
connect('/sse', {
	version: (v) => {
		if (v !== document.body.dataset.version) window.location.reload();
	},
	mode: onMode,
});

// Run the visibility check on resize. The aside persists across calendar
// swaps; its children don't.
const calendarEl = document.querySelector('.calendar');
if (calendarEl) {
	new ResizeObserver(() =>
		requestAnimationFrame(checkCalendarVisibility),
	).observe(calendarEl);
}

// Initial visibility check, and again once the web font has replaced the
// fallback: the appointment lines wrap, so the font changes how tall each
// day is, and an uncached font arrives after this script has run.
checkCalendarVisibility();
document.fonts.ready.then(checkCalendarVisibility);

// Reload page once per day (midnight) - robust against NTP clock adjustments
const lastDayForReload = new Date().getDate();
setInterval(() => {
	if (new Date().getDate() !== lastDayForReload) {
		window.location.reload();
	}
}, 30000);
