// Compound command: send analytics.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"multimail-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

type emailStats struct {
	Period          string           `json:"period"`
	TotalSent       int              `json:"total_sent"`
	TotalReceived   int              `json:"total_received"`
	BounceCount     int              `json:"bounce_count"`
	DeliveryRate    float64          `json:"delivery_rate"`
	TopSenders      []correspondent  `json:"top_senders,omitempty"`
	TopRecipients   []correspondent  `json:"top_recipients,omitempty"`
	PeakHours       []hourBucket     `json:"peak_hours,omitempty"`
	DailyVolume     []dailyBucket    `json:"daily_volume,omitempty"`
	SyncedAt        string           `json:"synced_at,omitempty"`
}

type correspondent struct {
	Address string `json:"address"`
	Count   int    `json:"count"`
}

type hourBucket struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

type dailyBucket struct {
	Date     string `json:"date"`
	Sent     int    `json:"sent"`
	Received int    `json:"received"`
}

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var period string
	var topN int
	var mailboxID string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Send/receive analytics from cached data",
		Long: `Send/receive volume, top correspondents, peak hours, and delivery
rate over a configurable period. Understand email patterns at a glance.

Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # Last 30 days of stats
  multimail-pp-cli stats --period 30d

  # Last 7 days for a specific mailbox
  multimail-pp-cli stats --period 7d --mailbox 01ABC123

  # JSON output
  multimail-pp-cli stats --period 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'multimail-pp-cli sync' first.", err)
			}
			defer db.Close()

			days, err := parsePeriod(period)
			if err != nil {
				return err
			}

			result, err := computeStats(db, days, topN, mailboxID)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if flags.compact {
					type compactStats struct {
						TotalSent     int     `json:"total_sent"`
						TotalReceived int     `json:"total_received"`
						DeliveryRate  float64 `json:"delivery_rate"`
						BounceCount   int     `json:"bounce_count"`
					}
					return enc.Encode(compactStats{
						TotalSent:     result.TotalSent,
						TotalReceived: result.TotalReceived,
						DeliveryRate:  result.DeliveryRate,
						BounceCount:   result.BounceCount,
					})
				}
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Email Stats (%s)\n", result.Period)
			fmt.Fprintln(cmd.OutOrStdout(), "============================")
			fmt.Fprintf(cmd.OutOrStdout(), "Sent:          %d\n", result.TotalSent)
			fmt.Fprintf(cmd.OutOrStdout(), "Received:      %d\n", result.TotalReceived)
			fmt.Fprintf(cmd.OutOrStdout(), "Bounced:       %d\n", result.BounceCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Delivery Rate: %.1f%%\n", result.DeliveryRate*100)

			if len(result.TopSenders) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop Senders:")
				for _, s := range result.TopSenders {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %d\n", s.Address, s.Count)
				}
			}

			if len(result.TopRecipients) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop Recipients:")
				for _, r := range result.TopRecipients {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %d\n", r.Address, r.Count)
				}
			}

			if len(result.PeakHours) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nPeak Hours (UTC):")
				for _, h := range result.PeakHours {
					bar := ""
					maxCount := result.PeakHours[0].Count
					if maxCount > 0 {
						width := h.Count * 30 / maxCount
						for i := 0; i < width; i++ {
							bar += "█"
						}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %02d:00  %3d  %s\n", h.Hour, h.Count, bar)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&period, "period", "30d", "Analysis period (e.g. 7d, 30d, 90d)")
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top correspondents to show")
	cmd.Flags().StringVar(&mailboxID, "mailbox", "", "Filter to specific mailbox ID")

	return cmd
}

func parsePeriod(period string) (int, error) {
	if len(period) < 2 {
		return 0, fmt.Errorf("invalid period format: %s (use e.g. 7d, 30d, 90d)", period)
	}
	unit := period[len(period)-1]
	numStr := period[:len(period)-1]
	var num int
	_, err := fmt.Sscanf(numStr, "%d", &num)
	if err != nil {
		return 0, fmt.Errorf("invalid period format: %s (use e.g. 7d, 30d, 90d)", period)
	}
	switch unit {
	case 'd':
		return num, nil
	case 'w':
		return num * 7, nil
	default:
		return 0, fmt.Errorf("unsupported period unit '%c' (use d for days, w for weeks)", unit)
	}
}

func computeStats(db *store.Store, days, topN int, mailboxFilter string) (*emailStats, error) {
	sqlDB := db.DB()
	since := time.Now().AddDate(0, 0, -days)
	sinceStr := since.Format(time.RFC3339)

	// Query all emails in the period
	query := `SELECT data FROM emails WHERE json_extract(data, '$.received_at') >= ?`
	args := []any{sinceStr}
	if mailboxFilter != "" {
		query += ` AND parent_id = ?`
		args = append(args, mailboxFilter)
	}

	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying emails: %w", err)
	}
	defer rows.Close()

	var totalSent, totalReceived, bounced int
	senders := make(map[string]int)
	recipients := make(map[string]int)
	hourCounts := make(map[int]int)
	dailyCounts := make(map[string]*dailyBucket)

	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			continue
		}

		var email map[string]any
		if err := json.Unmarshal([]byte(dataStr), &email); err != nil {
			continue
		}

		direction, _ := email["direction"].(string)
		from, _ := email["from"].(string)
		receivedAt, _ := email["received_at"].(string)

		// Parse time for hour and daily buckets
		t, _ := time.Parse(time.RFC3339, receivedAt)
		hour := t.Hour()
		hourCounts[hour]++

		dateKey := t.Format("2006-01-02")
		bucket, exists := dailyCounts[dateKey]
		if !exists {
			bucket = &dailyBucket{Date: dateKey}
			dailyCounts[dateKey] = bucket
		}

		if direction == "outbound" {
			totalSent++
			bucket.Sent++
			// Track recipients
			if toList, ok := email["to"].([]any); ok {
				for _, addr := range toList {
					if s, ok := addr.(string); ok {
						recipients[s]++
					}
				}
			}
		} else {
			totalReceived++
			bucket.Received++
			senders[from]++
		}

		// Bounces indicated by bounced_at field, not status
		if _, hasBounce := email["bounced_at"]; hasBounce && email["bounced_at"] != nil {
			bounced++
		}
	}

	deliveryRate := 1.0
	if totalSent > 0 {
		deliveryRate = float64(totalSent-bounced) / float64(totalSent)
	}

	// Build top senders
	topSenders := buildTopN(senders, topN)
	topRecips := buildTopN(recipients, topN)

	// Build peak hours (sorted by count descending)
	var peakHours []hourBucket
	for h, c := range hourCounts {
		peakHours = append(peakHours, hourBucket{Hour: h, Count: c})
	}
	sort.Slice(peakHours, func(i, j int) bool {
		return peakHours[i].Count > peakHours[j].Count
	})
	if len(peakHours) > 5 {
		peakHours = peakHours[:5]
	}

	// Build daily volume (sorted by date)
	var dailyVolume []dailyBucket
	for _, b := range dailyCounts {
		dailyVolume = append(dailyVolume, *b)
	}
	sort.Slice(dailyVolume, func(i, j int) bool {
		return dailyVolume[i].Date < dailyVolume[j].Date
	})

	syncedAt := db.GetLastSyncedAt("emails")

	return &emailStats{
		Period:        fmt.Sprintf("last %d days", days),
		TotalSent:     totalSent,
		TotalReceived: totalReceived,
		BounceCount:   bounced,
		DeliveryRate:  deliveryRate,
		TopSenders:    topSenders,
		TopRecipients: topRecips,
		PeakHours:     peakHours,
		DailyVolume:   dailyVolume,
		SyncedAt:      syncedAt,
	}, nil
}

func buildTopN(counts map[string]int, n int) []correspondent {
	type kv struct {
		Key   string
		Count int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	result := make([]correspondent, len(sorted))
	for i, s := range sorted {
		result[i] = correspondent{Address: s.Key, Count: s.Count}
	}
	return result
}
