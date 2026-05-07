// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newDeliveriesTriageCmd assembles a self-contained incident bundle from
// recent deliveries that match a template and status filter. Reads from the
// local synced deliveries table (or live API if --live), groups by error
// reason, and writes summary.md + deliveries.jsonl + recipients.txt to the
// supplied bundle directory.
func newDeliveriesTriageCmd(flags *rootFlags) *cobra.Command {
	var template, status, since, bundleDir string
	var live bool
	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Assemble an incident bundle from recent deliveries (summary.md + deliveries.jsonl + recipients.txt)",
		Long: `Filter recent deliveries by template, status (e.g. bounced, failed,
dropped), and time window; write a self-contained bundle to the supplied
directory:

  bundle/summary.md       — Markdown overview with grouped error reasons
  bundle/deliveries.jsonl — One delivery per line (raw API/store data)
  bundle/recipients.txt   — Distinct recipient list

Reads from the local synced deliveries table by default; --live forces a
fresh live API fetch.`,
		Example: strings.Trim(`
  customer-io-pp-cli deliveries triage --template tx_91 --status bounced --since 1h --bundle ./incident
  customer-io-pp-cli deliveries triage --status failed --since 24h --bundle ./failures --json
  customer-io-pp-cli deliveries triage --template tx_91 --status bounced --live --bundle ./incident
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if bundleDir == "" {
				return usageErr(fmt.Errorf("--bundle <dir> is required"))
			}
			cutoff, err := parseSinceCutoff(since)
			if err != nil {
				return usageErr(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			deliveries, source, err := loadDeliveriesForTriage(ctx, flags, template, status, cutoff, live)
			if err != nil {
				return apiErr(err)
			}

			if err := os.MkdirAll(bundleDir, 0o755); err != nil {
				return apiErr(fmt.Errorf("creating bundle dir: %w", err))
			}
			summary, recipients, byReason := summarizeDeliveries(deliveries)
			if err := writeBundle(bundleDir, deliveries, summary, recipients, byReason); err != nil {
				return apiErr(err)
			}

			out := map[string]any{
				"bundle_dir": bundleDir,
				"deliveries": len(deliveries),
				"recipients": len(recipients),
				"by_reason":  byReason,
				"source":     source,
				"window":     since,
				"template":   template,
				"status":     status,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bundle written: %s\n", bundleDir)
			fmt.Fprintf(cmd.OutOrStdout(), "  deliveries: %d\n  recipients: %d\n  source:     %s\n", len(deliveries), len(recipients), source)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Top reasons:")
			for _, kv := range topReasons(byReason, 5) {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", kv.reason, kv.count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "Filter to deliveries from this transactional template ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter to deliveries in this state (e.g. bounced, failed, dropped, spammed)")
	cmd.Flags().StringVar(&since, "since", "1h", "Only include deliveries newer than this duration (e.g. 1h, 24h, 7d)")
	cmd.Flags().StringVar(&bundleDir, "bundle", "", "Output directory for the bundle (required)")
	cmd.Flags().BoolVar(&live, "live", false, "Force a fresh live API fetch instead of reading the local synced cache")
	return cmd
}

func loadDeliveriesForTriage(ctx context.Context, flags *rootFlags, template, status string, cutoff int64, live bool) ([]json.RawMessage, string, error) {
	if live {
		// Live triage hits /v1/environments/{environment_id}/deliveries which
		// requires an env_id; the current command surface doesn't carry one,
		// so fall back to the local store path. Future enhancement: thread
		// --environment-id through and call the live endpoint.
		return nil, "live", fmt.Errorf("--live mode is not yet wired (requires environment_id plumbing); rerun without --live to read from the local synced store")
	}

	db, err := openTimelineStore()
	if err != nil {
		return nil, "local", fmt.Errorf("opening local store: %w (try --live or run 'customer-io-pp-cli sync --resources deliveries' first)", err)
	}
	defer db.Close()
	q := `SELECT data FROM deliveries WHERE 1=1`
	args := []any{}
	if template != "" {
		q += ` AND (json_extract(data, '$.transactional_message_id') = ? OR json_extract(data, '$.template_id') = ?)`
		args = append(args, template, template)
	}
	if status != "" {
		q += ` AND LOWER(IFNULL(json_extract(data, '$.state'), '')) = LOWER(?)`
		args = append(args, status)
	}
	if cutoff > 0 {
		q += ` AND IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) >= ?`
		args = append(args, cutoff)
	}
	q += ` ORDER BY IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) DESC LIMIT 1000`
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "local", err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data []byte
		if scanErr := rows.Scan(&data); scanErr != nil {
			return nil, "local", scanErr
		}
		out = append(out, json.RawMessage(append([]byte{}, data...)))
	}
	return out, "local", rows.Err()
}

func filterDeliveries(rows []json.RawMessage, template, status string, cutoff int64) []json.RawMessage {
	var out []json.RawMessage
	for _, row := range rows {
		var d struct {
			TransactionalMessageID string `json:"transactional_message_id"`
			TemplateID             string `json:"template_id"`
			State                  string `json:"state"`
			Created                int64  `json:"created"`
			Updated                int64  `json:"updated"`
		}
		_ = json.Unmarshal(row, &d)
		if template != "" && d.TransactionalMessageID != template && d.TemplateID != template {
			continue
		}
		if status != "" && !strings.EqualFold(d.State, status) {
			continue
		}
		ts := d.Created
		if ts == 0 {
			ts = d.Updated
		}
		if cutoff > 0 && ts < cutoff {
			continue
		}
		out = append(out, row)
	}
	return out
}

func summarizeDeliveries(deliveries []json.RawMessage) (string, []string, map[string]int) {
	byReason := map[string]int{}
	recipientSet := map[string]struct{}{}
	for _, row := range deliveries {
		var d struct {
			Recipient      string `json:"recipient"`
			CustomerID     string `json:"customer_id"`
			FailureMessage string `json:"failure_message"`
			State          string `json:"state"`
		}
		_ = json.Unmarshal(row, &d)
		who := d.Recipient
		if who == "" {
			who = d.CustomerID
		}
		if who != "" {
			recipientSet[who] = struct{}{}
		}
		reason := d.FailureMessage
		if reason == "" {
			reason = strings.ToLower(d.State)
		}
		if reason == "" {
			reason = "unknown"
		}
		byReason[reason]++
	}
	recipients := make([]string, 0, len(recipientSet))
	for r := range recipientSet {
		recipients = append(recipients, r)
	}
	sort.Strings(recipients)
	var b strings.Builder
	fmt.Fprintf(&b, "# Delivery triage bundle\n\n")
	fmt.Fprintf(&b, "Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Deliveries: %d\nDistinct recipients: %d\n\n", len(deliveries), len(recipients))
	fmt.Fprintf(&b, "## Reasons (count)\n\n")
	for _, kv := range topReasons(byReason, 25) {
		fmt.Fprintf(&b, "- **%s** — %d\n", kv.reason, kv.count)
	}
	return b.String(), recipients, byReason
}

type reasonCount struct {
	reason string
	count  int
}

func topReasons(by map[string]int, limit int) []reasonCount {
	pairs := make([]reasonCount, 0, len(by))
	for r, c := range by {
		pairs = append(pairs, reasonCount{r, c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].reason < pairs[j].reason
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	return pairs
}

func writeBundle(dir string, deliveries []json.RawMessage, summary string, recipients []string, _ map[string]int) error {
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte(summary), 0o644); err != nil {
		return fmt.Errorf("writing summary.md: %w", err)
	}
	jsonlPath := filepath.Join(dir, "deliveries.jsonl")
	jf, err := os.Create(jsonlPath)
	if err != nil {
		return fmt.Errorf("creating deliveries.jsonl: %w", err)
	}
	defer jf.Close()
	for _, row := range deliveries {
		if _, werr := jf.Write(append([]byte(row), '\n')); werr != nil {
			return werr
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "recipients.txt"), []byte(strings.Join(recipients, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing recipients.txt: %w", err)
	}
	return nil
}

// silence unused-import warning for sql; consumed via the local store in helper.
var _ = sql.ErrNoRows
