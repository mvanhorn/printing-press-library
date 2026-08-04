// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored: registers the novel commands (digest, filters, image, read,
// shop, since, variants) with the generated root via the novel-command hook,
// so regeneration never drops them from the command tree.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelDigestCmd(flags))
		addNovelCommandIfAbsent(root, newNovelFiltersCmd(flags))
		addNovelCommandIfAbsent(root, newNovelImageCmd(flags))
		addNovelCommandIfAbsent(root, newNovelReadCmd(flags))
		addNovelCommandIfAbsent(root, newNovelShopCmd(flags))
		addNovelCommandIfAbsent(root, newNovelSinceCmd(flags))
		addNovelCommandIfAbsent(root, newNovelVariantsCmd(flags))
	})
}
