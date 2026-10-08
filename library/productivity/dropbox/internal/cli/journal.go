// pp:data-source local
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type journalBatchView struct {
	store.DropboxJournalBatch
	Counts               map[string]int           `json:"counts"`
	RestoreDaysRemaining *int                     `json:"restore_days_remaining"`
	Ops                  []store.DropboxJournalOp `json:"ops,omitempty"`
}
type journalList struct {
	Batches []journalBatchView `json:"batches"`
}

func journalView(ctx context.Context, db *store.Store, b store.DropboxJournalBatch, withOps bool) (journalBatchView, error) {
	ops, err := db.ListDropboxJournalOps(ctx, b.ID)
	if err != nil {
		return journalBatchView{}, err
	}
	v := journalBatchView{DropboxJournalBatch: b, Counts: map[string]int{"ok": 0, "failed": 0, "unknown": 0, "pending": 0, "skipped": 0}}
	hasDelete := false
	for _, op := range ops {
		v.Counts[op.Result]++
		if op.Op == "delete" || op.Op == "delete_child" {
			hasDelete = true
		}
	}
	if hasDelete {
		at, err := time.Parse(time.RFC3339, b.CreatedAt)
		if err == nil {
			elapsed := int(time.Since(at).Hours() / 24)
			if elapsed < 0 {
				elapsed = 0
			}
			remaining := b.RestoreDays - elapsed
			if remaining < 0 {
				remaining = 0
			}
			v.RestoreDaysRemaining = &remaining
		}
	}
	if withOps {
		v.Ops = ops
	}
	return v, nil
}
func newNovelJournalCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var withOps bool
	cmd := &cobra.Command{Use: "journal [batch-id]", Short: "Inspect applied and undone Dropbox batches", Long: "List batches newest first, including operation results and remaining soft-delete restore days.", Example: strings.Trim(`
  dropbox-pp-cli journal --agent
  dropbox-pp-cli journal 20261006-153012-abcd --ops --agent`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--limit=5"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "journal")
		}
		if len(args) > 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("journal accepts at most one batch id"))
		}
		if limit < 1 {
			return usageErr(fmt.Errorf("--limit must be positive"))
		}
		if err := localSourceOnly(flags); err != nil {
			return err
		}
		result := journalList{Batches: make([]journalBatchView, 0)}
		db, found, err := openIndex(cmd, flags, dbPath)
		if err != nil {
			return err
		}
		if !found {
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		defer db.Close()
		batches := make([]store.DropboxJournalBatch, 0)
		if len(args) == 1 {
			b, ok, err := db.GetDropboxJournalBatch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if !ok {
				return usageErr(fmt.Errorf("journal batch %q not found", args[0]))
			}
			batches = append(batches, b)
		} else {
			batches, err = db.ListDropboxJournalBatches(cmd.Context(), limit)
			if err != nil {
				return err
			}
		}
		for _, b := range batches {
			v, err := journalView(cmd.Context(), db, b, withOps)
			if err != nil {
				return err
			}
			result.Batches = append(result.Batches, v)
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		for _, b := range result.Batches {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s ok=%d failed=%d unknown=%d pending=%d skipped=%d\n", b.ID, b.Status, b.Counts["ok"], b.Counts["failed"], b.Counts["unknown"], b.Counts["pending"], b.Counts["skipped"])
			if withOps {
				for _, op := range b.Ops {
					target := op.Path
					if op.Op == "move" {
						target = op.FromPath + " -> " + op.ToPath
					}
					if op.Op == "revoke_link" {
						target = op.URL
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %d %s %s %s job=%s %s\n", op.Seq, op.Op, op.Result, target, op.AsyncJobID, op.Error)
				}
			}
		}
		return nil
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum batches to list")
	cmd.Flags().BoolVar(&withOps, "ops", false, "Include each operation")
	return cmd
}
