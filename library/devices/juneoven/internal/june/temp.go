package june

import "math"

// The oven works in milli-degrees Celsius on the wire.

// FahrenheitToMilliC converts °F to milli-°C (350°F -> 176667).
func FahrenheitToMilliC(f float64) int {
	return int(math.Round((f - 32) * 5 / 9 * 1000))
}

// MilliCToFahrenheit converts milli-°C to °F (176667 -> 350).
func MilliCToFahrenheit(mc int) int {
	return int(math.Round(float64(mc)/1000*9/5 + 32))
}

// CelsiusToMilliC converts °C to milli-°C.
func CelsiusToMilliC(c float64) int {
	return int(math.Round(c * 1000))
}

// MilliCToCelsius converts milli-°C to °C (52000 -> 52).
func MilliCToCelsius(mc int) int {
	return int(math.Round(float64(mc) / 1000))
}
