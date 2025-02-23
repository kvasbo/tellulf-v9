import { google } from 'googleapis';
import { DateTime } from 'luxon';
import { env } from '$env/dynamic/private';

const SCOPES = 'https://www.googleapis.com/auth/calendar.readonly';

function getGoogleKey() {
	// Get the Google key from the environment variable
	return JSON.parse(Buffer.from(env.GOOGLE_KEY_B64, 'base64').toString('utf8'));
}

/**
 * Calendar class, with methods for fetching events, birthdays and dinners.
 * @class
 */
export class Calendar {
	private static instance: Calendar;

	events = []; // RawEvents
	birthdays = []; // RawEvents
	dinners = []; // RawEvents
	// Display height of calendar events in pixels to ensure we don't overflow
	displayHeights = {
		event: 25,
		birthday: 25,
		dayInfo: 51
	};

	public static getInstance() {
		if (!Calendar.instance) {
			Calendar.instance = new Calendar();
		}
		return Calendar.instance;
	}

	/**
	 * Constructor for the Calendar class, that refreshes events, birthdays and dinners every 15 minutes.
	 * @class
	 * @param {number} [interval] - Optional interval for main event reload frequency.
	 * @param {number} [interval] - Optional interval for birthday and dinner reload in seconds.
	 */
	private constructor() {
		this.refreshEvents();
		this.refreshBirthdays();
		this.refreshDinners();

		setInterval(() => {
			this.refreshEvents();
		}, 15 * 1000);

		setInterval(() => {
			this.refreshBirthdays();
			this.refreshDinners();
		}, 900 * 1000);
	}

	/**
	 * Try to calculate the height of a day in pixels based on the number of events and birthdays.
	 * @param {Date} jsDate The date to check
	 * @returns {number} The estimated height in pixels
	 */
	calculateDisplayHeightForDay(jsDate: Date) {
		const eventCount = this.getEventsForDate(jsDate).length;
		const birthdays = this.getBirthdaysForDate(jsDate).length;
		const height =
			this.displayHeights.dayInfo +
			eventCount * this.displayHeights.event +
			birthdays * this.displayHeights.birthday;
		return height;
	}

	/**
	 * Retrieves events for a given date.
	 * @param {Date} jsDate - The JavaScript Date object representing the date.
	 * @returns {Array} - An array of events for the given date.
	 */
	getEventsForDate(jsDate: Date) {
		return this.events
			.filter((e) => this.checkEventForDate(e, jsDate))
			.map((e) => this.enrichEvent(e, 'event', jsDate));
	}

	/**
	 * Retrieves birthdays for a given date.
	 * @param {Date} jsDate - The JavaScript Date object representing the date.
	 * @returns {Array} - An array of birthdays for the given date.
	 */
	getBirthdaysForDate(jsDate: Date) {
		return this.birthdays
			.filter((e) => this.checkEventForDate(e, jsDate))
			.map((e) => this.enrichEvent(e, 'birthday', jsDate));
	}

	/**
	 * Retrieves the dinner events for a given date.
	 * @param {Date} jsDate - The JavaScript Date object representing the date.
	 * @returns {Array} An array of dinner events for the given date.
	 */
	getDinnerForDate(jsDate: Date) {
		return this.dinners
			.filter((e) => this.checkEventForDate(e, jsDate))
			.map((e) => this.enrichEvent(e, 'dinner', jsDate));
	}

	/**
	 * Checks if an event falls on a specific date.
	 * @param {object} event - The event object.
	 * @param {Date} jsDate - The JavaScript Date object representing the date to check.
	 * @returns {boolean} - Returns true if the event falls on the specified date, false otherwise.
	 */
	checkEventForDate(event: any, jsDate: Date) {
		// Find start of day for all of the fuckers
		const dt = DateTime.fromJSDate(jsDate).startOf('day');
		const eventStart = DateTime.fromJSDate(event.start).startOf('day');
		const eventEnd = DateTime.fromJSDate(event.end).startOf('day');

		// It starts before or ends after!
		if (eventStart.toMillis() <= dt.toMillis() && eventEnd.toMillis() >= dt.toMillis()) {
			return true;
		}

		// Nothing to see here
		return false;
	}

	enrichEvent(event, type = '', forDate) {
		const displayTitle = Calendar.getDisplayTitle(event, type);
		const dayType = Calendar.getDayType(event, forDate);
		const displayTime = Calendar.getEventDisplayTime(event, dayType);

		return {
			...event,
			displayTitle: displayTitle,
			dayType: dayType,
			displayTime: displayTime
		};
	}

