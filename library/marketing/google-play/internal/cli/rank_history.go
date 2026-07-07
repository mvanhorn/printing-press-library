// Hand-authored transcendence command: one app's chart-rank history.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

type rankHistoryView struct {
	AppID      string            `json:"appId"`
	Collection string            `json:"collection"`
	Category   string            `json:"category"`
	Country    string            `json:"country"`
	Points     []store.RankPoint `json:"points"`
	FirstSeen  int64             `json:"firstSeen,omitempty"`
	LastSeen   int64             `json:"lastSeen,omitempty"`
	PeakRank   int               `json:"peakRank,omitempty"`
	Note       string            `json:"note,omitempty"`
}

// pp:data-source local
func newNovelRankHistoryCmd(flags *rootFlags) *cobra.Command {
	var collection, category string
	cmd := &cobra.Command{
		Use:   "rank-history <appId>",
		Short: "Show one app's rank trajectory over time within a chart (first-seen, peak, last-seen).",
		Long: "Read local chart snapshots for one app and emit its rank time-series within a collection/category/country, " +
			"plus first-seen, peak, and last-seen rank. Reads snapshots written by 'top'.\n\n" +
			"Use this command for ONE app's rank trajectory over time. For ranking changes across the whole chart between two snapshots, use 'movers' instead.",
		Example: "  google-play-pp-cli rank-history com.dreamgames.royalkingdom --collection TOP_GROSSING --category GAME --country us --agent",
		Args:    cobra.ArbitraryArgs,
		// Reads local snapshots; any appId with no recorded history returns a
		// valid empty-state view, which is not a bad-input error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read chart-rank history")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			wire, ok := gplay.NormalizeCollection(collection)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--collection must be one of %s", strings.Join(gplay.CollectionNames(), ", ")))
			}
			category = strings.ToUpper(category)
			country, _ := localeOf(cmd)
			db := resolveDBFlag(cmd)
			hint := fmt.Sprintf("run: google-play-pp-cli top --collection %s --category %s --country %s   (repeatedly over time)", collection, category, country)
			if !dbFileExists(db) {
				hintStderr(cmd, db, hint)
				return emit(cmd, flags, rankHistoryView{
					AppID: args[0], Collection: collection, Category: category, Country: country,
					Points: []store.RankPoint{}, Note: "no local snapshots yet; " + hint,
				})
			}
			s, err := openStoreFor(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			points, err := s.AppRankSeries(cmd.Context(), args[0], wire, category, country)
			if err != nil {
				return apiErr(err)
			}
			view := rankHistoryView{AppID: args[0], Collection: collection, Category: category, Country: country, Points: points}
			if len(points) == 0 {
				view.Points = []store.RankPoint{}
				view.Note = "no snapshots yet contain this app in that chart; " + hint
				return emit(cmd, flags, view)
			}
			view.FirstSeen = points[0].CapturedAt
			view.LastSeen = points[len(points)-1].CapturedAt
			peak := points[0].Rank
			for _, p := range points {
				if p.Rank < peak {
					peak = p.Rank
				}
			}
			view.PeakRank = peak
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "TOP_FREE", "Chart collection: TOP_FREE, TOP_PAID, or GROSSING")
	cmd.Flags().StringVar(&category, "category", "GAME", "Category (e.g. GAME, GAME_PUZZLE)")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}
