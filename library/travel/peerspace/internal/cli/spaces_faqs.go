// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Space event FAQs from message-host HAR.
// GET /v1/spaces/{space_id}/faqs/event

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSpacesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "spaces",
		Short:       "Space-level resources (FAQs, etc.)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSpacesFaqsEventCmd(flags))
	return cmd
}

func newSpacesFaqsEventCmd(flags *rootFlags) *cobra.Command {
	var flagSpaceID string
	cmd := &cobra.Command{
		Use:     "faqs-event",
		Short:   "Event FAQs for a space (GET /v1/spaces/{space_id}/faqs/event).",
		Example: `  peerspace-pp-cli spaces faqs-event --space-id 68d458dba45ae0878156d4b6`,
		Annotations: map[string]string{
			"pp:endpoint":   "spaces.faqs.event",
			"pp:method":     "GET",
			"pp:path":       "/v1/spaces/{space_id}/faqs/event",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			spaceID := strings.TrimSpace(flagSpaceID)
			if spaceID == "" {
				return fmt.Errorf("--space-id is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/v1/spaces/%s/faqs/event", spaceID)
			data, _, err := resolveReadWithStrategyAndResponsePath(
				cmd.Context(), c, flags, "live", "spaces", false, path, nil, listingAuthHeaders(c), "", cmd.ErrOrStderr(),
			)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagSpaceID, "space-id", "", "Space id (listing parentSpaceId)")
	_ = cmd.MarkFlagRequired("space-id")
	return cmd
}
