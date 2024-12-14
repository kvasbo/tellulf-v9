import { XMLParser } from "fast-xml-parser";

export class Entur {
  trains = [];

  constructor() {
    this.Update();
    setInterval(() => {
      this.Update();
    }, 60000);
  }

  getTrains() {
    return this.trains;
  }

  async Update() {
    try {
      const parser = new XMLParser();
      const xmlData = await fetch(
        "https://api.entur.io/realtime/v1/rest/et?datasetId=RUT",
      );
      const text = await xmlData.text();
      const parsed = parser.parse(text);
      const trips =
        parsed.Siri.ServiceDelivery.EstimatedTimetableDelivery
          .EstimatedJourneyVersionFrame.EstimatedVehicleJourney;
      const filteredTrips = trips.filter((trip) => {
        if (
          !trip.EstimatedCalls?.EstimatedCall ||
          trip.EstimatedCalls.EstimatedCall.length === 0
        ) {
          return false;
        }
        if (trip.LineRef !== "RUT:Line:1" || trip.DirectionRef !== 1) {
          return false;
        }
        if (
          !Array.isArray(trip.EstimatedCalls.EstimatedCall) ||
          trip.EstimatedCalls.EstimatedCall.filter(
            (call) => call.StopPointName === "Slemdal",
          ).length === 0
        ) {
          return false;
        }
        return true;
      });
      const filteredTrains = filteredTrips.map((trip) => {
        const found = trip.EstimatedCalls.EstimatedCall.find(
          (stop) => stop.StopPointName === "Slemdal",
        );
        return {
          time: found.ExpectedDepartureTime,
          destination: found.DestinationDisplay,
        };
      });

      this.trains = filteredTrains.sort((a, b) => {
        return new Date(a.time) - new Date(b.time);
      });

      console.log(`Entur updated with ${this.trains.length} trains`);
    } catch (error) {
      console.error("Error: ", error);
      this.trains = [];
    }
  }
}
