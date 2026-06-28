// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/acris/internal/cliutil"

	"github.com/spf13/cobra"
)

// partyMatch is one party record matching a substring name search.
type partyMatch struct {
	DocumentID string `json:"document_id"`
	PartyType  string `json:"party_type"`
	Name       string `json:"name"`
	Address1   string `json:"address_1,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	Zip        string `json:"zip,omitempty"`
}

// partySearchView is the result of a party substring search.
type partySearchView struct {
	Query   string       `json:"query"`
	Count   int          `json:"count"`
	Matches []partyMatch `json:"matches"`
	Note    string       `json:"note,omitempty"`
}

func newNovelPartySearchCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "party-search",
		Short: "Find recorded documents by partial party name (grantor, grantee, mortgagor, mortgagee).",
		Long: "Search the ACRIS Parties dataset for a partial (substring) party name and return\n" +
			"the matching parties and the document IDs they appear on. The raw dataset only\n" +
			"supports exact equality or full-text $q; this builds a case-insensitive\n" +
			"upper(name) LIKE filter for reliable substring lookups.",
		Example:     "  acris-pp-cli party-search --name MADISON --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search recorded parties by partial name")
				return nil
			}
			if strings.TrimSpace(flagName) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--name is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			if cliutil.IsDogfoodEnv() && limit > 10 {
				limit = 10
			}

			needle := strings.ToUpper(strings.TrimSpace(flagName))
			// Socrata SoQL LIKE uses backslash as its default escape character and
			// rejects an explicit `ESCAPE` clause, so soqlLikePattern backslash-escapes
			// %, _ and \ and we pass the pattern with no ESCAPE clause.
			where := fmt.Sprintf("upper(name) like %s", soqlLikePattern(needle))
			rows, err := fetchACRISRows(ctx, c, acrisPartiesPath, map[string]string{
				"$where": where,
				"$limit": strconv.Itoa(limit),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := partySearchView{Query: flagName, Matches: []partyMatch{}}
			for _, r := range rows {
				view.Matches = append(view.Matches, partyMatch{
					DocumentID: strField(r, "document_id"),
					PartyType:  strField(r, "party_type"),
					Name:       strField(r, "name"),
					Address1:   strField(r, "address_1"),
					City:       strField(r, "city"),
					State:      strField(r, "state"),
					Zip:        strField(r, "zip"),
				})
			}
			view.Count = len(view.Matches)
			if view.Count == 0 {
				view.Note = "no parties matched; names are recorded in uppercase and may be abbreviated"
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if view.Count == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no parties matched %q\n", flagName)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d parties matching %q\n", view.Count, flagName)
				humanRows := make([]map[string]any, 0, len(view.Matches))
				for _, m := range view.Matches {
					humanRows = append(humanRows, map[string]any{
						"name":        m.Name,
						"party_type":  m.PartyType,
						"document_id": m.DocumentID,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), humanRows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagName, "name", "", "Partial party name to search for (case-insensitive substring)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum matching parties to return")
	return cmd
}
