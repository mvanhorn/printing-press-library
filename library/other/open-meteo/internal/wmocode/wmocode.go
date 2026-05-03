// Package wmocode translates WMO weather codes (table 4501) returned by
// Open-Meteo into human-readable descriptions.
//
// Open-Meteo emits weather_code as an integer (e.g., 0, 1, 61, 95). The full
// WMO 4501 catalog has hundreds of codes; Open-Meteo emits a curated subset
// covering the conditions a forecast/archive surface returns. The mapping
// here covers every value the docs list as a possible Open-Meteo response.
package wmocode

import "fmt"

// Describe returns the human-readable description for a WMO weather code, or
// "Code <n>" when the code is outside the curated subset.
func Describe(code int) string {
	if d, ok := descriptions[code]; ok {
		return d
	}
	return fmt.Sprintf("Code %d", code)
}

// All returns a copy of the curated WMO code-to-description map. The map is
// stable across calls.
func All() map[int]string {
	out := make(map[int]string, len(descriptions))
	for k, v := range descriptions {
		out[k] = v
	}
	return out
}

var descriptions = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	56: "Light freezing drizzle",
	57: "Dense freezing drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	66: "Light freezing rain",
	67: "Heavy freezing rain",
	71: "Slight snow fall",
	73: "Moderate snow fall",
	75: "Heavy snow fall",
	77: "Snow grains",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers",
	86: "Heavy snow showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

// Bucket returns a coarse category for a WMO code: "clear", "partly_cloudy",
// "overcast", "fog", "drizzle", "rain", "snow", "showers", "thunderstorm",
// or "unknown". Used by weather-mix to aggregate over time windows.
func Bucket(code int) string {
	switch {
	case code == 0 || code == 1:
		return "clear"
	case code == 2:
		return "partly_cloudy"
	case code == 3:
		return "overcast"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case code >= 61 && code <= 67:
		return "rain"
	case code >= 71 && code <= 77:
		return "snow"
	case code >= 80 && code <= 86:
		return "showers"
	case code >= 95 && code <= 99:
		return "thunderstorm"
	default:
		return "unknown"
	}
}
