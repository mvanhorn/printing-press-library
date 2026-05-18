package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// tmDocument represents a document in the trademark prosecution file.
type tmDocument struct {
	DocID       string `json:"docId"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	Description string `json:"description"`
	PageCount   int    `json:"pageCount,omitempty"`
}

func newTrademarkDocsCmd(flags *rootFlags) *cobra.Command {
	var filterType string

	cmd := &cobra.Command{
		Use:   "docs <serialNumber>",
		Short: "List all documents in the prosecution file",
		Long: `Lists every document filed or issued for a trademark application:
office actions, responses, specimens, registration certificates,
and more. Use --filter-type to narrow by document type code.`,
		Example: strings.Trim(`
  uspto-tsdr-pp-cli trademark docs 97123456
  uspto-tsdr-pp-cli trademark docs 97123456 --json
  uspto-tsdr-pp-cli trademark docs 97123456 --filter-type SPE`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			serial := args[0]
			caseID := normalizeCaseID(serial)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// PATCH: the /casedocs/{caseid}/info endpoint returns XML only
			// (HTTP 406 for Accept: application/json). Try JSON first; on
			// 406 or XML response, print a clear message instead of empty output.
			path := replacePathParam("/casedocs/{caseid}/info", "caseid", caseID)
			data, err := c.GetJSON(path, nil)
			if err != nil {
				// Check if this is a 406 Not Acceptable (endpoint doesn't support JSON)
				var apiErr interface{ Error() string }
				if errors.As(err, &apiErr) && strings.Contains(apiErr.Error(), "406") {
					fmt.Fprintf(cmd.ErrOrStderr(), "The TSDR document listing endpoint (/casedocs/%s/info) only supports XML responses.\n", caseID)
					fmt.Fprintf(cmd.ErrOrStderr(), "Document listing via JSON is not yet supported. Use the USPTO TSDR web interface instead:\n")
					fmt.Fprintf(cmd.ErrOrStderr(), "  https://tsdr.uspto.gov/#caseNumber=%s&caseSearchType=US_APPLICATION&caseType=DEFAULT&searchType=statusSearch\n", serial)
					return nil
				}
				return classifyAPIError(err, flags)
			}

			// Guard: if response starts with XML, it slipped through
			if len(data) > 0 && (data[0] == '<' || (len(data) > 5 && string(data[:5]) == "<?xml")) {
				fmt.Fprintf(cmd.ErrOrStderr(), "The TSDR document listing endpoint returned XML (not JSON).\n")
				fmt.Fprintf(cmd.ErrOrStderr(), "Document listing via JSON is not yet supported. Use the USPTO TSDR web interface instead:\n")
				fmt.Fprintf(cmd.ErrOrStderr(), "  https://tsdr.uspto.gov/#caseNumber=%s&caseSearchType=US_APPLICATION&caseType=DEFAULT&searchType=statusSearch\n", serial)
				return nil
			}

			docs := parseTMDocuments(data)

			// Apply type filter
			if filterType != "" {
				upper := strings.ToUpper(filterType)
				var filtered []tmDocument
				for _, d := range docs {
					if strings.Contains(strings.ToUpper(d.Type), upper) {
						filtered = append(filtered, d)
					}
				}
				docs = filtered
			}

			// Sort by date descending (most recent first)
			sort.Slice(docs, func(i, j int) bool {
				return docs[i].Date > docs[j].Date
			})

			if len(docs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no documents found for %s\n", serial)
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), docs, flags)
			}

			// Table output
			headers2 := []string{"Date", "Type", "Description", "Pages"}
			rows := make([][]string, len(docs))
			for i, d := range docs {
				pages := ""
				if d.PageCount > 0 {
					pages = fmt.Sprintf("%d", d.PageCount)
				}
				rows[i] = []string{d.Date, d.Type, truncate(d.Description, 50), pages}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Documents for %s (%d total)\n\n", serial, len(docs))
			return flags.printTable(cmd, headers2, rows)
		},
	}
	cmd.Flags().StringVar(&filterType, "filter-type", "", "Filter documents by type code (e.g., SPE for specimens)")
	return cmd
}

func parseTMDocuments(data json.RawMessage) []tmDocument {
	var docs []tmDocument

	// Try as array of objects
	var items []map[string]interface{}
	if json.Unmarshal(data, &items) == nil && len(items) > 0 {
		return extractDocItems(items)
	}

	// Try as object with nested arrays
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return docs
	}

	// Look for document bag
	for _, key := range []string{"DocumentBag", "documentBag", "documents", "Documents",
		"casedocs", "results", "data", "items"} {
		if raw, ok := root[key]; ok {
			if json.Unmarshal(raw, &items) == nil && len(items) > 0 {
				return extractDocItems(items)
			}
		}
	}

	return docs
}

func extractDocItems(items []map[string]interface{}) []tmDocument {
	var docs []tmDocument
	for _, item := range items {
		doc := tmDocument{}
		doc.DocID = extractStringField(item, "DocumentIdentifier", "documentIdentifier",
			"DocId", "docId", "DocumentID", "documentID", "id")
		doc.Date = trimDate(extractStringField(item, "DocumentDate", "documentDate",
			"MailRoomDate", "mailRoomDate",
			"Date", "date", "CreateDate", "createDate"))
		doc.Type = extractStringField(item, "DocumentTypeCode", "documentTypeCode",
			"TypeCode", "typeCode", "DocumentType", "documentType",
			"Type", "type", "Category", "category")
		doc.Description = extractStringField(item, "DocumentTypeDescriptionText", "documentTypeDescriptionText",
			"DocumentDescription", "documentDescription",
			"Description", "description", "Title", "title")

		// Page count
		if v, ok := item["PageCount"]; ok {
			if f, ok := v.(float64); ok {
				doc.PageCount = int(f)
			}
		}
		if v, ok := item["pageCount"]; ok {
			if f, ok := v.(float64); ok {
				doc.PageCount = int(f)
			}
		}

		if doc.DocID != "" || doc.Date != "" || doc.Description != "" {
			docs = append(docs, doc)
		}
	}
	return docs
}
