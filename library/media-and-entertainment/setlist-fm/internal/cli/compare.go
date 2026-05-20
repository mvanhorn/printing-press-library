package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/internal/store"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var tours []string

	cmd := &cobra.Command{
		Use:     "compare <artist>",
		Short:   "Compare two tours: overlap, dropped songs, added songs, position shifts",
		Example: `  setlist-fm-pp-cli compare "Radiohead" --tour "In Rainbows Tour" --tour "A Moon Shaped Pool Tour"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(tours) != 2 {
				return fmt.Errorf("exactly two --tour flags required")
			}
			db, err := openStoreOrFail(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			mbid, name, err := resolveArtistFromStore(db, args[0])
			if err != nil {
				return err
			}

			songsA, err := collectTourSongs(db, mbid, tours[0])
			if err != nil {
				return err
			}
			songsB, err := collectTourSongs(db, mbid, tours[1])
			if err != nil {
				return err
			}

			var overlap, dropped, added []string
			for song := range songsA {
				if _, ok := songsB[song]; ok {
					overlap = append(overlap, song)
				} else {
					dropped = append(dropped, song)
				}
			}
			for song := range songsB {
				if _, ok := songsA[song]; !ok {
					added = append(added, song)
				}
			}
			sort.Strings(overlap)
			sort.Strings(dropped)
			sort.Strings(added)

			totalUnique := len(songsA)
			for song := range songsB {
				if _, ok := songsA[song]; !ok {
					totalUnique++
				}
			}
			overlapPct := 0.0
			if totalUnique > 0 {
				overlapPct = float64(len(overlap)) / float64(totalUnique) * 100
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":      name,
					"tour_a":      tours[0],
					"tour_b":      tours[1],
					"overlap_pct": fmt.Sprintf("%.1f%%", overlapPct),
					"overlap":     overlap,
					"dropped":     dropped,
					"added":       added,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tour Comparison for %s\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "  A: %s (%d unique songs)\n", tours[0], len(songsA))
			fmt.Fprintf(cmd.OutOrStdout(), "  B: %s (%d unique songs)\n", tours[1], len(songsB))
			fmt.Fprintf(cmd.OutOrStdout(), "  Overlap: %d songs (%.1f%%)\n\n", len(overlap), overlapPct)

			if len(dropped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Dropped (in A, not B):\n")
				for _, s := range dropped {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", s)
				}
			}
			if len(added) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nAdded (in B, not A):\n")
				for _, s := range added {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", s)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&tours, "tour", nil, "Tour name (specify twice)")
	return cmd
}

// collectTourSongs returns the set of unique lowercased song names for a tour.
func collectTourSongs(db *store.Store, mbid, tourName string) (map[string]bool, error) {
	setlists, err := db.GetSetlistsByTour(mbid, tourName)
	if err != nil {
		return nil, err
	}
	songs := map[string]bool{}
	for _, sl := range setlists {
		items, err := db.GetSetlistSongs(sl.ID)
		if err != nil {
			continue
		}
		for _, song := range items {
			if song.Name != "" {
				songs[strings.ToLower(song.Name)] = true
			}
		}
	}
	return songs, nil
}
