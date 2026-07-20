// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelIssuesContextCmd(flags *rootFlags) *cobra.Command {
	var flagRepo string
	var flagRefresh bool
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:   "context <number>",
		Short: "Assemble one JSON envelope for an issue: the issue, its comments, and the recent commits that mention it.",
		Long: `Assemble an agent-ready working set for one issue from the local mirror in a
single offline read: the issue, its comments, and synced commits whose message
mentions the issue number. Replaces several rate-limited online calls with one.

Pass --repo owner/repo the first time to populate; later runs reuse it offline.
For a raw single issue use 'github-pp-cli issues get'; for a string sweep use
'github-pp-cli mentions'.`,
		Example:     "  github-pp-cli issues context 456 --repo cli/cli --agent --select issue.title,comments.body,commits.sha",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an issue number is required"))
			}
			number := args[0]
			num, perr := strconv.ParseInt(number, 10, 64)
			if perr != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("issue number must be numeric, got %q", number))
			}

			st, err := openNovelStoreRW(cmd)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if err := nvEnsurePopulated(ctx, cmd, flags, st, flagRepo, flagRefresh, flagMaxPages, "issues", "issues_comments", "commits"); err != nil {
				return err
			}
			db := st.DB()

			var issueData string
			row := db.QueryRow(
				`SELECT data FROM resources WHERE resource_type = 'issues' AND json_extract(data, '$.number') = ? LIMIT 1`,
				num,
			)
			if err := row.Scan(&issueData); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					hint := "try --repo owner/repo to populate"
					if flagRepo != "" {
						hint = "the issue may be outside the fetched window; raise --max-pages or pass --refresh"
					}
					return notFoundErr(fmt.Errorf("issue #%s not found in the local mirror (%s)", number, hint))
				}
				return fmt.Errorf("loading issue #%s: %w", number, err)
			}

			comments, err := nvScanRaw(db,
				`SELECT data FROM resources WHERE resource_type = 'issues_comments' AND json_extract(data, '$.issue_url') LIKE ? ORDER BY json_extract(data, '$.created_at')`,
				"%/issues/"+number)
			if err != nil {
				return fmt.Errorf("loading comments for issue #%s: %w", number, err)
			}
			commitCandidates, err := nvScanRaw(db,
				`SELECT data FROM resources WHERE resource_type = 'commits' AND data LIKE ? ORDER BY id`,
				"%#"+number+"%")
			if err != nil {
				return fmt.Errorf("loading commits for issue #%s: %w", number, err)
			}
			commits := make([]json.RawMessage, 0, len(commitCandidates))
			for _, raw := range commitCandidates {
				if nvCommitReferencesIssue(raw, number) {
					commits = append(commits, raw)
				}
			}

			env := struct {
				Issue    json.RawMessage   `json:"issue"`
				Comments []json.RawMessage `json:"comments"`
				Commits  []json.RawMessage `json:"commits"`
			}{
				Issue:    json.RawMessage(issueData),
				Comments: comments,
				Commits:  commits,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := nvDecode([]byte(issueData))
				fmt.Fprintf(cmd.OutOrStdout(), "#%s  [%s]  %s\n", number, nvStr(m, "state"), nvStr(m, "title"))
				fmt.Fprintf(cmd.OutOrStdout(), "%d comment(s), %d mentioning commit(s)\n", len(comments), len(commits))
				return nil
			}
			return flags.printJSON(cmd, env)
		},
	}
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch even if the local mirror already has the data")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch per resource when populating")
	return cmd
}

// nvScanRaw runs a single-column data query and returns the rows as raw JSON.
func nvScanRaw(db *sql.DB, query string, args ...any) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nvCommitReferencesIssue(raw json.RawMessage, number string) bool {
	message := nvNestedStr(nvDecode(raw), "commit", "message")
	if message == "" {
		return false
	}
	pattern := `(^|[^[:alnum:]_])#` + regexp.QuoteMeta(number) + `($|[^[:alnum:]_])`
	return regexp.MustCompile(pattern).MatchString(message)
}
