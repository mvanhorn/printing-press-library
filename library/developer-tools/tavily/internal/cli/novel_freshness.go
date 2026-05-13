// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel freshness-check command — find stale cached pages for re-fetch.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tavily/internal/store"
)

// parseFreshnessDuration parses durations with optional "d" suffix (days).
// Falls back to standard time.ParseDuration for other units.
func parseFreshnessDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		// e.g. "7d" → 7 * 24h
		var days int
		if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func newFreshnessCheckCmd(flags *rootFlags) *cobra.Command {
	var olderThanStr string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "freshness-check",
		Short: "Find cached pages older than a threshold and output a re-fetch list",
		Long: `Scan the local extract cache for pages that were last fetched more
than the given duration ago. Outputs a prioritized list of URLs to
re-extract. Pipe into 'tavily-pp-cli extract' or save to a file for
batch re-fetching.

Duration accepts standard Go units (e.g. 24h, 168h) or a shorthand 'd'
for days (e.g. 7d = 7 days = 168h).`,
		Example: `  tavily-pp-cli freshness-check --older-than 7d
  tavily-pp-cli freshness-check --older-than 24h --output stale-urls.txt
  tavily-pp-cli freshness-check --older-than 72h --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var olderThan time.Duration
			if olderThanStr == "" {
				olderThan = 7 * 24 * time.Hour // default: 7 days
			} else {
				var err error
				olderThan, err = parseFreshnessDuration(olderThanStr)
				if err != nil {
					return fmt.Errorf("--older-than: %w", err)
				}
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			staleExtracts, err := st.ExtractsOlderThan(olderThan)
			if err != nil {
				return fmt.Errorf("reading stale extracts: %w", err)
			}

			type StaleEntry struct {
				URL      string `json:"url"`
				FetchedAt string `json:"fetched_at"`
				AgeHours  float64 `json:"age_hours"`
			}

			var stale []StaleEntry
			for _, e := range staleExtracts {
				var urls []string
				json.Unmarshal([]byte(e.URLs), &urls)
				age := time.Since(e.CreatedAt).Hours()
				for _, u := range urls {
					stale = append(stale, StaleEntry{
						URL:      u,
						FetchedAt: e.CreatedAt.Format(time.RFC3339),
						AgeHours:  age,
					})
				}
			}

			if flags.asJSON {
				out := map[string]any{
					"older_than": olderThan.String(),
					"stale_count": len(stale),
					"pages":       stale,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			if len(stale) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No pages older than %s found in cache.\n", olderThan)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d stale pages (older than %s):\n\n", len(stale), olderThan)
			var urlLines string
			for _, s := range stale {
				line := fmt.Sprintf("  %.0fh  %s\n", s.AgeHours, s.URL)
				fmt.Fprint(cmd.OutOrStdout(), line)
				urlLines += s.URL + "\n"
			}

			if outputFile != "" && outputFile != "-" {
				if err := writeFile(outputFile, urlLines); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nURL list written to %s\n", outputFile)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&olderThanStr, "older-than", "7d", "Age threshold: supports days (7d) or standard Go units (24h, 168h)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Write URL list to file (one per line)")
	return cmd
}
