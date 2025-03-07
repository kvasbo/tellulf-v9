import { TibberFeed, TibberQuery, type IConfig } from 'tibber-api';
import { env } from '$env/dynamic/private';
import { Places } from '$lib/Enums';

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
	currentPrice: 0
};

export class Tibber {
	private readonly feedHome: TibberFeed;
	private readonly feedCabin: TibberFeed;
	private readonly data: { home: PowerData; cabin: PowerData } = {
		home: { ...initValues },
		cabin: { ...initValues }
	};
	private readonly query: TibberQuery;

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
		this.feedHome = new TibberFeed(new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_HOME }));
		this.feedCabin = new TibberFeed(
			new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_CABIN })
		);

		this.setupFeed(this.feedHome, Places.Home);
		this.setupFeed(this.feedCabin, Places.Cabin);

		// Connect feeds
		this.feedHome.connect();
		this.feedCabin.connect();

		// Start power price fetching loop
		this.updatePowerPrice(Places.Home);
		this.updatePowerPrice(Places.Cabin);

		// Save the interval ID for power price updates
		this.priceUpdateInterval = setInterval(() => {
			this.updatePowerPrice(Places.Home);
			this.updatePowerPrice(Places.Cabin);
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
			this.data[where].accumulatedCost = data.accumulatedCost - data.accumulatedReward;
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

	private async updatePowerPrice(where: Places) {
		try {
			const id = where === 'home' ? env.TIBBER_ID_HOME : env.TIBBER_ID_CABIN;
			const price = await this.query.getCurrentEnergyPrice(id);
			this.data[where].currentPrice = price.total;
			console.log(`Updated power price for ${where} to ${price.total}`);
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
