// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelHandoffCmd(flags *rootFlags) *cobra.Command { return dpHandoff(flags) }
