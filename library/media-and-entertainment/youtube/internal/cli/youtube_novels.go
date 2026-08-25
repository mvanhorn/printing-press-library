// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

// Registration hook for the nine carried novel commands under the `youtube`
// resource parent. Ported from the prior print (press 4.28 tree) into the
// current framework's registerNovelCommand pattern so regeneration preserves
// the wiring without root.go edits.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		youtubeCmd, _, err := root.Find([]string{"youtube"})
		if err != nil || youtubeCmd == nil || youtubeCmd == root {
			return
		}
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeSearchBulkCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosTranscriptCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosEmbedCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosRelatedCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosCommentsCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeChannelUploadsCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubePlaylistEnrichCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosEnrichCmd(flags))
		addNovelCommandIfAbsent(youtubeCmd, newYoutubeVideosLinksCmd(flags))
	})
}
