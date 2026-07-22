// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// kit assembles a coherent streamer kit (overlay, panels, emotes, webcam
// frame, screens) for a style and flags missing slots — a matched-set join the
// Placeit UI never assembles.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/spf13/cobra"
)

// kitSlots maps a streamer asset slot to the name/tag keywords that identify it.
var kitSlots = []struct {
	Name     string
	Keywords []string
}{
	{"overlay", []string{"overlay"}},
	{"panels", []string{"panel"}},
	{"emote", []string{"emote"}},
	{"webcam_frame", []string{"webcam", "facecam", "cam frame", "camera frame"}},
	{"screen", []string{"screen", "brb", "starting soon", "be right back", "stream ending", "offline"}},
	{"banner", []string{"banner", "header"}},
}

func newNovelKitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kit <style>",
		Short: "Assemble a matched streamer kit (overlay, panels, emotes, frame) and flag missing slots.",
		Long: strings.Trim(`
Search your local mirror for gaming/Twitch templates matching a style, group
them into streamer asset slots (overlay, panels, emote, webcam frame, screens,
banner), pick the most popular template per slot, and report which slots are
covered and which are missing. Run 'sync' first (a gaming/Twitch-scoped sync
works best, e.g. 'sync --query twitch').

Use to assemble a streamer asset set from one style family.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli kit "neon gaming" --agent
  placeit-pp-cli kit "retro twitch"`, "\n"),
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
				return usageErr(fmt.Errorf("a style query is required (e.g. \"neon gaming\")"))
			}
			style := strings.Join(args, " ")

			// Build the candidate pool either live (Algolia, no sync needed)
			// or from the offline mirror (--data-source local).
			var pool []map[string]any
			livePool := false
			if flags.dataSource != "local" {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				ac := algolia.New(flags.timeout)
				res, serr := ac.Search(ctx, algolia.SearchParams{
					Index:       algolia.IndexBestSelling,
					Query:       style,
					HitsPerPage: 300,
				})
				if serr != nil {
					if flags.dataSource == "live" {
						return apiErr(serr)
					}
				} else {
					for _, h := range res.Hits {
						if m, cerr := cleanStage(h); cerr == nil {
							pool = append(pool, m)
						}
					}
					livePool = true
				}
			}
			if !livePool {
				db, ok, err := openCatalogStore(cmd, flags)
				if err != nil || !ok {
					return err
				}
				defer db.Close()
				if !hintIfUnsynced(cmd, db, templateResource) {
					hintIfStale(cmd, db, templateResource, flags.maxAge)
				}
				all, err := loadCatalog(db)
				if err != nil {
					return apiErr(err)
				}
				for _, m := range all {
					if matchesQuery(m, style) {
						pool = append(pool, m)
					}
				}
			}

			type cand struct {
				m         map[string]any
				purchases float64
			}
			bySlot := map[string][]cand{}
			for _, m := range pool {
				hay := strings.ToLower(fmt.Sprint(m["name"]) + " " +
					strings.Join(tagValues(m, "stage_tags"), " ") + " " +
					strings.Join(tagValues(m, "device_tags"), " "))
				for _, slot := range kitSlots {
					for _, kw := range slot.Keywords {
						if strings.Contains(hay, kw) {
							bySlot[slot.Name] = append(bySlot[slot.Name], cand{m: m, purchases: asFloat(m["purchases"])})
							break
						}
					}
				}
			}

			slots := make([]map[string]any, 0, len(kitSlots))
			var missing []string
			covered := 0
			for _, slot := range kitSlots {
				cands := bySlot[slot.Name]
				entry := map[string]any{"slot": slot.Name, "matches": len(cands)}
				if len(cands) == 0 {
					entry["status"] = "missing"
					missing = append(missing, slot.Name)
				} else {
					sort.SliceStable(cands, func(i, j int) bool { return cands[i].purchases > cands[j].purchases })
					entry["status"] = "covered"
					entry["pick"] = projectStage(cands[0].m)
					covered++
				}
				slots = append(slots, entry)
			}
			return flags.printJSON(cmd, map[string]any{
				"style":         style,
				"slots":         slots,
				"covered":       covered,
				"total_slots":   len(kitSlots),
				"missing_slots": missing,
				"complete":      len(missing) == 0,
			})
		},
	}
	return cmd
}
