import { describe, expect, test } from 'bun:test';
import {
	buildSkyState,
	getArc,
	getCondition,
	getPhase,
	getPrecip,
} from './sky.js';

const at = (h: number, m = 0): Date => new Date(2026, 5, 7, h, m, 0);
const sunrise = at(4, 0);
const sunset = at(22, 0);

describe('getPhase', () => {
	test('midday is day', () => {
		expect(getPhase(at(13), sunrise, sunset)).toBe('day');
	});
	test('deep night is night', () => {
		expect(getPhase(at(1), sunrise, sunset)).toBe('night');
	});
	test('around sunrise is dawn', () => {
		expect(getPhase(at(4, 10), sunrise, sunset)).toBe('dawn');
		expect(getPhase(at(3, 30), sunrise, sunset)).toBe('dawn');
	});
	test('around sunset is dusk', () => {
		expect(getPhase(at(22, 20), sunrise, sunset)).toBe('dusk');
	});
	test('just outside the twilight window flips to day/night', () => {
		expect(getPhase(at(5, 0), sunrise, sunset)).toBe('day');
		expect(getPhase(at(23, 0), sunrise, sunset)).toBe('night');
	});
});

describe('getCondition', () => {
	test.each([
		['clearsky_day', 'clear'],
		['clearsky_night', 'clear'],
		['partlycloudy_day', 'partly'],
		['fair_night', 'partly'],
		['cloudy', 'cloudy'],
		['fog', 'cloudy'],
		['lightrain', 'precip'],
		['heavysnowshowers_day', 'precip'],
		['sleet', 'precip'],
		['rainandthunder', 'precip'],
	] as const)('%s -> %s', (symbol, expected) => {
		expect(getCondition(symbol)).toBe(expected);
	});
	test('undefined symbol is cloudy', () => {
		expect(getCondition(undefined)).toBe('cloudy');
	});
});

describe('getPrecip', () => {
	test('non-precip is none', () => {
		expect(getPrecip('clearsky_day', 10)).toBe('none');
	});
	test('rain symbol is rain', () => {
		expect(getPrecip('lightrain', 8)).toBe('rain');
	});
	test('snow symbol is snow', () => {
		expect(getPrecip('heavysnow', -3)).toBe('snow');
	});
	test('sleet falls back to temperature', () => {
		expect(getPrecip('sleet', -2)).toBe('snow');
		expect(getPrecip('sleet', 6)).toBe('rain');
	});
});

describe('getArc', () => {
	test('sunrise is 0, sunset is 1, noon ~0.5', () => {
		expect(getArc(sunrise, sunrise, sunset)).toBeCloseTo(0, 5);
		expect(getArc(sunset, sunrise, sunset)).toBeCloseTo(1, 5);
		expect(getArc(at(13), sunrise, sunset)).toBeCloseTo(0.5, 1);
	});
	test('night progress stays within 0..1', () => {
		const a = getArc(at(1), sunrise, sunset);
		expect(a).toBeGreaterThanOrEqual(0);
		expect(a).toBeLessThanOrEqual(1);
	});
});

describe('buildSkyState', () => {
	test('returns all uniforms and css vars for every phase/condition', () => {
		const phases = ['night', 'dawn', 'day', 'dusk'] as const;
		const conditions = ['clear', 'partly', 'cloudy', 'precip'] as const;
		for (const phase of phases) {
			for (const condition of conditions) {
				const s = buildSkyState(phase, condition, 'none', 0.5);
				for (const c of [s.colors.c1, s.colors.c2, s.colors.c3]) {
					expect(c).toHaveLength(3);
					for (const ch of c) {
						expect(ch).toBeGreaterThanOrEqual(0);
						expect(ch).toBeLessThanOrEqual(1);
					}
				}
				expect(s.css.fg).toMatch(/^#|rgba/);
				expect(s.css.glassBg).toContain('rgba');
			}
		}
	});
	test('precip flattens the gradient vs clear (lower saturation spread)', () => {
		const spread = (s: ReturnType<typeof buildSkyState>) =>
			Math.abs(s.colors.c1[2] - s.colors.c1[0]);
		const clear = buildSkyState('day', 'clear', 'none', 0.5);
		const precip = buildSkyState('day', 'precip', 'rain', 0.5);
		expect(spread(precip)).toBeLessThan(spread(clear));
	});
});
