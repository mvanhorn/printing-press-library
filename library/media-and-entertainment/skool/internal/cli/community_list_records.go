// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.
//
// PATCH(amend-2026-08-12: hydrate posts and members) — `posts list` and
// `members list` expose the same community-page unwrapping that sync uses, so
// the spec's list endpoints have real commands behind them and the README's
// documented `posts list` recipe works.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newPostsListCmd(flags *rootFlags) *cobra.Command {
	return newCommunityRecordsListCmd(flags, "posts", "List posts in a community (newest first)")
}

func newMembersListCmd(flags *rootFlags) *cobra.Command {
	return newCommunityRecordsListCmd(flags, "members", "List members of a community")
}

func newCommunityRecordsListCmd(flags *rootFlags, resource, short string) *cobra.Command {
	var flagCommunity string
	var flagPages int
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       short,
		Example:     "  skool-pp-cli " + resource + " list --community <community-slug> --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required (or set template_vars.community in config)"))
			}

			path := "/_next/data/{buildId}/{community}.json"
			path = replacePathParam(path, "community", community)

			if flagPages < 1 {
				flagPages = 1
			}
			seen := map[string]struct{}{}
			collected := []json.RawMessage{}
			var lastKeys []string
			for page := 1; page <= flagPages; page++ {
				params := map[string]string{"g": community}
				if resource == "members" {
					params["t"] = "members"
				}
				if page > 1 {
					params["p"] = strconv.Itoa(page)
				}
				raw, err := c.Get(path, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				items, keys := extractSkoolPageRecords(raw, resource)
				lastKeys = keys
				added := 0
				for _, item := range items {
					id := recordIdentity(item)
					if id == "" {
						continue
					}
					if _, dup := seen[id]; dup {
						continue
					}
					seen[id] = struct{}{}
					collected = append(collected, item)
					added++
				}
				if added == 0 {
					break
				}
				if flagLimit > 0 && len(collected) >= flagLimit {
					break
				}
			}
			if len(collected) == 0 {
				return fmt.Errorf("no %s records found in the community page envelope (pageProps keys seen: %v)", resource, lastKeys)
			}
			if flagLimit > 0 && flagLimit < len(collected) {
				collected = collected[:flagLimit]
			}
			out, err := json.Marshal(collected)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), out, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}

	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().IntVar(&flagPages, "pages", 1, "Number of community pages to walk")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum records to return (0 = no limit)")
	return cmd
}
