// Hand-authored transcendence command: chart movers between two snapshots.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

type moverEntry struct {
	AppID    string `json:"appId"`
	Title    string `json:"title"`
	Status   string `json:"status"` // climber|dropper|new|dropped
	PrevRank int    `json:"prevRank,omitempty"`
	Rank     int    `json:"rank,omitempty"`
	Delta    int    `json:"delta,omitempty"` // positive = moved up
}

type moversView struct {
	Collection   string       `json:"collection"`
	Category     string       `json:"category"`
	Country      string       `json:"country"`
	PrevSnapshot int64        `json:"prevSnapshot"`
	Snapshot     int64        `json:"snapshot"`
	Climbers     []moverEntry `json:"climbers"`
	Droppers     []moverEntry `json:"droppers"`
	NewEntries   []moverEntry `json:"newEntries"`
	DroppedOut   []moverEntry `json:"droppedOut"`
	Note         string       `json:"note,omitempty"`
}

// pp:data-source local
func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var collection, category string
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "See which apps climbed, dropped, entered, or fell off a chart between two snapshots.",
		Long: "Diff the two most recent local snapshots of a chart and classify every app as a climber, dropper, " +
			"new entry, or dropped-out, with rank deltas. Reads local snapshots written by 'top'; run 'top' for the same " +
			"chart at two different times first.\n\n" +
			"Use this command for the whole-chart diff between two points in time. For a single app's trajectory, use 'rank-history' instead.",
		Example:     "  google-play-pp-cli movers --collection TOP_GROSSING --category GAME_PUZZLE --country us --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the two most recent chart snapshots")
				return nil
			}
			wire, ok := gplay.NormalizeCollection(collection)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--collection must be one of %s", strings.Join(gplay.CollectionNames(), ", ")))
			}
			category = strings.ToUpper(category)
			country, _ := localeOf(cmd)
			db := resolveDBFlag(cmd)
			hint := fmt.Sprintf("run: google-play-pp-cli top --collection %s --category %s --country %s   (twice, at different times)", collection, category, country)
			emptyView := moversView{
				Collection: collection, Category: category, Country: country,
				Climbers: []moverEntry{}, Droppers: []moverEntry{}, NewEntries: []moverEntry{}, DroppedOut: []moverEntry{},
			}
			if !dbFileExists(db) {
				hintStderr(cmd, db, hint)
				emptyView.Note = "no local snapshots yet; " + hint
				return emit(cmd, flags, emptyView)
			}
			s, err := openStoreFor(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			prevT, latestT, hasTwo, err := s.LatestTwoChartSnapshots(cmd.Context(), wire, category, country)
			if err != nil {
				return apiErr(err)
			}
			view := moversView{
				Collection: collection, Category: category, Country: country,
				Climbers: []moverEntry{}, Droppers: []moverEntry{}, NewEntries: []moverEntry{}, DroppedOut: []moverEntry{},
			}
			if !hasTwo {
				view.Snapshot = latestT
				view.Note = "need at least two snapshots to compute movers; " + hint
				return emit(cmd, flags, view)
			}
			prev, err := s.ChartSnapshotAt(cmd.Context(), wire, category, country, prevT)
			if err != nil {
				return apiErr(err)
			}
			latest, err := s.ChartSnapshotAt(cmd.Context(), wire, category, country, latestT)
			if err != nil {
				return apiErr(err)
			}
			view.PrevSnapshot, view.Snapshot = prevT, latestT
			computeMovers(&view, prev, latest)
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "TOP_FREE", "Chart collection: TOP_FREE, TOP_PAID, or GROSSING")
	cmd.Flags().StringVar(&category, "category", "GAME", "Category (e.g. GAME, GAME_PUZZLE)")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

func computeMovers(v *moversView, prev, latest []store.ChartRow) {
	prevRank := map[string]int{}
	for _, r := range prev {
		prevRank[r.AppID] = r.Rank
	}
	latestSet := map[string]bool{}
	for _, r := range latest {
		latestSet[r.AppID] = true
		pr, existed := prevRank[r.AppID]
		if !existed {
			v.NewEntries = append(v.NewEntries, moverEntry{AppID: r.AppID, Title: r.Title, Status: "new", Rank: r.Rank})
			continue
		}
		delta := pr - r.Rank // positive = moved up (toward rank 1)
		e := moverEntry{AppID: r.AppID, Title: r.Title, PrevRank: pr, Rank: r.Rank, Delta: delta}
		switch {
		case delta > 0:
			e.Status = "climber"
			v.Climbers = append(v.Climbers, e)
		case delta < 0:
			e.Status = "dropper"
			v.Droppers = append(v.Droppers, e)
		}
	}
	for _, r := range prev {
		if !latestSet[r.AppID] {
			v.DroppedOut = append(v.DroppedOut, moverEntry{AppID: r.AppID, Title: r.Title, Status: "dropped", PrevRank: r.Rank})
		}
	}
}
