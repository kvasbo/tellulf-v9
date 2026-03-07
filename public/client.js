// Clock - updates every second
function getDateWeek(date) {
	const currentDate = typeof date === 'object' ? date : new Date();
	const januaryFirst = new Date(currentDate.getFullYear(), 0, 1);
	const daysToNextMonday = januaryFirst.getDay() === 1 ? 0 : (7 - januaryFirst.getDay()) % 7;
	const nextMonday = new Date(
		currentDate.getFullYear(),
		0,
		januaryFirst.getDate() + daysToNextMonday
	);
	return currentDate < nextMonday
		? 52
		: currentDate > nextMonday
			? Math.ceil((currentDate.getTime() - nextMonday.getTime()) / (24 * 3600 * 1000) / 7)
			: 1;
}

function updateClock() {
	const now = new Date();
	const time = now.toLocaleTimeString('nb-NO', { hour: 'numeric', minute: '2-digit' });
	const date = now.toLocaleDateString('nb-NO', { weekday: 'long', day: 'numeric' });
	const week = getDateWeek(now);

	document.getElementById('now_time').textContent = time;
	document.getElementById('now_date').textContent = date;
	document.getElementById('now_week').textContent = `Uke ${week}`;
}

updateClock();
setInterval(updateClock, 1000);

// Reload page when clicking the clock
document.getElementById('now_time').addEventListener('click', () => window.location.reload());

// Calendar visibility - hide days that overflow
function checkCalendarVisibility() {
	const calendarEl = document.querySelector('calendar');
	if (!calendarEl) return;

	const containerHeight = calendarEl.clientHeight;
	const dayElements = Array.from(calendarEl.querySelectorAll('day'));

	dayElements.forEach((el) => el.classList.remove('day-hidden'));

	let cumulativeHeight = 0;
	let firstHiddenIndex = -1;

	for (let i = 0; i < dayElements.length; i++) {
		const item = dayElements[i];
		if (!item.offsetHeight) continue;
		const itemHeight = item.offsetHeight;

		if (cumulativeHeight + itemHeight > containerHeight && i > 0) {
			firstHiddenIndex = i;
			break;
		}
		cumulativeHeight += itemHeight;
	}

	dayElements.forEach((el, i) => {
		if (firstHiddenIndex !== -1 && i >= firstHiddenIndex) {
			el.classList.add('day-hidden');
		}
	});
}

// Run visibility check when calendar content is swapped by HTMX
document.body.addEventListener('htmx:sseMessage', (evt) => {
	if (evt.detail.type === 'calendar') {
		requestAnimationFrame(checkCalendarVisibility);
	}
});

// Also run on resize
const calendarEl = document.querySelector('calendar');
if (calendarEl) {
	new ResizeObserver(() => requestAnimationFrame(checkCalendarVisibility)).observe(calendarEl);
}

// Initial visibility check
requestAnimationFrame(checkCalendarVisibility);

// Reload page on the hour for cache-busting
const now = new Date();
const startOfNextHour = new Date();
startOfNextHour.setUTCHours(now.getUTCHours() + 1, 0, 1, 0);
const diff = startOfNextHour.getTime() - now.getTime();
setTimeout(() => window.location.reload(), diff);

// Version check - reload if server restarted
const initialVersion = document.getElementById('server-version')?.dataset?.version;
document.body.addEventListener('htmx:sseMessage', (evt) => {
	if (evt.detail.type === 'version') {
		const el = document.getElementById('server-version');
		if (el && initialVersion && el.dataset.version !== initialVersion) {
			window.location.reload();
		}
	}
});