	/**
	 * Parse the title of an event, if it is a birthday then change it to display
	 * the age of the person if the event title ends in a string.
	 * @param {object} event Event object
	 * @param {string} type The type of object. Either "event" or "birthday"
	 * @returns {string} The display title
	 */
	static getDisplayTitle(event: any, type: 'birthday' | 'event' | 'dinner') {
		let title = event.title;
		if (type === 'birthday') {
			const regex = /[A-Za-z0-9 ]+\s[0-9]+/i;
			const foundYear = regex.test(event.title);
			if (foundYear) {
				const y = Number(event.title.slice(-4));
				const now = DateTime.now();
				const age = now.year - 1 * y;
				title = `${event.title.substring(0, event.title.length - 5)} (${age} år)`;
			}
		}
		return title;
	}

	async refreshEvents() {
		if (env.CAL_ID_FELLES) {
			this.events = await Calendar.getCalendarData(env.CAL_ID_FELLES);

			console.log(this.events.length + ' events fetched.');
		} else {
			this.events = [];
		}
	}

	async refreshDinners() {
		if (env.CAL_ID_MIDDAG) {
			this.dinners = await Calendar.getCalendarData(env.CAL_ID_MIDDAG);

			console.log(this.dinners.length + ' dinners fetched.');
		} else {
			this.dinners = [];

			console.log('Dinner calendar ID not found :(');
		}
	}

	async refreshBirthdays() {
		if (env.CAL_ID_BURSDAG) {
			this.birthdays = await Calendar.getCalendarData(env.CAL_ID_BURSDAG);

			console.log(this.birthdays.length + ' birthdays fetched.');
		} else {
			this.birthdays = [];
		}
	}

	/**
	 * Get the content of a calendar
	 * @param calendarId
	 */
	static async getCalendarData(calendarId: string) {
		const jwtClient = new google.auth.JWT(
			getGoogleKey().client_email,
			undefined,
			getGoogleKey().private_key,
			SCOPES
		);

		const calendar = google.calendar({
			version: 'v3',
			auth: jwtClient
		});

		const out = []; // Rawevent

		const result = await calendar.events.list({
			calendarId: calendarId,
			timeMin: DateTime.now().toISO(),
			timeMax: DateTime.now().plus({ weeks: 2 }).toISO(),
			maxResults: 2000,
			singleEvents: true,
			orderBy: 'startTime'
		});

		if (result?.data?.items?.length) {
			result.data.items.forEach((event) => {
				const e = Calendar.parseGoogleEvent(event);
				out.push(e);
			});
		} else {
			console.log(
				JSON.stringify({
					message: `No upcoming events found for ${calendarId}`
				})
			);
		}
		return out;
	}

	// Get the correct displaytime for the event
	static getEventDisplayTime(event, dayType) {
		if (dayType === 'middleDay') {
			return {
				start: '',
				end: '',
				spacer: '...'
			};
		}

		const arrow = '–';

		if (dayType === 'lastDay') {
			return {
				start: '',
				end: DateTime.fromJSDate(event.end).toFormat('HH:mm'),
				spacer: arrow
			};
		}

		if (dayType === 'firstDay') {
			return {
				start: DateTime.fromJSDate(event.start).toFormat('HH:mm'),
				end: '',
				spacer: arrow
			};
		}

		//  First or only day
		return {
			start: DateTime.fromJSDate(event.start).toFormat('HH:mm'),
			end: DateTime.fromJSDate(event.end).toFormat('HH:mm'),
			spacer: arrow
		};
	}

	// Figure out if same date or whether it spans multiple days. For displaying purposes
	static getDayType(event, date: Date) {
		const dtStart = DateTime.fromJSDate(event.start);
		const dtEnd = DateTime.fromJSDate(event.end);
		const dtDate = DateTime.fromJSDate(date);

		let dayType = 'singleDay';

		// Single day event
		if (dtStart.hasSame(dtEnd, 'day')) {
			dayType = 'singleDay';
		} else if (dtStart.hasSame(dtDate, 'day')) {
			dayType = 'firstDay';
		} else if (dtEnd.hasSame(dtDate, 'day')) {
			dayType = 'lastDay';
		} else {
			// Neither first nor last day
			dayType = 'middleDay';
		}

		return dayType;
	}

	// Main parsing of an event from Google
	static parseGoogleEvent(event) {
		const ev = event;

		const title = event.summary ? event.summary : '';

		if (ev.start.date && ev.end.date) {
			// Fullday!
			const dtStart = DateTime.fromISO(ev.start.date).startOf('day');
			const dtEnd = DateTime.fromISO(ev.end.date).minus({ days: 1 }).startOf('day');
			const start = dtStart.toJSDate();
			const end = dtEnd.toJSDate();
			const fullDay = true;
			return { title, start, end, fullDay };
		} else if (ev.start.dateTime && ev.end.dateTime) {
			const dtStart = DateTime.fromISO(ev.start.dateTime);
			const dtEnd = DateTime.fromISO(ev.end.dateTime);
			const start = dtStart.toJSDate();
			const end = dtEnd.toJSDate();
			const fullDay = false;
			return { title, start, end, fullDay };
		} else {
			throw new Error('Invalid event');
		}
	}
}
