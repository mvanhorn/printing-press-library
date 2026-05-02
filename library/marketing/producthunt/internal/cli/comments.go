package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newCommentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Get individual Product Hunt comments by id",
	}
	cmd.AddCommand(newCommentsGetCmd(flags))
	return cmd
}

func newCommentsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch a Product Hunt comment by its numeric id; returns id, body (HTML), votesCount, createdAt; commenter user fields are redacted by PH (id 0, username/name `[REDACTED]`)",
		Long: strings.Trim(`
Returns one comment by its numeric Product Hunt id. The commenter user fields
are redacted by Product Hunt's policy (id "0", username/name "[REDACTED]");
the body, votes, and timestamp are intact.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli comments get 5332581 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			var resp phgql.CommentResponse
			if _, err := c.Query(cmd.Context(), phgql.CommentQuery, map[string]any{"id": args[0]}, &resp); err != nil {
				return err
			}
			if resp.Comment.ID == "" {
				return fmt.Errorf("comment not found: %s", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Comment, flags)
		},
	}
	return cmd
}
