// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type policyChangeView struct {
	Action         string                 `json:"action"`
	DryRun         bool                   `json:"dry_run,omitempty"`
	Section        string                 `json:"section,omitempty"`
	Mode           string                 `json:"mode"`
	Applied        bool                   `json:"applied"`
	AlreadyPresent bool                   `json:"already_present,omitempty"`
	CreatedSection bool                   `json:"created_section,omitempty"`
	ETag           string                 `json:"etag"`
	NewETag        string                 `json:"new_etag,omitempty"`
	Validation     tsadmin.ValidateResult `json:"validation"`
	Diff           policyDiffView         `json:"diff"`
	Backup         *policyBackup          `json:"backup,omitempty"`
	Source         *policyBackup          `json:"source_backup,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
	Next           string                 `json:"next,omitempty"`
}

func newNovelPolicyAddEntryCmd(flags *rootFlags) *cobra.Command {
	var tailnet, entry, entryFile string
	cmd := &cobra.Command{
		Use:   "add-entry <section>",
		Short: "Append one nodeAttrs, grants, acls, or ssh entry to the policy file with comments preserved, a local backup, validation, a diff, and a concurrency check.",
		Long: strings.TrimSpace(`
Use this command to append one nodeAttrs or grants entry to the live policy
file. Do NOT use this command to roll the policy back to an earlier version; use
'policy restore' instead.

<section> is nodeAttrs, grants, acls, or ssh. The command reads the policy as
HuJSON (so comments survive) together with its ETag, appends the entry with a
comment-preserving patch, validates the result with the API, and prints the
added and removed lines. An entry equal to one already present is a no-op.

