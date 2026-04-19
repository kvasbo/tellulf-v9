import { type IConfig, TibberFeed, TibberQuery } from 'tibber-api';
import { Places } from '../Enums.js';
import { ConsumptionTracker } from './ConsumptionTracker.js';
import { NorgesprisCalculator } from './NorgesprisCalculator.js';

// Define EnergyResolution enum since it's not exported from main tibber-api
enum EnergyResolution {
	HOURLY = 'HOURLY',
	DAILY = 'DAILY',
	WEEKLY = 'WEEKLY',
	MONTHLY = 'MONTHLY',
	ANNUAL = 'ANNUAL',
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
	monthlyCost: number;
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
	monthlyCost: 0,
	subsidizedConsumption: 0,
	marketConsumption: 0,
	effectivePrice: 0,
	cap: 0,
};

export class Tibber {
	private readonly feedHome: TibberFeed;
	private readonly feedCabin: TibberFeed;
	private readonly data: { home: PowerData; cabin: PowerData } = {
		home: { ...initValues },
		cabin: { ...initValues },
	};
	private readonly query: TibberQuery;
	private readonly consumptionTracker: ConsumptionTracker;
	private readonly norgesprisCalculator: NorgesprisCalculator;
	private monthlyConsumptionCache: {
		home?: number;
		cabin?: number;
		lastFetch?: Date;
	} = {};
	private monthlyCostCache: { home?: number; cabin?: number } = {};

	private lastCabinProduction = 0;

	// Connection management constants
	private static readonly STALE_THRESHOLD_MS = 5 * 60 * 1000;
	private static readonly HEALTH_CHECK_MS = 60 * 1000;
	private static readonly RECONNECT_VERIFY_MS = 45 * 1000;
	private static readonly HOURLY_RESET_MS = 60 * 60 * 1000;
	private static readonly RECONNECT_BASE_MS = 30 * 1000;
	private static readonly RECONNECT_MAX_MS = 10 * 60 * 1000;
	private static readonly MIN_CONNECT_GAP_MS = 60 * 1000;
	private static readonly HOURLY_RESET_FRESH_MS = 60 * 1000;

	// Resources that need cleanup
	private readonly priceUpdateInterval: NodeJS.Timeout;
	private readonly healthCheckInterval: NodeJS.Timeout;
	private readonly hourlyResetInterval: NodeJS.Timeout;
	private pendingReconnect: { home?: NodeJS.Timeout; cabin?: NodeJS.Timeout } =
		{};
	private verifyTimers: { home?: NodeJS.Timeout; cabin?: NodeJS.Timeout } = {};
	private reconnectAttempts: { home: number; cabin: number } = {
		home: 0,
		cabin: 0,
	};
	private lastDataReceived: { home: number; cabin: number } = {
		home: Date.now(),
		cabin: Date.now(),
	};
	private lastConnectAttempt: { home: number; cabin: number } = {
		home: 0,
		cabin: 0,
	};

