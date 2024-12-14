import { Calendar } from '$lib/server/Calendar.mjs';
import { Days } from '$lib/server/Days.mjs';
import { Smarthouse } from '$lib/server/Smarthouse.mjs';
import { MqttClient } from '$lib/server/MQTT.mjs';
import { PowerPrice } from '$lib/server/PowerPrice.mjs';
import { Weather } from '$lib/server/Weather.mjs';

const calendar = new Calendar();
const days = new Days(calendar);
const mqttClient = new MqttClient();
// Create smarthouse connector
const smart = new Smarthouse(mqttClient);
smart.startMqtt();
const powerPriceGetter = new PowerPrice();
const weather = new Weather();

// For now, this is a kind of "all stuff comes from here" endpoint
// TODO: Separate concerns.
export async function GET(): Promise<Response> {
	try {
		const out = {
			days: days.generateComingDays(),
			currentTemperature: days.weather.getCurrentWeather().temperature,
			currentWeatherIcon: days.weather.getCurrentWeather().symbol,
			hourlyWeather: days.weather.getHourlyForecasts(),
			dangerData: days.weather.getDangerData(),
			homey: smart.getData(),
			powerPrice: powerPriceGetter.getPowerPrice(),
			currentWeather: weather.getCurrentWeather(),
			longTermForecast:  weather.getDailyForecasts(),
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
