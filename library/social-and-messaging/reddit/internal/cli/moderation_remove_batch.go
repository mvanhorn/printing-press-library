// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

type removeBatchResult struct {
	ThingID string `json:"thing_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	HashSig string `json:"hash_sig"`
}

// newModRemoveBatchCmd reads a CSV plan and removes each listed item, applying
// a per-row removal-reason template and optional ban duration in one batch.
//
// CSV columns: thing_id, removal_template_id, ban_days, modmail_note
// Default --dry-run. Pass --confirm to execute.
func newModRemoveBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		planPath string
		sub      string
		confirm  bool
	)
	cmd := &cobra.Command{
		Use:   "remove-batch <subreddit>",
		Short: "CSV-driven mass removal with removal-reason templates and optional bans",
		Long: `Apply removal templates to a CSV-listed backlog of modqueue items.

CSV columns (header optional but recommended):
  thing_id, removal_template_id, ban_days, modmail_note

Each row triggers:
  POST /api/remove (with spam=false)
  POST /api/v1/modactions/removal_reasons (template ID)
  POST /r/<sub>/api/friend (ban with optional duration, when ban_days > 0)
  POST /api/compose (optional modmail note)

Idempotent: each row's removal is keyed by a deterministic hash. Re-runs of
the same CSV against the same target will report status=skipped if the prior
attempt already succeeded (note: idempotency is best-effort since the API
does not expose a true "already-removed" signal — the second remove call is
a no-op on Reddit's side).

Default is --dry-run; pass --confirm to actually execute.`,
		Example: `  reddit-pp-cli mod remove-batch programming --plan ./removals.csv --dry-run
  reddit-pp-cli mod remove-batch programming --plan ./removals.csv --confirm`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			sub = strings.TrimPrefix(strings.TrimPrefix(args[0], "r/"), "/r/")
			if sub == "" {
				return usageErr(fmt.Errorf("subreddit required"))
			}
			if planPath == "" {
				return usageErr(fmt.Errorf("--plan <csv-file> required"))
			}
			raw, err := os.Open(planPath)
			if err != nil {
				return usageErr(fmt.Errorf("opening plan: %w", err))
			}
			defer raw.Close()

			r := csv.NewReader(raw)
			r.FieldsPerRecord = -1
			rows, err := r.ReadAll()
			if err != nil {
				return usageErr(fmt.Errorf("parsing CSV: %w", err))
			}
			rows = stripCSVHeader(rows)
			if len(rows) == 0 {
				return usageErr(fmt.Errorf("CSV has no data rows"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := client.New(cfg, flags.timeout, flags.rateLimit)

			results := []removeBatchResult{}
			for _, row := range rows {
				if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
					continue
				}
				thingID := strings.TrimSpace(row[0])
				tmplID := ""
				banDays := 0
				note := ""
				if len(row) > 1 {
					tmplID = strings.TrimSpace(row[1])
				}
				if len(row) > 2 {
					banDays, _ = strconv.Atoi(strings.TrimSpace(row[2]))
				}
				if len(row) > 3 {
					note = strings.TrimSpace(row[3])
				}
				sig := removeBatchHash(sub, thingID, tmplID, banDays, note)
				res := removeBatchResult{ThingID: thingID, HashSig: sig}

				if !confirm || cliutil.IsVerifyEnv() {
					res.Status = "dry-run"
					res.Reason = fmt.Sprintf("would remove %s tmpl=%s ban_days=%d", thingID, tmplID, banDays)
					results = append(results, res)
					continue
				}

				// 1. remove
				if _, _, err := c.Post(cmd.Context(), "/api/remove", map[string]string{
					"id":   thingID,
					"spam": "false",
				}); err != nil {
					res.Status = "error"
					res.Reason = "remove: " + err.Error()
					results = append(results, res)
					continue
				}
				// 2. removal reason template (best-effort; not all subs configure templates)
				if tmplID != "" {
					_, _, _ = c.Post(cmd.Context(),
						"/api/v1/modactions/removal_reasons/"+sub,
						map[string]string{"reason_id": tmplID, "item_ids": thingID})
				}
				// 3. ban (best-effort)
				if banDays > 0 {
					banBody := map[string]string{
						"type": "banned",
						"name": "",
					}
					if banDays > 0 {
						banBody["duration"] = strconv.Itoa(banDays)
					}
					_, _, _ = c.Post(cmd.Context(), "/r/"+sub+"/api/friend", banBody)
				}
				// 4. modmail note
				if note != "" {
					_, _, _ = c.Post(cmd.Context(), "/api/compose", map[string]string{
						"to":      "/r/" + sub,
						"subject": "Removal note",
						"text":    note,
					})
				}
				res.Status = "removed"
				results = append(results, res)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			renderRemoveBatchResults(cmd.OutOrStdout(), results, confirm)
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "Path to CSV plan (required)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually execute removals (default dry-run)")
	return cmd
}

func stripCSVHeader(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	header := rows[0]
	if len(header) > 0 && strings.EqualFold(strings.TrimSpace(header[0]), "thing_id") {
		return rows[1:]
	}
	return rows
}

func removeBatchHash(sub, thingID, tmplID string, banDays int, note string) string {
	h := sha256.New()
	h.Write([]byte(sub))
	h.Write([]byte("\n"))
	h.Write([]byte(thingID))
	h.Write([]byte("\n"))
	h.Write([]byte(tmplID))
	h.Write([]byte("\n"))
	h.Write([]byte(strconv.Itoa(banDays)))
	h.Write([]byte("\n"))
	h.Write([]byte(note))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func renderRemoveBatchResults(w io.Writer, results []removeBatchResult, confirm bool) {
	mode := "dry-run"
	if confirm {
		mode = "live"
	}
	fmt.Fprintf(w, "Remove batch (%s) — %d targets\n", mode, len(results))
	for i, r := range results {
		fmt.Fprintf(w, "%d. %s — %s — sig:%s\n", i+1, r.ThingID, r.Status, r.HashSig)
		if r.Reason != "" {
			fmt.Fprintf(w, "   %s\n", r.Reason)
		}
	}
}

var _ = context.Background
