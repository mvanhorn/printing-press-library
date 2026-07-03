// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// streamGpsStdin reads "lon,lat" lines from stdin (CSV or whitespace) and
// emits one JSON line per input row to stdout. Errors per row are surfaced
// inline rather than aborting the stream.
func streamGpsStdin(cmd *cobra.Command, lookupOne func(lon, lat float64) (json.RawMessage, error), strict bool, flags *rootFlags) error {
	r := csv.NewReader(bufio.NewReader(cmd.InOrStdin()))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	out := cmd.OutOrStdout()
	enc := json.NewEncoder(out)

	rowNum := 0
	for {
		row, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			// Skip malformed CSV rows but report.
			fmt.Fprintf(cmd.ErrOrStderr(), "skip row %d: %v\n", rowNum, err)
			rowNum++
			continue
		}
		rowNum++
		if len(row) < 2 {
			// Also allow whitespace-separated "lon lat"
			joined := strings.Join(row, ",")
			parts := strings.Fields(joined)
			if len(parts) < 2 {
				_ = enc.Encode(map[string]any{"row": rowNum, "error": "need at least 2 numeric fields (lon, lat)"})
				continue
			}
			row = parts[:2]
		}
		lon, err1 := strconv.ParseFloat(strings.TrimSpace(row[0]), 64)
		lat, err2 := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err1 != nil || err2 != nil {
			_ = enc.Encode(map[string]any{"row": rowNum, "input": row, "error": "invalid coordinates"})
			continue
		}
		if !pointInItalyBBox(lon, lat) {
			_ = enc.Encode(map[string]any{"row": rowNum, "lon": lon, "lat": lat, "error": "outside Italy bbox"})
			continue
		}
		data, err := lookupOne(lon, lat)
		if err != nil {
			_ = enc.Encode(map[string]any{"row": rowNum, "lon": lon, "lat": lat, "error": err.Error()})
			continue
		}
		if strict && isEmptyAjaxResult(data) {
			_ = enc.Encode(map[string]any{"row": rowNum, "lon": lon, "lat": lat, "error": "no parcel found"})
			continue
		}
		// Wrap each row with input echo for downstream pipelines.
		var payload map[string]any
		_ = json.Unmarshal(data, &payload)
		envelope := map[string]any{
			"row":    rowNum,
			"lon":    lon,
			"lat":    lat,
			"result": payload,
		}
		_ = enc.Encode(envelope)
	}
}

// Ensure isEmptyAjaxResult, pointInItalyBBox live in gps.go and are
// reachable here in the same package.
var _ = json.Marshal
