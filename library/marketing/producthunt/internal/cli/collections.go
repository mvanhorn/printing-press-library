package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newCollectionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "Get and list Product Hunt curated collections",
	}
	cmd.AddCommand(newCollectionsGetCmd(flags))
	cmd.AddCommand(newCollectionsListCmd(flags))
	return cmd
}

func newCollectionsGetCmd(flags *rootFlags) *cobra.Command {
	var asID bool
	cmd := &cobra.Command{
		Use:   "get [id-or-slug]",
		Short: "Fetch a collection with its post list (curator user redacted by Product Hunt)",
		Long: strings.Trim(`
Returns one collection: name, description, tagline, followersCount, curator
(redacted by PH), and the inline post list.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli collections get tools-for-builders --json
  producthunt-pp-cli collections get 8890 --id --json
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
			vars := map[string]any{}
			if asID {
				vars["id"] = args[0]
			} else {
				vars["slug"] = args[0]
			}
			var resp phgql.CollectionResponse
			if _, err := c.Query(cmd.Context(), phgql.CollectionQuery, vars, &resp); err != nil {
				return err
			}
			if resp.Collection.ID == "" {
				return fmt.Errorf("collection not found: %s", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Collection, flags)
		},
	}
	cmd.Flags().BoolVar(&asID, "id", false, "Treat the positional argument as a numeric id")
	return cmd
}

func newCollectionsListCmd(flags *rootFlags) *cobra.Command {
	var (
		count    int
		featured bool
		featSet  bool
		userID   string
		postID   string
		order    string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Product Hunt collections (filterable by featured / user / post)",
		Long: strings.Trim(`
Returns the GraphQL collections connection. Curators (collection.user) come back
redacted; the collection content itself is intact.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli collections list --featured --count 10 --json
  producthunt-pp-cli collections list --post-id 1132754 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"first": count}
			if featSet {
				vars["featured"] = featured
			}
			if userID != "" {
				vars["userId"] = userID
			}
			if postID != "" {
				vars["postId"] = postID
			}
			if order != "" {
				vars["order"] = order
			}
			var resp phgql.CollectionsResponse
			if _, err := c.Query(cmd.Context(), phgql.CollectionsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Collections, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "Number of collections to return per page (max 20)")
	cmd.Flags().BoolVar(&featured, "featured", false, "Only featured collections")
	cmd.Flags().StringVar(&userID, "user-id", "", "Limit to collections curated by this user id")
	cmd.Flags().StringVar(&postID, "post-id", "", "Limit to collections containing this post id")
	cmd.Flags().StringVar(&order, "order", "FOLLOWERS_COUNT", "Order: FOLLOWERS_COUNT | NEWEST | FEATURED_AT")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		featSet = cmd.Flags().Changed("featured")
	}
	return cmd
}
