// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"

	"github.com/spf13/cobra"
)

func newNovelConformanceCmd(flags *rootFlags) *cobra.Command {
	var flagFramework string

	cmd := &cobra.Command{
		Use:   "conformance <protocol>",
		Short: "Enumerate concrete conformers and ancestor protocols from DocC relationshipsSections",
		Long: strings.TrimSpace(`
For a protocol, return the concrete types that conform to it and the protocols
it itself inherits from, by parsing the relationshipsSections block on the
protocol's DocC page.

If only the protocol slug is given (e.g. 'View'), --framework must be set so
the path can be resolved (e.g. swiftui/view).

The result is a structured list — kimsungwhee's get_related_apis returns
untyped See-Also; this returns typed conformsTo / inheritsFrom rows.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli conformance View --framework swiftui --agent
  apple-docs-pp-cli conformance swiftui/animatable --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch protocol page and walk relationshipsSections")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<protocol> is required"))
			}
			arg := args[0]
			path := arg
			if !strings.Contains(arg, "/") {
				if flagFramework == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--framework is required when <protocol> is a bare slug"))
				}
				path = strings.ToLower(flagFramework) + "/" + strings.ToLower(arg)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := applejson.FetchDoc(cmd.Context(), c, path)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rel, err := extractRelationships(page.RelationshipsSections, page.References)
			if err != nil {
				return err
			}
			rel.Symbol = page.Title
			rel.SymbolURL = page.URL
			return emitJSON(cmd, flags, rel)
		},
	}
	cmd.Flags().StringVar(&flagFramework, "framework", "", "Framework slug; required when <protocol> is a bare name (e.g. 'View')")
	return cmd
}

type conformanceResult struct {
	Symbol          string            `json:"symbol"`
	SymbolURL       string            `json:"symbol_url,omitempty"`
	ConformsTo      []relationshipRow `json:"conforms_to,omitempty"`
	InheritsFrom    []relationshipRow `json:"inherits_from,omitempty"`
	ConformingTypes []relationshipRow `json:"conforming_types,omitempty"`
	Other           []relationshipRow `json:"other,omitempty"`
	Note            string            `json:"note,omitempty"`
}

type relationshipRow struct {
	Title string `json:"title"`
	Path  string `json:"path,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// extractRelationships reads the per-section structure directly from the
// applejson.DocPage's RelationshipsSections (populated during ParseDoc), so
// the parsed sections aren't re-Unmarshaled on every call.
func extractRelationships(sections []json.RawMessage, refs map[string]applejson.Reference) (*conformanceResult, error) {
	out := &conformanceResult{}
	for _, sec := range sections {
		var meta struct {
			Type        string   `json:"type"`
			Kind        string   `json:"kind"`
			Title       string   `json:"title"`
			Identifiers []string `json:"identifiers"`
		}
		if err := json.Unmarshal(sec, &meta); err != nil {
			return nil, fmt.Errorf("parsing relationshipsSections entry: %w", err)
		}
		bucket := meta.Title
		rows := make([]relationshipRow, 0, len(meta.Identifiers))
		for _, id := range meta.Identifiers {
			ref, ok := refs[id]
			if !ok {
				rows = append(rows, relationshipRow{Title: id})
				continue
			}
			rows = append(rows, relationshipRow{
				Title: ref.Title,
				Path:  ref.URL,
				Kind:  ref.Kind,
			})
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Title < rows[j].Title })
		switch {
		case meta.Type == "conformsTo", strings.EqualFold(bucket, "Conforms To"):
			out.ConformsTo = append(out.ConformsTo, rows...)
		case meta.Type == "inheritsFrom", strings.EqualFold(bucket, "Inherits From"):
			out.InheritsFrom = append(out.InheritsFrom, rows...)
		case meta.Type == "conformingTypes", strings.Contains(strings.ToLower(bucket), "conforming type"):
			out.ConformingTypes = append(out.ConformingTypes, rows...)
		default:
			out.Other = append(out.Other, rows...)
		}
	}
	if len(out.ConformsTo) == 0 && len(out.InheritsFrom) == 0 && len(out.ConformingTypes) == 0 && len(out.Other) == 0 {
		out.Note = "no relationshipsSections on this page; the symbol may not be a protocol/class, or DocC didn't expose typed relationships"
	}
	return out, nil
}
