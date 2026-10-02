// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newNovelInterchangesCmd(flags *rootFlags) *cobra.Command { return dpInterchanges(flags) }
