// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newDocstoreGroupCmd is the visible top-level "docstore" parent for the novel
// compound workflows that complement the generated "document-store" resource.
func newDocstoreGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docstore",
		Aliases: []string{"document-stores"},
		Short:   "Compound document-store workflows: folder ingest, RAG drift report",
		Long: `Compound document-store workflows that build on the generated document-store
resource commands.

` + "`docstore ingest`" + ` walks a folder of source material and pushes every matching
file into a Flowise document store, then triggers vector indexing — useful for
ingesting MLS exports, market PDFs, or any batch of files into RAG.

` + "`docstore drift`" + ` joins the local document_store table with upsert_history to
show which stores got new content in a time window and which chatflows
reference each store.`,
	}
	cmd.AddCommand(newDocstoreIngestCmd(flags))
	cmd.AddCommand(newDocstoreDriftCmd(flags))
	return cmd
}