Without --yes nothing is written (as an MCP tool it is always plan-only).
With --yes it saves a local backup first,
then writes with If-Match set to the ETag, so a concurrent edit is never
overwritten (HTTP 412 aborts the write). Undo with 'policy restore'.`),
		Example: strings.Trim(`
  tailscale-pp-cli policy add-entry nodeAttrs --entry '{"target":["autogroup:member"],"attr":["drive:access"]}'
  tailscale-pp-cli policy add-entry grants --entry-file grant.hujson --yes`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			// Over MCP these tools are plan-only: --yes is blocked, so the
			// read-only hint is accurate. Writes need the CLI with --yes.
			"mcp:read-only":   "true",
			"mcp:write-flags": "yes",
			"pp:happy-args":   `section=nodeAttrs;--entry={"target":["autogroup:member"],"attr":["drive:access"]}`,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && (len(args) == 0 || cliutil.IsVerifyEnv()) {
				return writeDryRun(cmd.OutOrStdout(), flags, "policy add-entry")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<section> is required: one of %s", strings.Join(tsadmin.ArraySections, ", ")))
			}
			section := tsadmin.CanonicalSection(args[0])
			if section == "" {
				return usageErr(fmt.Errorf("section %q is not supported; use one of %s", args[0], strings.Join(tsadmin.ArraySections, ", ")))
			}
			if (entry == "") == (entryFile == "") {
				return usageErr(errors.New("pass exactly one of --entry '<json>' or --entry-file <path|->"))
			}
			body := []byte(entry)
			if entryFile != "" {
				var err error
				if entryFile == "-" {
					body, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
				} else {
					body, err = os.ReadFile(filepath.Clean(entryFile))
				}
				if err != nil {
					return usageErr(fmt.Errorf("reading --entry-file: %w", err))
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			snap, err := fetchPolicyHuJSON(ctx, c, tailnet)
			if err != nil {
				return err
			}
			res, err := tsadmin.AddEntry(snap.Text, section, body)
			if err != nil {
				return usageErr(err)
			}
			view := policyChangeView{Action: "add-entry", Section: section, ETag: snap.ETag, AlreadyPresent: res.AlreadyPresent, CreatedSection: res.CreatedSection}
			if res.FormatNote != "" {
				view.Warnings = append(view.Warnings, res.FormatNote)
			}
			return finishPolicyChange(ctx, cmd, flags, c, policyChange{
				tailnet: tailnet, snap: snap, candidate: res.Text, view: view,
				noChange: res.AlreadyPresent, backupReason: "before-add-entry",
				invalidMsg: "the policy with this entry fails validation",
			})
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "The entry to append, as a JSON or HuJSON object")
	cmd.Flags().StringVar(&entryFile, "entry-file", "", "Read the entry from a file ('-' for stdin)")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}

// policyChange is one candidate policy write shared by add-entry and restore.
type policyChange struct {
	tailnet      string
	snap         policySnapshot
	candidate    []byte
	view         policyChangeView
	noChange     bool
	backupReason string
	invalidMsg   string
}

// finishPolicyChange runs the common tail of every policy write: diff,
// validate, stop at the plan without --yes, back up, then write with If-Match.
func finishPolicyChange(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *client.Client, pc policyChange) error {
	view := pc.view
	view.DryRun = flags.dryRun
	view.Diff = buildPolicyDiff(pc.snap.Text, pc.candidate)
	if pc.noChange {
		view.Mode = "no-change"
		view.Validation = tsadmin.ValidateResult{OK: true}
		return emitPolicyChange(cmd, flags, view)
	}
	var err error
	view.Validation, err = validatePolicy(ctx, c, pc.tailnet, pc.candidate)
	if err != nil {
		return err
	}
	if !view.Validation.OK {
		view.Mode = "invalid"
		if perr := emitPolicyChange(cmd, flags, view); perr != nil {
			return perr
		}
		return usageErr(fmt.Errorf("%s: %s", pc.invalidMsg, view.Validation.Message))
	}
	if tsPlanOnly(flags) {
		view.Mode = "plan"
		view.Next = tsApplyHint
		return emitPolicyChange(cmd, flags, view)
	}
	scope, err := policyBackupScope(c, pc.tailnet)
	if err != nil {
		return err
	}
	backup, err := savePolicyBackup(scope, pc.tailnet, pc.snap, pc.backupReason)
	if err != nil {
		return err
	}
	view.Backup = &backup
	newETag, err := writePolicy(ctx, c, pc.tailnet, pc.candidate, pc.snap.ETag)
	if err != nil {
		return err
	}
	view.Mode = "applied"
	view.Applied = true
	view.NewETag = newETag
	return emitPolicyChange(cmd, flags, view)
}

func emitPolicyChange(cmd *cobra.Command, flags *rootFlags, v policyChangeView) error {
	if ok, err := emitMachine(cmd, flags, v); ok {
		return err
	}
	w := cmd.OutOrStdout()
	label := "policy " + v.Action
	if v.Section != "" {
		label += " " + v.Section
	}
	fmt.Fprintf(w, "%s: %s\n", label, v.Mode)
	switch {
	case v.Validation.OK && v.Validation.Warnings:
		fmt.Fprintf(w, "  validation: passed with warnings (%s)\n", v.Validation.Message)
	case v.Validation.OK:
		fmt.Fprintln(w, "  validation: passed")
	default:
		fmt.Fprintf(w, "  validation: FAILED (%s)\n", v.Validation.Message)
		if len(v.Validation.Data) > 0 {
			fmt.Fprintf(w, "  details: %s\n", string(v.Validation.Data))
		}
	}
	fmt.Fprintf(w, "  %d line(s) added, %d removed\n", len(v.Diff.Added), len(v.Diff.Removed))
	if v.Diff.Text != "" {
		fmt.Fprint(w, v.Diff.Text)
	}
	if v.Backup != nil {
		fmt.Fprintf(w, "  backup: %s\n", v.Backup.ID)
	}
	for _, warn := range v.Warnings {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
	if v.Next != "" {
		fmt.Fprintf(w, "  %s\n", v.Next)
	}
	return nil
}
