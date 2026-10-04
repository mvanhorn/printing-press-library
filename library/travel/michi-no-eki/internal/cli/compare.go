// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
// pp:client-call through the real configured source runner in michi_core.go.
package cli

import "github.com/spf13/cobra"

func newNovelCompareCmd(flags *rootFlags) *cobra.Command { return newMichiCompareCmd(flags) }
