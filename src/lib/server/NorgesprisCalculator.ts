import { Places } from '../Enums.js';

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
	isActive: boolean;
}

export class NorgesprisCalculator {
	private config: NorgesprisConfig;

	constructor(config?: Partial<NorgesprisConfig>) {
		this.config = {
			subsidizedPrice: 0.5,
			homeCap: 5000,
			cabinCap: 1000,
			startDate: new Date('2025-10-01'),
			...config,
		};
	}

	public isNorgesprisActive(): boolean {
		return Date.now() >= this.config.startDate.getTime();
	}

	public getCap(place: Places): number {
		return place === Places.Home ? this.config.homeCap : this.config.cabinCap;
	}

	public getSubsidizedPrice(): number {
		return this.config.subsidizedPrice;
	}

	public calculateCost(
		place: Places,
		monthlyConsumption: number,
		spotPrice: number,
	): NorgesprisCalculation {
		if (!this.isNorgesprisActive()) {
			return {
				subsidizedConsumption: 0,
				marketConsumption: monthlyConsumption,
				effectivePrice: spotPrice,
				isActive: false,
			};
		}

		const cap = this.getCap(place);

		const subsidizedConsumption = Math.min(monthlyConsumption, cap);
		const marketConsumption = Math.max(0, monthlyConsumption - cap);

		const effectivePrice =
			monthlyConsumption > cap ? spotPrice : this.config.subsidizedPrice;

		return {
			subsidizedConsumption,
			marketConsumption,
			effectivePrice,
			isActive: true,
		};
	}

	public calculateAccumulatedCost(
		place: Places,
		_monthlyConsumption: number,
		historicalSpotPrices: {
			timestamp: Date;
			price: number;
			consumption: number;
		}[],
	): number {
		if (!this.isNorgesprisActive()) {
			return historicalSpotPrices.reduce(
				(sum, hour) => sum + hour.price * hour.consumption,
				0,
			);
		}

		const cap = this.getCap(place);
		let accumulatedConsumption = 0;
		let totalCost = 0;

		for (const hour of historicalSpotPrices) {
			const previousAccumulated = accumulatedConsumption;
			accumulatedConsumption += hour.consumption;

			if (previousAccumulated < cap) {
				const subsidizedPortion = Math.min(
					hour.consumption,
					cap - previousAccumulated,
				);
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
		spotPrice: number,
	): number {
		if (!this.isNorgesprisActive()) {
			return spotPrice;
		}

		const cap = this.getCap(place);

		if (monthlyConsumption >= cap) {
			return spotPrice;
		}

		return this.config.subsidizedPrice;
	}
}
