// Hand-authored novel feature: ACL audit across calendars. Survives regen.
package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newACLAuditCmd(flags *rootFlags) *cobra.Command {
	var calendarsCSV, role string
	cmd := &cobra.Command{
		Use:   "acl-audit",
		Short: "Flatten every calendar's sharing rules into one access table",
		Long: "Audit who has access to which calendars in one flat view, optionally filtered by --role.\n\n" +
			"Joins the calendar list with per-calendar ACL rules — the Google UI has no cross-calendar sharing\n" +
			"view, so this answers \"who can write to any of my calendars?\" in a single command.",
		Example: "  google-calendar-pp-cli acl-audit --role writer --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cals := resolveCalendars(calendarsCSV)
			rules, _, err := gcalLoadACL(cmd, flags, cals)
			if err != nil {
				return err
			}
			roleFilter := strings.ToLower(strings.TrimSpace(role))
			var out []gcalACLRule
			for _, r := range rules {
				if roleFilter != "" && strings.ToLower(r.Role) != roleFilter {
					continue
				}
				out = append(out, r)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].CalendarID != out[j].CalendarID {
					return out[i].CalendarID < out[j].CalendarID
				}
				return out[i].Role < out[j].Role
			})
			if out == nil {
				out = []gcalACLRule{}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&calendarsCSV, "calendars", "primary", "Comma-separated calendar IDs to audit")
	cmd.Flags().StringVar(&role, "role", "", "Filter to a single role (reader, writer, owner, freeBusyReader)")
	return cmd
}
