// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// labels.go declares the `labels` namespace and its subcommands. v1.1 ships
// `labels list` (MCP parity with list_labels). Future verbs (`labels add`,
// `labels remove`) can plug into the same namespace.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/gmail"
)

// newLabelsCmd registers the `labels` namespace.
func newLabelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labels",
		Short: "Gmail labels (list)",
		Long: `Gmail labels. Subcommands:

  list                   List every system + user label visible to the active account

System labels (INBOX, SENT, DRAFT, SPAM, TRASH, IMPORTANT, STARRED, UNREAD,
CATEGORY_*) come first; user-created labels follow alphabetical.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newLabelsListCmd(flags))
	return cmd
}

// newLabelsListCmd registers `labels list`.
func newLabelsListCmd(flags *rootFlags) *cobra.Command {
	var systemOnly, userOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels visible to the active account",
		Long: `List every Gmail label visible to the active account.

--system-only filters to the 11 built-in labels; --user-only filters to
user-created labels. Without either flag, both groups print (system first).`,
		Example: "  superhuman-pp-cli labels list\n  superhuman-pp-cli labels list --user-only --json\n  superhuman-pp-cli labels list --system-only",
		Annotations: map[string]string{
			"pp:endpoint":   "labels.list",
			"pp:method":     "GET",
			"pp:path":       "/users/me/labels",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if systemOnly && userOnly {
				return usageErr(fmt.Errorf("labels list: pass at most one of --system-only / --user-only"))
			}
			return runLabelsList(cmd, flags, systemOnly, userOnly)
		},
	}
	cmd.Flags().BoolVar(&systemOnly, "system-only", false, "List only Gmail's system labels")
	cmd.Flags().BoolVar(&userOnly, "user-only", false, "List only user-created labels")
	return cmd
}

func runLabelsList(cmd *cobra.Command, flags *rootFlags, systemOnly, userOnly bool) error {
	if cliutil.IsVerifyEnv() {
		fmt.Fprintln(cmd.OutOrStdout(), "would call gmail.googleapis.com/.../labels")
		return nil
	}

	acct, err := resolveActiveAccount(flags)
	if err != nil {
		return authErr(fmt.Errorf("labels list: %w", err))
	}
	gc := gmail.New(acct.Store, acct.Email, acct.GoogleID, acct.AccessToken)
	gc.Stderr = cmd.ErrOrStderr()

	all, err := gc.ListLabels(cmd.Context())
	if err != nil {
		if gmail.IsAuth(err) {
			return authErr(fmt.Errorf("labels list: %w", err))
		}
		return apiErr(fmt.Errorf("labels list: %w", err))
	}

	// Filter.
	filtered := all
	if systemOnly {
		filtered = filterByType(all, "system")
	} else if userOnly {
		filtered = filterByType(all, "user")
	}

	// JSON envelope.
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		rows := make([]map[string]any, 0, len(filtered))
		for _, l := range filtered {
			rows = append(rows, map[string]any{
				"id":   l.ID,
				"name": l.Name,
				"type": l.Type,
			})
		}
		envelope := map[string]any{
			"action":   "labels.list",
			"resource": "labels",
			"path":     "/users/me/labels",
			"success":  true,
			"data":     rows,
			"count":    len(filtered),
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human path.
	if len(filtered) == 0 {
		if userOnly {
			fmt.Fprintln(cmd.OutOrStdout(), "No user labels.")
		} else if systemOnly {
			fmt.Fprintln(cmd.OutOrStdout(), "No system labels (unexpected).")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No labels.")
		}
		return nil
	}
	rows := make([]map[string]any, 0, len(filtered))
	for _, l := range filtered {
		rows = append(rows, map[string]any{
			"id":   l.ID,
			"name": l.Name,
			"type": l.Type,
		})
	}
	if perr := printAutoTable(cmd.OutOrStdout(), rows); perr != nil {
		for _, l := range filtered {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", l.ID, l.Name, l.Type)
		}
	}
	return nil
}

// filterByType returns the subset of labels whose Type matches the filter.
func filterByType(labels []gmail.Label, t string) []gmail.Label {
	out := make([]gmail.Label, 0, len(labels))
	for _, l := range labels {
		if l.Type == t {
			out = append(out, l)
		}
	}
	return out
}
