// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelMentionsCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagRepo string
	var flagLimit, flagMaxPages int
	var flagRefresh bool

	cmd := &cobra.Command{
		Use:   "mentions <term>",
		Short: "One query across issue bodies, PR comments, and commit messages returning every place a symbol or error string appears",
		Long: `Search every synced issue body, pull-request body, issue/PR comment, and commit
message for a term in a single offline query, tagging each hit by entity type —
a cross-entity view no single GitHub API call returns.

Pass --repo owner/repo the first time to populate; later runs reuse it offline.
To match one entity type only, use 'github-pp-cli search --type <resource>'.
To find open issues that duplicate each other, use 'github-pp-cli issues dupes'.`,
		Example:     "  github-pp-cli mentions \"ParseConfig\" --repo cli/cli --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			term := args[0]

			var cutoff time.Time
			if flagSince != "" {
				d, derr := cliutil.ParseDurationLoose(flagSince)
				if derr != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, derr))
				}
				cutoff = time.Now().Add(-d)
			}

			st, err := openNovelStoreRW(cmd)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if err := nvEnsurePopulated(ctx, cmd, flags, st, flagRepo, flagRefresh, flagMaxPages, "issues", "pulls", "commits", "issues_comments"); err != nil {
				return err
			}

			hits := make([]mentionRow, 0, flagLimit*3)
			for _, rt := range []string{"issues", "pulls", "commits"} {
				matches, serr := st.Search(term, flagLimit, rt)
				if serr != nil {
					return fmt.Errorf("searching %s: %w", rt, serr)
				}
				for _, raw := range matches {
					if !cutoff.IsZero() && !nvWithinSince(raw, rt, cutoff) {
						continue
					}
					hits = append(hits, nvMentionRow(rt, raw))
				}
			}
			commentHits, err := nvCommentMentions(st, term, flagLimit, cutoff)
			if err != nil {
				return fmt.Errorf("searching comments: %w", err)
			}
			hits = append(hits, commentHits...)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(hits) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no synced entities mention %q (try --repo owner/repo to populate)\n", term)
				}
				tr := make([][]string, 0, len(hits))
				for _, h := range hits {
					ref := h.Ref
					if ref == "" && h.Number > 0 {
						ref = "#" + strconv.FormatInt(h.Number, 10)
					}
					tr = append(tr, []string{h.Type, ref, h.Title})
				}
				return flags.printTable(cmd, []string{"TYPE", "REF", "TITLE"}, tr)
			}
			return flags.printJSON(cmd, hits)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only include entities updated within this window (e.g. 30d, 24h, 1w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum hits per entity type")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch entities even if the local mirror already has them")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch per resource when populating")
	return cmd
}

// nvWithinSince reports whether a synced resource's activity timestamp is at or
// after cutoff. Issues/pulls use updated_at; commits use commit.author.date.
func nvWithinSince(raw []byte, resourceType string, cutoff time.Time) bool {
	m := nvDecode(raw)
	var ts string
	if resourceType == "commits" {
		ts = nvNestedStr(m, "commit", "author", "date")
	} else {
		ts = nvStr(m, "updated_at")
	}
	t, ok := nvParseTime(ts)
	if !ok {
		return true // undated rows are not excluded by --since
	}
	return !t.Before(cutoff)
}

// nvCommentMentions LIKE-scans synced issue/PR comments (stored under
// resource_type 'issues_comments') for the term.
func nvCommentMentions(st *store.Store, term string, limit int, cutoff time.Time) ([]mentionRow, error) {
	if limit <= 0 {
		limit = 25
	}
	query := `SELECT data FROM resources WHERE resource_type = 'issues_comments' AND data LIKE ? ESCAPE '\'`
	args := []any{"%" + nvEscapeLike(term) + "%"}
	if !cutoff.IsZero() {
		query += ` AND (json_extract(data, '$.updated_at') IS NULL OR json_extract(data, '$.updated_at') >= ?)`
		args = append(args, cutoff.UTC().Format(time.RFC3339))
	}
	query += ` ORDER BY json_extract(data, '$.updated_at') DESC LIMIT ?`
	args = append(args, limit)
	rows, err := st.DB().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mentionRow
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if !cutoff.IsZero() && !nvWithinSince([]byte(data), "issues_comments", cutoff) {
			continue
		}
		m := nvDecode([]byte(data))
		ref := ""
		if iu := nvStr(m, "issue_url"); iu != "" {
			if i := strings.LastIndex(iu, "/"); i >= 0 {
				ref = "#" + iu[i+1:]
			}
		}
		out = append(out, mentionRow{
			Type:  "comment",
			Ref:   ref,
			Title: nvFirstLine(nvStr(m, "body")),
			URL:   nvStr(m, "html_url"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nvEscapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}
