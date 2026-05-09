package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

func newLatencyCmd(flags *rootFlags) *cobra.Command {
	var lastN int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "latency",
		Short: "Analyze AI response latency by pairing user messages with AI responses",
		Example: strings.Trim(`
  zylos-pp-cli latency
  zylos-pp-cli latency --last 50 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("zylos-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zylos-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM resources
				 WHERE resource_type = 'conversations'
				 ORDER BY json_extract(data, '$.timestamp') ASC`,
			)
			if err != nil {
				return fmt.Errorf("querying messages: %w", err)
			}
			defer rows.Close()

			type message struct {
				Direction string `json:"direction"`
				Content   string `json:"content"`
				Timestamp string `json:"timestamp"`
			}

			type latencyPair struct {
				UserTimestamp  string  `json:"user_timestamp"`
				AITimestamp    string  `json:"ai_timestamp"`
				LatencySeconds float64 `json:"latency_seconds"`
				ContentPreview string  `json:"content_preview"`
			}

			type latencyResult struct {
				Pairs   []latencyPair `json:"pairs"`
				Average float64       `json:"average_seconds"`
				Min     float64       `json:"min_seconds"`
				Max     float64       `json:"max_seconds"`
			}

			// Collect all messages
			var messages []message
			for rows.Next() {
				var dataStr string
				if err := rows.Scan(&dataStr); err != nil {
					continue
				}
				var msg message
				if err := json.Unmarshal([]byte(dataStr), &msg); err != nil {
					continue
				}
				messages = append(messages, msg)
			}

			// Pair "in" messages with the next "out" message
			var pairs []latencyPair
			for i, msg := range messages {
				if msg.Direction != "in" {
					continue
				}
				userTS, err := time.Parse(time.RFC3339, msg.Timestamp)
				if err != nil {
					continue
				}
				// Find the next "out" message after this "in"
				for j := i + 1; j < len(messages); j++ {
					if messages[j].Direction == "out" {
						aiTS, err := time.Parse(time.RFC3339, messages[j].Timestamp)
						if err != nil {
							break
						}
						latency := aiTS.Sub(userTS).Seconds()
						if latency >= 0 {
							pairs = append(pairs, latencyPair{
								UserTimestamp:  msg.Timestamp,
								AITimestamp:    messages[j].Timestamp,
								LatencySeconds: latency,
								ContentPreview: truncate(msg.Content, 80),
							})
						}
						break
					}
				}
			}

			// Take last N pairs
			if lastN > 0 && len(pairs) > lastN {
				pairs = pairs[len(pairs)-lastN:]
			}

			result := latencyResult{Pairs: pairs}
			if len(pairs) > 0 {
				var sum, mn, mx float64
				mn = pairs[0].LatencySeconds
				mx = pairs[0].LatencySeconds
				for _, p := range pairs {
					sum += p.LatencySeconds
					if p.LatencySeconds < mn {
						mn = p.LatencySeconds
					}
					if p.LatencySeconds > mx {
						mx = p.LatencySeconds
					}
				}
				result.Average = sum / float64(len(pairs))
				result.Min = mn
				result.Max = mx
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().IntVar(&lastN, "last", 20, "Analyze last N response pairs")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
