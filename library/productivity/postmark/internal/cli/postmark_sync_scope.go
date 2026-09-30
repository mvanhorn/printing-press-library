// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if syncCmd, _, err := root.Find([]string{"sync"}); err == nil && syncCmd.Name() == "sync" {
			if f := syncCmd.Flags().Lookup("no-prune"); f != nil {
				f.Usage = "Keep local rows the API no longer returns; always on for --full here because the archive holds every synced server's rows, and --no-prune=false is refused"
			}
		}
		for _, path := range [][]string{{"sync"}, {"workflow", "archive"}} {
			if cmd, _, err := root.Find(path); err == nil && cmd != root && cmd.Name() == path[len(path)-1] {
				scopePostmarkCheckpointWriter(cmd, flags)
			}
		}
	})
}

// scopePostmarkCheckpointWriter wraps a command that reads and writes sync
// checkpoints. The whole run holds a lock beside the database, so two servers
// syncing into one archive take turns, and the run first claims the
// checkpoints for the selected server: when another server wrote them, every
// checkpoint is reset so this server's older messages are not skipped.
// Argument and flag validation in PersistentPreRunE has already passed.
func scopePostmarkCheckpointWriter(cmd *cobra.Command, flags *rootFlags) {
	inner := cmd.RunE
	if inner == nil {
		return
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if flags.dryRun || postmarkSelection.sandbox || cliutil.IsVerifyEnv() {
			return inner(c, args)
		}
		client, err := flags.newClient()
		if err != nil {
			return inner(c, args) // the command reports client and auth errors itself
		}
		scope := postmarkServerScope(client)
		if scope == "" {
			return inner(c, args) // account-only run: no server checkpoints to protect
		}
		dbPath := ""
		if f := c.Flags().Lookup("db"); f != nil {
			dbPath = strings.TrimSpace(f.Value.String())
		}
		if dbPath == "" {
			dbPath = defaultDBPath("postmark-pp-cli")
		}
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return fmt.Errorf("creating the archive directory for %s: %w", dbPath, err)
		}
		lockCtx, cancel := postmarkSyncLockContext(c)
		release, err := acquirePostmarkSyncLock(lockCtx, dbPath+".sync.lock")
		cancel()
		if err != nil {
			return err
		}
		defer release()
		db, err := store.OpenWithContext(c.Context(), dbPath)
		if err != nil {
			return fmt.Errorf("opening %s to claim sync checkpoints: %w", dbPath, err)
		}
		reset, err := db.ClaimPostmarkSyncScope(c.Context(), scope)
		_ = db.Close()
		if err != nil {
			return err
		}
		if reset {
			fmt.Fprintln(c.ErrOrStderr(), "note: this archive's sync checkpoints belonged to another server; they were reset, so this run reads the selected server from the beginning (nothing is pruned).")
		}
		return inner(c, args)
	}
}

// postmarkSyncLockContext bounds the wait for another sync by the command's
// own --timeout (workflow archive's local flag or the global one); zero means
// wait until interrupted.
func postmarkSyncLockContext(c *cobra.Command) (context.Context, context.CancelFunc) {
	if f := c.Flags().Lookup("timeout"); f != nil {
		if d, err := time.ParseDuration(f.Value.String()); err == nil && d > 0 {
			return context.WithTimeout(c.Context(), d)
		}
	}
	return context.WithCancel(c.Context())
}
