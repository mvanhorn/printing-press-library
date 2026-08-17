// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: current-issue reading digest grouped by specialty or type.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var limit int

	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Triage the current issue grouped by specialty or type with abstracts and free/paywalled flags.",
		Long:        "Groups current-issue articles by specialty (default) or article type, showing title, authors, abstract snippet, and free/paywalled flag. Requires synced data; enriched articles show richer grouping.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli digest\n  nejm-pp-cli digest --group type --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			items, err := nejmQueryArticles(db, nejmCurrentIssueWhere, nil, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no current-issue articles found. Run 'nejm-pp-cli sync' to fetch the current issue.")
				if flags.asJSON {
					// digest's populated output is a groups object, so the
					// empty shape is {} rather than the list commands' [].
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("{}"), flags)
				}
				return nil
			}

			// Group by specialty or article_type
			groupField := "specialties"
			if strings.ToLower(groupBy) == "type" {
				groupField = "article_type"
			}

			groups := make(map[string][]map[string]any)
			var groupOrder []string
			seen := make(map[string]bool)

			for _, item := range items {
				key, _ := item[groupField].(string)
				if key == "" {
					key = "(ungrouped)"
				}
				if !seen[key] {
					groupOrder = append(groupOrder, key)
					seen[key] = true
				}
				
				// 🔧 JAVÍTÁS: UTF-8 biztos csonkítás 200 karakterre
				if abs, ok := item["abstract"].(string); ok {
					// UTF-8 karakterek számlálása, nem bájtoké
					runeCount := utf8.RuneCountInString(abs)
					if runeCount > 200 {
						// 197 karakter + "..." (összesen 200)
						runes := []rune(abs)
						item["abstract"] = string(runes[:197]) + "..."
					}
				}
				
				groups[key] = append(groups[key], item)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(groups)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current Issue Digest (%d articles)\n\n", len(items))
			for _, key := range groupOrder {
				groupItems := groups[key]
				fmt.Fprintf(cmd.OutOrStdout(), "📂 %s (%d)\n", key, len(groupItems))
				fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 40))
				
				for _, item := range groupItems {
					title, _ := item["title"].(string)
					authors, _ := item["authors"].(string)
					abstract, _ := item["abstract"].(string)
					free, _ := item["is_free"].(bool)
					doi, _ := item["doi"].(string)
					
					freeFlag := "🔒"
					if free {
						freeFlag = "🔓"
					}
					
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", freeFlag, title)
					if authors != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "    by %s\n", authors)
					}
					if abstract != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", abstract)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    DOI: %s\n", doi)
					fmt.Fprintln(cmd.OutOrStdout())
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&groupBy, "group", "specialty", "Group by 'specialty' or 'type'")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit number of articles")
	
	return cmd
}
