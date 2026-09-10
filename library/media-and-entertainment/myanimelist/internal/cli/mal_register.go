// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Registers the hand-written MyAnimeList commands that the generator did not
// scaffold from research.json (airing, watch-order, and the local library).
// Novel hooks run after generated parent groups are attached, so
// addNovelCommandIfAbsent can defer to a generated command of the same name.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if animeCmd, _, err := root.Find([]string{"anime"}); err == nil {
			addNovelCommandIfAbsent(animeCmd, newAnimeShowCmd(flags))
		}
		if mangaCmd, _, err := root.Find([]string{"manga"}); err == nil {
			addNovelCommandIfAbsent(mangaCmd, newMangaShowCmd(flags))
		}
		addNovelCommandIfAbsent(root, newNovelAiringCmd(flags))
		addNovelCommandIfAbsent(root, newNovelWatchOrderCmd(flags))
		addNovelCommandIfAbsent(root, newNovelTrackCmd(flags))
		addNovelCommandIfAbsent(root, newNovelNextCmd(flags))
		addNovelCommandIfAbsent(root, newNovelSuggestCmd(flags))
	})
}
