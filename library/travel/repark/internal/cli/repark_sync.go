// pp:data-source live
package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
)

var reparkRangeRE = regexp.MustCompile(`^C(-?[0-9]+(?:\.[0-9]+)?),(-?[0-9]+(?:\.[0-9]+)?)N(-?[0-9]+(?:\.[0-9]+)?)W(-?[0-9]+(?:\.[0-9]+)?)S(-?[0-9]+(?:\.[0-9]+)?)E(-?[0-9]+(?:\.[0-9]+)?)$`)

func validateReparkSyncParams(p *syncUserParams) error {
	v := map[string]string{}
	p.applyTo("site", v, false)
	// An operator-supplied environment window is a reusable explicit scope.
	// Flags take precedence, including an explicitly empty (invalid) range.
	if _, supplied := v["range"]; !supplied {
		if window := strings.TrimSpace(os.Getenv("REPARK_SYNC_RANGE")); window != "" {
			p.flatGlobal["range"] = window
			v["range"] = window
		}
	}
	m := reparkRangeRE.FindStringSubmatch(v["range"])
	if len(m) != 7 {
		return fmt.Errorf("sync requires an explicit bounded marker window: --param 'range=C34.663534,135.516310N34.664W135.515S34.663E135.517'; use parking commands for live planning facts")
	}
	values := make([]float64, 6)
	for i, s := range m[1:] {
		n, e := strconv.ParseFloat(s, 64)
		if e != nil {
			return fmt.Errorf("invalid source range coordinate")
		}
		values[i] = n
	}
	center := repark.Coordinates{Latitude: values[0], Longitude: values[1]}
	north, west, south, east := values[2], values[3], values[4], values[5]
	if err := repark.ValidateCoordinates(center); err != nil {
		return err
	}
	if !(south < center.Latitude && center.Latitude < north && west < center.Longitude && center.Longitude < east) {
		return fmt.Errorf("source range bounds must surround the explicitly supplied center")
	}
	for _, edge := range []repark.Coordinates{{Latitude: north, Longitude: center.Longitude}, {Latitude: south, Longitude: center.Longitude}, {Latitude: center.Latitude, Longitude: west}, {Latitude: center.Latitude, Longitude: east}} {
		if err := repark.ValidateCoordinates(edge); err != nil {
			return err
		}
		d := repark.DistanceM(center, edge)
		if d > 2100 {
			return fmt.Errorf("source snapshot range extends more than 2 km from the supplied center; reduce its bounds")
		}
	}
	return nil
}
