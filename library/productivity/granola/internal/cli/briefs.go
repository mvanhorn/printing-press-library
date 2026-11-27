// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

func newBriefsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "briefs",
		Short: "Read Granola pre-meeting briefs and follow-up email drafts",
	}
	cmd.AddCommand(newBriefsListCmd(flags))
	cmd.AddCommand(newBriefsFollowUpsCmd(flags))
	return cmd
}

func newBriefsListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Fetch pre-meeting briefs via the internal API (maple shard)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("briefs list requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetPreMeetingBriefs()
			if err != nil {
				return classifyInternalAPIErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}

func newBriefsFollowUpsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "follow-ups",
		Short: "Fetch generated follow-up email drafts via the internal API (berry shard)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("briefs follow-ups requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetFollowUpEmails()
			if err != nil {
				return classifyInternalAPIErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}
