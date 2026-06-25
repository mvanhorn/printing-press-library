// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

type urlMentionsResult struct {
	Input            string                   `json:"input"`
	NormalizedTarget string                   `json:"normalized_target"`
	Query            string                   `json:"query"`
	Since            string                   `json:"since,omitempty"`
	Results          []collectionItemSnapshot `json:"results"`
	Collection       string                   `json:"collection,omitempty"`
	CollectionAdded  int                      `json:"collection_added,omitempty"`
	Monitor          string                   `json:"monitor,omitempty"`
	MonitorCreated   bool                     `json:"monitor_created,omitempty"`
}

func newNovelURLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "url",
		Short: "URL and domain mention workflows",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelURLMentionsCmd(flags))
	return cmd
}

func newNovelURLMentionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, collection, monitor string
	var limit int
	cmd := &cobra.Command{
		Use:   "mentions <url-or-domain>",
		Short: "Find recent X posts mentioning a URL or domain",
		Example: `  x-twitter-pp-cli url mentions https://example.com --since 7d --agent
  x-twitter-pp-cli url mentions github.com/org/repo --collection launch-feedback --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query, target, err := normalizeURLMentionQuery(args[0])
			if err != nil {
				return err
			}
			records, err := recentSearchRecords(cmd, flags, query, limit, since, "", parseIncludeSet("author,media,links,refs,metrics"))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := urlMentionsResult{Input: args[0], NormalizedTarget: target, Query: query, Since: since, Collection: collection, Monitor: monitor}
			if collection != "" || monitor != "" {
				db, err := openWorkflowDB(cmd, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				if monitor != "" {
					def := monitorDefinition{Name: monitor, Kind: "url", Query: query, SourceURL: target}
					if err := upsertMonitorDefinition(cmd, db, def); err != nil {
						return err
					}
					result.MonitorCreated = true
				}
				if collection != "" && len(records) > 0 {
					added, _, _, err := saveCollectionItems(cmd, db, collection, records, "saved from url mentions "+target, nil)
					if err != nil {
						return err
					}
					result.CollectionAdded = added
				}
			}
			for _, rec := range records {
				result.Results = append(result.Results, collectionItemFromPost(rec, ""))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([][]string, 0, len(result.Results))
				for _, item := range result.Results {
					rows = append(rows, []string{item.TweetID, item.URL, authorDisplay(item.Author), truncatePlain(item.Text, 88)})
				}
				return flags.printTable(cmd, []string{"ID", "URL", "AUTHOR", "TEXT"}, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("url mentions", "live"),
				Results: result,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&since, "since", "7d", "Search window such as 24h, 7d, RFC3339, or YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	cmd.Flags().StringVar(&collection, "collection", "", "Save results into a local collection")
	cmd.Flags().StringVar(&monitor, "monitor", "", "Create or update a monitor for this URL/domain")
	return cmd
}
