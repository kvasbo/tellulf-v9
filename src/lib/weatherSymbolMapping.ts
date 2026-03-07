type WeatherIconMapping = {
	[key: string]: string;
}

export const weatherIconMapping: WeatherIconMapping = {
	// Clear sky conditions
	'clearsky_day': 'clear-day',
	'clearsky_night': 'clear-night',
	'clearsky_polartwilight': 'clear-day', // Using day version as fallback

	// Cloudy conditions
	'cloudy': 'cloudy',
	'fair_day': 'partly-cloudy-day',
	'fair_night': 'partly-cloudy-night',
	'fair_polartwilight': 'partly-cloudy-day', // Using day version as fallback
	'partlycloudy_day': 'partly-cloudy-day',
	'partlycloudy_night': 'partly-cloudy-night',
	'partlycloudy_polartwilight': 'partly-cloudy-day', // Using day version as fallback

	// Fog
	'fog': 'fog',

	// Rain variations
	'heavyrain': 'extreme-rain',
	'heavyrainandthunder': 'thunderstorms-rain',
	'heavyrainshowers_day': 'extreme-day-rain',
	'heavyrainshowers_night': 'extreme-night-rain',
	'heavyrainshowers_polartwilight': 'extreme-day-rain',
	'heavyrainshowersandthunder_day': 'thunderstorms-day-rain',
	'heavyrainshowersandthunder_night': 'thunderstorms-night-rain',
	'heavyrainshowersandthunder_polartwilight': 'thunderstorms-day-rain',
	'lightrain': 'rain',
	'lightrainandthunder': 'thunderstorms-rain',
	'lightrainshowers_day': 'partly-cloudy-day-rain',
	'lightrainshowers_night': 'partly-cloudy-night-rain',
	'lightrainshowers_polartwilight': 'partly-cloudy-day-rain',
	'lightrainshowersandthunder_day': 'thunderstorms-day-rain',
	'lightrainshowersandthunder_night': 'thunderstorms-night-rain',
	'lightrainshowersandthunder_polartwilight': 'thunderstorms-day-rain',
	'rain': 'rain',
	'rainandthunder': 'thunderstorms-rain',
	'rainshowers_day': 'partly-cloudy-day-rain',
	'rainshowers_night': 'partly-cloudy-night-rain',
	'rainshowers_polartwilight': 'partly-cloudy-day-rain',
	'rainshowersandthunder_day': 'thunderstorms-day-rain',
	'rainshowersandthunder_night': 'thunderstorms-night-rain',
	'rainshowersandthunder_polartwilight': 'thunderstorms-day-rain',

	// Sleet variations
	'heavysleet': 'extreme-sleet',
	'heavysleetandthunder': 'thunderstorms-sleet',
	'heavysleetshowers_day': 'overcast-day-sleet',
	'heavysleetshowers_night': 'overcast-night-sleet',
	'heavysleetshowers_polartwilight': 'overcast-day-sleet',
	'heavysleetshowersandthunder_day': 'thunderstorms-day-sleet',
	'heavysleetshowersandthunder_night': 'thunderstorms-night-sleet',
	'heavysleetshowersandthunder_polartwilight': 'thunderstorms-day-sleet',
	'lightsleet': 'sleet',
	'lightsleetandthunder': 'thunderstorms-sleet',
	'lightsleetshowers_day': 'partly-cloudy-day-sleet',
	'lightsleetshowers_night': 'partly-cloudy-night-sleet',
	'lightsleetshowers_polartwilight': 'partly-cloudy-day-sleet',
	'lightssleetshowersandthunder_day': 'thunderstorms-day-sleet',
	'lightssleetshowersandthunder_night': 'thunderstorms-night-sleet',
	'lightssleetshowersandthunder_polartwilight': 'thunderstorms-day-sleet',
	'sleet': 'sleet',
	'sleetandthunder': 'thunderstorms-sleet',
	'sleetshowers_day': 'partly-cloudy-day-sleet',
	'sleetshowers_night': 'partly-cloudy-night-sleet',
	'sleetshowers_polartwilight': 'partly-cloudy-day-sleet',
	'sleetshowersandthunder_day': 'thunderstorms-day-sleet',
	'sleetshowersandthunder_night': 'thunderstorms-night-sleet',
	'sleetshowersandthunder_polartwilight': 'thunderstorms-day-sleet',

	// Snow variations
	'heavysnow': 'extreme-snow',
	'heavysnowandthunder': 'thunderstorms-snow',
	'heavysnowshowers_day': 'extreme-day-snow',
	'heavysnowshowers_night': 'extreme-night-snow',
	'heavysnowshowers_polartwilight': 'extreme-day-snow',
	'heavysnowshowersandthunder_day': 'thunderstorms-day-snow',
	'heavysnowshowersandthunder_night': 'thunderstorms-night-snow',
	'heavysnowshowersandthunder_polartwilight': 'thunderstorms-day-snow',
	'lightsnow': 'snow',
	'lightsnowandthunder': 'thunderstorms-snow',
	'lightsnowshowers_day': 'partly-cloudy-day-snow',
	'lightsnowshowers_night': 'partly-cloudy-night-snow',
	'lightsnowshowers_polartwilight': 'partly-cloudy-day-snow',
	'lightssnowshowersandthunder_day': 'thunderstorms-day-snow',
	'lightssnowshowersandthunder_night': 'thunderstorms-night-snow',
	'lightssnowshowersandthunder_polartwilight': 'thunderstorms-day-snow',
	'snow': 'snow',
	'snowandthunder': 'thunderstorms-snow',
	'snowshowers_day': 'partly-cloudy-day-snow',
	'snowshowers_night': 'partly-cloudy-night-snow',
	'snowshowers_polartwilight': 'partly-cloudy-day-snow',
	'snowshowersandthunder_day': 'thunderstorms-day-snow',
	'snowshowersandthunder_night': 'thunderstorms-night-snow',
	'snowshowersandthunder_polartwilight': 'thunderstorms-day-snow'
};

// Usage example:
// const mappedIcon = weatherIconMapping['clearsky_day']; // Returns 'clear-day'