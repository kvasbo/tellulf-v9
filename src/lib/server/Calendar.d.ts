export interface Event {
	title: string;
	start: Date;
	end: Date;
	fullDay: boolean;
}

export interface EnrichedEvent extends Event {
	displayTitle: string;
	dayType: string;
	displayTime: { start: string; end: string; spacer: string };
	hyttaWeather?: {
		temperature: number;
		rainProbability: number;
	};
}
