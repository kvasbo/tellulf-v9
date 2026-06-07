// Pure logic for the living sky background.
// The server owns all weather/time reasoning; the client only renders.
// This module turns (time, sun times, weather symbol) into a SkyState:
//   - numeric uniforms for the WebGL shader
//   - CSS variable strings for adaptive text/glass readability

export type Phase = 'night' | 'dawn' | 'day' | 'dusk';
export type Condition = 'clear' | 'partly' | 'cloudy' | 'precip';
export type Precip = 'none' | 'rain' | 'snow';

type RGB = [number, number, number];

export interface SkyState {
	phase: Phase;
	condition: Condition;
	precip: Precip;
	/** 0..1 position of the luminary along its arc (sun by day, moon by night). */
	arc: number;
	/** Gradient stops (top, middle, horizon), each rgb in 0..1, for the shader. */
	colors: { c1: RGB; c2: RGB; c3: RGB };
	/** Adaptive CSS variables for text and glass. */
	css: {
		fg: string;
		fgMuted: string;
		fgFaint: string;
		line: string;
		glassBg: string;
		glassBorder: string;
		glassShadow: string;
	};
}

const MS_PER_DAY = 24 * 60 * 60 * 1000;

const clamp01 = (n: number): number => Math.min(1, Math.max(0, n));

/**
 * Time-of-day phase. Dawn/dusk are windows of `twilightMin` minutes on each
 * side of the sun event; outside them it's day (sun up) or night (sun down).
 */
export function getPhase(
	now: Date,
	sunrise: Date,
	sunset: Date,
	twilightMin = 45,
): Phase {
	const t = now.getTime();
	const win = twilightMin * 60 * 1000;
	const rise = sunrise.getTime();
	const set = sunset.getTime();

	if (Math.abs(t - rise) <= win) return 'dawn';
	if (Math.abs(t - set) <= win) return 'dusk';
	if (t > rise && t < set) return 'day';
	return 'night';
}

/** Bucket a MET Norway symbol code into a coarse condition. */
export function getCondition(symbol: string | undefined): Condition {
	if (!symbol) return 'cloudy';
	const s = symbol.toLowerCase();
	if (/rain|snow|sleet|thunder/.test(s)) return 'precip';
	if (s.includes('clearsky')) return 'clear';
	if (s.includes('partlycloudy') || s.includes('fair')) return 'partly';
	return 'cloudy'; // cloudy, fog, unknown
}

/** Decide rain vs snow for precipitation, using the symbol then temperature. */
export function getPrecip(
	symbol: string | undefined,
	airTemperature: number | undefined,
): Precip {
	if (getCondition(symbol) !== 'precip') return 'none';
	const s = (symbol ?? '').toLowerCase();
	if (s.includes('snow')) return 'snow';
	if (s.includes('rain')) return 'rain';
	// sleet / ambiguous: fall back to temperature
	return (airTemperature ?? 5) <= 1 ? 'snow' : 'rain';
}

/**
 * 0..1 progress of the luminary along its arc. By day it tracks the sun from
 * sunrise (0) to sunset (1); at night it tracks the moon across the dark hours.
 */
export function getArc(now: Date, sunrise: Date, sunset: Date): number {
	const t = now.getTime();
	const rise = sunrise.getTime();
	const set = sunset.getTime();
	const dayMs = set - rise;

	if (t >= rise && t <= set) return clamp01((t - rise) / dayMs);

	const nightMs = MS_PER_DAY - dayMs;
	// After sunset: progress from this sunset toward the next sunrise.
	// Before sunrise: progress from the previous sunset (set - 1 day).
	const nightStart = t > set ? set : set - MS_PER_DAY;
	return clamp01((t - nightStart) / nightMs);
}

// --- Palettes -------------------------------------------------------------

const PHASE_GRADIENT: Record<Phase, { c1: RGB; c2: RGB; c3: RGB }> = {
	night: {
		c1: [0.03, 0.04, 0.11],
		c2: [0.06, 0.08, 0.17],
		c3: [0.11, 0.13, 0.24],
	},
	dawn: {
		c1: [0.18, 0.22, 0.42],
		c2: [0.52, 0.43, 0.54],
		c3: [0.98, 0.71, 0.5],
	},
	day: {
		c1: [0.26, 0.54, 0.85],
		c2: [0.5, 0.72, 0.93],
		c3: [0.83, 0.92, 0.98],
	},
	dusk: {
		c1: [0.14, 0.16, 0.36],
		c2: [0.5, 0.35, 0.46],
		c3: [0.96, 0.55, 0.4],
	},
};

// How much each condition flattens (desaturates) and darkens the gradient.
const CONDITION_MOD: Record<Condition, { desat: number; darken: number }> = {
	clear: { desat: 0, darken: 0 },
	partly: { desat: 0.2, darken: 0.02 },
	cloudy: { desat: 0.5, darken: 0.06 },
	precip: { desat: 0.6, darken: 0.12 },
};

const luminance = ([r, g, b]: RGB): number => 0.299 * r + 0.587 * g + 0.114 * b;

function applyCondition(c: RGB, desat: number, darken: number): RGB {
	const l = luminance(c);
	const mixed: RGB = [
		c[0] + (l - c[0]) * desat,
		c[1] + (l - c[1]) * desat,
		c[2] + (l - c[2]) * desat,
	];
	const k = 1 - darken;
	return [mixed[0] * k, mixed[1] * k, mixed[2] * k];
}

// Two readability schemes. Glass background supplies the contrast for the text,
// so each scheme is internally consistent regardless of the raw sky behind it.
const LIGHT_UI = {
	fg: '#1a1d22',
	fgMuted: '#4a4f57',
	fgFaint: 'rgba(20, 22, 28, 0.42)',
	line: 'rgba(20, 30, 45, 0.22)',
	glassBg: 'rgba(255, 255, 255, 0.16)',
	glassBorder: 'rgba(255, 255, 255, 0.38)',
	glassShadow: 'rgba(30, 50, 80, 0.18)',
};

const DARK_UI = {
	fg: '#eef2f8',
	fgMuted: '#aab4c4',
	fgFaint: 'rgba(230, 238, 250, 0.5)',
	line: 'rgba(200, 215, 240, 0.22)',
	glassBg: 'rgba(18, 26, 44, 0.32)',
	glassBorder: 'rgba(150, 170, 210, 0.22)',
	glassShadow: 'rgba(0, 0, 10, 0.35)',
};

// Dawn/day use dark text on light glass; dusk/night use light text on dark glass.
const PHASE_SCHEME: Record<Phase, typeof LIGHT_UI> = {
	day: LIGHT_UI,
	dawn: LIGHT_UI,
	dusk: DARK_UI,
	night: DARK_UI,
};

export function buildSkyState(
	phase: Phase,
	condition: Condition,
	precip: Precip,
	arc: number,
): SkyState {
	const base = PHASE_GRADIENT[phase];
	const { desat, darken } = CONDITION_MOD[condition];
	return {
		phase,
		condition,
		precip,
		arc,
		colors: {
			c1: applyCondition(base.c1, desat, darken),
			c2: applyCondition(base.c2, desat, darken),
			c3: applyCondition(base.c3, desat, darken),
		},
		css: PHASE_SCHEME[phase],
	};
}
