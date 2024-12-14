import zod from "zod";
import { DateTime } from "luxon";

export class PowerPrice {
  // Validator for API data
  prisData = zod.array(
    zod.object({
      NOK_per_kWh: zod.number(),
      EUR_per_kWh: zod.number(),
      EXR: zod.number(),
      time_start: zod.string(),
      time_end: zod.string(),
    }),
  );

  powerPrice = -999;

  getPowerPrice() {
    return this.powerPrice;
  }

  constructor() {
    this.getData();
    setInterval(
      () => {
        this.getData();
      },
      1000 * 60 * 15,
    );
  }

  async getData() {
    try {
      const now = DateTime.now();
      const dateString = now.toFormat("yyyy/MM-dd");
      const url = `https://www.hvakosterstrommen.no/api/v1/prices/${dateString}_NO1.json`;
      const data = await fetch(url);

      if (!data.ok) {
        throw new Error(`Failed to fetch data from ${url}`);
      }

      const jsonData = await data.json();
      const validated = this.prisData.safeParse(jsonData);

      if (!validated.success) {
        throw new Error(
          `Failed to validate data", ${validated.error.flatten()}`,
        );
      }

      // Find the element in the array that contains the current time between time_start and time_end
      const currentPrice = validated.data.find((element) => {
        const start = DateTime.fromISO(element.time_start);
        const end = DateTime.fromISO(element.time_end);
        return start <= now && end >= now;
      });

      if (!currentPrice) {
        throw new Error("Could not find current power price");
      }

      const powerPrice = currentPrice.NOK_per_kWh * 1.25;

      console.log(`Current power price is ${powerPrice} NOK/kWh`);

      this.powerPrice = powerPrice;
    } catch (error) {
      console.error("Error fetching power price", error);
      this.powerPrice = -999;
    }
  }
}
