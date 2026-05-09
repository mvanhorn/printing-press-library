// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/source/fjudkm"
)

// newJudicialKnowledgeCmd builds the FJUDKM (Judicial Knowledge Base) parent
// command with topics/topic/get/search subcommands plus the cross-source
// `link` novel feature.
func newJudicialKnowledgeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Browse the Judicial Knowledge Base (fjudkm.judicial.gov.tw)",
		Long: `462 hierarchical topics of curated case-law commentary spanning civil,
criminal, administrative, family, IP, and commercial law.`,
	}
	cmd.AddCommand(newKnowledgeTopicsReal(flags))
	cmd.AddCommand(newKnowledgeTopicReal(flags))
	cmd.AddCommand(newKnowledgeGetReal(flags))
	cmd.AddCommand(newKnowledgeSearchReal(flags))
	cmd.AddCommand(newKnowledgeLinkCmd(flags))
	return cmd
}

func newKnowledgeTopicsReal(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "topics",
		Short:       "List all 462 Judicial Knowledge Base topics",
		Example:     `  judgementtw-pp-cli knowledge topics --json --select id,title`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := fjudkmClient(flags)
			topics, err := c.Topics(cmd.Context())
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), topics, flags)
		},
	}
	return cmd
}

func newKnowledgeTopicReal(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "topic [id]",
		Short:       "Fetch a single topic with its case-commentary list",
		Example:     `  judgementtw-pp-cli knowledge topic 474 --json   # 法律行為(民法第71條~第98條)`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := 0
			_, _ = fmtSscanInt(args[0], &id)
			if id == 0 {
				return usageErr(errMsg("topic id must be a positive integer (e.g. 474)"))
			}
			c := fjudkmClient(flags)
			t, err := c.Topic(cmd.Context(), id)
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), t, flags)
		},
	}
	return cmd
}

func newKnowledgeGetReal(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get [par]",
		Short:       "Fetch a single Knowledge Base case commentary by its par-token",
		Example:     `  judgementtw-pp-cli knowledge get H4sF6HdN%2fbyjjMYJ42ZPATLh%2fu2Al%2f83pT2w0OTOytP6IvrcKVCjLQ%3d%3d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c := fjudkmClient(flags)
			d, err := c.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), d, flags)
		},
	}
	return cmd
}

func newKnowledgeSearchReal(flags *rootFlags) *cobra.Command {
	var court, caseChar string
	var no, limit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Full-text search the Judicial Knowledge Base",
		Example:     `  judgementtw-pp-cli knowledge search 不當得利 --limit 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c := fjudkmClient(flags)
			res, err := c.Search(cmd.Context(), fjudkm.SearchParams{
				Query:    args[0],
				Court:    court,
				CaseChar: caseChar,
				No:       no,
				Limit:    limit,
			})
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&court, "court", "", "Limit to a single court")
	cmd.Flags().StringVar(&caseChar, "case-char", "", "Filter by 字別")
	cmd.Flags().IntVar(&no, "no", 0, "Filter by case number")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

// errMsg is a tiny helper to wrap a string in an error without importing fmt
// at the call site.
type stringErr string

func (s stringErr) Error() string { return string(s) }
func errMsg(s string) error       { return stringErr(s) }

// fmtSscanInt is a thin Sscanf wrapper isolated for testability.
func fmtSscanInt(s string, dst *int) (int, error) {
	return fmtSscanInt2(s, dst)
}
