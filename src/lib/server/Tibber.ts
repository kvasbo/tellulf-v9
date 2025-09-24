import { TibberFeed, TibberQuery, type IConfig } from 'tibber-api';
import { env } from '$env/dynamic/private';
import { Places } from '$lib/Enums';
import { ConsumptionTracker } from './ConsumptionTracker';
import { NorgesprisCalculator } from './NorgesprisCalculator';

// Define EnergyResolution enum since it's not exported from main tibber-api
enum EnergyResolution {
	HOURLY = 'HOURLY',
	DAILY = 'DAILY',
	WEEKLY = 'WEEKLY',
	MONTHLY = 'MONTHLY',
	ANNUAL = 'ANNUAL'
}

export interface PowerData {
	timestamp: string;
	accumulatedConsumption: number;
	currentPower: number;
	accumulatedProduction: number;
	accumulatedCost: number;
	accumulatedReward: number;
	minPower: number;
	averagePower: number;
	maxPower: number;
	powerProduction: number;
	minPowerProduction: number;
	maxPowerProduction: number;
	currentPrice: number;
	monthlyConsumption: number;
	subsidizedConsumption: number;
	marketConsumption: number;
	effectivePrice: number;
	cap: number;
}

const initValues: PowerData = {
	timestamp: '',
	accumulatedConsumption: 0,
	currentPower: 0,
	accumulatedProduction: 0,
	accumulatedCost: 0,
	accumulatedReward: 0,
	minPower: 0,
	averagePower: 0,
	maxPower: 0,
	powerProduction: 0,
	minPowerProduction: 0,
	maxPowerProduction: 0,
	currentPrice: 0,
	monthlyConsumption: 0,
	subsidizedConsumption: 0,
	marketConsumption: 0,
	effectivePrice: 0,
	cap: 0
};

export class Tibber {
	private readonly feedHome: TibberFeed;
	private readonly feedCabin: TibberFeed;
	private readonly data: { home: PowerData; cabin: PowerData } = {
		home: { ...initValues },
		cabin: { ...initValues }
	};
	private readonly query: TibberQuery;
	private readonly consumptionTracker: ConsumptionTracker;
	private readonly norgesprisCalculator: NorgesprisCalculator;
	private monthlyConsumptionCache: { home?: number; cabin?: number; lastFetch?: Date } = {};

	private lastCabinProduction = 0;

	// Resources that need cleanup
	private readonly priceUpdateInterval: NodeJS.Timeout;
	private readonly reconnectionTimers: NodeJS.Timeout[] = [];

	private readonly config: IConfig = {
		active: true,
		apiEndpoint: {
			apiKey: env.TIBBER_KEY,
			queryUrl: 'https://api.tibber.com/v1-beta/gql'
		},
		timestamp: true,
		power: true,
		accumulatedConsumption: true,
		accumulatedProduction: true,
		accumulatedCost: true,
		accumulatedReward: true,
		minPower: true,
		averagePower: true,
		maxPower: true,
		powerProduction: true,
		minPowerProduction: true,
		maxPowerProduction: true
	};

