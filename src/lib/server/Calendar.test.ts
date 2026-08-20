import { describe, expect, test } from 'bun:test';
import type { Event } from './Calendar.d.js';
import { Calendar } from './Calendar.js';

// The kids are exchanged at 16:00 on days where we switch.
const day = new Date(2026, 2, 10);
const at = (h: number, m = 0): Date => new Date(2026, 2, 10, h, m, 0);

const felles = (start: Date, end: Date): Event => ({
	title: 'Noe felles',
	start,
	end,
	fullDay: false,
	source: 'felles',
});

const faded = (
	event: Event,
	kidsStatus: Parameters<typeof Calendar.isEventFaded>[2],
) => Calendar.isEventFaded(event, day, kidsStatus);

describe('isEventFaded', () => {
	test('kids at Hanne all week: always faded', () => {
		expect(faded(felles(at(9), at(10)), null)).toBe(true);
		expect(faded(felles(at(18), at(19)), null)).toBe(true);
	});

	test('kids at Audun all week: never faded', () => {
		expect(faded(felles(at(9), at(10)), 'full')).toBe(false);
		expect(faded(felles(at(18), at(19)), 'full')).toBe(false);
	});

	test('leaving (Audun to Hanne): faded only after the handover', () => {
		expect(faded(felles(at(9), at(10)), 'leaving')).toBe(false);
		expect(faded(felles(at(15, 59), at(17)), 'leaving')).toBe(false);
		expect(faded(felles(at(16), at(17)), 'leaving')).toBe(true);
		expect(faded(felles(at(18), at(19)), 'leaving')).toBe(true);
	});

	test('arriving (Hanne to Audun): faded only before the handover', () => {
		expect(faded(felles(at(9), at(10)), 'arriving')).toBe(true);
		expect(faded(felles(at(15, 59), at(17)), 'arriving')).toBe(true);
		expect(faded(felles(at(16), at(17)), 'arriving')).toBe(false);
		expect(faded(felles(at(18), at(19)), 'arriving')).toBe(false);
	});

	test("Audun's own events are never faded", () => {
		const own: Event = { ...felles(at(18), at(19)), source: 'audun' };
		expect(faded(own, null)).toBe(false);
		expect(faded(own, 'leaving')).toBe(false);
		expect(faded(own, 'arriving')).toBe(false);
	});

	test('full-day events span the day, so exchange days render them black', () => {
		const fullDay: Event = { ...felles(day, day), fullDay: true };
		expect(faded(fullDay, 'leaving')).toBe(false);
		expect(faded(fullDay, 'arriving')).toBe(false);
		expect(faded(fullDay, 'full')).toBe(false);
		expect(faded(fullDay, null)).toBe(true);
	});

	test('a multi-day event that started earlier spans the day', () => {
		const middleDay = felles(
			new Date(2026, 2, 9, 20),
			new Date(2026, 2, 11, 8),
		);
		expect(faded(middleDay, 'leaving')).toBe(false);
		expect(faded(middleDay, 'arriving')).toBe(false);

		const lastDay = felles(new Date(2026, 2, 9, 20), at(9));
		expect(faded(lastDay, 'leaving')).toBe(false);
		expect(faded(lastDay, 'arriving')).toBe(false);
	});

	test('an event starting today and running into tomorrow uses its start time', () => {
		const firstDay = felles(at(17), new Date(2026, 2, 11, 8));
		expect(faded(firstDay, 'leaving')).toBe(true);
		expect(faded(firstDay, 'arriving')).toBe(false);
	});
});
