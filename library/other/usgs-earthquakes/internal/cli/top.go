// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newTopCmd(flags *rootFlags) *cobra.Command {
	var (
		window  string
		limit   int
		score   string
		minMag  float64
		dataSrc string
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank recent events by composite editorial score (sig × alert × felt × tsunami)",
		Long: `Rank recent events by a composite editorial score:
  composite = sig × alertWeight(alert) × (1 + ln(1+felt)) × (1 + 2*tsunami)

Useful when "events that matter right now" beats "events by raw magnitude".
Defaults to a 24h window, top 10. Reads from the local store first, falls
back to live FDSN.`,
		Example: strings.Trim(`
  # Top 10 events in the past 24h by composite editorial score
  usgs-earthquakes-pp-cli top --window 24h --limit 10 --json

  # By raw significance only (FDSN's sig score)
  usgs-earthquakes-pp-cli top --window 7d --limit 20 --score sig --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			startT, err := parseSinceArg(window)
			if err != nil {
				return usageErr(err)
			}

			events, err := loadTopEvents(ctx, flags, startT, minMag, dataSrc)
			if err != nil {
				return err
			}

			for i := range events {
				switch score {
				case "sig":
					events[i].Score = float64(events[i].Sig)
				default:
					events[i].Score = round2(compositeScore(float64(events[i].Sig), events[i].Alert, events[i].Felt, events[i].Tsunami))
				}
			}
			sort.Slice(events, func(i, j int) bool { return events[i].Score > events[j].Score })
			if limit > 0 && len(events) > limit {
				events = events[:limit]
			}

			out := map[string]any{
				"window_start": startT.Format(time.RFC3339),
				"score_type":   score,
				"count":        len(events),
				"events":       events,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "SCORE\tID\tMAG\tALERT\tFELT\tTSU\tPLACE\tTIME")
			for _, e := range events {
				ts := time.Unix(e.TimeMs/1000, 0).UTC().Format(time.RFC3339)
				alert := e.Alert
				if alert == "" {
					alert = "-"
				}
				tsu := "-"
				if e.Tsunami != 0 {
					tsu = "Y"
				}
				fmt.Fprintf(w, "%.1f\t%s\tM%.1f\t%s\t%d\t%s\t%s\t%s\n",
					e.Score, e.ID, e.Mag, alert, e.Felt, tsu, e.Place, ts)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&window, "window", "24h", "Lookback window (24h, 7d, 30d, or ISO 8601 timestamp)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max events to return")
	cmd.Flags().StringVar(&score, "score", "composite", "Score type: composite | sig")
	cmd.Flags().Float64Var(&minMag, "min-mag", 0, "Skip events below this magnitude")
	cmd.Flags().StringVar(&dataSrc, "data-source", "auto", "Data source: auto, live, local")
	return cmd
}

type topEvent struct {
	ID      string  `json:"id"`
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	TimeMs  int64   `json:"time_ms"`
	Alert   string  `json:"alert"`
	Felt    int64   `json:"felt"`
	Sig     int64   `json:"sig"`
	Tsunami int64   `json:"tsunami"`
	MMI     float64 `json:"mmi"`
	Score   float64 `json:"score"`
}

func loadTopEvents(ctx context.Context, flags *rootFlags, startT time.Time, minMag float64, dataSrc string) ([]topEvent, error) {
	if dataSrc != "live" {
		db, err := openLocalStore(ctx)
		if err == nil {
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx, `
				SELECT id,
				       json_extract(data, '$.properties.mag')     AS mag,
				       json_extract(data, '$.properties.place')   AS place,
				       json_extract(data, '$.properties.time')    AS t,
				       json_extract(data, '$.properties.alert')   AS alert,
				       json_extract(data, '$.properties.felt')    AS felt,
				       json_extract(data, '$.properties.sig')     AS sig,
				       json_extract(data, '$.properties.tsunami') AS tsunami,
				       json_extract(data, '$.properties.mmi')     AS mmi
				FROM resources
				WHERE resource_type='events'
				  AND CAST(t AS INTEGER) >= ?
			`, startT.UnixMilli())
			if err == nil {
				defer rows.Close()
				var events []topEvent
				for rows.Next() {
					var id sql.NullString
					var mag, mmi sql.NullFloat64
					var place, alert sql.NullString
					var t, felt, sig, tsunami sql.NullInt64
					if rows.Scan(&id, &mag, &place, &t, &alert, &felt, &sig, &tsunami, &mmi) != nil {
						continue
					}
					if mag.Valid && mag.Float64 < minMag {
						continue
					}
					events = append(events, topEvent{
						ID:      id.String,
						Mag:     mag.Float64,
						Place:   place.String,
						TimeMs:  t.Int64,
						Alert:   alert.String,
						Felt:    felt.Int64,
						Sig:     sig.Int64,
						Tsunami: tsunami.Int64,
						MMI:     mmi.Float64,
					})
				}
				if len(events) > 0 || dataSrc == "local" {
					return events, nil
				}
			}
		}
		if dataSrc == "local" {
			return nil, nil
		}
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// Order by time (not magnitude) so the post-fetch composite ranking
	// sees every event in the window — a widely-felt M3.8 in an urban area
	// can outscore a remote M5.5, and a magnitude-ordered prefix of 20000
	// would silently exclude the felt M3.8 during active swarms. FDSN has
	// no server-side significance ordering, so this is the smallest fetch
	// shape that preserves the ranking semantics. Cap at FDSN's hard
	// limit of 20000 per request.
	params := map[string]string{
		"format":    "geojson",
		"starttime": fdsnTimeFormat(startT),
		"orderby":   "time",
		"limit":     "20000",
	}
	if minMag > 0 {
		params["minmagnitude"] = strconv.FormatFloat(minMag, 'f', -1, 64)
	}
	data, err := c.Get("/query", params)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var fc struct {
		Features []map[string]any `json:"features"`
	}
	if json.Unmarshal(data, &fc) != nil {
		return nil, fmt.Errorf("parse top response")
	}
	var events []topEvent
	for _, f := range fc.Features {
		props, _ := f["properties"].(map[string]any)
		id, _ := f["id"].(string)
		mag, _ := props["mag"].(float64)
		place, _ := props["place"].(string)
		t, _ := props["time"].(float64)
		alert, _ := props["alert"].(string)
		feltF, _ := props["felt"].(float64)
		sigF, _ := props["sig"].(float64)
		tsuF, _ := props["tsunami"].(float64)
		mmiF, _ := props["mmi"].(float64)
		events = append(events, topEvent{
			ID:      id,
			Mag:     mag,
			Place:   place,
			TimeMs:  int64(t),
			Alert:   alert,
			Felt:    int64(feltF),
			Sig:     int64(sigF),
			Tsunami: int64(tsuF),
			MMI:     mmiF,
		})
	}
	return events, nil
}
