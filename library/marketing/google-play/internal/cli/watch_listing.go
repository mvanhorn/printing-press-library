// Hand-authored transcendence command: listing change detection.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type listingChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

type watchListingView struct {
	AppID        string          `json:"appId"`
	PrevSnapshot int64           `json:"prevSnapshot"`
	Snapshot     int64           `json:"snapshot"`
	Changes      []listingChange `json:"changes"`
	Note         string          `json:"note,omitempty"`
}

// watchedFields are the listing fields whose changes matter for competitive
// monitoring. Screenshots are compared by count to avoid noisy URL churn.
var watchedFields = []string{
	"title", "version", "price", "currency", "offersIAP", "iapRange",
	"containsAds", "score", "installs", "recentChanges", "androidVersion", "contentRating",
}

// pp:data-source local
func newNovelWatchListingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch-listing <appId>",
		Short: "Diff the two latest snapshots of a listing to surface what changed.",
		Long: "Field-by-field diff of the two most recent local snapshots of an app listing (title, version, price, IAP range, " +
			"ads flag, score, installs, recent changes). Reads snapshots written by 'app'; run 'app <appId>' at two different times first.\n\n" +
			"Use this command to see what changed on a tracked listing over time. For a current full field dump, use 'app'.",
		Example: "  google-play-pp-cli watch-listing com.dreamgames.royalkingdom --agent",
		Args:    cobra.ArbitraryArgs,
		// Reads local snapshots; any appId without two snapshots returns a valid
		// empty-state view, which is not a bad-input error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the two latest listing snapshots")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			db := resolveDBFlag(cmd)
			hint := fmt.Sprintf("run: google-play-pp-cli app %s   (twice, at different times)", args[0])
			if !dbFileExists(db) {
				hintStderr(cmd, db, hint)
				return emit(cmd, flags, watchListingView{AppID: args[0], Changes: []listingChange{}, Note: "no local snapshots yet; " + hint})
			}
			s, err := openStoreFor(cmd.Context(), db)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			snaps, err := s.LatestAppSnapshots(cmd.Context(), args[0], 2)
			if err != nil {
				return apiErr(err)
			}
			view := watchListingView{AppID: args[0], Changes: []listingChange{}}
			if len(snaps) < 2 {
				if len(snaps) == 1 {
					view.Snapshot = snaps[0].CapturedAt
				}
				view.Note = "need at least two snapshots to diff a listing; " + hint
				return emit(cmd, flags, view)
			}
			latest, prev := snaps[0], snaps[1]
			view.Snapshot, view.PrevSnapshot = latest.CapturedAt, prev.CapturedAt
			view.Changes = diffListings(prev.Data, latest.Data)
			if len(view.Changes) == 0 {
				view.Note = "no watched fields changed between the two latest snapshots"
			}
			return emit(cmd, flags, view)
		},
	}
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

func diffListings(prevRaw, latestRaw json.RawMessage) []listingChange {
	var prev, latest map[string]any
	if json.Unmarshal(prevRaw, &prev) != nil || json.Unmarshal(latestRaw, &latest) != nil {
		return []listingChange{}
	}
	changes := []listingChange{}
	for _, f := range watchedFields {
		if !valuesEqual(prev[f], latest[f]) {
			changes = append(changes, listingChange{Field: f, From: prev[f], To: latest[f]})
		}
	}
	// Screenshot count change (compare lengths, not URLs).
	if pc, lc := countOf(prev["screenshots"]), countOf(latest["screenshots"]); pc != lc {
		changes = append(changes, listingChange{Field: "screenshotCount", From: pc, To: lc})
	}
	return changes
}

func valuesEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func countOf(v any) int {
	if arr, ok := v.([]any); ok {
		return len(arr)
	}
	return 0
}
