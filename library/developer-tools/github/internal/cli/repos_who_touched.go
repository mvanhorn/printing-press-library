// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelReposWhoTouchedCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagRepo string
	var flagLimit, flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "who-touched <path>",
		Short: "Rank committers by number of commits touching a file or directory, with first/last touch dates.",
		Long: `Rank the committers who have touched a path (file or directory prefix) by commit
count, with first and last touch dates — file-level ownership for onboarding,
from the live GitHub commits API filtered by path.

owner/repo are inferred from your synced data when --repo is omitted. Scan effort
is bounded by --max-scan-pages (separate from --limit, which caps authors).`,
		Example:     "  github-pp-cli repos who-touched internal/parser --repo cli/cli --since 90d",
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
				return usageErr(fmt.Errorf("a file or directory path is required"))
			}
			path := args[0]
			owner, repo, err := nvResolveOwnerRepo(flagRepo)
			if err != nil {
				return err
			}

			params := map[string]string{"path": path, "per_page": "100"}
			if flagSince != "" {
				d, derr := cliutil.ParseDurationLoose(flagSince)
				if derr != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, derr))
				}
				params["since"] = time.Now().Add(-d).UTC().Format(time.RFC3339)
			}

			maxScan := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxScan > 1 {
				maxScan = 1
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var all []any
			scanCapHit := true
			apiPath := fmt.Sprintf("/repos/%s/%s/commits", owner, repo)
			for page := 1; page <= maxScan; page++ {
				params["page"] = strconv.Itoa(page)
				data, gerr := c.Get(ctx, apiPath, params)
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				var batch []any
				if uerr := json.Unmarshal(data, &batch); uerr != nil {
					return fmt.Errorf("parsing commits page %d: %w", page, uerr)
				}
				all = append(all, batch...)
				if len(batch) < 100 {
					scanCapHit = false
					break
				}
			}

			authors := nvWhoTouched(all)
			if flagLimit > 0 && len(authors) > flagLimit {
				authors = authors[:flagLimit]
			}
			view := struct {
				Path           string      `json:"path"`
				Repo           string      `json:"repo"`
				ScannedCommits int         `json:"scanned_commits"`
				MaxScanPages   int         `json:"max_scan_pages"`
				Authors        []touchStat `json:"authors"`
				Note           string      `json:"note,omitempty"`
			}{
				Path:           path,
				Repo:           owner + "/" + repo,
				ScannedCommits: len(all),
				MaxScanPages:   maxScan,
				Authors:        authors,
			}
			if len(authors) == 0 {
				view.Note = fmt.Sprintf("no commits found touching %q within the scan window; check the path exists and widen --since or raise --max-scan-pages", path)
			} else if scanCapHit {
				view.Note = fmt.Sprintf("scan cap of %d page(s) reached; raise --max-scan-pages to scan deeper history", maxScan)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s/%s  path=%s  (scanned %d commits)\n", owner, repo, path, len(all))
				tr := make([][]string, 0, len(authors))
				for _, a := range authors {
					tr = append(tr, []string{a.Author, strconv.Itoa(a.Commits), a.FirstSeen, a.LastSeen})
				}
				if err := flags.printTable(cmd, []string{"AUTHOR", "COMMITS", "FIRST", "LAST"}, tr); err != nil {
					return err
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", view.Note)
				}
				return nil
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Only count commits within this window (e.g. 90d, 6w)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo (inferred from synced data when omitted)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum authors to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Maximum commit pages (100/page) to scan before returning")
	return cmd
}
