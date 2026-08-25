// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: channel history backfill. Implemented body — regeneration preserves this file.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"

	"github.com/spf13/cobra"
)

type backfillResult struct {
	Channel       *resolvedChannel `json:"channel"`
	VideosStored  int              `json:"videosStored"`
	SnapshotAt    string           `json:"snapshotAt"`
	QuotaUnitsEst int              `json:"quotaUnitsEst"`
	RotatedKeys   []string         `json:"rotatedKeys,omitempty"`
	Note          string           `json:"note,omitempty"`
}

func newNovelBackfillCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxVideos int
	var rotate bool
	cmd := &cobra.Command{
		Use:     "backfill [@handle|channelId]",
		Short:   "Pull a channel's complete upload history with statistics into the local databank in one command.",
		Long:    "Use this command to pull a channel's complete upload history and statistics into the local store.\nDo NOT use it for a quick recent-uploads peek; use 'youtube channel-uploads' instead.",
		Example: "  youtube-pp-cli backfill @mkbhd --json\n  youtube-pp-cli backfill UCBJycsmduvYEL83R_U4JriQ --max-videos 500",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "channel=@GoogleDevelopers;--max-videos=5",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backfill channel history")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("channel handle or id is required"))
			}
			limit := maxVideos
			if limit <= 0 {
				limit = 20000
			}
			if cliutil.IsDogfoodEnv() && limit > 50 {
				limit = 50
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			g, err := newRotatingGetter(flags, rotate)
			if err != nil {
				return err
			}
			ch, err := resolveChannelInput(ctx, g, args[0])
			if err != nil {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			if ch.UploadsPlaylist == "" {
				return notFoundErr(fmt.Errorf("channel %s has no uploads playlist", ch.ChannelID))
			}
			ids, err := walkUploadsPlaylist(ctx, g, ch.UploadsPlaylist, limit)
			if err != nil && len(ids) == 0 {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			partialWalk := err != nil
			rows, err := fetchVideoStats(ctx, g, ids)
			if err != nil && len(rows) == 0 {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			partialStats := err != nil
			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			now := nowUTC()
			stored, err := writeVideoRows(ctx, db, rows, now)
			if err != nil {
				return apiErr(fmt.Errorf("stored %d rows, then: %w", stored, err))
			}
			if err := writeChannelSnapshot(ctx, db, ch, now); err != nil {
				return apiErr(err)
			}
			// Quota: 1 channels.list + one playlistItems page per 50 ids
			// + one videos.list batch per 50 ids.
			pages := (len(ids) + 49) / 50
			res := backfillResult{
				Channel:       ch,
				VideosStored:  stored,
				SnapshotAt:    now,
				QuotaUnitsEst: 1 + pages + pages,
				RotatedKeys:   g.Rotated,
			}
			if partialWalk || partialStats {
				res.Note = "partial: the API errored mid-run; stored what was fetched — rerun to complete"
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+res.Note)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&maxVideos, "max-videos", 0, "Cap on videos to backfill (0 = entire history)")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}
