// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTopicsGetCmd(flags *rootFlags) *cobra.Command {
	var flagTopic string

	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Fetch topic overview with relevant BetrVG paragraphs and legal sources",
		Example:     "  betriebsrat topics get --topic kuendigung",
		Annotations: map[string]string{"pp:endpoint": "topics.get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("topic") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "topic")
			}
			if dryRunOK(flags) {
				return nil
			}

			slug := strings.ToLower(strings.TrimSpace(flagTopic))

			// Find matching topic from embedded list
			var found *topicEntry
			for i := range embeddedTopics {
				if embeddedTopics[i].Slug == slug {
					found = &embeddedTopics[i]
					break
				}
			}

			// Find matching articles
			articles, hasArticles := topicArticles[slug]

			type topicOverview struct {
				Topic      string         `json:"topic"`
				Paragraphs string         `json:"paragraphs,omitempty"`
				SourceURL  string         `json:"source_url"`
				Articles   []legalArticle `json:"articles,omitempty"`
			}

			overview := topicOverview{
				Topic:     flagTopic,
				SourceURL: "https://www.gesetze-im-internet.de/betrvg/",
			}
			if found != nil {
				overview.Paragraphs = found.Paragraphs
				overview.SourceURL = found.SourceURL
			}
			if hasArticles {
				overview.Articles = articles
			} else {
				overview.Articles = defaultArticles
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(overview)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Thema: %s\n", flagTopic)
			if overview.Paragraphs != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Paragrafen: %s\n", overview.Paragraphs)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Quelle: %s\n\n", overview.SourceURL)
			for _, a := range overview.Articles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s – %s\n  %s\n\n", a.Paragraph, a.Law, a.Title, a.SourceURL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagTopic, "topic", "", "Topic slug (e.g. kuendigung, betriebsvereinbarungen, homeoffice, ki)")

	return cmd
}
