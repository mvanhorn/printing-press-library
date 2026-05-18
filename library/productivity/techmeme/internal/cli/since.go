// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

// parseDurationString parses human-friendly duration strings like "4h", "12h", "1d", "1w", "30m".
func parseDurationString(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([dhwm])$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return 0, fmt.Errorf("expected format like 4h, 12h, 1d, 1w, or 30m")
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}

	switch matches[2] {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown unit %q", matches[2])
	}
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "Show headlines from the last N hours/minutes/days",
		Long: `Query local SQLite for headlines within the specified duration.
Duration format: 4h (hours), 30m (minutes), 1d (days).

Requires synced data. Run 'techmeme-pp-cli sync' first.`,
		Example: `  # Headlines from the last 4 hours
  techmeme-pp-cli since 4h

  # Headlines from the last day
  techmeme-pp-cli since 1d

  # Headlines from the last 30 minutes
  techmeme-pp-cli since 30m`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			dur, err := parseDurationString(args[0])
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", args[0], err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			since := time.Now().Add(-dur)
			items, err := db.HeadlinesSince(since)
			if err != nil {
				return fmt.Errorf("querying headlines: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cached headlines. Run 'techmeme-pp-cli sync' first.")
				return nil
			}

			type row struct {
				Time     string `json:"time"`
				Source   string `json:"source"`
				Headline string `json:"headline"`
				Link     string `json:"link,omitempty"`
			}

			var rows []row
			for _, item := range items {
				var obj map[string]any
				if err := json.Unmarshal(item, &obj); err != nil {
					continue
				}
				r := row{}
				if v, ok := obj["timestamp"].(string); ok {
					r.Time = v
				} else if v, ok := obj["pubDate"].(string); ok {
					r.Time = v
				} else if v, ok := obj["time"].(string); ok {
					r.Time = v
				}
				if v, ok := obj["source"].(string); ok {
					r.Source = v
				} else if v, ok := obj["author"].(string); ok {
					r.Source = v
				}
				if v, ok := obj["title"].(string); ok {
					r.Headline = v
				} else if v, ok := obj["headline"].(string); ok {
					r.Headline = v
				}
				if v, ok := obj["link"].(string); ok {
					r.Link = v
				} else if v, ok := obj["url"].(string); ok {
					r.Link = v
				}
				rows = append(rows, r)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"TIME", "SOURCE", "HEADLINE"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					truncate(r.Time, 20),
					truncate(r.Source, 25),
					truncate(r.Headline, 70),
				})
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d headlines in the last %s\n", len(rows), args[0])
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	return cmd
}
