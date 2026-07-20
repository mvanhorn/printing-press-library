// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelReposChangelogCmd(flags *rootFlags) *cobra.Command {
	var flagBase, flagHead, flagRepo string

	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "List every commit between two refs, grouped by author, for release-note prep.",
		Long: `Compare two refs (tags, branches, or SHAs) live via the GitHub compare API and
group the commits between them by author — what merged since the last tag,
organized by contributor, in one command.

owner/repo are inferred from your synced data when --repo is omitted; pass
--repo owner/repo to target another repository.`,
		Example:     "  github-pp-cli repos changelog --repo cli/cli --base v2.93.0 --head v2.94.0",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagBase == "" || flagHead == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--base and --head are required"))
			}
			owner, repo, err := nvResolveOwnerRepo(flagRepo)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, flagBase, flagHead)
			var commits []any
			totalCommits := 0
			for page := 1; ; page++ {
				data, err := c.Get(ctx, path, map[string]string{
					"page":     strconv.Itoa(page),
					"per_page": "100",
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var cmp struct {
					Commits      []any `json:"commits"`
					TotalCommits int   `json:"total_commits"`
				}
				if err := json.Unmarshal(data, &cmp); err != nil {
					return fmt.Errorf("parsing compare response page %d: %w", page, err)
				}
				if page == 1 {
					totalCommits = cmp.TotalCommits
				}
				commits = append(commits, cmp.Commits...)
				if len(cmp.Commits) < 100 || (totalCommits > 0 && len(commits) >= totalCommits) {
					break
				}
			}
			if totalCommits == 0 {
				totalCommits = len(commits)
			}
			byAuthor := nvGroupCommitsByAuthor(commits)
			view := struct {
				Base            string         `json:"base"`
				Head            string         `json:"head"`
				Repo            string         `json:"repo"`
				TotalCommits    int            `json:"total_commits"`
				ReturnedCommits int            `json:"returned_commits"`
				ByAuthor        []authorChange `json:"by_author"`
			}{
				Base:            flagBase,
				Head:            flagHead,
				Repo:            owner + "/" + repo,
				TotalCommits:    totalCommits,
				ReturnedCommits: len(commits),
				ByAuthor:        byAuthor,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s/%s  %s...%s  (%d commits)\n", owner, repo, flagBase, flagHead, totalCommits)
				if len(commits) < totalCommits {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: GitHub returned %d of %d commits; author totals are incomplete\n", len(commits), totalCommits)
				}
				tr := make([][]string, 0, len(byAuthor))
				for _, a := range byAuthor {
					tr = append(tr, []string{a.Author, strconv.Itoa(a.Commits)})
				}
				return flags.printTable(cmd, []string{"AUTHOR", "COMMITS"}, tr)
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagBase, "base", "", "Base ref (tag, branch, or SHA)")
	cmd.Flags().StringVar(&flagHead, "head", "", "Head ref (tag, branch, or SHA)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo (inferred from synced data when omitted)")
	return cmd
}

// nvResolveOwnerRepo returns the target owner/repo from an explicit
// --repo owner/repo flag, else infers it from the local mirror's synced data.
func nvResolveOwnerRepo(flagRepo string) (string, string, error) {
	if o, r, ok := nvRepoFromFlag(flagRepo); ok {
		return o, r, nil
	}
	dbPath := novelDBPath()
	if _, err := os.Stat(dbPath); err == nil {
		if st, oerr := store.OpenReadOnly(dbPath); oerr == nil {
			defer st.Close()
			if o, r, ok := nvDeriveRepo(st); ok {
				return o, r, nil
			}
		}
	}
	return "", "", usageErr(fmt.Errorf("could not determine repository: pass --repo owner/repo, or populate a repo first (e.g. github-pp-cli issues dupes <term> --repo owner/repo)"))
}
