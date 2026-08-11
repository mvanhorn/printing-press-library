package cli

// documents_write.go closes GAP-046: the documents family could list, read,
// create and edit, but a document could never be removed or restored, so every
// draft the CLI ever created was permanent.
//
// documentDelete returns DocumentArchivePayload rather than DeletePayload:
// Linear trashes a document instead of erasing it, which is why documentUnarchive
// is the matching restore verb rather than a separate untrash mutation.
//
// The leaves are registered on the existing `documents` parent in documents.go.

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// pwResolveDocumentID maps any document reference the read path accepts (UUID,
// bare slugId, URL slug, or full document URL) to the UUID the lifecycle
// mutations require. A reference that already normalizes to a UUID costs no
// round trip.
func pwResolveDocumentID(c graphqlQueryer, ref string) (string, error) {
	if idOrSlug := normalizeDocumentRef(ref); store.IsUUID(idOrSlug) {
		return idOrSlug, nil
	}
	raw, err := fetchDocumentLive(c, ref)
	if err != nil {
		return "", err
	}
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("parsing document %q: %w", ref, err)
	}
	if doc.ID == "" {
		return "", fmt.Errorf("document %q did not include an id", ref)
	}
	return doc.ID, nil
}

func newDocumentsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <document-ref>",
		Short: "Delete a Linear document",
		Long: `Delete a document via the documentDelete mutation. Linear moves the document
to the trash rather than erasing it, so 'documents unarchive' brings it back.

The reference accepts every form the read path accepts: document UUID, bare
slugId, URL slug, or the full document URL. Deletion is confirmed interactively
unless --yes is passed, --agent implies --yes, and with --ignore-missing an
already-deleted document exits 0 as a no-op.`,
		Example: `  linear-pp-cli documents delete <document-uuid> --yes --agent
  linear-pp-cli documents delete my-runbook-f7f48ab36080 --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<document-ref> is required"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_document", "documentDelete", map[string]any{"document": args[0]})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			docID, err := pwResolveDocumentID(c, args[0])
			if err != nil {
				if flags != nil && flags.ignoreMissing && ExitCode(err) == 3 {
					return writeNoop(flags, "already_deleted", "already deleted (no-op)")
				}
				return classifyLiveReadError(err, flags)
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Delete document %s?", docID)); err != nil {
				return err
			}
			resp, err := c.Mutate(client.DocumentDeleteMutation, map[string]any{"id": docID})
			if err != nil {
				return classifyGraphQLMutationError("documentDelete", err, flags)
			}
			doc, err := extractMutationObject(resp, "documentDelete", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, doc, "documents")
		},
	}
	return cmd
}

func newDocumentsUnarchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unarchive <document-id>",
		Short: "Restore a deleted Linear document",
		Long: `Restore a trashed document via the documentUnarchive mutation, the counterpart
to 'documents delete'.

This verb takes a document UUID. A trashed document is not returned by the
slugId lookup the other document commands use, so there is nothing to resolve a
slug against.`,
		Example: `  linear-pp-cli documents unarchive <document-uuid> --agent
  linear-pp-cli documents unarchive <document-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<document-id> is required"))
			}
			docID := normalizeDocumentRef(args[0])
			if !store.IsUUID(docID) {
				return usageErr(fmt.Errorf("<document-id> expects a document UUID, got %q: a trashed document cannot be looked up by slugId", args[0]))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_unarchive_document", "documentUnarchive", map[string]any{"id": docID})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.DocumentUnarchiveMutation, map[string]any{"id": docID})
			if err != nil {
				return classifyGraphQLMutationError("documentUnarchive", err, flags)
			}
			doc, err := extractMutationObject(resp, "documentUnarchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, doc, "documents")
		},
	}
	return cmd
}
