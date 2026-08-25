// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/store"
	"github.com/spf13/cobra"
)

// corpusHit is one matched archived article in the corpus search output.
type corpusHit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
}

// pp:data-source local
// corpus reads exclusively from the local SQLite store (full-text/regex
// search over everything previously archived); it makes no live API calls.
func newNovelCorpusCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var useRegex bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "corpus <query>",
		Short: "Full-text and regex search across everything you have synced locally (authors, publications, tags).",
		Example: strings.Trim(`
  medium-reader-pp-cli corpus design --limit 20 --agent
  medium-reader-pp-cli corpus "claude\s+code" --regex
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				query := "<query>"
				if len(args) > 0 {
					query = args[0]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would search local corpus for %q\n", query)
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<query> is required"))
			}
			query := args[0]
			if limit <= 0 {
				limit = 20
			}

			if dbPath == "" {
				dbPath = defaultDBPath("medium-reader-pp-cli")
			}

			// Missing-mirror guard: no DB means nothing has been archived yet.
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local mirror found; run 'medium-reader-pp-cli author-archive <username>' first")
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), make([]corpusHit, 0), flags)
				}
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.List("articles", 0)
			if err != nil {
				return fmt.Errorf("reading local store: %w", err)
			}

			var re *regexp.Regexp
			if useRegex {
				re, err = regexp.Compile(query)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid regex %q: %w", query, err))
				}
			}
			needle := strings.ToLower(query)

			hits := make([]corpusHit, 0)
			for _, raw := range rows {
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				title := asString(obj["title"])
				subtitle := asString(obj["subtitle"])
				markdown := asString(obj["markdown"])

				matched := false
				if useRegex {
					matched = re.MatchString(title) || re.MatchString(subtitle) || re.MatchString(markdown)
				} else {
					matched = strings.Contains(strings.ToLower(title), needle) ||
						strings.Contains(strings.ToLower(subtitle), needle) ||
						strings.Contains(strings.ToLower(markdown), needle)
				}
				if !matched {
					continue
				}
				hits = append(hits, corpusHit{
					ID:          asString(obj["id"]),
					Title:       title,
					Author:      asString(obj["author"]),
					URL:         asString(obj["url"]),
					PublishedAt: asString(obj["published_at"]),
				})
				if len(hits) >= limit {
					break
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(hits) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no matches for %q\n", query)
					return nil
				}
				headers := []string{"ID", "TITLE", "AUTHOR", "PUBLISHED"}
				tableRows := make([][]string, 0, len(hits))
				for _, h := range hits {
					tableRows = append(tableRows, []string{h.ID, h.Title, h.Author, h.PublishedAt})
				}
				return flags.printTable(cmd, headers, tableRows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of matches to return")
	cmd.Flags().BoolVar(&useRegex, "regex", false, "Treat the query as a Go regular expression")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}

// asString coerces an arbitrary decoded JSON value to a string. Numbers are
// rendered without trailing zeros so an id stored as a JSON number still
// prints cleanly. Non-scalar values yield "".
func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case bool:
		if t {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return ""
	}
}
