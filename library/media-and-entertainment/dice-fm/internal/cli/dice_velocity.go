// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "velocity show" command: cumulative ticket-sales pace for
// one event, bucketed by day or hour and offset from the on-sale datetime.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// velocityBucket is one time bucket in the sales-pace series.
type velocityBucket struct {
	Bucket         string `json:"bucket"`
	PeriodSold     int    `json:"period_sold"`
	CumulativeSold int    `json:"cumulative_sold"`
	HourOffset     int    `json:"hour_offset"`
}

const (
	bucketDay  = "day"
	bucketHour = "hour"
)

// eventOnSale reads the onSaleDatetime for an event from the events store. Returns
// "" when the event is not present or has no on-sale datetime.
func eventOnSale(ctx context.Context, db *sql.DB, eventID string) (string, error) {
	// The resources table is keyed by (resource_type, id), so look the event up
	// directly instead of scanning every synced event.
	var data string
	err := db.QueryRowContext(ctx,
		`SELECT data FROM resources WHERE resource_type = 'events' AND id = ?`, eventID).Scan(&data)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var e struct {
		ID             string `json:"id"`
		OnSaleDatetime string `json:"onSaleDatetime"`
	}
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		return "", nil
	}
	return e.OnSaleDatetime, nil
}

// bucketKey truncates an RFC3339 timestamp to the start of its day or hour
// bucket and returns both the bucket label and the bucket-start time. The bool
// reports whether the timestamp parsed.
func bucketKey(purchasedAt, bucket string) (label string, start time.Time, ok bool) {
	t, err := time.Parse(time.RFC3339, purchasedAt)
	if err != nil {
		return "", time.Time{}, false
	}
	t = t.UTC()
	if bucket == bucketHour {
		start = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
		return start.Format("2006-01-02T15:00Z"), start, true
	}
	start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return start.Format("2006-01-02"), start, true
}

// computeVelocity buckets an event's orders by day or hour, returning a
// chronologically sorted series with a monotonic cumulative count and an
// hour-offset from the event's on-sale datetime (0 when on-sale is unknown).
func computeVelocity(ctx context.Context, db *sql.DB, eventID, bucket string) ([]velocityBucket, error) {
	if bucket != bucketHour {
		bucket = bucketDay
	}
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	onSale, err := eventOnSale(ctx, db, eventID)
	if err != nil {
		return nil, err
	}
	var onSaleTime time.Time
	haveOnSale := false
	if onSale != "" {
		if t, perr := time.Parse(time.RFC3339, onSale); perr == nil {
			onSaleTime = t.UTC()
			haveOnSale = true
		}
	}

	type bucketAgg struct {
		start time.Time
		sold  int
	}
	buckets := map[string]*bucketAgg{}
	for _, o := range orders {
		if o.Event.ID != eventID {
			continue
		}
		label, start, ok := bucketKey(o.PurchasedAt, bucket)
		if !ok {
			continue
		}
		qty := o.Quantity
		if qty <= 0 {
			qty = 1
		}
		b := buckets[label]
		if b == nil {
			b = &bucketAgg{start: start}
			buckets[label] = b
		}
		b.sold += qty
	}

	labels := make([]string, 0, len(buckets))
	for l := range buckets {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool {
		return buckets[labels[i]].start.Before(buckets[labels[j]].start)
	})

	out := make([]velocityBucket, 0, len(labels))
	cumulative := 0
	for _, l := range labels {
		b := buckets[l]
		cumulative += b.sold
		offset := 0
		if haveOnSale {
			offset = int(b.start.Sub(onSaleTime).Hours())
		}
		out = append(out, velocityBucket{
			Bucket:         l,
			PeriodSold:     b.sold,
			CumulativeSold: cumulative,
			HourOffset:     offset,
		})
	}
	return out, nil
}

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Ticket-sales pace analytics from the local order store",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newVelocityShowCmd(flags))
	return cmd
}

// pp:data-source local
func newVelocityShowCmd(flags *rootFlags) *cobra.Command {
	var event, bucket string
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show cumulative ticket sales by day or hour relative to on-sale",
		Example:     "  dice-fm-pp-cli velocity show --event RXZlbnQ6MTIzNDU= --bucket day --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if event == "" {
				return fmt.Errorf("--event is required (the event whose sales pace you want)")
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []velocityBucket{}, flags)
			}
			defer s.Close()
			rows, err := computeVelocity(cmd.Context(), s.DB(), event, bucket)
			if err != nil {
				return fmt.Errorf("computing velocity: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Event ID to chart sales pace for (required)")
	cmd.Flags().StringVar(&bucket, "bucket", bucketDay, "Time bucket: day or hour")
	return cmd
}
