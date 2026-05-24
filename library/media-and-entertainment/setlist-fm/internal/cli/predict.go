package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newPredictCmd(flags *rootFlags) *cobra.Command {
	var last int
	var songs int
	var venueID string

	cmd := &cobra.Command{
		Use:   "predict <artist>",
		Short: "Generate a predicted setlist using recency-weighted song probability",
		Long: `Analyzes the artist's recent setlists and predicts a likely setlist for
an upcoming show. Uses exponential decay weighting so recent performances
have more influence. Optionally filter by venue for venue-specific predictions.`,
		Example: `  setlist-fm-pp-cli predict "Radiohead" --last 30 --songs 22
  setlist-fm-pp-cli predict "Radiohead" --venue 4bd6ca6e`,
		Args: cobra.ExactArgs(1),
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

			setlists, err := db.GetSetlistsByArtistLimit(mbid, last)
			if err != nil {
				return fmt.Errorf("querying setlists: %w", err)
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for %s; run 'sync artist %s' first", name, args[0])
			}

			// Filter by venue if specified
			if venueID != "" {
				var venueFiltered = setlists[:0]
				for _, sl := range setlists {
					if sl.VenueID == venueID {
						venueFiltered = append(venueFiltered, sl)
					}
				}
				if len(venueFiltered) > 0 {
					setlists = venueFiltered
				}
			}

			// Compute per-song probability with exponential decay
			type songProb struct {
				Name  string  `json:"name"`
				Prob  float64 `json:"probability"`
				Plays int     `json:"plays"`
				Last  string  `json:"last_played"`
			}
			songMap := map[string]*songProb{}
			totalWeight := 0.0
			decay := 0.95 // Decay factor per setlist

			for i, sl := range setlists {
				weight := math.Pow(decay, float64(i))
				totalWeight += weight

				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if song.Name == "" {
						continue
					}
					key := strings.ToLower(song.Name)
					if _, ok := songMap[key]; !ok {
						songMap[key] = &songProb{Name: song.Name}
					}
					songMap[key].Prob += weight
					songMap[key].Plays++
					if songMap[key].Last == "" || sl.EventDate > songMap[key].Last {
						songMap[key].Last = sl.EventDate
					}
				}
			}

			// Normalize probabilities
			var predictions []songProb
			for _, sp := range songMap {
				sp.Prob = sp.Prob / totalWeight * 100
				predictions = append(predictions, *sp)
			}
			sort.Slice(predictions, func(i, j int) bool {
				return predictions[i].Prob > predictions[j].Prob
			})

			limit := songs
			if limit <= 0 || limit > len(predictions) {
				limit = len(predictions)
			}
			predictions = predictions[:limit]

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":      name,
					"based_on":    len(setlists),
					"predictions": predictions,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Predicted setlist for %s (based on last %d shows):\n\n", name, len(setlists))
			headers := []string{"#", "SONG", "PROB", "PLAYS", "LAST PLAYED"}
			var rows [][]string
			for i, p := range predictions {
				_ = i
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					p.Name,
					fmt.Sprintf("%.1f%%", p.Prob),
					fmt.Sprintf("%d", p.Plays),
					p.Last,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&last, "last", 20, "Number of recent setlists to analyze")
	cmd.Flags().IntVar(&songs, "songs", 25, "Number of songs to include in the predicted setlist")
	cmd.Flags().StringVar(&venueID, "venue", "", "Filter by venue ID")
	return cmd
}
