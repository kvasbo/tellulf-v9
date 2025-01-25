import { TibberFeed, TibberQuery, TibberQueryBase, type IConfig } from 'tibber-api';
import { env } from '$env/dynamic/private';

interface powerData {
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
	currentPrice: number;
}

const initValues: powerData = {
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
	currentPrice: 0
};

export class Tibber {
	private feedHome: TibberFeed;
	private feedCabin: TibberFeed;
	private data: { home: powerData; cabin: powerData } = {
		home: { ...initValues },
		cabin: { ...initValues }
	};
	private query: TibberQuery;

	private config: IConfig = {
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

	public getPowerData() {
		return this.data;
	}

	private setupFeed(feed: TibberFeed, where: 'home' | 'cabin' = 'home') {
		feed.on('data', (data) => {
			this.data[where].timestamp = new Date().toISOString();
			this.data[where].accumulatedConsumption = data.accumulatedConsumption;
			this.data[where].currentPower = data.power;
			this.data[where].maxPower = data.maxPower;
		});

		feed.on('error', (error) => {
			console.error('Feed Error:', error);
		});

		feed.on('connected', () => {
			console.log(`${where} connected to Tibber`);
		});

		feed.on('disconnected', () => {
			console.log(`${where} disconnected from Tibber`);
			setTimeout(() => feed.connect(), 10000);
		});
	}

	constructor() {
		this.query = new TibberQuery(this.config);
		this.feedHome = new TibberFeed(new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_HOME }));

		this.feedCabin = new TibberFeed(
			new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_CABIN })
		);

		this.setupFeed(this.feedHome, 'home');
		this.setupFeed(this.feedCabin, 'cabin');

		this.feedHome.connect();
		this.feedCabin.connect();

		// Start power price fetching loop
		this.updatePowerPrice('home');
		this.updatePowerPrice('cabin');

		setInterval(() => {
			this.updatePowerPrice('home');
			this.updatePowerPrice('cabin');
		}, 60000);
	}

	private async updatePowerPrice(where: 'home' | 'cabin') {
		const id = where === 'home' ? env.TIBBER_ID_HOME : env.TIBBER_ID_CABIN;
		const price = await this.query.getCurrentEnergyPrice(id);
		this.data[where].currentPrice = price.total;
		console.log(`Updated power price for ${where} to ${price.total}`);
	}
}
