// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: offline search over previously fetched tree listings.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source local

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/store"
)

type searchHit struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	SHA   string `json:"sha"`
}

type searchView struct {
	Query string      `json:"query"`
	Hits  []searchHit `json:"hits"`
	Count int         `json:"count"`
	Note  string      `json:"note,omitempty"`
}

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <pattern>",
		Short: "Search previously fetched repo listings from the local store — zero API calls",
		Long: strings.Trim(`
Search tree listings that 'plan' and 'fetch' persisted into the local store.
Matches are case-insensitive substring matches on the file path. Nothing
touches the network, so results reflect the last plan/fetch of each target.
`, "\n"),
		Example: strings.Trim(`
  # Where did I see that transformers book?
  github-contents-pp-cli search transformers --limit 20

  # All PDFs seen across fetched repos, as JSON for an agent
  github-contents-pp-cli search .pdf --json --select hits.path,hits.size
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A pattern with zero hits is a real, complete answer for a
			// local search command — exit 0 with hits=[] + note IS the
			// contract. Opt out of the dogfood matrix's non-zero-exit
			// error-path probe.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local store for fetched tree listings")
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search pattern is required, e.g. 'search transformers'"))
			}
			pattern := args[0]
			if limit <= 0 {
				limit = 20
			}

			// search is advertised as MCP read-only, so it must never create
			// or migrate local state: a missing store is a valid empty result,
			// and an existing store is opened read-only (no schema writes).
			dbPath := defaultDBPath("github-contents-pp-cli")
			if _, statErr := os.Stat(dbPath); errors.Is(statErr, fs.ErrNotExist) {
				view := searchView{Query: pattern, Hits: []searchHit{}, Count: 0,
					Note: "no local store yet; run 'plan' or 'fetch' on a target first to index its listing"}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store read-only: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "trees") {
				hintIfStale(cmd, db, "trees", flags.maxAge)
			}

			// Escape LIKE metacharacters so the documented contract — the
			// pattern is a literal substring — holds for inputs containing
			// %, _ or \ (e.g. "my_file" must not match "myXfile").
			likePattern := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(pattern)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data, '$.owner'), ''),
					COALESCE(json_extract(data, '$.repo'), ''),
					COALESCE(json_extract(data, '$.ref'), ''),
					COALESCE(json_extract(data, '$.path'), ''),
					COALESCE(json_extract(data, '$.size'), 0),
					COALESCE(json_extract(data, '$.sha'), '')
				FROM resources
				WHERE resource_type = 'trees'
				  AND LOWER(COALESCE(json_extract(data, '$.path'), '')) LIKE '%' || LOWER(?) || '%' ESCAPE '\'
				ORDER BY json_extract(data, '$.path')
				LIMIT ?`, likePattern, limit)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			defer rows.Close()

			hits := make([]searchHit, 0)
			for rows.Next() {
				var h searchHit
				var size sql.NullInt64
				if err := rows.Scan(&h.Owner, &h.Repo, &h.Ref, &h.Path, &size, &h.SHA); err != nil {
					return fmt.Errorf("reading search results: %w", err)
				}
				h.Size = size.Int64
				hits = append(hits, h)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading search results: %w", err)
			}

			view := searchView{Query: pattern, Hits: hits, Count: len(hits)}
			if len(hits) == 0 {
				view.Note = "no matches in the local store; run 'plan' or 'fetch' on a target first to index its listing"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%s/%s@%s\t%s\t%d\n",
					ghfetch.SanitizeTerminal(h.Owner), ghfetch.SanitizeTerminal(h.Repo),
					ghfetch.SanitizeTerminal(h.Ref), ghfetch.SanitizeTerminal(h.Path), h.Size)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matches to return")
	return cmd
}
