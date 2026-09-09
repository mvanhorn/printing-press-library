// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: wire the hand-built listings get/count subcommands under the
// generated `listings` parent group. They are not spec endpoints because the
// generator's GraphQL-endpoint path cannot map a positional or filter flags
// into the query variables.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		listingsCmd, _, err := root.Find([]string{"listings"})
		if err != nil || listingsCmd == nil {
			return
		}
		addNovelCommandIfAbsent(listingsCmd, newNovelListingsGetCmd(flags))
		addNovelCommandIfAbsent(listingsCmd, newNovelListingsCountCmd(flags))
	})
}
