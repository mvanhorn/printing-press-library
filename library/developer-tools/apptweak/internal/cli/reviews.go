// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelReviewsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "reviews",
		Short:       "Work with reviews",
		Example:     "  apptweak-pp-cli reviews sentiment --app 284882215 --country us --device iphone --word crash --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelReviewsSentimentCmd(flags))
	return cmd
}
