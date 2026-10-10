package weather

import "time"

// The structs below describe just the parts of MET Norway's responses that
// Tellulf reads. encoding/json ignores everything else, and fails loudly if a
// field has the wrong type, which covers most of what the zod schemas did.

// locationforecast/2.0/complete
type forecastResponse struct {
	Properties struct {
		Timeseries []timeSeries `json:"timeseries"`
	} `json:"properties"`
}

type timeSeries struct {
	Time time.Time `json:"time"`
	Data struct {
		Instant struct {
			Details instant `json:"details"`
		} `json:"instant"`
		Next1Hours *struct {
			Summary struct {
				SymbolCode string `json:"symbol_code"`
			} `json:"summary"`
			Details struct {
				PrecipitationAmount        float64 `json:"precipitation_amount"`
				ProbabilityOfPrecipitation float64 `json:"probability_of_precipitation"`
			} `json:"details"`
		} `json:"next_1_hours"`
	} `json:"data"`
}

// Pointers mark the values MET may leave out.
type instant struct {
	AirTemperature    *float64 `json:"air_temperature"`
	WindFromDirection *float64 `json:"wind_from_direction"`
	WindSpeed         *float64 `json:"wind_speed"`
	WindSpeedOfGust   *float64 `json:"wind_speed_of_gust"`
}

// subseasonal/1.0/complete
type longTermResponse struct {
	Properties struct {
		Timeseries []longTermDay `json:"timeseries"`
	} `json:"properties"`
}

type longTermDay struct {
	Time time.Time `json:"time"`
	Data struct {
		Next24Hours struct {
			Details struct {
				AirTemperatureMax               float64 `json:"air_temperature_max"`
				AirTemperatureMin               float64 `json:"air_temperature_min"`
				ProbabilityOfPrecipitation      float64 `json:"probability_of_precipitation"`
				ProbabilityOfHeavyPrecipitation float64 `json:"probability_of_heavy_precipitation"`
			} `json:"details"`
		} `json:"next_24_hours"`
	} `json:"data"`
}
