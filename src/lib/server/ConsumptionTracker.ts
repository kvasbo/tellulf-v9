import { Places } from '$lib/Enums';

interface MonthlyConsumption {
	year: number;
	month: number;
	consumption: number;
	lastUpdated: Date;
}

export class ConsumptionTracker {
	private consumptionData: Map<Places, MonthlyConsumption>;

	constructor() {
		this.consumptionData = new Map();
		this.initializeCurrentMonth();
	}

	private initializeCurrentMonth() {
		const now = new Date();
		const currentYear = now.getFullYear();
		const currentMonth = now.getMonth() + 1;

		for (const place of [Places.Home, Places.Cabin]) {
			this.consumptionData.set(place, {
				year: currentYear,
				month: currentMonth,
				consumption: 0,
				lastUpdated: now
			});
		}
	}

	public updateConsumption(place: Places, consumption: number): void {
		const now = new Date();
		const currentYear = now.getFullYear();
		const currentMonth = now.getMonth() + 1;

		const data = this.consumptionData.get(place);

		if (!data || data.year !== currentYear || data.month !== currentMonth) {
			this.consumptionData.set(place, {
				year: currentYear,
				month: currentMonth,
				consumption: consumption,
				lastUpdated: now
			});
		} else {
			this.consumptionData.set(place, {
				...data,
				consumption: consumption,
				lastUpdated: now
			});
		}
	}

	public getMonthlyConsumption(place: Places): number {
		const now = new Date();
		const currentYear = now.getFullYear();
		const currentMonth = now.getMonth() + 1;

		const data = this.consumptionData.get(place);

		if (!data || data.year !== currentYear || data.month !== currentMonth) {
			this.updateConsumption(place, 0);
			return 0;
		}

		return data.consumption;
	}

	public resetIfNewMonth(place: Places): void {
		const now = new Date();
		const currentYear = now.getFullYear();
		const currentMonth = now.getMonth() + 1;

		const data = this.consumptionData.get(place);

		if (!data || data.year !== currentYear || data.month !== currentMonth) {
			this.updateConsumption(place, 0);
		}
	}
}