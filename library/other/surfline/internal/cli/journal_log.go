// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: snapshot a spot's current forecast into the
// local journal table.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelJournalLogCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "log <spotId>",
		Short: "Snapshot a spot's current forecast into the local journal.",
		Long: "Captures the current wave, swell, wind and rating for a spot into local SQLite.\n\n" +
			"Use this command (e.g. from cron) to build a history. To review captures use 'journal show'.",
		Example: strings.Trim(`
  surfline-pp-cli journal log 5842041f4e65fad6a7708807
  surfline-pp-cli journal log 5842041f4e65fad6a7708807 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot the spot's current forecast")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required"))
			}
			spotID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			waves, err := fetchWave(ctx, c, spotID, 1, 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(waves) == 0 {
				return notFoundErr(fmt.Errorf("no wave forecast returned for spot %q", spotID))
			}
			winds, _ := fetchWind(ctx, c, spotID, 1, 1)
			ratings, _ := fetchRating(ctx, c, spotID, 1)
			name := fetchSpotName(ctx, c, spotID)

			wv := waves[0]
			snap := journalSnapshot{
				SpotID:     spotID,
				SpotName:   name,
				CapturedAt: time.Now().Unix(),
				SurfMin:    wv.Surf.Min,
				SurfMax:    wv.Surf.Max,
			}
			if sw, ok := wv.topSwell(); ok {
				snap.SwellHt = sw.Height
				snap.SwellPer = sw.Period
			}
			if len(winds) > 0 {
				snap.WindKts = winds[0].Speed
				snap.WindType = winds[0].DirectionType
			}
			if len(ratings) > 0 {
				snap.RatingKey = ratings[0].Rating.Key
			}
			raw, _ := json.Marshal(wv)
			snap.Snapshot = raw

			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			id, err := saveJournalSnapshot(ctx, db, snap)
			if err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}
			snap.ID = id

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				snap.Snapshot = nil
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged %s: surf %.0f-%.0fft, swell %.1fft@%.0fs, wind %.0fkt %s, rating %s\n",
				name, snap.SurfMin, snap.SurfMax, snap.SwellHt, snap.SwellPer, snap.WindKts, snap.WindType, snap.RatingKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	return cmd
}
