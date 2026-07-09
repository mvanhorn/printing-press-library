// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type wbDiffChange struct {
	Date string   `json:"date"`
	Old  *float64 `json:"old"`
	New  *float64 `json:"new"`
}

type wbDiffView struct {
	Country     string         `json:"country"`
	Indicator   string         `json:"indicator"`
	HadSnapshot bool           `json:"had_snapshot"`
	Added       []wbDiffChange `json:"added"`
	Removed     []wbDiffChange `json:"removed"`
	Changed     []wbDiffChange `json:"changed"`
	Note        string         `json:"note,omitempty"`
}

func wbSnapshotPath(country, indicator string) string {
	dir := filepath.Join(filepath.Dir(defaultDBPath("world-bank-pp-cli")), "snapshots")
	name := strings.ToUpper(country) + "_" + indicator + ".json"
	name = strings.ReplaceAll(name, "/", "_")
	return filepath.Join(dir, name)
}

func newNovelDataDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "diff <country> <indicator>",
		Short:       "Compare a fresh pull against the last snapshot to surface revised observations.",
		Long:        "Compare a fresh pull against the last saved snapshot for one country+indicator to detect World Bank data revisions.\nThe first run records a baseline; later runs report added/removed/changed observations.",
		Example:     "  world-bank-pp-cli data diff USA NY.GDP.MKTP.CD",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("country and indicator are required"))
			}
			country, indicator := args[0], args[1]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obs, err := wbFetchObservations(ctx, c, country, indicator, map[string]string{"per_page": "1000"}, 20)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			fresh := map[string]*float64{}
			for _, o := range obs {
				fresh[o.Date] = o.Value
			}

			snapPath := wbSnapshotPath(country, indicator)
			view := wbDiffView{Country: country, Indicator: indicator, Added: []wbDiffChange{}, Removed: []wbDiffChange{}, Changed: []wbDiffChange{}}

			var prior map[string]*float64
			// #nosec G304 -- snapPath is built from the internal DB dir plus a sanitized country+indicator, not from user-supplied paths.
			if b, readErr := os.ReadFile(snapPath); readErr == nil {
				if uErr := json.Unmarshal(b, &prior); uErr != nil {
					return fmt.Errorf("snapshot %s is unreadable (corrupt or truncated): %w; not overwriting — delete it to record a fresh baseline", snapPath, uErr)
				}
			}
			if prior != nil {
				view.HadSnapshot = true
				for date, nv := range fresh {
					ov, existed := prior[date]
					if !existed {
						view.Added = append(view.Added, wbDiffChange{Date: date, New: nv})
					} else if !floatEq(ov, nv) {
						view.Changed = append(view.Changed, wbDiffChange{Date: date, Old: ov, New: nv})
					}
				}
				for date, ov := range prior {
					if _, ok := fresh[date]; !ok {
						view.Removed = append(view.Removed, wbDiffChange{Date: date, Old: ov})
					}
				}
				sortChanges(view.Added)
				sortChanges(view.Removed)
				sortChanges(view.Changed)
			} else {
				view.Note = fmt.Sprintf("no prior snapshot; baseline of %d observations saved. Re-run later to see revisions.", len(fresh))
			}

			// Persist fresh snapshot.
			if mkErr := os.MkdirAll(filepath.Dir(snapPath), 0o750); mkErr != nil {
				return fmt.Errorf("creating snapshot dir %s: %w; diff not shown — the new baseline could not be saved", filepath.Dir(snapPath), mkErr)
			}
			b, mErr := json.Marshal(fresh)
			if mErr != nil {
				return fmt.Errorf("encoding snapshot: %w", mErr)
			}
			// #nosec G304 G306 -- snapPath is the internal DB dir plus a sanitized country+indicator; 0600 perms.
			if wErr := os.WriteFile(snapPath, b, 0o600); wErr != nil {
				return fmt.Errorf("writing snapshot %s: %w; diff not shown — the new baseline could not be saved", snapPath, wErr)
			}
			return flags.printJSON(cmd, view)
		},
	}
	return cmd
}

func floatEq(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sortChanges(c []wbDiffChange) {
	sort.Slice(c, func(i, j int) bool { return c[i].Date < c[j].Date })
}
