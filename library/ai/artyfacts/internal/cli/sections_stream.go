// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSectionsStreamStartCmd(flags *rootFlags) *cobra.Command {
	var flagAgentName string

	cmd := &cobra.Command{
		Use:   "stream-start <artifact-id> <section-id>",
		Short: "Mark a section as in-progress (is_streaming=true)",
		Long: `Sets is_streaming=true on a section to signal that content is being written.
Artyfacts UI shows a live-edit indicator while streaming is active.
Call 'stream-end' when the content is complete.

Designed for CI pipelines that cannot use the MCP server's streaming pattern.`,
		Example:     "  artyfacts-pp-cli sections stream-start <artifact-id> build-log\n  make build 2>&1 | artyfacts-pp-cli sections update <artifact-id> build-log --body -\n  artyfacts-pp-cli sections stream-end <artifact-id> build-log",
		Annotations: map[string]string{"pp:endpoint": "sections.update"},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			artifactID := args[0]
			sectionID := args[1]

			body := map[string]any{"is_streaming": true}
			if flagAgentName != "" {
				body["agent_name"] = flagAgentName
			}

			path := fmt.Sprintf("/artifacts/%s/sections/%s", artifactID, sectionID)
			data, _, err := c.Patch(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Section %s/%s marked as streaming.\n", artifactID, sectionID)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAgentName, "agent-name", "", "Agent name to record on the section")
	return cmd
}

func newSectionsStreamEndCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream-end <artifact-id> <section-id>",
		Short: "Mark a section as complete (is_streaming=false)",
		Long: `Sets is_streaming=false on a section to signal that content writing is done.
Call this after 'stream-start' and after all section content has been written.`,
		Example:     "  artyfacts-pp-cli sections stream-end <artifact-id> build-log",
		Annotations: map[string]string{"pp:endpoint": "sections.update"},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			artifactID := args[0]
			sectionID := args[1]

			path := fmt.Sprintf("/artifacts/%s/sections/%s", artifactID, sectionID)
			data, _, err := c.Patch(path, map[string]any{"is_streaming": false})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Section %s/%s streaming complete.\n", artifactID, sectionID)
			return nil
		},
	}
	return cmd
}
