// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/config"
)

// pullTimeout is the default deadline for mirror pull.
const pullTimeout = 10 * time.Minute

func newMirrorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mirror",
		Short:   "Download your budget straight from the Actual server into a local read-only mirror",
		Example: "  actual-budget-pp-cli mirror pull\n  actual-budget-pp-cli mirror status --json",
		Long: `The mirror is a copy of your budget's own SQLite database, downloaded directly
from your Actual sync server (no Node, no actual-http-api sidecar). Every offline
command — ledger, report, audit, templates, changes — reads it.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMirrorPullCmd(flags), newMirrorStatusCmd(flags))
	return cmd
}

func newMirrorPullCmd(flags *rootFlags) *cobra.Command {
	var keepZip bool
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Download (and decrypt) the budget from the Actual server into the local mirror",
		Long: `Logs in to the Actual sync server with ACTUAL_PASSWORD (or ACTUAL_SESSION_TOKEN),
downloads the budget named by ACTUAL_SYNC_ID / --budget-sync-id, decrypts it with
ACTUAL_ENCRYPTION_PASSWORD when the budget is end-to-end encrypted, and replaces
the local mirror. Use --keep-zip to also keep a timestamped copy of the budget
zip, which Actual's "Import → Actual" can restore — a backup that does not
depend on a matching client version.`,
		Example: `  actual-budget-pp-cli mirror pull
  actual-budget-pp-cli mirror pull --keep-zip --json
  actual-budget-pp-cli mirror pull --budget-sync-id 1cfdbb80-6274-49bf-b0c2-737235a4c81f`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror pull")
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("mirror pull downloads from the Actual server; there is no local data source for it"))
			}
			syncID := resolveSyncID(flags)
			if syncID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no budget selected: set %s or pass --budget-sync-id (list ids with 'actual-budget-pp-cli budgets-remote')", config.EnvSyncID))
			}
			c, err := newActualClient(flags)
			if err != nil {
				return err
			}
			// Downloading a large budget can outlast the default per-command
			// --timeout; allow pullTimeout unless --timeout was set explicitly.
			deadline := flags.timeout
			if !flags.timeoutExplicit && deadline < pullTimeout {
				deadline = pullTimeout
			}
			ctx, cancel := cmd.Context(), context.CancelFunc(func() {})
			if deadline > 0 {
				ctx, cancel = context.WithTimeout(ctx, deadline)
			}
			defer cancel()
			started := time.Now()
			res, err := actual.Pull(ctx, c, actual.PullOptions{
				Password:           config.ActualPassword(),
				SessionToken:       config.ActualSessionToken(),
				SyncID:             syncID,
				EncryptionPassword: config.ActualEncryptionPassword(),
				KeepZip:            keepZip,
			})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return mapActualErr(fmt.Errorf("mirror pull did not finish within %s (raise it with --timeout): %w", deadline, err))
				}
				return mapActualErr(err)
			}
			if res.Manifest.SyncID != syncID {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s is a file id; the budget's sync id is %s (set %s=%s)\n", syncID, res.Manifest.SyncID, config.EnvSyncID, res.Manifest.SyncID)
			}
			out := map[string]any{
				"budget":         res.Manifest.Name,
				"sync_id":        res.Manifest.SyncID,
				"encrypted":      res.Manifest.Encrypted,
				"server_version": res.Manifest.ServerVersion,
				"pulled_at":      res.Manifest.PulledAt.Format(time.RFC3339),
				"db_path":        res.DBPath,
				"db_bytes":       res.Manifest.DBBytes,
				"elapsed_ms":     time.Since(started).Milliseconds(),
			}
			if res.ZipPath != "" {
				out["backup_zip"] = res.ZipPath
			}
			if h, err := openMirrorAt(res.DBPath); err == nil {
				out["counts"] = mirrorCounts(cmd, h)
				h.Close()
			}
			return emitRows(cmd, flags, out, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "Pulled %q (%s) from Actual %s — %d KB\n", res.Manifest.Name, res.Manifest.SyncID, res.Manifest.ServerVersion, res.Manifest.DBBytes/1024)
				if counts, ok := out["counts"].(map[string]int); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d accounts, %d transactions, %d categories, %d payees\n", counts["accounts"], counts["transactions"], counts["categories"], counts["payees"])
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  mirror: %s\n", res.DBPath)
				if res.ZipPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  backup: %s\n", res.ZipPath)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&keepZip, "keep-zip", false, "Also save a timestamped copy of the budget zip under the mirror's backups/ directory")
	return cmd
}

// mirrorCounts returns live row counts for the headline entities.
func mirrorCounts(cmd *cobra.Command, h *mirrorHandle) map[string]int {
	counts := map[string]int{}
	for name, q := range map[string]string{
		"accounts":     "SELECT COUNT(*) FROM accounts WHERE tombstone = 0",
		"transactions": "SELECT COUNT(*) FROM transactions WHERE tombstone = 0 AND COALESCE(isParent, 0) = 0",
		"categories":   "SELECT COUNT(*) FROM categories WHERE tombstone = 0",
		"payees":       "SELECT COUNT(*) FROM payees WHERE tombstone = 0 AND transfer_acct IS NULL",
		"rules":        "SELECT COUNT(*) FROM rules WHERE tombstone = 0",
		"schedules":    "SELECT COUNT(*) FROM schedules WHERE tombstone = 0",
	} {
		var n int
		if err := h.Ledger.DB.QueryRowContext(cmd.Context(), q).Scan(&n); err == nil {
			counts[name] = n
		}
	}
	return counts
}

func newMirrorStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show which budget is mirrored locally, when it was pulled, and its row counts",
		Example:     "  actual-budget-pp-cli mirror status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror status")
			}
			syncID := resolveSyncID(flags)
			status := mirrorStatus(cmd, syncID)
			return emitRows(cmd, flags, status, func() error {
				if syncID == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No budget selected. Set %s or pass --budget-sync-id.\n", config.EnvSyncID)
					return nil
				}
				if status["mirrored"] != true {
					fmt.Fprintf(cmd.OutOrStdout(), "Budget %s is not mirrored yet. Run: %s\n", syncID, mirrorPullHint)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%v (%s)\n  pulled: %v (%v min ago)\n  path:   %v\n", status["budget"], syncID, status["pulled_at"], status["age_minutes"], status["db_path"])
				if bt, ok := status["budget_type"].(string); ok && bt != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  type:   %s\n", bt)
				}
				if counts, ok := status["counts"].(map[string]int); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d accounts, %d transactions, %d categories, %d payees, %d rules, %d schedules\n",
						counts["accounts"], counts["transactions"], counts["categories"], counts["payees"], counts["rules"], counts["schedules"])
				}
				return nil
			})
		},
	}
	return cmd
}

// mirrorStatus describes the local mirror of syncID. db_path is where the
// mirror is (or where a pull would write it); mirrored reports whether it
// exists.
func mirrorStatus(cmd *cobra.Command, syncID string) map[string]any {
	status := map[string]any{"sync_id": syncID, "mirrored": false}
	if syncID == "" {
		return status
	}
	path, m, found, err := actual.LocateMirror(syncID)
	if err != nil {
		return status
	}
	if !found {
		if p, err := actual.DBPath(syncID); err == nil {
			status["db_path"] = p
		}
		return status
	}
	status["db_path"] = path
	status["mirrored"] = true
	if m != nil {
		status["budget"] = m.Name
		status["pulled_at"] = m.PulledAt.Format(time.RFC3339)
		status["age_minutes"] = int(time.Since(m.PulledAt).Minutes())
		status["server_version"] = m.ServerVersion
		status["encrypted"] = m.Encrypted
		if m.BudgetType != "" {
			status["budget_type"] = m.BudgetType
		}
	}
	if h, err := openMirrorAt(path); err == nil {
		status["counts"] = mirrorCounts(cmd, h)
		h.Close()
	}
	return status
}

func newBudgetsRemoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budgets-remote",
		Short: "List the budgets stored on your Actual server, with the sync id each one needs",
		Long: `Talks to the Actual sync server directly (ACTUAL_SERVER_URL + ACTUAL_PASSWORD) and
