package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newPlaylistCmd(flags *rootFlags) *cobra.Command {
	var last int
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "playlist <artist>",
		Short: "Export songs from recent setlists as CSV, M3U, or Spotify search URIs",
		Long: `Generates a playlist from the artist's most recent setlist, or merged
from the last N setlists. Output formats: table (default), csv, m3u,
or spotify-search.`,
		Example: `  setlist-fm-pp-cli playlist "Radiohead"
  setlist-fm-pp-cli playlist "Radiohead" --last 5 --output spotify-search
  setlist-fm-pp-cli playlist "Radiohead" --output m3u`,
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
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for %s", name)
			}

			// Collect unique songs maintaining order
			seen := map[string]bool{}
			type playlistSong struct {
				Name      string `json:"name"`
				Artist    string `json:"artist"`
				SearchURI string `json:"spotify_search_uri,omitempty"`
			}
			var songs []playlistSong

			for _, sl := range setlists {
				items, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range items {
					if song.Name == "" {
						continue
					}
					key := strings.ToLower(song.Name)
					if seen[key] {
						continue
					}
					seen[key] = true

					searchQuery := url.QueryEscape(fmt.Sprintf("%s %s", song.Name, name))
					songs = append(songs, playlistSong{
						Name:      song.Name,
						Artist:    name,
						SearchURI: fmt.Sprintf("https://open.spotify.com/search/%s", searchQuery),
					})
				}
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":   name,
					"based_on": len(setlists),
					"songs":    songs,
				}, flags)
			}

			switch outputFmt {
			case "csv":
				fmt.Fprintln(cmd.OutOrStdout(), "song,artist,spotify_search")
				for _, s := range songs {
					songEsc := strings.ReplaceAll(s.Name, ",", " ")
					fmt.Fprintf(cmd.OutOrStdout(), "%s,%s,%s\n", songEsc, s.Artist, s.SearchURI)
				}
			case "m3u":
				fmt.Fprintln(cmd.OutOrStdout(), "#EXTM3U")
				for _, s := range songs {
					fmt.Fprintf(cmd.OutOrStdout(), "#EXTINF:-1,%s - %s\n", s.Artist, s.Name)
					fmt.Fprintln(cmd.OutOrStdout(), s.SearchURI)
				}
			case "spotify-search":
				for _, s := range songs {
					fmt.Fprintln(cmd.OutOrStdout(), s.SearchURI)
				}
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "Playlist from %s (last %d setlists, %d songs):\n\n", name, len(setlists), len(songs))
				headers := []string{"#", "SONG", "SPOTIFY SEARCH"}
				var rows [][]string
				for i, s := range songs {
					rows = append(rows, []string{
						fmt.Sprintf("%d", i+1),
						s.Name,
						s.SearchURI,
					})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&last, "last", 1, "Number of recent setlists to merge")
	cmd.Flags().StringVar(&outputFmt, "output", "table", "Output format: table, csv, m3u, spotify-search")
	return cmd
}
