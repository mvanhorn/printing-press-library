// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

// newImportCmd is the explicit local-file counterpart to a remote sync. It
// delegates to the catalog indexer and never claims to contact ihatepdf.cv.
func newImportCmd(flags *rootFlags) *cobra.Command {
	cmd := newCatalogIndexCmd(flags)
	cmd.Use = "import [paths...]"
	cmd.Short = "Import local PDFs into the searchable catalog without uploading them."
	cmd.Example = "  ihatepdf-cv-pp-cli import ./reports --recursive --agent"
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newImportCmd(flags)) })
}
