// Hand-authored transcendence command: live keyword-rank capture + persist.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/cliutil"
)

type keywordRankView struct {
	Term     string `json:"term"`
	Country  string `json:"country"`
	AppID    string `json:"appId"`
	Rank     int    `json:"rank"`    // 1-based; 0 = not found within scan
	Scanned  int    `json:"scanned"` // results examined
	Found    bool   `json:"found"`
	Note     string `json:"note,omitempty"`
	Captured int64  `json:"capturedAt"`
}

// pp:data-source live
func newNovelKeywordRankCmd(flags *rootFlags) *cobra.Command {
	var app string
	var scan int
	cmd := &cobra.Command{
		Use:   "keyword-rank <term>",
		Short: "Find where an app ranks in store search for a term, and record the data point.",
		Long: "Run a live store search for a term and record where the target app ranks among the results, persisting a " +
			"keyword-rank snapshot for trend analysis.\n\n" +
			"Use this command to capture today's rank for a term (and persist it). For raw search results use 'search-store'; " +
			"for the trend over time use 'keyword-history'.",
		Example:     "  google-play-pp-cli keyword-rank \"merge puzzle\" --country us --app com.yalla.yallagames --agent",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search and record keyword rank")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			if app == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--app <appId> is required"))
			}
			if cliutil.IsDogfoodEnv() && scan > 50 {
				scan = 50
			}
			term := strings.Join(args, " ")
			country, _ := localeOf(cmd)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			results, err := c.Search(ctx, term, "all", scan)
			if err != nil {
				return classifyGplayErr(err)
			}
			view := keywordRankView{Term: term, Country: country, AppID: app, Scanned: len(results), Captured: nowUnix()}
			for i, r := range results {
				if r.AppID == app {
					view.Rank = i + 1
					view.Found = true
					break
				}
			}
			if !view.Found {
				view.Note = fmt.Sprintf("app not found within the top %d results; raise --scan to widen", len(results))
			}
			// Persist the observation (rank 0 means "scanned, not found").
			// Persistence is the whole point of keyword-rank, so a write failure
			// is surfaced on stderr rather than silently dropped — keyword-history
			// would otherwise be missing this point with no indication why.
			if s, serr := openStoreFor(cmd.Context(), resolveDBFlag(cmd)); serr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: keyword rank fetched but could not open the local store to record it (keyword-history will miss this point): %v\n", serr)
			} else {
				if ierr := s.InsertKeywordRank(cmd.Context(), term, country, app, view.Captured, view.Rank, view.Scanned); ierr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: keyword rank fetched but not persisted (keyword-history will miss this point): %v\n", ierr)
				}
				_ = s.Close()
			}
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Target appId to locate in the results (required)")
	cmd.Flags().IntVar(&scan, "scan", 100, "Maximum search results to scan for the target app")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}
