// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
	"github.com/spf13/cobra"
)

type webhookDeliverySummary struct {
	ObservationID    int64
	EventID          string
	EventType        string
	OccurredAt       time.Time
	ReceivedAt       time.Time
	ReceivedAtSource string
}

func loadWebhookDeliveriesSince(db *store.Store, cutoff time.Time) ([]webhookDeliverySummary, error) {
	rows, err := db.DB().Query(`SELECT observation_id, event_id, event_type, occurred_at, received_at, received_at_source
		FROM webhook_deliveries WHERE received_at_ns >= ? ORDER BY received_at_ns, observation_id`, cutoff.UTC().UnixNano())
	if err != nil {
		if strings.Contains(err.Error(), "no such table: webhook_deliveries") {
			return nil, nil
		}
		return nil, err
	}
	var deliveries []webhookDeliverySummary
	for rows.Next() {
		var delivery webhookDeliverySummary
		var occurred sql.NullTime
		if err := rows.Scan(&delivery.ObservationID, &delivery.EventID, &delivery.EventType, &occurred, &delivery.ReceivedAt, &delivery.ReceivedAtSource); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if occurred.Valid {
			delivery.OccurredAt = occurred.Time
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func newNovelWebhookHealthCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Measure duplicates, ordering, delivery lag, gaps, and subscription changes from locally received webhooks.",
		Example:     "  square-pp-cli webhook health --since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook health")
			}
			cutoff, err := parseSinceDuration(flagSince)
			if err != nil {
				return fmt.Errorf("invalid value %q for --since: %s", flagSince, err)
			}
			dbPath := defaultDBPath("square-pp-cli")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("no local webhook log found; record deliveries with 'square-pp-cli webhook ingest --body FILE'")
			}
			// A writable open performs bounded-batch retention maintenance before
			// this read, so expired receipts cannot survive indefinitely on a
			// health-only installation.
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local webhook log: %w", err)
			}
			defer db.Close()
			deliveries, err := loadWebhookDeliveriesSince(db, cutoff)
			if err != nil {
				return fmt.Errorf("loading webhook deliveries: %w", err)
			}
			subscriptionHistory, err := loadResourceHistory(cmd.Context(), db, []string{"webhooks-subscriptions"}, cutoff)
			if err != nil {
				return fmt.Errorf("loading webhook subscription history: %w", err)
			}

			seen := map[string]int{}
			duplicates, ordering := 0, 0
			var lastOccurred time.Time
			var totalLag, maxLag time.Duration
			lagSamples := 0
			eventTypes := map[string]int{}
			timestampSources := map[string]int{}
			for _, delivery := range deliveries {
				timestampSources[delivery.ReceivedAtSource]++
				seen[delivery.EventID]++
				if seen[delivery.EventID] > 1 {
					duplicates++
				}
				if !delivery.OccurredAt.IsZero() {
					if !lastOccurred.IsZero() && delivery.OccurredAt.Before(lastOccurred) {
						ordering++
					}
					lastOccurred = delivery.OccurredAt
					lag := delivery.ReceivedAt.Sub(delivery.OccurredAt)
					if lag < 0 {
						lag = 0
					}
					totalLag += lag
					lagSamples++
					if lag > maxLag {
						maxLag = lag
					}
				}
				eventType := delivery.EventType
				if eventType == "" {
					eventType = "unknown"
				}
				eventTypes[eventType]++
			}
			avgLag := time.Duration(0)
			if lagSamples > 0 {
				avgLag = totalLag / time.Duration(lagSamples)
			}
			const gapThreshold = 5 * time.Minute
			var totalGap, maxGap time.Duration
			gapsOverThreshold := 0
			for i := 1; i < len(deliveries); i++ {
				gap := deliveries[i].ReceivedAt.Sub(deliveries[i-1].ReceivedAt)
				totalGap += gap
				if gap > maxGap {
					maxGap = gap
				}
				if gap > gapThreshold {
					gapsOverThreshold++
				}
			}
			avgGap := time.Duration(0)
			if len(deliveries) > 1 {
				avgGap = totalGap / time.Duration(len(deliveries)-1)
			}

			type subscriptionChange struct {
				ID         string        `json:"id"`
				BaselineAt time.Time     `json:"baseline_at"`
				LatestAt   time.Time     `json:"latest_at"`
				Changes    []fieldChange `json:"changes"`
			}
			groups := map[string][]resourceHistoryRecord{}
			for _, version := range subscriptionHistory {
				groups[version.ResourceID] = append(groups[version.ResourceID], version)
			}
			subscriptionChanges := make([]subscriptionChange, 0)
			uncomparedSubscriptions := 0
			for id, versions := range groups {
				sort.SliceStable(versions, func(i, j int) bool {
					if versions[i].ObservedAt.Equal(versions[j].ObservedAt) {
						return versions[i].Sequence < versions[j].Sequence
					}
					return versions[i].ObservedAt.Before(versions[j].ObservedAt)
				})
				latest := versions[len(versions)-1]
				if latest.ObservedAt.Before(cutoff) {
					continue
				}
				baseline := -1
				for i := range versions {
					if versions[i].ObservedAt.Before(cutoff) {
						baseline = i
					}
				}
				if baseline < 0 && len(versions) > 1 {
					baseline = 0
				}
				if baseline < 0 || baseline == len(versions)-1 {
					uncomparedSubscriptions++
					continue
				}
				before := redactWebhookSecrets(versions[baseline].Data).(map[string]any)
				after := redactWebhookSecrets(latest.Data).(map[string]any)
				changes := meaningfulFieldChanges(before, after)
				if len(changes) > 0 {
					subscriptionChanges = append(subscriptionChanges, subscriptionChange{ID: id, BaselineAt: versions[baseline].ObservedAt, LatestAt: latest.ObservedAt, Changes: changes})
				}
			}
			sort.Slice(subscriptionChanges, func(i, j int) bool { return subscriptionChanges[i].ID < subscriptionChanges[j].ID })

			gapMetrics := map[string]any{
				"threshold": gapThreshold.String(), "interval_count": max(0, len(deliveries)-1),
				"gaps_over_threshold": gapsOverThreshold, "average_received_gap": avgGap.String(),
				"maximum_received_gap": maxGap.String(),
			}
			if len(deliveries) > 0 {
				gapMetrics["first_received_at"] = deliveries[0].ReceivedAt
				gapMetrics["last_received_at"] = deliveries[len(deliveries)-1].ReceivedAt
			}
			limitations := []string{"Gap metrics describe quiet intervals between locally ingested deliveries; without an expected delivery schedule they cannot prove that an event is missing."}
			if len(deliveries) == 0 {
				limitations = append(limitations, "No locally ingested webhook deliveries fall inside the requested window.")
			}
			return flags.printJSON(cmd, map[string]any{
				"data_source": "local-webhook-log", "since": flagSince, "cutoff": cutoff,
				"deliveries": len(deliveries), "duplicate_event_ids": duplicates,
				"ordering_problems": ordering, "delivery_lag_samples": lagSamples,
				"average_delivery_lag": avgLag.String(), "maximum_delivery_lag": maxLag.String(),
				"event_types": eventTypes, "gap_metrics": gapMetrics,
				"received_at_sources":      timestampSources,
				"subscription_changes":     subscriptionChanges,
				"uncompared_subscriptions": uncomparedSubscriptions, "limitations": limitations,
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Analyze locally ingested webhook deliveries from this period (for example 24h or 7d)")
	return cmd
}
