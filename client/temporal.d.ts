// Minimal Temporal types until TypeScript ships them
declare namespace Temporal {
	interface PlainDate {
		readonly weekOfYear: number;
	}
	interface Now {
		plainDateISO(): PlainDate;
	}
	const Now: Now;
}