	private readonly config: IConfig = {
		active: true,
		apiEndpoint: {
			apiKey: process.env.TIBBER_KEY!,
			queryUrl: 'https://api.tibber.com/v1-beta/gql',
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
		maxPowerProduction: true,
	};

	constructor() {
		// Initialize TibberQuery and feeds
		this.query = new TibberQuery(this.config);
		this.consumptionTracker = new ConsumptionTracker();
		this.norgesprisCalculator = new NorgesprisCalculator();
		this.feedHome = new TibberFeed(
			new TibberQuery({ ...this.config, homeId: process.env.TIBBER_ID_HOME! }),
		);
		this.feedCabin = new TibberFeed(
			new TibberQuery({ ...this.config, homeId: process.env.TIBBER_ID_CABIN! }),
		);

		// Set initial caps
		this.data.home.cap = 5000;
		this.data.cabin.cap = 1000;

		this.setupFeed(this.feedHome, Places.Home);
		this.setupFeed(this.feedCabin, Places.Cabin);

		// Connect feeds (with verification)
		this.connectFeed(this.feedHome, Places.Home);
		this.connectFeed(this.feedCabin, Places.Cabin);

		// Start power price fetching loop
		this.updatePowerPrice(Places.Home);
		this.updatePowerPrice(Places.Cabin);

		// Fetch initial monthly consumption after a short delay to ensure feeds are connected
		setTimeout(() => {
			this.updateMonthlyConsumption(Places.Home);
			this.updateMonthlyConsumption(Places.Cabin);
		}, 2000);

		// Health check: if no data received recently, force reconnect
		this.healthCheckInterval = setInterval(() => {
			const now = Date.now();
			if (now - this.lastDataReceived.home > Tibber.STALE_THRESHOLD_MS) {
				console.warn('Home feed stale, forcing reconnect');
				this.forceReconnect(this.feedHome, Places.Home);
			}
			if (now - this.lastDataReceived.cabin > Tibber.STALE_THRESHOLD_MS) {
				console.warn('Cabin feed stale, forcing reconnect');
				this.forceReconnect(this.feedCabin, Places.Cabin);
			}
		}, Tibber.HEALTH_CHECK_MS);

		// Hourly belt-and-suspenders: only reset feeds that haven't seen data recently
		this.hourlyResetInterval = setInterval(() => {
			const now = Date.now();
			if (now - this.lastDataReceived.home > Tibber.HOURLY_RESET_FRESH_MS) {
				console.log('Hourly Tibber reset (home stale)');
				this.forceReconnect(this.feedHome, Places.Home);
			}
			if (now - this.lastDataReceived.cabin > Tibber.HOURLY_RESET_FRESH_MS) {
				console.log('Hourly Tibber reset (cabin stale)');
				this.forceReconnect(this.feedCabin, Places.Cabin);
			}
		}, Tibber.HOURLY_RESET_MS);

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

	private forceReconnect(feed: TibberFeed, where: Places) {
		try {
			feed.close();
		} catch {
			// Ignore close errors
		}
		// Don't reset reconnectAttempts here — let the data handler reset it
		// once data actually flows. This preserves backoff during a ban.
		this.scheduleReconnect(feed, where);
	}

	private scheduleReconnect(feed: TibberFeed, where: Places) {
		// Coalesce: if a reconnect is already pending, leave it
		if (this.pendingReconnect[where]) return;

		const attempt = this.reconnectAttempts[where];
		// Backoff: 5s, 10s, 20s, 40s, 60s max
		const delay = Math.min(
			Tibber.RECONNECT_BASE_MS * 2 ** attempt,
			Tibber.RECONNECT_MAX_MS,
		);
		console.log(
			`Scheduling ${where} reconnect attempt ${attempt + 1} in ${delay / 1000}s`,
		);
		this.pendingReconnect[where] = setTimeout(() => {
			this.pendingReconnect[where] = undefined;
			this.reconnectAttempts[where]++;
			console.log(
				`Attempting ${where} reconnect (attempt ${this.reconnectAttempts[where]})`,
			);
			this.connectFeed(feed, where);
		}, delay);
	}

	private connectFeed(feed: TibberFeed, where: Places) {
		// Throttle: never call feed.connect() more than once per MIN_CONNECT_GAP_MS
		const now = Date.now();
		const elapsed = now - this.lastConnectAttempt[where];
		if (
			this.lastConnectAttempt[where] > 0 &&
			elapsed < Tibber.MIN_CONNECT_GAP_MS
		) {
			const wait = Tibber.MIN_CONNECT_GAP_MS - elapsed;
			console.log(
				`${where} connect throttled, deferring ${Math.round(wait / 1000)}s`,
			);
			if (this.pendingReconnect[where]) {
				clearTimeout(this.pendingReconnect[where]);
			}
			this.pendingReconnect[where] = setTimeout(() => {
				this.pendingReconnect[where] = undefined;
				this.connectFeed(feed, where);
			}, wait);
			return;
		}

		// Cancel any pending verification from a prior attempt
		if (this.verifyTimers[where]) {
			clearTimeout(this.verifyTimers[where]);
		}
		this.lastConnectAttempt[where] = now;
		feed.connect();
		// Verify data starts flowing; if not, schedule another reconnect
		this.verifyTimers[where] = setTimeout(() => {
			this.verifyTimers[where] = undefined;
			if (this.lastDataReceived[where] < this.lastConnectAttempt[where]) {
				console.warn(
					`${where} reconnect produced no data within ${Tibber.RECONNECT_VERIFY_MS / 1000}s, retrying`,
				);
				this.scheduleReconnect(feed, where);
			}
		}, Tibber.RECONNECT_VERIFY_MS);
	}

	private setupFeed(feed: TibberFeed, where: Places) {
		feed.on('data', (data) => {
			this.lastDataReceived[where] = Date.now();
			this.reconnectAttempts[where] = 0;
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

			// Update monthly consumption with realtime data
			// Use cached historical data plus current day's realtime consumption
			if (this.monthlyConsumptionCache[where] !== undefined) {
				const monthlyTotalBeforeToday =
					this.monthlyConsumptionCache[where] || 0;
				const currentDayConsumption =
					this.data[where].accumulatedConsumption || 0;
				const monthlyTotal = monthlyTotalBeforeToday + currentDayConsumption;

				this.data[where].monthlyConsumption = monthlyTotal;
				this.consumptionTracker.updateConsumption(where, monthlyTotal);
			}

			// Calculate cost with Norgespris if active
			if (this.norgesprisCalculator.isNorgesprisActive()) {
				// For daily accumulated cost calculation with Norgespris
				const dayConsumption = this.data[where].accumulatedConsumption;
				const monthlyConsumption =
					this.consumptionTracker.getMonthlyConsumption(where);
				const spotPrice = this.data[where].currentPrice || 0;
				const cap = this.data[where].cap;

				// Calculate how much of today's consumption is subsidized vs market price
				// This assumes monthly consumption includes today's consumption so far
				const monthlyBeforeToday = Math.max(
					0,
					monthlyConsumption - dayConsumption,
				);

				let subsidizedToday: number;
				let marketToday: number;

				if (monthlyBeforeToday >= cap) {
					// Already over cap before today, all of today is at market price
					subsidizedToday = 0;
					marketToday = dayConsumption;
				} else {
					// Some or all of today might be subsidized
					const remainingSubsidized = cap - monthlyBeforeToday;
					subsidizedToday = Math.min(dayConsumption, remainingSubsidized);
					marketToday = Math.max(0, dayConsumption - subsidizedToday);
				}

				// Calculate total cost for today
				this.data[where].accumulatedCost =
					subsidizedToday * 0.5 + marketToday * spotPrice;

				// Update the effective price for display
				const effectivePrice =
					this.norgesprisCalculator.getEffectivePriceForCurrentHour(
						where,
						monthlyConsumption,
						spotPrice,
					);
				this.data[where].effectivePrice = effectivePrice;
			} else {
				this.data[where].accumulatedCost =
					data.accumulatedCost - data.accumulatedReward;
			}

			this.data[where].accumulatedReward = data.accumulatedReward;

			if (this.monthlyCostCache[where] !== undefined) {
				const monthlyCostBeforeToday = this.monthlyCostCache[where] || 0;
				this.data[where].monthlyCost =
					monthlyCostBeforeToday + this.data[where].accumulatedCost;
			}

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
			this.scheduleReconnect(feed, where);
		});
	}

	private async updateMonthlyConsumption(where: Places) {
		try {
			const now = new Date();
			const cacheValid =
				this.monthlyConsumptionCache.lastFetch &&
				now.getTime() - this.monthlyConsumptionCache.lastFetch.getTime() <
					3600000;

			let monthlyTotalBeforeToday = 0;

			if (!cacheValid) {
				const id =
					where === 'home'
						? process.env.TIBBER_ID_HOME!
						: process.env.TIBBER_ID_CABIN!;
				const currentDay = now.getDate();
				const hoursToFetch = currentDay * 24 + now.getHours() + 3;

				const consumption = await this.query.getConsumption(
					EnergyResolution.HOURLY,
					hoursToFetch,
					id,
				);

				let monthlyCostBeforeToday = 0;

				if (consumption && Array.isArray(consumption)) {
					const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
					const startOfToday = new Date(
						now.getFullYear(),
						now.getMonth(),
						now.getDate(),
					);
					const cap = this.data[where].cap;
					const norgesprisActive =
						this.norgesprisCalculator.isNorgesprisActive();
					let accumulated = 0;

					const monthHours = consumption
						.filter((hour) => {
							const ts = new Date(hour.from);
							return ts >= startOfMonth && ts < startOfToday;
						})
						.sort((a, b) => {
							return new Date(a.from).getTime() - new Date(b.from).getTime();
						});

					for (const hour of monthHours) {
						const hourConsumption = hour.consumption || 0;
						const hourCost = hour.cost || 0;
						monthlyTotalBeforeToday += hourConsumption;

						if (norgesprisActive && cap > 0) {
							const previousAccumulated = accumulated;
							accumulated += hourConsumption;

							if (previousAccumulated >= cap) {
								monthlyCostBeforeToday += hourCost;
							} else if (accumulated <= cap) {
								monthlyCostBeforeToday += hourConsumption * 0.5;
							} else {
								const subsidizedPortion = cap - previousAccumulated;
								const marketPortion = hourConsumption - subsidizedPortion;
								const pricePerKwh =
									hourConsumption > 0 ? hourCost / hourConsumption : 0;
								monthlyCostBeforeToday +=
									subsidizedPortion * 0.5 + marketPortion * pricePerKwh;
							}
						} else {
							monthlyCostBeforeToday += hourCost;
						}
					}
				}

				this.monthlyConsumptionCache[where] = monthlyTotalBeforeToday;
				this.monthlyCostCache[where] = monthlyCostBeforeToday;
				this.monthlyConsumptionCache.lastFetch = now;
			} else {
				// Use cached value for consumption before today
				monthlyTotalBeforeToday = this.monthlyConsumptionCache[where] || 0;
			}

			const currentDayConsumption =
				this.data[where].accumulatedConsumption || 0;
			const monthlyTotal = monthlyTotalBeforeToday + currentDayConsumption;

			this.consumptionTracker.updateConsumption(where, monthlyTotal);
			this.data[where].monthlyConsumption = monthlyTotal;

			const monthlyCostBeforeToday = this.monthlyCostCache[where] || 0;
			this.data[where].monthlyCost =
				monthlyCostBeforeToday + this.data[where].accumulatedCost;
		} catch (error) {
			console.error(
				`Failed to update monthly consumption for ${where}:`,
				error,
			);
		}
	}

	private async updatePowerPrice(where: Places) {
		try {
			const id =
				where === 'home'
					? process.env.TIBBER_ID_HOME!
					: process.env.TIBBER_ID_CABIN!;
			const price = await this.query.getCurrentEnergyPrice(id);
			const spotPrice = price.total || 0;

			// Get current monthly consumption
			const monthlyConsumption = this.data[where].monthlyConsumption || 0;

			// Calculate Norgespris values
			const calculation = this.norgesprisCalculator.calculateCost(
				where,
				monthlyConsumption,
				spotPrice,
			);

			// Update data with Norgespris calculations
			this.data[where].currentPrice = spotPrice;
			this.data[where].effectivePrice = calculation.effectivePrice;
			this.data[where].subsidizedConsumption =
				calculation.subsidizedConsumption;
			this.data[where].marketConsumption = calculation.marketConsumption;

			console.log(
				`Updated power price for ${where}: spot=${spotPrice.toFixed(2)}, effective=${calculation.effectivePrice.toFixed(2)}`,
			);
		} catch (error) {
			console.error(`Failed to update power price for ${where}:`, error);
		}
	}

	// Add a destroy method for cleanup
	public destroy() {
		// Clear intervals
		clearInterval(this.priceUpdateInterval);
		clearInterval(this.healthCheckInterval);
		clearInterval(this.hourlyResetInterval);

		// Clear pending reconnect and verify timers
		if (this.pendingReconnect.home) clearTimeout(this.pendingReconnect.home);
		if (this.pendingReconnect.cabin) clearTimeout(this.pendingReconnect.cabin);
		if (this.verifyTimers.home) clearTimeout(this.verifyTimers.home);
		if (this.verifyTimers.cabin) clearTimeout(this.verifyTimers.cabin);

		// Disconnect feeds and remove event listeners
		this.feedCabin.close();
		this.feedHome.close();
		this.feedHome.removeAllListeners();
		this.feedCabin.removeAllListeners();

		console.log('Tibber instance destroyed and cleaned up');
	}
}
