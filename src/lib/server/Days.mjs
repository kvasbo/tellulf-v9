import { Weather } from "./Weather.mjs";
import { DateTime } from "luxon";

export class Days {
  // Cutoff for adding more days to the calendar
  maxCalendarHeight = 1100;

  constructor(calendar) {
    this.weather = new Weather();
    this.calendar = calendar;
  }

  generateComingDays(maxNumberOfDays = 10) {
    const days = [];
    let currentHeight = 0;
    for (let i = 0; i < maxNumberOfDays; i++) {
      const dt = DateTime.now().plus({ days: i });
      const day = this.getDataForDate(dt.toJSDate());
      // Check if we are overflowing
      if (currentHeight + day.displayHeight > this.maxCalendarHeight) {
        break;
      }
      currentHeight += day.displayHeight;
      days.push(day);
    }
    return days;
  }

  GenerateToday() {
    return this.getDataForDate(DateTime.now().toJSDate());
  }

  getDataForDate(jsDate) {
    // Create a Luxon DateTime object
    const dt = DateTime.fromJSDate(jsDate).setLocale("nb");

    const date = dt.toISODate()?.toString();

    const daily = this.weather.getDailyForecasts();

    const dayHeight = this.calendar.calculateDisplayHeightForDay(jsDate);

    return {
      isoDate: date,
      date: Days.createNiceDate(jsDate),
      weekday: Days.createNiceDate(jsDate, true),
      daily_forecast: daily[date],
      events: this.calendar.getEventsForDate(jsDate),
      birthdays: this.calendar.getBirthdaysForDate(jsDate),
      dinner: this.calendar.getDinnerForDate(jsDate),
      displayHeight: dayHeight,
    };
  }

  /**
   * Nicely format a date
   * To be switched to using Luxon
   * @param jsDate
   * @param relative
   */
  static createNiceDate(jsDate, relative = false) {
    const dt = DateTime.fromJSDate(jsDate).setLocale("nb").startOf("day");
    if (relative) {
      if (dt.hasSame(DateTime.local(), "day")) {
        return "i dag";
      } else if (dt.hasSame(DateTime.local().plus({ days: 1 }), "day")) {
        return "i morgen";
      }
    }
    return dt.toFormat("cccc d.");
  }
}
