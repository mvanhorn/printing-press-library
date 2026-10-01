// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type backupListView struct {
	Tailnet string         `json:"tailnet"`
	Scope   string         `json:"scope"`
	Dir     string         `json:"dir"`
	Items   []policyBackup `json:"items"`
}

func newNovelPolicyRestoreCmd(flags *rootFlags) *cobra.Command {
	var tailnet string
	var list bool
	cmd := &cobra.Command{
		Use:   "restore [backup]",
		Short: "Roll the policy file back to a local backup after validating it and showing the diff.",
		Long: strings.TrimSpace(`
Use this command to roll the live policy file back to a local backup or to list
backups. Do NOT use this command to add a new entry; use 'policy add-entry'
instead.

--list shows local backups, newest first (policy add-entry and policy restore
take one before every write). Backups are filed under the credential and
tailnet selector they were taken with. A credential belongs to one tailnet, so
a backup is never listed or restored by ID under a credential for another
tailnet. After a key rotation, older backups are not listed; they stay under
the policy-backups state directory and a file path restores one directly. [backup] is a backup ID from --list, "latest" for the newest
backup, or a path to a HuJSON file. The command validates the backup with the
API and prints the diff from the current policy. Without --yes nothing is written (as an MCP
tool it is always plan-only). With --yes it
backs up the current policy, then writes with If-Match on the current ETag.`),
		Example: strings.Trim(`
  tailscale-pp-cli policy restore --list
  tailscale-pp-cli policy restore latest
  tailscale-pp-cli policy restore 20260930T120000-000000000Z-before-add-entry
  tailscale-pp-cli policy restore ./policy.hujson --yes`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			// Over MCP these tools are plan-only: --yes is blocked, so the
			// read-only hint is accurate. Writes need the CLI with --yes.
			"mcp:read-only":   "true",
			"mcp:write-flags": "yes",
			"pp:happy-args":   "backup=latest",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if list {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				c, err := tsLiveClient(ctx, flags)
				if err != nil {
					return err
				}
				scope, err := policyBackupScope(c, tailnet)
				if err != nil {
					return err
				}
				backups, err := listPolicyBackups(scope)
				if err != nil {
					return err
				}
				dir, _ := policyBackupDir(scope)
				view := backupListView{Tailnet: tailnet, Scope: scope, Dir: dir, Items: backups}
				if ok, err := emitMachine(cmd, flags, view); ok {
					return err
				}
				w := cmd.OutOrStdout()
				if len(backups) == 0 {
					label := fmt.Sprintf("tailnet %q", tailnet)
					if tailnet == "-" {
						label = "the credential's own tailnet"
					}
					fmt.Fprintf(w, "No policy backups yet for %s with this credential (%s).\n", label, dir)
				} else {
					tw := newTabWriter(w)
					fmt.Fprintln(tw, "ID\tCREATED\tREASON\tBYTES")
					for _, b := range backups {
						fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", b.ID, b.CreatedAt, b.Reason, b.Bytes)
					}
					if err := tw.Flush(); err != nil {
						return err
					}
				}
				return nil
			}
			if dryRunOK(flags) && (len(args) == 0 || cliutil.IsVerifyEnv()) {
				return writeDryRun(cmd.OutOrStdout(), flags, "policy restore")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("pass a backup ID or path, or --list to see backups"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			scope, err := policyBackupScope(c, tailnet)
			if err != nil {
				return err
			}
			source, text, err := loadPolicyBackup(scope, tailnet, args[0])
			if err != nil {
				return err
			}
			if _, err := tsadmin.StandardizeJSON(text); err != nil {
				return usageErr(fmt.Errorf("backup %s is not valid HuJSON: %w", source.ID, err))
			}
			snap, err := fetchPolicyHuJSON(ctx, c, tailnet)
			if err != nil {
				return err
			}
			return finishPolicyChange(ctx, cmd, flags, c, policyChange{
				tailnet: tailnet, snap: snap, candidate: text,
				view:         policyChangeView{Action: "restore", ETag: snap.ETag, Source: &source},
				noChange:     bytes.Equal(bytes.TrimSpace(snap.Text), bytes.TrimSpace(text)),
				backupReason: "before-restore",
				invalidMsg:   fmt.Sprintf("backup %s fails validation against the current tailnet", source.ID),
			})
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "List local policy backups, newest first")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}
