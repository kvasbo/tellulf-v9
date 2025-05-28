import * as z from 'zod';

export const InstantSchema = z.object({
	details: z.object({
		air_pressure_at_sea_level: z.number().optional(),
		air_temperature: z.number().optional(),
		air_temperature_percentile_10: z.number().optional(),
		air_temperature_percentile_90: z.number().optional(),
		cloud_area_fraction: z.number().optional(),
		cloud_area_fraction_high: z.number().optional(),
		cloud_area_fraction_low: z.number().optional(),
		cloud_area_fraction_medium: z.number().optional(),
		dew_point_temperature: z.number().optional(),
		fog_area_fraction: z.number().optional(),
		relative_humidity: z.number().optional(),
		ultraviolet_index_clear_sky: z.number().optional(),
		wind_from_direction: z.number().optional(),
		wind_speed: z.number().optional(),
		wind_speed_of_gust: z.number().optional(),
		wind_speed_percentile_10: z.number().optional(),
		wind_speed_percentile_90: z.number().optional()
	})
});

export const Next1HourSchema = z.object({
	summary: z.object({
		symbol_code: z.string()
	}),
	details: z.object({
		precipitation_amount: z.number(),
		precipitation_amount_max: z.number(),
		precipitation_amount_min: z.number(),
		probability_of_precipitation: z.number(),
		probability_of_thunder: z.number()
	})
});

export const Next6HoursSchema = z.object({
	summary: z.object({
		symbol_code: z.string()
	}),
	details: z.object({
		air_temperature_max: z.number(),
		air_temperature_min: z.number(),
		precipitation_amount: z.number(),
		precipitation_amount_max: z.number(),
		precipitation_amount_min: z.number(),
		probability_of_precipitation: z.number()
	})
});

export const Next12HoursSchema = z.object({
	summary: z.object({
		symbol_code: z.string(),
		symbol_confidence: z.string()
	}),
	details: z.object({
		probability_of_precipitation: z.number()
	})
});

export const TimeSeriesSchema = z.object({
	time: z.string().datetime({ offset: true }),
	data: z.object({
		instant: InstantSchema,
		next_1_hours: Next1HourSchema.optional(),
		next_6_hours: Next6HoursSchema.optional(),
		next_12_hours: Next12HoursSchema.optional()
	})
});

export const YrCompleteResponseSchema = z.object({
	type: z.string(),
	geometry: z.object({
		type: z.string(),
		coordinates: z.array(z.number())
	}),
	properties: z.object({
		meta: z.object({
			updated_at: z.string(),
			units: z.object({
				air_pressure_at_sea_level: z.string().optional(),
				air_temperature: z.string().optional(),
				air_temperature_max: z.string().optional(),
				air_temperature_min: z.string().optional(),
				air_temperature_percentile_10: z.string().optional(),
				air_temperature_percentile_90: z.string().optional(),
				cloud_area_fraction: z.string().optional(),
				cloud_area_fraction_high: z.string().optional(),
				cloud_area_fraction_low: z.string().optional(),
				cloud_area_fraction_medium: z.string().optional(),
				dew_point_temperature: z.string().optional(),
				fog_area_fraction: z.string().optional(),
				precipitation_amount: z.string().optional(),
				precipitation_amount_max: z.string().optional(),
				precipitation_amount_min: z.string().optional(),
				probability_of_precipitation: z.string().optional(),
				probability_of_thunder: z.string().optional(),
				relative_humidity: z.string().optional(),
				ultraviolet_index_clear_sky: z.string().optional(),
				wind_from_direction: z.string().optional(),
				wind_speed: z.string().optional(),
				wind_speed_of_gust: z.string().optional(),
				wind_speed_percentile_10: z.string().optional(),
				wind_speed_percentile_90: z.string().optional()
			})
		}),
		timeseries: z.array(TimeSeriesSchema)
	})
});

// Define types
const LongTermForecastDayDetailsSchema = z.object({
	air_temperature_max: z.number(),
	air_temperature_max_percentile_10: z.number(),
	air_temperature_max_percentile_90: z.number(),
	air_temperature_mean: z.number(),
	air_temperature_mean_percentile_10: z.number(),
	air_temperature_mean_percentile_90: z.number(),
	air_temperature_min: z.number(),
	air_temperature_min_percentile_10: z.number(),
	air_temperature_min_percentile_90: z.number(),
	precipitation_amount: z.number(),
	precipitation_amount_percentile_10: z.number(),
	precipitation_amount_percentile_90: z.number(),
	probability_of_frost: z.number(),
	probability_of_heavy_precipitation: z.number(),
	probability_of_precipitation: z.number()
});

const LongTermForecastDay7DaysDetailsSchema = z.object({
	precipitation_amount: z.number(),
	precipitation_amount_percentile_10: z.number(),
	precipitation_amount_percentile_90: z.number(),
	probability_of_frost: z.number()
});

const LongTermForecastDaySchema = z.object({
	time: z.string(),
	data: z.object({
		next_24_hours: z.object({
			details: LongTermForecastDayDetailsSchema
		}),
		next_7_days: z
			.object({
				details: LongTermForecastDay7DaysDetailsSchema
			})
			.optional()
	})
});

export const LongTermForecastSchema = z.object({
	properties: z.object({
		timeseries: z.array(LongTermForecastDaySchema)
	})
});

// Infer TypeScript types from Zod schemas
export type Instant = z.infer<typeof InstantSchema>;
export type Next1Hour = z.infer<typeof Next1HourSchema>;
export type Next6Hours = z.infer<typeof Next6HoursSchema>;
export type Next12Hours = z.infer<typeof Next12HoursSchema>;
export type TimeSeries = z.infer<typeof TimeSeriesSchema>;
export type YrCompleteResponse = z.infer<typeof YrCompleteResponseSchema>;
export type LongTermForecastDay = z.infer<typeof LongTermForecastDaySchema>;
export type LongTermForecast = z.infer<typeof LongTermForecastSchema>;