import pg from "pg";
const { Client } = pg;
const client = new Client();

export class Database {
  constructor() {
    try {
      client.connect();
    } catch (error) {
      console.error("Error connecting to database", error);
    }
  }

  async putTemperaturePoint(temp) {
    try {
      if (!client._connected) {
        await client.connect();
      }
      const tempInt = parseInt(temp * 100);
      const query = {
        text: "INSERT INTO temperature (temperature) VALUES ($1)",
        values: [tempInt],
      };
      await client.query(query);
    } catch (error) {
      // Fail silently
      console.error("Error putting temperature point", error);
    }
  }
}
