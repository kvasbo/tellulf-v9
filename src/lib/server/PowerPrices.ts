export class PowerPrices {
	static SUPPORT_CUTOFF_USAGE = 5000;
	static SUPPORT_ENTRY_PRICE = 0.7;
	static WINTER_NIGHT_OR_WEEKEND_PRICE = 0.2895;
	static WINTER_DAY_PRICE = 0.352;
	static SUMMER_NIGHT_OR_WEEKEND_PRICE = 0.373;
	static SUMMER_DAY_PRICE = 0.4355;

	static getCurrentTransportCost(when: Date) {
		const winter = PowerPrices.isItWinterPrice(when);
		const nightOrWeekend = PowerPrices.isItNightOrWeekendPrice(when);
		if (winter) {
			if (nightOrWeekend) {
				return PowerPrices.WINTER_NIGHT_OR_WEEKEND_PRICE;
			} else {
				return PowerPrices.WINTER_DAY_PRICE;
			}
		} else {
			if (nightOrWeekend) {
				return PowerPrices.SUMMER_NIGHT_OR_WEEKEND_PRICE;
			} else {
				return PowerPrices.SUMMER_DAY_PRICE;
			}
		}
	}

	getPowerPrice()  {
		return 0;
	}

	static getCurrentPriceAfterSupport(price: number, usedThisMonthSoFar = 0) {
		// If we are under usage threshold and over price thtreshold, we get a discount
		let support = 0;
		if (
			usedThisMonthSoFar < PowerPrices.SUPPORT_CUTOFF_USAGE &&
			price > PowerPrices.SUPPORT_ENTRY_PRICE
		) {
			support = (price - PowerPrices.SUPPORT_ENTRY_PRICE) * 0.9;
		}
		console.table({
			price,
			support,
			usedThisMonthSoFar,
			after: price - support,
		});
		return price - support;
	}

	/**
	 * Correct for night or weekend prices
	 * @param time
	 * @returns
	 */
	static isItNightOrWeekendPrice(time: Date) {
		const hour = time.getHours();
		const day = time.getDay();
		// Weekend
		if (day === 0 || day === 6) {
			return true;
		}
		// Night
		if ((hour >= 0 && hour <= 5) || (hour >= 22 && hour <= 23)) {
			return true;
		}
		return false;
	}

	/**
	 * Correct for winter price
	 * @param time
	 * @returns
	 */
	static isItWinterPrice(time: Date) {
		const month = time.getMonth();
		if (month >= 0 && month <= 2) {
			return true;
		}
		return false;
	}
}
