// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: deploy gate — checkpoint backup, wait for completion, optional purge.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/store"
)

type guardView struct {
	Install       string `json:"install"`
	BackupID      string `json:"backup_id"`
	BackupStatus  string `json:"backup_status"`
	WaitedSeconds int    `json:"waited_seconds"`
	Purged        string `json:"purged,omitempty"`
	Gate          string `json:"gate"`
}

func newNovelGuardCmd(flags *rootFlags) *cobra.Command {
	var flagPurge string
	var flagDescription string
	var flagEmails []string
	var flagPollInterval time.Duration
	var flagWait time.Duration

	cmd := &cobra.Command{
		Use:   "guard [install]",
		Short: "Gate a deploy: create a checkpoint backup, wait until it completes, then optionally purge cache",
		Long: strings.Trim(`
Use this command to gate a deploy: it creates a checkpoint backup, waits for
completion (requested -> in_progress -> completed), and optionally purges
cache, with CI-friendly exit codes (0 = backup completed; non-zero = backup
aborted, timed out, or the API rejected the request).

The install may be given as a UUID or, when a synced mirror exists, as the
install name.

Do NOT use it to trigger a backup without waiting; use
'installs backups create' instead. Do NOT use it to purge cache alone; use
'installs purge-cache create' instead.
`, "\n"),
		Example: strings.Trim(`
  # Backup, wait for completion, then purge the CDN cache
  wpengine-pp-cli guard my-install --purge cdn

  # Backup-only gate for CI (exit 0 = safe to deploy)
  wpengine-pp-cli guard 294deacc-d8b8-4005-82c4-0727ba8ddde0 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "install=294deacc-d8b8-4005-82c4-0727ba8ddde0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				target := "<install>"
				if len(args) > 0 {
					target = args[0]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would create a checkpoint backup for %s, wait for completion", target)
				if flagPurge != "" {
					fmt.Fprintf(cmd.OutOrStdout(), ", then purge %s cache", flagPurge)
				}
				fmt.Fprintln(cmd.OutOrStdout())
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would create checkpoint backup and wait for completion (verify environment)")
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an install UUID or name is required"))
			}
			switch flagPurge {
			case "", "object", "page", "cdn", "all":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --purge %q: must be one of object, page, cdn, all", flagPurge))
			}

			// The overall wait is governed by --wait, not the per-request
			// --timeout: checkpoint backups routinely take minutes, and the
			// root --timeout default (60s) is a per-request bound. Individual
			// HTTP requests are still bounded by the client's own timeout.
			wait := flagWait
			if wait <= 0 {
				wait = 15 * time.Minute
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), wait)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve install name -> UUID via the local mirror when needed.
			// UUIDs are case-insensitive; WP Engine install names are lowercase,
			// so lowering is safe for both branches.
			installID := strings.ToLower(strings.TrimSpace(args[0]))
			if !uuidRe.MatchString(installID) {
				dbPath := defaultDBPath("wpengine-pp-cli")
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					return usageErr(fmt.Errorf("%q is not a UUID and no local mirror exists to resolve names; pass the install UUID or run 'wpengine-pp-cli sync' first", args[0]))
				}
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("opening local database to resolve install name: %w", err)
				}
				var resolved string
				lookupErr := db.DB().QueryRowContext(ctx, `SELECT id FROM installs WHERE name = ?`, installID).Scan(&resolved)
				_ = db.Close()
				if errors.Is(lookupErr, sql.ErrNoRows) || (lookupErr == nil && resolved == "") {
					return notFoundErr(fmt.Errorf("install %q not found in the local mirror; pass the UUID or run 'wpengine-pp-cli sync'", args[0]))
				}
				if lookupErr != nil {
					return fmt.Errorf("resolving install name in the local mirror: %w", lookupErr)
				}
				// Re-validate: the id column feeds URL path construction, so a
				// corrupted mirror row must not be able to retarget the request.
				if !uuidRe.MatchString(resolved) {
					return notFoundErr(fmt.Errorf("install %q resolved to a non-UUID id in the local mirror; re-run 'wpengine-pp-cli sync' or pass the UUID directly", args[0]))
				}
				installID = resolved
			}

			// notification_emails is required by the API; default to the
			// authenticated user's email when not provided.
			emails := flagEmails
			if len(emails) == 0 {
				userData, err := c.Get(ctx, "/user", nil)
				if err != nil {
					return apiErr(fmt.Errorf("fetching current user for notification email (or pass --email): %w", err))
				}
				var u struct {
					Email string `json:"email"`
				}
				if err := json.Unmarshal(userData, &u); err != nil || u.Email == "" {
					return usageErr(fmt.Errorf("could not determine a notification email from /user; pass --email"))
				}
				emails = []string{u.Email}
			}

			desc := flagDescription
			if desc == "" {
				desc = "Checkpoint backup created by wpengine-pp-cli guard"
			}

			start := time.Now()
			body := map[string]any{"description": desc, "notification_emails": emails}
			created, _, err := c.Post(ctx, "/installs/"+installID+"/backups", body)
			if err != nil {
				return apiErr(fmt.Errorf("creating checkpoint backup: %w", err))
			}
			var backup struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(created, &backup); err != nil || !uuidRe.MatchString(backup.ID) {
				return apiErr(fmt.Errorf("unexpected backup-create response (no valid backup id)"))
			}

			// Poll while the backup is in a known non-terminal state. Anything
			// else (completed, aborted, or an unrecognized status like
			// "failed") is treated as terminal so the gate cannot spin on a
			// dead backup. Dogfood curtails to a single poll so the live
			// matrix stays inside its per-command timeout; a hard poll cap
			// backstops even an undeadlined context.
			interval := flagPollInterval
			if interval <= 0 {
				interval = 5 * time.Second
			}
			maxPolls := 720 // hard cap: 1h at the default 5s interval
			if cliutil.IsDogfoodEnv() {
				maxPolls = 1
			}
			status := backup.Status
			polls := 0
			for status == "requested" || status == "in_progress" {
				if polls >= maxPolls {
					break
				}
				select {
				case <-ctx.Done():
					return apiErr(fmt.Errorf("timed out waiting for backup %s (last status %q); raise --wait", backup.ID, status))
				case <-time.After(interval):
				}
				polls++
				data, err := c.GetNoCache(ctx, "/installs/"+installID+"/backups/"+backup.ID, nil)
				if err != nil {
					return apiErr(fmt.Errorf("polling backup %s: %w", backup.ID, err))
				}
				var st struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(data, &st); err == nil && st.Status != "" {
					status = st.Status
				}
			}

			view := guardView{
				Install:       installID,
				BackupID:      backup.ID,
				BackupStatus:  status,
				WaitedSeconds: int(time.Since(start).Seconds()),
				Gate:          "pass",
			}

			switch status {
			case "completed":
				// fall through to purge/pass
			case "requested", "in_progress":
				// Poll cap hit while still running. Dogfood's single-poll
				// curtailment reports softly; in real use an incomplete backup
				// must not exit 0 — the documented CI contract is
				// 0 = completed.
				view.Gate = "pending"
				if cliutil.IsDogfoodEnv() {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
				return apiErr(fmt.Errorf("backup %s still %q after the poll cap; deploy gate pending, not passed", backup.ID, status))
			default:
				// aborted, failed, or any unrecognized terminal status.
				view.Gate = "fail"
				_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
				return apiErr(fmt.Errorf("backup %s ended with status %q; deploy gate failed", backup.ID, status))
			}

			if flagPurge != "" {
				if _, _, err := c.Post(ctx, "/installs/"+installID+"/purge_cache", map[string]any{"type": flagPurge}); err != nil {
					view.Gate = "backup-ok-purge-failed"
					_ = printJSONFiltered(cmd.OutOrStdout(), view, flags)
					return apiErr(fmt.Errorf("backup completed but cache purge failed: %w", err))
				}
				view.Purged = flagPurge
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagPurge, "purge", "", "Cache type to purge after the backup completes: object, page, cdn, or all")
	cmd.Flags().StringVar(&flagDescription, "description", "", "Backup description (default: checkpoint note naming this tool)")
	cmd.Flags().StringSliceVar(&flagEmails, "email", nil, "Notification email(s) for the backup (default: the authenticated user's email)")
	cmd.Flags().DurationVar(&flagPollInterval, "poll-interval", 5*time.Second, "How often to poll backup status while waiting")
	cmd.Flags().DurationVar(&flagWait, "wait", 15*time.Minute, "Maximum time to wait for the backup to complete (the root --timeout only bounds individual requests)")
	return cmd
}
