import { DateTime } from 'luxon';
import type { MqttClient } from './MQTT.js';

export class Smarthouse {
	private mqttClient: MqttClient;

	constructor(mqttClient: MqttClient) {
		this.mqttClient = mqttClient;
	}

	temp = -9999;
	lastTempTime = 0;
	hum = -9999;
	pressure = 0;

	getData() {
		return {
			tempOut: this.temp,
			lastTempTime: this.lastTempTime,
			humOut: this.hum,
			pressure: this.pressure,
		};
	}

	startMqtt() {
		this.mqttClient.client.on('message', (topic: string, message: Buffer) => {
			switch (topic) {
				case 'tellulf/weather/tempOut':
					this.temp = parseFloat(message.toString());
					this.mqttClient.log('Temperature set to:', this.temp);
					break;
				case 'tellulf/weather/tempOutTime':
					this.lastTempTime = DateTime.fromFormat(
						message.toString(),
						'dd-MM-yyyy HH:mm',
					).toMillis();
					this.mqttClient.log(
						'Temperature set at time:',
						new Date(this.lastTempTime).toUTCString(),
					);
					break;
				case 'tellulf/weather/humidity':
					this.hum = parseFloat(message.toString());
					this.mqttClient.log('Humidity set to:', this.hum);
					break;
				case 'tellulf/weather/pressure':
					this.pressure = parseFloat(message.toString());
					this.mqttClient.log('Pressure set to:', this.pressure);
					break;
				default:
					break;
			}
		});
	}
}
