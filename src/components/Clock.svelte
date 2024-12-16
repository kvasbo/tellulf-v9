<script lang="ts">

	import { Moon, Hemisphere } from "lunarphase-js";

	function getDateWeek(date: Date) {
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

	function getStrings(jsDate: Date): { date: string; time: string; week: string } {
		const date = jsDate.toLocaleDateString('nb-NO', { weekday: 'long', day: 'numeric' });
		const time = jsDate.toLocaleTimeString('nb-NO', { hour: '2-digit', minute: '2-digit' });
		const week = getDateWeek(jsDate).toString();
		return { date, time, week };
	}
	function updateTime(): void {
		const now = new Date();
		week = getStrings(now).week;
		time = getStrings(now).time;
		date = getStrings(now).date;
	}
	const now = new Date();
	let week = getStrings(now).week;
	let time = getStrings(now).time;
	let date = getStrings(now).date;
	setInterval(updateTime, 1000);
</script>

<clock>
	<div id="now_time">{time}</div>
	<div id="date_week">
		<div id="now_date">{date}</div>
		<div id="now_week">{Moon.lunarPhaseEmoji(new Date(), {
			hemisphere: Hemisphere.SOUTHERN,
		})} Uke {week}</div>
	</div>
</clock>
