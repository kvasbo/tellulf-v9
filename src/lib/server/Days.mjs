import { DateTime } from 'luxon';

export class Days {
	constructor(calendar, weather) {
		this.weather = weather;
		this.calendar = calendar;
	}

	generateComingDays(maxNumberOfDays = 14) {
		const days = [];
		for (let i = 0; i < maxNumberOfDays; i++) {
			const dt = DateTime.now().plus({ days: i });
			const day = this.getDataForDate(dt.toJSDate());
			days.push(day);
		}
		return days;
	}

	getDataForDate(jsDate) {
		// Create a Luxon DateTime object
		const dt = DateTime.fromJSDate(jsDate).setLocale('nb');

		const date = dt.toISODate()?.toString();

		const daily = this.weather.getDailyForecasts();

		return {
			isoDate: date,
			date: Days.createNiceDate(jsDate),
			weekday: Days.createNiceDate(jsDate, true),
			daily_forecast: daily[date],
			events: this.calendar.getEventsForDate(jsDate),
			birthdays: this.calendar.getBirthdaysForDate(jsDate),
			dinner: this.calendar.getDinnerForDate(jsDate)
		};
	}

	/**
	 * Nicely format a date
	 * To be switched to using Luxon
	 * @param jsDate
	 * @param relative
	 */
	static createNiceDate(jsDate, relative = false) {
		const dt = DateTime.fromJSDate(jsDate).setLocale('nb').startOf('day');
		if (relative) {
			if (dt.hasSame(DateTime.local(), 'day')) {
				return 'i dag';
			} else if (dt.hasSame(DateTime.local().plus({ days: 1 }), 'day')) {
				return 'i morgen';
			}
		}
		return dt.toFormat('cccc d.');
	}
}
