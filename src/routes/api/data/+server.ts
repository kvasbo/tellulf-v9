import { Calendar } from '$lib/server/Calendar';
import { Days } from '$lib/server/Days.mjs';
import { Smarthouse } from '$lib/server/Smarthouse.mjs';
import { MqttClient } from '$lib/server/MQTT';
import { Weather } from '$lib/server/Weather';
import { VERSION } from '$lib/server/version';

const weather = new Weather();
const calendar = Calendar.getInstance();
const days = new Days(calendar, weather);
const mqttClient = MqttClient.getInstance();
// Create smarthouse connector
const smart = new Smarthouse(mqttClient);
smart.startMqtt();

// For now, this is a kind of "all stuff comes from here" endpoint
// TODO: Separate concerns.
export async function GET(): Promise<Response> {
	try {
		const out = {
			days: days.generateComingDays(),
			currentTemperature: weather.getCurrentWeather().temperature,
			currentWeatherIcon: weather.getCurrentWeather().symbol,
			hourlyWeather: weather.getHourlyForecasts(),
			dangerData: weather.getDangerData(),
			homey: smart.getData(),
			currentWeather: weather.getCurrentWeather(),
			longTermForecast: weather.getDailyForecasts(),
			version: VERSION
		};
		return new Response(JSON.stringify(out), {
			headers: {
				'Content-Type': 'application/json'
			}
		});
	} catch (error: unknown) {
		const err = error as Error;
		console.error(err.message);
		return new Response(
			JSON.stringify({
				success: false,
				error: err.message
			}),
			{
				status: 500,
				headers: {
					'Content-Type': 'application/json'
				}
			}
		);
	}
}
