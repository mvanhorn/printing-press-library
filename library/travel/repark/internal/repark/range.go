package repark

import (
	"fmt"
	"regexp"
	"strconv"
)

var markerRangeRE = regexp.MustCompile(`^C(-?[0-9]+(?:\.[0-9]+)?),(-?[0-9]+(?:\.[0-9]+)?)N(-?[0-9]+(?:\.[0-9]+)?)W(-?[0-9]+(?:\.[0-9]+)?)S(-?[0-9]+(?:\.[0-9]+)?)E(-?[0-9]+(?:\.[0-9]+)?)$`)

// ValidateMarkerRange keeps every public marker interface within an explicit
// Japan window. The 100 m allowance accommodates rounded source map bounds.
func ValidateMarkerRange(window string) error {
	m := markerRangeRE.FindStringSubmatch(window)
	if len(m) != 7 {
		return fmt.Errorf("range requires an explicit bounded marker window in C(lat,lon)N(north)W(west)S(south)E(east) notation")
	}
	values := make([]float64, 6)
	for i, s := range m[1:] {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid source range coordinate")
		}
		values[i] = n
	}
	center := Coordinates{Latitude: values[0], Longitude: values[1]}
	north, west, south, east := values[2], values[3], values[4], values[5]
	if err := ValidateCoordinates(center); err != nil {
		return err
	}
	if !(south < center.Latitude && center.Latitude < north && west < center.Longitude && center.Longitude < east) {
		return fmt.Errorf("source range bounds must surround the explicitly supplied center")
	}
	for _, edge := range []Coordinates{{Latitude: north, Longitude: center.Longitude}, {Latitude: south, Longitude: center.Longitude}, {Latitude: center.Latitude, Longitude: west}, {Latitude: center.Latitude, Longitude: east}} {
		if err := ValidateCoordinates(edge); err != nil {
			return err
		}
		if DistanceM(center, edge) > 2100 {
			return fmt.Errorf("source range extends more than 2 km from the supplied center; reduce its bounds")
		}
	}
	return nil
}
