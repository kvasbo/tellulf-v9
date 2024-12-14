import { DateTime } from "luxon";

/**
 * Clock class, with static method getTime, that returns an object with time, date and week.
 */
export class Clock {
  /**
   * Get time, date and week.
   * @param {number} time - Optional time in milliseconds.
   * @returns {object} - Object with time, date and week.
   * @property {string} time - Time in human readable format
   * @property {string} date - Date in human readable format
   * @property {string} week - Week number
   */
  static getTime(time = new Date().getTime()) {
    const dt = DateTime.fromMillis(time).setLocale("nb");
    return {
      time: dt.toLocaleString(DateTime.TIME_SIMPLE),
      date: dt.toFormat("cccc d."),
      week: dt.toFormat("W"),
    };
  }
}
