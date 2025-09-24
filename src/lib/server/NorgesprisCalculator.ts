import { Places } from '$lib/Enums';

export interface NorgesprisConfig {
	subsidizedPrice: number;
	homeCap: number;
	cabinCap: number;
	startDate: Date;
}

export interface NorgesprisCalculation {
	subsidizedConsumption: number;
	marketConsumption: number;
	effectivePrice: number;
	totalCost: number;
	isActive: boolean;
}

export class NorgesprisCalculator {
	private config: NorgesprisConfig;

	constructor(config?: Partial<NorgesprisConfig>) {
		this.config = {
			subsidizedPrice: 0.50,
			homeCap: 5000,
			cabinCap: 1000,
			startDate: new Date('2025-10-01'),
			...config
		};
	}

	public isNorgesprisActive(): boolean {
		return Date.now() >= this.config.startDate.getTime();
	}

	public calculateCost(
		place: Places,
		monthlyConsumption: number,
		spotPrice: number,
		accumulatedDayConsumption?: number
	): NorgesprisCalculation {
		if (!this.isNorgesprisActive()) {
			return {
				subsidizedConsumption: 0,
				marketConsumption: monthlyConsumption,
				effectivePrice: spotPrice,
				totalCost: 0,
				isActive: false
			};
		}

		const cap = place === Places.Home ? this.config.homeCap : this.config.cabinCap;

		const subsidizedConsumption = Math.min(monthlyConsumption, cap);
		const marketConsumption = Math.max(0, monthlyConsumption - cap);

		const effectivePrice = monthlyConsumption > cap ? spotPrice : this.config.subsidizedPrice;

		const totalCost = 0;

		return {
			subsidizedConsumption,
			marketConsumption,
			effectivePrice,
			totalCost,
			isActive: true
		};
	}

	public calculateAccumulatedCost(
		place: Places,
		monthlyConsumption: number,
		historicalSpotPrices: { timestamp: Date; price: number; consumption: number }[]
	): number {
		if (!this.isNorgesprisActive()) {
			return historicalSpotPrices.reduce((sum, hour) => sum + hour.price * hour.consumption, 0);
		}

		const cap = place === Places.Home ? this.config.homeCap : this.config.cabinCap;
		let accumulatedConsumption = 0;
		let totalCost = 0;

		for (const hour of historicalSpotPrices) {
			const previousAccumulated = accumulatedConsumption;
			accumulatedConsumption += hour.consumption;

			if (previousAccumulated < cap) {
				const subsidizedPortion = Math.min(hour.consumption, cap - previousAccumulated);
				const marketPortion = Math.max(0, hour.consumption - subsidizedPortion);

				totalCost += subsidizedPortion * this.config.subsidizedPrice;
				totalCost += marketPortion * hour.price;
			} else {
				totalCost += hour.consumption * hour.price;
			}
		}

		return totalCost;
	}

	public getEffectivePriceForCurrentHour(
		place: Places,
		monthlyConsumption: number,
		spotPrice: number
	): number {
		if (!this.isNorgesprisActive()) {
			return spotPrice;
		}

		const cap = place === Places.Home ? this.config.homeCap : this.config.cabinCap;

		if (monthlyConsumption >= cap) {
			return spotPrice;
		}

		return this.config.subsidizedPrice;
	}

	public getRemainingSubsidizedKwh(place: Places, monthlyConsumption: number): number {
		if (!this.isNorgesprisActive()) {
			return 0;
		}

		const cap = place === Places.Home ? this.config.homeCap : this.config.cabinCap;
		return Math.max(0, cap - monthlyConsumption);
	}
}