// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type logErrorBucket struct {
	AnswerCode int    `json:"answer_code"`
	Message    string `json:"message,omitempty"`
	Count      int    `json:"count"`
}

type logsErrorsResult struct {
	Since   string           `json:"since"`
	Total   int              `json:"total"`
	Buckets []logErrorBucket `json:"buckets"`
}

func newNovelLogsErrorsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "errors",
		Short:       "Aggregate synced log entries by error code and link failed tasks for a digest of what went wrong.",
		Example:     "  algolia-pp-cli logs errors --since 24h",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "logs errors")
			}
			since := time.Duration(24) * time.Hour
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources logs to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), logsErrorsResult{Since: flagSince, Buckets: make([]logErrorBucket, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "logs") {
				hintIfStale(cmd, db, "logs", flags.maxAge)
			}

			cutoff := time.Now().Add(-since)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT data FROM resources WHERE resource_type = 'logs'`)
			if err != nil {
				return fmt.Errorf("querying logs: %w", err)
			}
			var rawLogs []json.RawMessage
			for rows.Next() {
				var d string
				if err := rows.Scan(&d); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan log: %w", err)
				}
				rawLogs = append(rawLogs, json.RawMessage(d))
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate logs: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close logs: %w", err)
			}

			type bucket struct {
				code    int
				message string
			}
			counts := make(map[bucket]int)
			total := 0
			for _, raw := range rawLogs {
				var entry struct {
					AnswerCode json.RawMessage `json:"answer_code"`
					Message    string          `json:"message"`
					Timestamp  string          `json:"timestamp"`
					Time       string          `json:"time"`
				}
				if json.Unmarshal(raw, &entry) != nil {
					continue
				}
				ts := entry.Timestamp
				if ts == "" {
					ts = entry.Time
				}
				if ts != "" {
					t, perr := time.Parse(time.RFC3339, ts)
					if perr != nil {
						if t, perr = time.Parse("2006-01-02T15:04:05Z", ts); perr != nil {
							t = time.Time{}
						}
					}
					if !t.IsZero() && t.Before(cutoff) {
						continue
					}
				}
				// answer_code arrives as a JSON number or a JSON string
				// depending on the log entry source; parse both.
				code := parseAnswerCode(entry.AnswerCode)
				if code >= 400 {
					counts[bucket{code: code, message: entry.Message}]++
					total++
				}
			}

			res := logsErrorsResult{Since: flagSince, Total: total, Buckets: make([]logErrorBucket, 0)}
			for b, c := range counts {
				res.Buckets = append(res.Buckets, logErrorBucket{AnswerCode: b.code, Message: b.message, Count: c})
			}
			sort.Slice(res.Buckets, func(i, j int) bool {
				if res.Buckets[i].AnswerCode != res.Buckets[j].AnswerCode {
					return res.Buckets[i].AnswerCode < res.Buckets[j].AnswerCode
				}
				return res.Buckets[i].Count > res.Buckets[j].Count
			})

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if total == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No error-class log entries in the last %s.\n", since)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d error-class log entries in the last %s:\n", total, since)
			for _, b := range res.Buckets {
				fmt.Fprintf(cmd.OutOrStdout(), "  HTTP %d x%d", b.AnswerCode, b.Count)
				if b.Message != "" {
					msg := b.Message
					if len(msg) > 80 {
						msg = msg[:80] + "..."
					}
					fmt.Fprintf(cmd.OutOrStdout(), " — %s", msg)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Lookback window (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// parseAnswerCode decodes an answer_code value that may be a JSON number
// (404) or a JSON string ("404") depending on the log entry source.
func parseAnswerCode(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var num int
	if json.Unmarshal(raw, &num) == nil {
		return num
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		var parsed int
		if _, err := fmt.Sscanf(str, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
