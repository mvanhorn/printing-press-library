// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
)

// compactSong is the trimmed song projection for --compact output. Drops
// the IDs, the index, and the start-time offset; agents skimming "what
// played" want title + artist + liked.
type compactSong struct {
	Title   string   `json:"title"`
	Artists []string `json:"artists"`
	Liked   bool     `json:"liked"`
}

func newRideCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ride",
		Short: "Ride metadata (show)",
	}
	cmd.AddCommand(newRideShowCmd(flags))
	return cmd
}

func newRideShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <ride-id>",
		Short: "Show a ride's metadata + playlist by id",
		Long: `Fetches /api/ride/{id}/details and prints the ride title, duration, and
playlist (song order, artists, liked-flag). Pair with ` + "`workouts show`" + `:
the workout's ride_id is the input here.

Some on-demand rides ship with empty playlists (instructor talk-only) —
that's a normal "songs": [] response, not an error.`,
		Example: `  peloton-pp-cli ride show 63b64b4c083948809c05a9de800c0a50
  peloton-pp-cli ride show <ride-id> --compact | jq '.songs[] | select(.liked)'`,
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return Errf(CodeAPI, "loading config: %w", err)
			}
			if cfg.Token == "" {
				return Errf(CodeAuth, "no token saved — run `peloton-pp-cli auth login` first")
			}
			c := client.New(cfg.Token)
			rd, err := c.GetRideDetails(args[0])
			if err != nil {
				return classify(err)
			}
			return emitRide(cmd, flags, rd)
		},
	}
	return cmd
}

func emitRide(cmd *cobra.Command, flags *rootFlags, rd client.RideDetails) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
	if flags.compact {
		songs := make([]compactSong, len(rd.Songs))
		for i, s := range rd.Songs {
			songs[i] = compactSong{Title: s.Title, Artists: s.Artists, Liked: s.Liked}
		}
		out := struct {
			RideID string        `json:"ride_id"`
			Title  string        `json:"title"`
			Songs  []compactSong `json:"songs"`
		}{rd.RideID, rd.Title, songs}
		enc := json.NewEncoder(cmd.OutOrStdout())
		return enc.Encode(out)
	}
	if wantJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rd)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s — %s (%d songs)\n", rd.RideID, rd.Title, len(rd.Songs))
	for _, s := range rd.Songs {
		mark := " "
		if s.Liked {
			mark = "♥"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %2d. %s — %s\n", mark, s.Index+1, s.Title, joinNames(s.Artists))
	}
	return nil
}

func joinNames(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	out := xs[0]
	for _, x := range xs[1:] {
		out += ", " + x
	}
	return out
}
