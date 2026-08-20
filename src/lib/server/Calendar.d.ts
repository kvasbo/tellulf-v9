export type KidsStatus = 'full' | 'leaving' | 'arriving' | null;

export interface Event {
	title: string;
	start: Date;
	end: Date;
	fullDay: boolean;
	source?: 'felles' | 'audun';
}

export interface EnrichedEvent extends Event {
	displayTitle: string;
	dayType: string;
	displayTime: { start: string; end: string; spacer: string };
	faded: boolean;
	hyttaWeather?: {
		temperature: number;
		rainProbability: number;
	};
}
