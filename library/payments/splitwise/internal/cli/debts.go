// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import "github.com/spf13/cobra"

func newNovelDebtsCmd(flags *rootFlags) *cobra.Command { return newDebtsCmd(flags) }
