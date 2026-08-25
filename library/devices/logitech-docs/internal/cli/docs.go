// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command — implemented from the absorb manifest transcendence row
// "Friendly doc-type search".
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// docTypeLabels maps friendly doc-type words to Logitech's webcontent label
// taxonomy on support.logi.com (a Zendesk Help Center).
var docTypeLabels = map[string]string{
	"spec": "webcontent=productspecs", "specs": "webcontent=productspecs",
	"specsheet": "webcontent=productspecs", "spec-sheet": "webcontent=productspecs",
	"manual": "webcontent=productdocument", "manuals": "webcontent=productdocument",
	"document": "webcontent=productdocument", "documents": "webcontent=productdocument",
	"install": "webcontent=productgettingstarted", "setup": "webcontent=productgettingstarted",
	"getting-started": "webcontent=productgettingstarted", "gettingstarted": "webcontent=productgettingstarted",
	"guide": "webcontent=productgettingstarted", "guides": "webcontent=productgettingstarted",
	"install-guide": "webcontent=productgettingstarted", "setup-guide": "webcontent=productgettingstarted",
	"faq": "webcontent=productfaq", "faqs": "webcontent=productfaq",
	"download": "webcontent=productdownload", "downloads": "webcontent=productdownload",
	"software": "webcontent=productdownload",
	"warranty": "webcontent=productwarranty",
	"video":    "webcontent=productvideo", "videos": "webcontent=productvideo",
}

// normalizeDocType folds the friendly spellings the help text advertises
// ("spec sheet", "install guide") onto the hyphenated keys in docTypeLabels.
func normalizeDocType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	return strings.Join(strings.Fields(s), "-")
}

// docHit is the flattened result row emitted by `docs`.
type docHit struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	DocType   string   `json:"doc_type"`
	Labels    []string `json:"label_names,omitempty"`
	HTMLURL   string   `json:"html_url"`
	SectionID int64    `json:"section_id"`
	UpdatedAt string   `json:"updated_at"`
}

// zendeskHit mirrors the Zendesk Help Center search result shape we read.
type zendeskHit struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Name      string   `json:"name"`
	HTMLURL   string   `json:"html_url"`
	SectionID int64    `json:"section_id"`
	UpdatedAt string   `json:"updated_at"`
	Labels    []string `json:"label_names"`
}

func newNovelDocsCmd(flags *rootFlags) *cobra.Command {
	var perPage int
	var page int
	var limit int
	var typeFlag string

	cmd := &cobra.Command{
		Use:   "docs <type> <query>",
		Short: "Search Logitech support docs by friendly type: spec sheet, manual, install guide, FAQ, download, warranty.",
		Long: "Search support.logi.com by friendly doc type. <type> is one of: spec(s), manual/document, install/setup, faq, download/software, warranty, video. " +
			"Give the type as the first argument or with --type; multi-word forms (\"spec sheet\", \"install guide\") are accepted. " +
			"The command maps the type to Logitech's internal webcontent label and searches the Help Center full-text index (titles and bodies).",
		Example: "  logitech-docs-pp-cli docs spec \"MeetUp\"\n" +
			"  logitech-docs-pp-cli docs manual \"MX Master 3S\" --json\n" +
			"  logitech-docs-pp-cli docs --type \"spec sheet\" \"MeetUp\"\n" +
			"  logitech-docs-pp-cli docs install \"Rally Bar\" --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "type=spec;query=MeetUp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docs search")
			}
			rawType, queryArgs := typeFlag, args
			if rawType == "" {
				if len(args) < 2 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("docs requires <type> and <query>, e.g. %s spec \"MeetUp\" (or %s --type spec \"MeetUp\")", cmd.CommandPath(), cmd.CommandPath()))
				}
				rawType, queryArgs = args[0], args[1:]
			} else if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("docs --type %s requires a <query>, e.g. %s --type %s \"MeetUp\"", rawType, cmd.CommandPath(), rawType))
			}
			docType := normalizeDocType(rawType)
			label, ok := docTypeLabels[docType]
			if !ok {
				return usageErr(fmt.Errorf("unknown doc type %q; want one of: spec, manual, install, faq, download, warranty, video", rawType))
			}
			query := strings.Join(queryArgs, " ")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"query":       query,
				"label_names": label,
				"per_page":    fmt.Sprintf("%d", perPage),
				"page":        fmt.Sprintf("%d", page),
			}
			data, err := c.Get(ctx, "/api/v2/help_center/articles/search.json", params)
			if err != nil {
				return apiErr(fmt.Errorf("searching docs: %w", err))
			}
			var envelope struct {
				Results []json.RawMessage `json:"results"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return apiErr(fmt.Errorf("parsing search response: %w", err))
			}

			results := make([]docHit, 0, len(envelope.Results))
			for _, raw := range envelope.Results {
				var zh zendeskHit
				if err := json.Unmarshal(raw, &zh); err != nil {
					continue
				}
				title := zh.Title
				if title == "" {
					title = zh.Name
				}
				results = append(results, docHit{
					ID:        zh.ID,
					Title:     title,
					DocType:   docType,
					Labels:    zh.Labels,
					HTMLURL:   zh.HTMLURL,
					SectionID: zh.SectionID,
					UpdatedAt: zh.UpdatedAt,
				})
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), results, flags, "live")
			}
			rows := make([]map[string]any, 0, len(results))
			for _, r := range results {
				rows = append(rows, map[string]any{
					"id":         r.ID,
					"title":      r.Title,
					"type":       r.DocType,
					"updated_at": r.UpdatedAt,
					"html_url":   r.HTMLURL,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching documents found.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().IntVar(&perPage, "per-page", 25, "Results per page")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results to return (0 = all on the page)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "Doc type, as an alternative to the first positional argument (e.g. \"spec sheet\")")
	return cmd
}
