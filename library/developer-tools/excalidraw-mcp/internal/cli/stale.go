// Copyright 2026 bk20260126-code. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleResult struct {
	Path        string    `json:"path"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	DaysSince   int       `json:"daysSince"`
	ElementHint string    `json:"elementHint,omitempty"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dir string
	var sinceStr string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale [--dir <path>] [--since <duration>]",
		Short: "List .excalidraw files that haven't been updated in N days.",
		Long: `Walk a directory tree for .excalidraw files and flag diagrams
that have not been modified within the specified time window.

Useful in CI to catch documentation diagrams that may be out of date.`,
		Example: strings.Trim(`
  excalidraw-mcp-pp-cli stale --dir ./docs --since 90d
  excalidraw-mcp-pp-cli stale --dir . --since 30d --json
  excalidraw-mcp-pp-cli stale --dir ./diagrams --since 180d --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would scan: %s (since: %s)\n", dir, sinceStr)
				return nil
			}

			threshold, err := parseDuration(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: use format like 30d, 90d, 180d", sinceStr)
			}

			cutoff := time.Now().Add(-threshold)

			results := []staleResult{}
			walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor") {
					return filepath.SkipDir
				}
				if !strings.HasSuffix(path, ".excalidraw") {
					return nil
				}
				info, statErr := d.Info()
				if statErr != nil {
					return nil
				}
				if info.ModTime().Before(cutoff) {
					days := int(time.Since(info.ModTime()).Hours() / 24)
					r := staleResult{
						Path:       path,
						ModifiedAt: info.ModTime(),
						DaysSince:  days,
					}
					results = append(results, r)
					if limit > 0 && len(results) >= limit {
						return fmt.Errorf("limit reached")
					}
				}
				return nil
			})
			if walkErr != nil && walkErr.Error() != "limit reached" {
				return fmt.Errorf("walking %s: %w", dir, walkErr)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				raw, _ := json.MarshalIndent(results, "", "  ")
				sel := json.RawMessage(raw)
				if flags.selectFields != "" {
					sel = filterFields(sel, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), sel, true)
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale diagrams found in %s (threshold: %s)\n", dir, sinceStr)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stale diagrams in %s (not modified in %s):\n\n", dir, sinceStr)
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  (%d days ago)\n", r.Path, r.DaysSince)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d file(s)\n", len(results))
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Directory to scan for .excalidraw files")
	cmd.Flags().StringVar(&sinceStr, "since", "90d", "Flag files not modified in this duration (e.g. 30d, 90d, 180d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results to return (0 = unlimited)")
	return cmd
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