	constructor() {
		// Initialize TibberQuery and feeds
		this.query = new TibberQuery(this.config);
		this.consumptionTracker = new ConsumptionTracker();
		this.norgesprisCalculator = new NorgesprisCalculator();
		this.feedHome = new TibberFeed(new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_HOME }));
		this.feedCabin = new TibberFeed(
			new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_CABIN })
		);

		// Set initial caps
		this.data.home.cap = 5000;
		this.data.cabin.cap = 1000;

		this.setupFeed(this.feedHome, Places.Home);
		this.setupFeed(this.feedCabin, Places.Cabin);

		// Connect feeds
		this.feedHome.connect();
		this.feedCabin.connect();

		// Start power price fetching loop
		this.updatePowerPrice(Places.Home);
		this.updatePowerPrice(Places.Cabin);

		// Fetch initial monthly consumption after a short delay to ensure feeds are connected
		setTimeout(() => {
			this.updateMonthlyConsumption(Places.Home);
			this.updateMonthlyConsumption(Places.Cabin);
		}, 2000);

		// Save the interval ID for power price updates
		this.priceUpdateInterval = setInterval(() => {
			this.updatePowerPrice(Places.Home);
			this.updatePowerPrice(Places.Cabin);
			// Update monthly consumption every hour
			if (new Date().getMinutes() === 0) {
				this.updateMonthlyConsumption(Places.Home);
				this.updateMonthlyConsumption(Places.Cabin);
			}
		}, 60000);
	}

	public getPowerData(where: Places): PowerData {
		return this.data[where];
	}

	private setupFeed(feed: TibberFeed, where: Places) {
		feed.on('data', (data) => {
			this.data[where].timestamp = new Date().toISOString();
			this.data[where].accumulatedConsumption =
				data.accumulatedConsumption - data.accumulatedProduction;
			this.data[where].currentPower = data.power;
			this.data[where].maxPower = data.maxPower;
			this.data[where].minPower = data.minPower;
			this.data[where].averagePower = data.averagePower;
			this.data[where].powerProduction = data.powerProduction;
			this.data[where].minPowerProduction = data.minPowerProduction;
			this.data[where].maxPowerProduction = data.maxPowerProduction;
			this.data[where].accumulatedProduction = data.accumulatedProduction;

			// Ensure we have the latest monthly consumption for cost calculations
			const currentMonthlyConsumption = this.consumptionTracker.getMonthlyConsumption(where);
			if (currentMonthlyConsumption > 0) {
				this.data[where].monthlyConsumption = currentMonthlyConsumption;
			}

			// Calculate cost with Norgespris if active
			if (this.norgesprisCalculator.isNorgesprisActive()) {
				// For daily accumulated cost calculation with Norgespris
				const dayConsumption = this.data[where].accumulatedConsumption;
				const monthlyConsumption = this.consumptionTracker.getMonthlyConsumption(where);
				const spotPrice = this.data[where].currentPrice || 0;
				const cap = this.data[where].cap;

				// Calculate how much of today's consumption is subsidized vs market price
				// This assumes monthly consumption includes today's consumption so far
				const monthlyBeforeToday = Math.max(0, monthlyConsumption - dayConsumption);

				let subsidizedToday = 0;
				let marketToday = 0;

				if (monthlyBeforeToday >= cap) {
					// Already over cap before today, all of today is at market price
					marketToday = dayConsumption;
				} else {
					// Some or all of today might be subsidized
					const remainingSubsidized = cap - monthlyBeforeToday;
					subsidizedToday = Math.min(dayConsumption, remainingSubsidized);
					marketToday = Math.max(0, dayConsumption - subsidizedToday);
				}

				// Calculate total cost for today
				this.data[where].accumulatedCost = (subsidizedToday * 0.50) + (marketToday * spotPrice);

				// Update the effective price for display
				const effectivePrice = this.norgesprisCalculator.getEffectivePriceForCurrentHour(
					where,
					monthlyConsumption,
					spotPrice
				);
				this.data[where].effectivePrice = effectivePrice;
			} else {
				this.data[where].accumulatedCost = data.accumulatedCost - data.accumulatedReward;
			}

			this.data[where].accumulatedReward = data.accumulatedReward;

			if (where === 'cabin' && data.powerProduction !== null) {
				this.lastCabinProduction = data.powerProduction;
			}

			if (where === 'cabin' && data.power === 0) {
				this.data[where].currentPower = this.lastCabinProduction * -1;
			}
		});

		feed.on('error', (error) => {
			console.error('Feed Error:', error);
		});

		feed.on('connected', () => {
			console.log(`${where} connected to Tibber`);
		});

		feed.on('disconnected', () => {
			console.log(`${where} disconnected from Tibber`);
			const reconnectTimer = setTimeout(() => feed.connect(), 10000); // Reconnect attempt in 10 seconds
			this.reconnectionTimers.push(reconnectTimer); // Track the timer for later cleanup
		});
	}

	private async updateMonthlyConsumption(where: Places) {
		try {
			const now = new Date();
			const cacheValid = this.monthlyConsumptionCache.lastFetch &&
				(now.getTime() - this.monthlyConsumptionCache.lastFetch.getTime()) < 3600000;

			if (!cacheValid) {
				const id = where === 'home' ? env.TIBBER_ID_HOME : env.TIBBER_ID_CABIN;
				const daysInCurrentMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
				const currentDay = now.getDate();
				const hoursToFetch = currentDay * 24 + now.getHours();

				const consumption = await this.query.getConsumption(EnergyResolution.HOURLY, hoursToFetch, id);

				let monthlyTotal = 0;
				if (consumption && Array.isArray(consumption)) {
					monthlyTotal = consumption.reduce((sum: number, hour: any) => {
						return sum + (hour.consumption || 0);
					}, 0);
				}

				this.consumptionTracker.updateConsumption(where, monthlyTotal);
				this.monthlyConsumptionCache[where] = monthlyTotal;
				this.monthlyConsumptionCache.lastFetch = now;

				this.data[where].monthlyConsumption = monthlyTotal;
			} else {
				this.data[where].monthlyConsumption = this.monthlyConsumptionCache[where] || 0;
			}
		} catch (error) {
			console.error(`Failed to update monthly consumption for ${where}:`, error);
		}
	}

	private async updatePowerPrice(where: Places) {
		try {
			const id = where === 'home' ? env.TIBBER_ID_HOME : env.TIBBER_ID_CABIN;
			const price = await this.query.getCurrentEnergyPrice(id);
			const spotPrice = price.total || 0;

			// Get current monthly consumption
			const monthlyConsumption = this.data[where].monthlyConsumption || 0;

			// Calculate Norgespris values
			const calculation = this.norgesprisCalculator.calculateCost(
				where,
				monthlyConsumption,
				spotPrice
			);

			// Update data with Norgespris calculations
			this.data[where].currentPrice = spotPrice;
			this.data[where].effectivePrice = calculation.effectivePrice;
			this.data[where].subsidizedConsumption = calculation.subsidizedConsumption;
			this.data[where].marketConsumption = calculation.marketConsumption;

			console.log(`Updated power price for ${where}: spot=${spotPrice.toFixed(2)}, effective=${calculation.effectivePrice.toFixed(2)}`);
		} catch (error) {
			console.error(`Failed to update power price for ${where}:`, error);
		}
	}

	// Add a destroy method for cleanup
	public destroydestroy() {
		// Clear the price update interval
		clearInterval(this.priceUpdateInterval);

		// Clear reconnection timers
		this.reconnectionTimers.forEach(clearTimeout);

		// Disconnect feeds and remove event listeners
		this.feedCabin.close();
		this.feedHome.close();
		this.feedHome.removeAllListeners();
		this.feedCabin.removeAllListeners();

		console.log('Tibber instance destroyed and cleaned up');
	}
}
