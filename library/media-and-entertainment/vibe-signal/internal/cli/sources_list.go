// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list registered aggregator sources (hand-authored).
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

type sourceInfo struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	AuthRequired    bool   `json:"auth_required"`
	TopicSearchable bool   `json:"topic_searchable"`
	SyncCommand     string `json:"sync_command"`
}

func newNovelSourcesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Show which sources are wired in, their auth needs, and which command syncs them",
		Example:     "  vibe-signal-pp-cli sources list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			infos := make([]sourceInfo, 0)
			for _, s := range source.All() {
				infos = append(infos, sourceInfo{
					Name:            s.Name(),
					Description:     s.Description(),
					AuthRequired:    s.AuthRequired(),
					TopicSearchable: s.TopicSearchable(),
					SyncCommand:     fmt.Sprintf("vibe-signal-pp-cli sources sync --source %s --query \"<topic>\"", s.Name()),
				})
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), infos, flags)
			}
			out := cmd.OutOrStdout()
			if len(infos) == 0 {
				fmt.Fprintln(out, "no sources registered")
				return nil
			}
			for _, i := range infos {
				auth := "no auth"
				if i.AuthRequired {
					auth = "auth required"
				}
				search := "topic-searchable"
				if !i.TopicSearchable {
					search = "feed (local query filter)"
				}
				fmt.Fprintf(out, "%-12s %s\n  %s · %s\n", i.Name, i.Description, auth, search)
			}
			return nil
		},
	}
	return cmd
}
