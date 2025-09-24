import { DateTime } from 'luxon';

export class Days {
	private weather: any;
	private calendar: any;

	constructor(calendar: any, weather: any) {
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

	getDataForDate(jsDate: Date) {
		// Create a Luxon DateTime object
		const dt = DateTime.fromJSDate(jsDate).setLocale('nb');

		const date = dt.toISODate()?.toString() || '';

		const daily = this.weather.getDailyForecasts();
		const dailyHytta = this.weather.getDailyForecasts('hytta');

		// Get events and check for Hytta events
		const events = this.calendar.getEventsForDate(jsDate);

		// Check if any full day event has title "Hytta" and add weather data
		const enrichedEvents = events.map((event: any) => {
			if (event.fullDay && event.title.toLowerCase().indexOf('hytta') !== -1 && date && dailyHytta[date]) {
				return {
					...event,
					hyttaWeather: {
						temperature: Math.round(dailyHytta[date].maxTemp),
						rainProbability: dailyHytta[date].lightRainProbability
					}
				};
			}
			return event;
		});

		return {
			isoDate: date,
			date: Days.createNiceDate(jsDate),
			weekday: Days.createNiceDate(jsDate, true),
			daily_forecast: date ? daily[date] : undefined,
			events: enrichedEvents,
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
	static createNiceDate(jsDate: Date, relative = false) {
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
