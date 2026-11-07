// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. Manifest absorbed row 12.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/spf13/cobra"
)

// filterAuditRecords keeps the newest `limit` matches, oldest first, so output
// reads top to bottom like a log even when capped.
func filterAuditRecords(records []client.AuditRecord, since time.Time, method, console, pathContains, outcome string, limit int) ([]client.AuditRecord, int) {
	matched := make([]client.AuditRecord, 0, len(records))
	for _, rec := range records {
		if !since.IsZero() {
			ts, err := time.Parse(time.RFC3339Nano, rec.Time)
			if err != nil || ts.Before(since) {
				continue
			}
		}
		if method != "" && !strings.EqualFold(rec.Method, method) {
			continue
		}
		if console != "" && rec.Console != console {
			continue
		}
		if pathContains != "" && !strings.Contains(rec.Path, pathContains) {
			continue
		}
		if outcome != "" && !strings.EqualFold(rec.Outcome, outcome) {
			continue
		}
		matched = append(matched, rec)
	}
	total := len(matched)
	if limit > 0 && len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, total
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var sinceRaw, method, pathContains, outcome string
	var limit int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Read the local log of every attempted mutation, with secrets scrubbed",
		Long: `Every attempted write made by this CLI or its MCP server is appended to a private
JSONL file, including refused and ambiguous ones. Secret-named fields are scrubbed
before they reach disk. Outcomes: applied, refused (the API said no), ambiguous
(timeout or 5xx, so the write may or may not have happened). Reads are not logged.`,
		Example: strings.Trim(`
  unifi-pp-cli audit --since 7d --json
  unifi-pp-cli audit --outcome ambiguous --agent
  unifi-pp-cli audit --method DELETE --path firewall --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "audit")
			}
			var since time.Time
			if sinceRaw != "" {
				window, err := cliutil.ParseDurationLoose(sinceRaw)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", sinceRaw, err))
				}
				since = time.Now().Add(-window)
			}
			path, err := client.AuditLogPath()
			if err != nil {
				return err
			}
			records, err := client.ReadAuditLog(path)
			if err != nil {
				return fmt.Errorf("reading audit log: %w", err)
			}
			// --console narrows only when given explicitly here: an operator asking
			// "what did I do" wants every console unless they say otherwise.
			matched, total := filterAuditRecords(records, since, method, unifiTarget.ConsoleID, pathContains, outcome, limit)
			view := map[string]any{"log_path": path, "matched": total, "shown": len(matched), "records": matched}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(matched) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching mutations in %s.\n", path)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TIME\tOUTCOME\tSTATUS\tMETHOD\tCONSOLE\tPATH")
			for _, rec := range matched {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n", rec.Time, rec.Outcome, rec.Status, rec.Method, rec.Console, rec.Path)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Only mutations newer than this, e.g. 24h, 7d")
	cmd.Flags().StringVar(&method, "method", "", "Only this HTTP method")
	cmd.Flags().StringVar(&pathContains, "path", "", "Only paths containing this text")
	cmd.Flags().StringVar(&outcome, "outcome", "", "Only this outcome: applied, refused or ambiguous")
	cmd.Flags().IntVar(&limit, "limit", 100, "Newest matches to show; 0 for all")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newAuditCmd(flags))
	})
}
