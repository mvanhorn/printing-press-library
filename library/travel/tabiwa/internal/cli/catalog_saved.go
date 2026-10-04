// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelCatalogSavedCmd(flags *rootFlags) *cobra.Command { return newCatalogSaved(flags) }
