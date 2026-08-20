package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newTourShapeCmd(flags *rootFlags) *cobra.Command {
	var tourName string

	cmd := &cobra.Command{
		Use:   "tour-shape <artist>",
		Short: "Analyze the shape of a tour: set lengths, encores, openers, closers",
		Long: `For one tour: median set length, average encore count, top 5 openers,
top 5 closers, and a song-position histogram. If no --tour is specified,
uses the most recent tour.`,
		Example: `  setlist-fm-pp-cli tour-shape "Radiohead" --tour "A Moon Shaped Pool Tour"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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

			// Resolve tour name
			if tourName == "" {
				tourName, err = db.GetLatestTourName(mbid)
				if err != nil || tourName == "" {
					return fmt.Errorf("no tour found for %s; specify --tour", name)
				}
			}

			setlists, err := db.GetSetlistsByTour(mbid, tourName)
			if err != nil {
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for tour %q by %s", tourName, name)
			}

			// Compute stats
			var setLengths []int
			encoreTotal := 0
			openerCounts := map[string]int{}
			closerCounts := map[string]int{}
			positionMap := map[string][]int{}

			for _, sl := range setlists {
				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil || len(songs) == 0 {
					continue
				}
				setLengths = append(setLengths, len(songs))
				encoreTotal += sl.EncoreCount

				// Opener
				openerCounts[songs[0].Name]++
				// Closer
				closerCounts[songs[len(songs)-1].Name]++

				// Position histogram
				for _, song := range songs {
					key := strings.ToLower(song.Name)
					positionMap[key] = append(positionMap[key], song.Position)
				}
			}

			// Median set length
			sort.Ints(setLengths)
			medianLen := 0
			if len(setLengths) > 0 {
				medianLen = setLengths[len(setLengths)/2]
			}
			avgEncore := 0.0
			if len(setlists) > 0 {
				avgEncore = float64(encoreTotal) / float64(len(setlists))
			}

			// Top openers/closers
			topOpeners := topN(openerCounts, 5)
			topClosers := topN(closerCounts, 5)

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":            name,
					"tour":              tourName,
					"shows":             len(setlists),
					"median_set_length": medianLen,
					"avg_encores":       fmt.Sprintf("%.1f", avgEncore),
					"top_openers":       topOpeners,
					"top_closers":       topClosers,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tour Shape: %s — %s (%d shows)\n\n", name, tourName, len(setlists))
			fmt.Fprintf(cmd.OutOrStdout(), "  Median set length:  %d songs\n", medianLen)
			fmt.Fprintf(cmd.OutOrStdout(), "  Avg encores:        %.1f\n\n", avgEncore)

			fmt.Fprintf(cmd.OutOrStdout(), "Top Openers:\n")
			for i, o := range topOpeners {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (%d times)\n", i+1, o.Name, o.Count)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTop Closers:\n")
			for i, c := range topClosers {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (%d times)\n", i+1, c.Name, c.Count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&tourName, "tour", "", "Tour name (default: most recent)")
	return cmd
}

type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func topN(counts map[string]int, n int) []nameCount {
	var items []nameCount
	for name, count := range counts {
		items = append(items, nameCount{name, count})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
	if n > 0 && n < len(items) {
		items = items[:n]
	}
	return items
}
