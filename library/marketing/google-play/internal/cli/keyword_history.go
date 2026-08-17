// Hand-authored transcendence command: keyword-rank history.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

type keywordHistoryView struct {
	Term    string                   `json:"term"`
	Country string                   `json:"country"`
	AppID   string                   `json:"appId"`
	Points  []store.KeywordRankPoint `json:"points"`
	Best    int                      `json:"bestRank,omitempty"`
	Note    string                   `json:"note,omitempty"`
}

// pp:data-source local
func newNovelKeywordHistoryCmd(flags *rootFlags) *cobra.Command {
	var app string
	cmd := &cobra.Command{
		Use:   "keyword-history <term>",
		Short: "Show an app's rank-over-time for a search term from captured keyword snapshots.",
		Long: "Read local keyword-rank snapshots and emit the rank-over-time series for a term, app, and country.\n\n" +
			"Use this command for the rank trend of one term/app/country over time. To capture a fresh data point first, run 'keyword-rank'.",
		Example:     "  google-play-pp-cli keyword-history \"merge puzzle\" --country us --app com.yalla.yallagames --agent",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read keyword-rank history")
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
			term := strings.Join(args, " ")
			country, _ := localeOf(cmd)
			db := resolveDBFlag(cmd)
			hint := fmt.Sprintf("run: google-play-pp-cli keyword-rank %q --country %s --app %s   (repeatedly over time)", term, country, app)
			if !dbFileExists(db) {
				hintStderr(cmd, db, hint)
				return emit(cmd, flags, keywordHistoryView{
					Term: term, Country: country, AppID: app,
					Points: []store.KeywordRankPoint{}, Note: "no keyword-rank observations recorded yet; " + hint,
				})
			}
			s, err := openStoreFor(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			points, err := s.KeywordRankSeries(cmd.Context(), term, country, app)
			if err != nil {
				return apiErr(err)
			}
			view := keywordHistoryView{Term: term, Country: country, AppID: app, Points: points}
			if len(points) == 0 {
				view.Points = []store.KeywordRankPoint{}
				view.Note = "no keyword-rank observations recorded yet; " + hint
				return emit(cmd, flags, view)
			}
			best := 0
			for _, p := range points {
				if p.Rank > 0 && (best == 0 || p.Rank < best) {
					best = p.Rank
				}
			}
			view.Best = best
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&app, "app", "", "Target appId (required)")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}
