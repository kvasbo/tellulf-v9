import * as mqtt from 'mqtt';
import { MQTT_HOST, MQTT_USER, MQTT_PASS } from '$env/static/private';

const options = {
	username: MQTT_USER,
	password: MQTT_PASS,
	clientId: 'tellulf-' + Math.random().toString(16).substring(2, 8),
	keepalive: 15
};

export class MqttClient {
	// Connect to MQTT broker
	constructor() {
		this.client = mqtt.connect(MQTT_HOST, options);
		this.client
			.on('connect', () => {
				this.log(`${options.clientId} connected to ${MQTT_HOST}`);
				this.client.subscribe('#');
				this.client.publish('tellulf/poll', 'Tellulf is online and polling');
			})
			.on('error', (error) => {
				console.log('MQTT Error', error.message);
				this.client.end();
				this.client.reconnect();
			})
			.on('reconnect', () => {
				console.log('MQTT reconnecting');
			});
	}

	/**
	 * Publish a message to the MQTT broker
	 * @param topic
	 * @param message
	 */
	publish(topic, message) {
		if (message !== null && message !== undefined) {
			this.client.publish(topic, message.toString());
		}
	}

	/**
	 * Just a central place to log MQTT messages!
	 * @param message
	 * @param value
	 */
	log(message, value = '') {
		const d = new Date();
		const t = d.toLocaleString('nb-NO', { timeZone: 'Europe/Oslo' });
		console.log(t, 'MQTT ' + message, value);
	}
}
