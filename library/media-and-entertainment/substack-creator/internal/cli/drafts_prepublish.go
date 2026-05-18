package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newDraftsPrepublishCmd validates a draft for publication and returns any blockers.
// Endpoint: POST /api/v1/drafts/{id}/prepublish (per-publication subdomain)
func newDraftsPrepublishCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepublish <id>",
		Short: "Validate a draft for publication — returns blockers if any.",
		Long: `Calls Substack's prepublish validation endpoint. Returns a list of issues
that would prevent the draft from being published (missing title, empty body,
unsupported content, etc.). Use this before 'drafts publish' or 'drafts schedule'
to catch problems early.

Requires --subdomain <publication-subdomain>.`,
		Example:     "  substack-creator-pp-cli drafts prepublish 12345 --subdomain mypub --json",
		Annotations: map[string]string{"pp:endpoint": "drafts.prepublish", "pp:method": "POST", "pp:path": "/drafts/{id}/prepublish", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/drafts/" + args[0] + "/prepublish"
			resp, status, err := c.Post(path, map[string]any{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				envelope := map[string]any{
					"action":   "prepublish",
					"resource": "drafts",
					"path":     path,
					"status":   status,
					"success":  status >= 200 && status < 300,
				}
				if len(resp) > 0 {
					var parsed any
					if json.Unmarshal(resp, &parsed) == nil {
						envelope["data"] = parsed
					}
				}
				out, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Draft %s prepublish: HTTP %d\n", args[0], status)
			return printOutputWithFlags(cmd.OutOrStdout(), resp, flags)
		},
	}
	return cmd
}
