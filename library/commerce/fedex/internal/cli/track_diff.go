// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

type trackDiffPerNumber struct {
	TrackingNumber string           `json:"tracking_number"`
	NewEvents      []map[string]any `json:"new_events"`
	TotalEvents    int              `json:"total_events"`
	Error          string           `json:"error,omitempty"`
}

func newTrackDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:         "diff [trackingNumbers...]",
		Short:       "Poll tracking numbers and emit only events newly seen since the last poll",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  fedex-pp-cli track diff 794633071234 794633071235
  fedex-pp-cli track diff --since 24h --limit 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, _ := store.Open("")
			if st != nil {
				defer st.Close()
			}
			ctx := context.Background()

			nums := append([]string{}, args...)
			if len(nums) == 0 && st != nil {
				dur, perr := time.ParseDuration(since)
				if perr != nil {
					dur = 7 * 24 * time.Hour
				}
				cutoff := time.Now().Add(-dur)
				rows, qerr := st.DB().QueryContext(ctx, `
					SELECT tracking_number FROM shipments
					WHERE created_at >= ?
					ORDER BY created_at DESC
					LIMIT ?
				`, cutoff, limit)
				if qerr == nil {
					for rows.Next() {
						var n string
						if scanErr := rows.Scan(&n); scanErr == nil && n != "" {
							nums = append(nums, n)
						}
					}
					rows.Close()
				}
			}
			if len(nums) == 0 {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results := make([]trackDiffPerNumber, 0, len(nums))
			for _, n := range nums {
				results = append(results, pollTrackingDiff(ctx, c, st, n))
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "168h", "When no positional given, poll shipments created within this duration (default 7d)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max shipments to poll when no positional given")
	return cmd
}

func pollTrackingDiff(ctx context.Context, c interface {
	Post(path string, body any) (json.RawMessage, int, error)
}, st *store.Store, num string) trackDiffPerNumber {
	body := map[string]any{
		"includeDetailedScans": true,
		"trackingInfo": []any{
			map[string]any{
				"trackingNumberInfo": map[string]any{"trackingNumber": num},
			},
		},
	}
	data, _, err := c.Post("/track/v1/trackingnumbers", body)
	if err != nil {
		return trackDiffPerNumber{TrackingNumber: num, Error: err.Error()}
	}
	events := parseTrackingScanEvents(data)
	newEvents := []map[string]any{}
	for _, ev := range events {
		if st == nil {
			newEvents = append(newEvents, ev.JSON)
			continue
		}
		isNew, _ := st.InsertTrackingEvent(ctx, store.TrackingEvent{
			TrackingNumber:      num,
			EventTimestamp:      ev.Timestamp,
			EventType:           ev.EventType,
			EventDescription:    ev.Description,
			StatusCode:          ev.StatusCode,
			StatusLocale:        ev.StatusLocale,
			ScanLocationCity:    ev.City,
			ScanLocationState:   ev.State,
			ScanLocationCountry: ev.Country,
			DeliveryAttempts:    ev.DeliveryAttempts,
			Raw:                 string(ev.Raw),
		})
		if isNew {
			newEvents = append(newEvents, ev.JSON)
		}
	}
	if st != nil {
		_ = st.MarkPolled(ctx, "track:"+num)
	}
	return trackDiffPerNumber{
		TrackingNumber: num,
		NewEvents:      newEvents,
		TotalEvents:    len(events),
	}
}

type parsedScanEvent struct {
	Timestamp        time.Time
	EventType        string
	Description      string
	StatusCode       string
	StatusLocale     string
	City             string
	State            string
	Country          string
	DeliveryAttempts int
	JSON             map[string]any
	Raw              json.RawMessage
}

func parseTrackingScanEvents(data json.RawMessage) []parsedScanEvent {
	var resp struct {
		Output struct {
			CompleteTrackResults []struct {
				TrackResults []struct {
					ScanEvents []json.RawMessage `json:"scanEvents"`
				} `json:"trackResults"`
			} `json:"completeTrackResults"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	out := []parsedScanEvent{}
	for _, ctr := range resp.Output.CompleteTrackResults {
		for _, tr := range ctr.TrackResults {
			for _, raw := range tr.ScanEvents {
				var ev struct {
					Date              string `json:"date"`
					EventType         string `json:"eventType"`
					EventDescription  string `json:"eventDescription"`
					DerivedStatusCode string `json:"derivedStatusCode"`
					DerivedStatus     string `json:"derivedStatus"`
					ScanLocation      struct {
						City    string `json:"city"`
						State   string `json:"stateOrProvinceCode"`
						Country string `json:"countryCode"`
					} `json:"scanLocation"`
					DelivertyAttempts int `json:"deliveryAttempts"`
				}
				_ = json.Unmarshal(raw, &ev)
				ts, _ := time.Parse(time.RFC3339, ev.Date)
				asMap := map[string]any{}
				_ = json.Unmarshal(raw, &asMap)
				out = append(out, parsedScanEvent{
					Timestamp:        ts,
					EventType:        ev.EventType,
					Description:      ev.EventDescription,
					StatusCode:       ev.DerivedStatusCode,
					StatusLocale:     ev.DerivedStatus,
					City:             ev.ScanLocation.City,
					State:            ev.ScanLocation.State,
					Country:          ev.ScanLocation.Country,
					DeliveryAttempts: ev.DelivertyAttempts,
					JSON:             asMap,
					Raw:              raw,
				})
			}
		}
	}
	_ = fmt.Sprintf // keep fmt referenced even if all callers strip strings
	return out
}