lists every budget file. The sync_id column is the value for ACTUAL_SYNC_ID.`,
		Example:     "  actual-budget-pp-cli budgets-remote\n  actual-budget-pp-cli budgets-remote --agent --select name,sync_id,encrypted",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "budgets-remote")
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("budgets-remote queries the Actual server; there is no local data source for it"))
			}
			c, err := newActualClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if err := actual.Authenticate(ctx, c, config.ActualPassword(), config.ActualSessionToken()); err != nil {
				return mapActualErr(err)
			}
			files, err := c.ListFiles(ctx)
			if err != nil {
				return mapActualErr(err)
			}
			current := resolveSyncID(flags)
			type row struct {
				Name      string `json:"name"`
				SyncID    string `json:"sync_id"`
				FileID    string `json:"file_id"`
				Encrypted bool   `json:"encrypted"`
				Mirrored  bool   `json:"mirrored"`
				Selected  bool   `json:"selected"`
			}
			rows := make([]row, 0, len(files))
			for _, f := range files {
				if f.Deleted != 0 {
					continue
				}
				r := row{Name: f.Name, SyncID: f.GroupID, FileID: f.FileID, Encrypted: f.EncryptKeyID != "", Selected: current != "" && (current == f.GroupID || current == f.FileID)}
				if _, _, found, err := actual.LocateMirror(f.GroupID); err == nil && found {
					r.Mirrored = true
				}
				rows = append(rows, r)
			}
			return emitRows(cmd, flags, rows, func() error {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No budgets on this server.")
					return nil
				}
				items := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					items = append(items, map[string]any{"name": r.Name, "sync_id": r.SyncID, "encrypted": r.Encrypted, "mirrored": r.Mirrored, "selected": r.Selected})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			})
		},
	}
	return cmd
}
