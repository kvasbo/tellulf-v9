import * as mqtt from 'mqtt';
import { requireEnv } from './env.js';

const options = {
	username: process.env.MQTT_USER,
	password: process.env.MQTT_PASS,
	clientId: `tellulf-${Math.random().toString(16).substring(2, 8)}`,
	keepalive: 15,
};

// Singleton class to handle MQTT communication
export class MqttClient {
	public client: mqtt.MqttClient;
	private static instance: MqttClient;

	public static getInstance(): MqttClient {
		if (!MqttClient.instance) {
			MqttClient.instance = new MqttClient();
		}
		return MqttClient.instance;
	}

	// Connect to MQTT broker
	private constructor() {
		const mqttHost = requireEnv('MQTT_HOST');
		console.log('Connecting to MQTT host:', mqttHost);
		this.client = mqtt.connect(mqttHost, options);
		this.client
			.on('connect', () => {
				this.log(`${options.clientId} connected to ${mqttHost}`);
				this.client.subscribe('tellulf/weather/#');
				this.client.publish('tellulf/poll', 'Tellulf is online and polling');
			})
			.on('disconnect', () => {
				this.log('Disconnected');
			})
			.on('error', (error) => {
				console.log('MQTT error:', mqttHost, error);
			})
			.on('reconnect', () => {
				console.log(`MQTT reconnecting to ${mqttHost}`);
			});
	}

	/**
	 * Just a central place to log MQTT messages!
	 * @param message
	 * @param value
	 */
	log(message: string, value: string | number = '') {
		const d = new Date();
		const t = d.toLocaleString('nb-NO', { timeZone: 'Europe/Oslo' });
		console.log(t, `MQTT ${message}`, value);
	}
}
