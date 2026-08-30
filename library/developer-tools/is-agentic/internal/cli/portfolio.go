// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source live
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/agentic"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/cliutil"
	"github.com/spf13/cobra"
)

type portfolioRow struct {
	Target     string   `json:"target"`
	Score      *float64 `json:"score,omitempty"`
	ScoreLabel string   `json:"score_label,omitempty"`
	IssueCount int      `json:"issue_count,omitempty"`
	ScannedAt  string   `json:"scanned_at,omitempty"`
	Error      string   `json:"error,omitempty"`
	raw        []byte
	fetchedAt  time.Time
}
type portfolioView struct {
	Items     []portfolioRow `json:"items"`
	Failed    int            `json:"failed"`
	Requested int            `json:"requested"`
	Note      string         `json:"note,omitempty"`
}

func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	var targets, file string
	var maxRequests, concurrency int
	cmd := &cobra.Command{Use: "portfolio", Short: "Compare a fleet of public sites in one sortable score and issue matrix.", Example: "  is-agentic-pp-cli portfolio --targets https://is-agentic.com,https://example.com --json --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "refresh a bounded target portfolio")
		}
		if targets == "" && file == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("one of --targets or --file is required"))
		}
		if targets != "" && file != "" {
			return usageErr(fmt.Errorf("use only one of --targets or --file"))
		}
		var raw []byte
		var err error
		if file != "" {
			raw, err = os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("reading --file: %w", err)
			}
		} else {
			raw = []byte(strings.ReplaceAll(targets, ",", "\n"))
		}
		list := make([]string, 0)
		seen := map[string]bool{}
		for _, line := range strings.Split(string(raw), "\n") {
			t := strings.TrimSpace(line)
			if t != "" && !strings.HasPrefix(t, "#") && !seen[t] {
				seen[t] = true
				list = append(list, t)
			}
		}
		if len(list) == 0 {
			return usageErr(fmt.Errorf("no targets found"))
		}
		if maxRequests <= 0 || maxRequests > len(list) {
			maxRequests = len(list)
		}
		if cliutil.IsDogfoodEnv() && maxRequests > 1 {
			maxRequests = 1
		}
		if concurrency < 1 {
			concurrency = 1
		}
		if concurrency > maxRequests {
			concurrency = maxRequests
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, path, err := openAgenticStore(ctx, flags)
		if err != nil {
			return err
		}
		if s == nil {
			return missingStore(cmd, flags, path)
		}
		defer s.Close()
		client := agentic.New()
		jobs := make(chan string)
		results := make(chan portfolioRow, maxRequests)
		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for target := range jobs {
					report, err := client.Fetch(ctx, target)
					if err != nil {
						results <- portfolioRow{Target: target, Error: err.Error()}
						continue
					}
					results <- portfolioRow{Target: report.Parsed.Target, Score: report.Parsed.Score, ScoreLabel: report.Parsed.ScoreLabel, IssueCount: len(report.Issues), ScannedAt: report.Parsed.ScannedAt, raw: report.Raw, fetchedAt: report.FetchedAt}
				}
			}()
		}
		go func() {
			for i := 0; i < maxRequests; i++ {
				jobs <- list[i]
			}
			close(jobs)
			wg.Wait()
			close(results)
		}()
		view := portfolioView{Items: make([]portfolioRow, 0, maxRequests), Requested: maxRequests}
		for row := range results {
			view.Items = append(view.Items, row)
		}
		// SQLite permits one writer at a time. Fetches may run concurrently,
		// but snapshot persistence is deliberately drained and serialized here.
		for i := range view.Items {
			if view.Items[i].Error != "" || len(view.Items[i].raw) == 0 {
				continue
			}
			if _, err := s.SaveAgenticSnapshot(ctx, view.Items[i].raw, view.Items[i].fetchedAt); err != nil {
				view.Items[i].Error = err.Error()
				view.Items[i].Score = nil
			}
		}
		sort.Slice(view.Items, func(i, j int) bool {
			if view.Items[i].Score == nil {
				return false
			}
			if view.Items[j].Score == nil {
				return true
			}
			return *view.Items[i].Score > *view.Items[j].Score
		})
		for _, row := range view.Items {
			if row.Error != "" {
				view.Failed++
			}
		}
		if view.Failed > 0 {
			view.Note = fmt.Sprintf("%d of %d target fetches failed; successful rows are retained", view.Failed, view.Requested)
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+view.Note)
		}
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}}
	cmd.Flags().StringVar(&targets, "targets", "", "comma-separated public URLs")
	cmd.Flags().StringVar(&file, "file", "", "newline-delimited target file")
	cmd.Flags().IntVar(&maxRequests, "max-requests", 100, "maximum targets to refresh in this invocation")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "maximum concurrent report requests")
	return cmd
}
