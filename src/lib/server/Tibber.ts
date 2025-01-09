import { TibberFeed, TibberQuery, TibberQueryBase, type IConfig } from 'tibber-api';
import { env } from '$env/dynamic/private';

export class Tibber {
	private feedHome: TibberFeed;
	private feedCabin: TibberFeed;
	private query: TibberQuery;

	private config: IConfig = {
		active: true,
		apiEndpoint: {
			apiKey: env.TIBBER_KEY,
			queryUrl: 'https://api.tibber.com/v1-beta/gql'
		}
	};

	constructor() {
		this.query = new TibberQuery(this.config);
		this.feedHome = new TibberFeed(new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_HOME }));
		this.feedCabin = new TibberFeed(new TibberQuery({ ...this.config, homeId: env.TIBBER_ID_CABIN }));
	}
}
