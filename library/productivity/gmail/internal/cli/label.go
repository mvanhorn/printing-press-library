// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: label messages by NAME with get-or-create, plus the archive/
// star/spam verbs that are just label operations underneath. Beats simplegmail
// (IDs only) and absorbs GongRzhe's get_or_create_label.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newLabelCmd(flags))
	})
}

// resolveLabelIDs maps label names to IDs, case-insensitively. Missing names
// are created when create is true; system labels (INBOX, UNREAD, STARRED,
// SPAM, TRASH, IMPORTANT) match by canonical uppercase name.
func resolveLabelIDs(cmd *cobra.Command, c *client.Client, names []string, create bool) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/labels", nil)
	if err != nil {
		return nil, err
	}
	var page struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing labels.list: %w", err)
	}
	byName := map[string]string{}
	for _, l := range page.Labels {
		byName[strings.ToLower(l.Name)] = l.ID
		byName[strings.ToLower(l.ID)] = l.ID // allow passing raw IDs too
	}
	var ids []string
	for _, name := range names {
		if id, ok := byName[strings.ToLower(name)]; ok {
			ids = append(ids, id)
			continue
		}
		if !create {
			return nil, fmt.Errorf("label %q not found (use --create to create missing labels)", name)
		}
		body := map[string]any{"name": name, "labelListVisibility": "labelShow", "messageListVisibility": "show"}
		resp, _, err := c.Post(cmd.Context(), "/gmail/v1/users/me/labels", body)
		if err != nil {
			return nil, fmt.Errorf("creating label %q: %w", name, err)
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp, &created); err != nil || created.ID == "" {
			return nil, fmt.Errorf("parsing created label %q", name)
		}
		ids = append(ids, created.ID)
	}
	return ids, nil
}

type labelResult struct {
	Messages int      `json:"messages_modified"`
	Added    []string `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Status   string   `json:"status"`
}

func newLabelCmd(flags *rootFlags) *cobra.Command {
	var add, remove []string
	var query string
	var limit int
	var noCreate bool

	cmd := &cobra.Command{
		Use:   "label [message-id...]",
		Short: "Add or remove labels by name (auto-creates missing labels)",
		Long: `Labels messages by display name instead of label ID, creating user
labels that do not exist yet. System verbs are just labels: archive is
--remove INBOX, star is --add STARRED, mark-read is --remove UNREAD.
Target explicit message ids, or every match of --query.`,
		Example: strings.Trim(`
  gmail-pp-cli label 18c2f5a9b3d4e6f7 --add "Receipts/2026" --remove INBOX
  gmail-pp-cli label --query "from:noreply@github.com older_than:30d" --remove INBOX,UNREAD
  gmail-pp-cli label 18c2f5a9b3d4e6f7 --add STARRED`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "message-id=example-message-id;--add=Example",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "modify message labels")
			}
			if len(add) == 0 && len(remove) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--add or --remove is required"))
			}
			if len(args) == 0 && strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("message id arguments or --query is required"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would modify labels on:", strings.Join(args, ", "))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			addIDs, err := resolveLabelIDs(cmd, c, add, !noCreate)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			removeIDs, err := resolveLabelIDs(cmd, c, remove, false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := args
			if len(ids) == 0 {
				ids, err = gmailListIDs(cmd.Context(), c, query, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			if len(ids) == 0 {
				res := labelResult{Status: "no matching messages"}
				if wantsJSONOutput(cmd, flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no matching messages")
				return nil
			}
			// batchModify takes up to 1000 ids per call at flat quota cost.
			for start := 0; start < len(ids); start += 1000 {
				end := start + 1000
				if end > len(ids) {
					end = len(ids)
				}
				body := map[string]any{"ids": ids[start:end]}
				if len(addIDs) > 0 {
					body["addLabelIds"] = addIDs
				}
				if len(removeIDs) > 0 {
					body["removeLabelIds"] = removeIDs
				}
				if _, _, err := c.Post(cmd.Context(), "/gmail/v1/users/me/messages/batchModify", body); err != nil {
					return classifyAPIError(err, flags)
				}
			}
			res := labelResult{Messages: len(ids), Added: add, Removed: remove, Status: "ok"}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "modified %d message(s)\n", len(ids))
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&add, "add", nil, "Label name(s) to add (auto-created unless --no-create)")
	cmd.Flags().StringSliceVar(&remove, "remove", nil, "Label name(s) to remove")
	cmd.Flags().StringVar(&query, "query", "", "Apply to every message matching this Gmail query")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum messages to modify (with --query)")
	cmd.Flags().BoolVar(&noCreate, "no-create", false, "Fail instead of creating missing labels")
	return cmd
}
