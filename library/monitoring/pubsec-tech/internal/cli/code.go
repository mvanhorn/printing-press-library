// Code resolve command — anti-hallucination NAICS/PSC guard.
//
// Transcendence feature 7: refuses to return a code unless it matches a real
// row in the local naics_codes / psc_codes tables. On miss, returns the top-K
// nearest matches and exits non-zero (notFoundErr code 3) rather than guessing.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newCodeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code",
		Short: "NAICS and PSC code lookup with anti-hallucination guards",
		Long: "Resolves a plain term (e.g. \"cloud\", \"cybersecurity\") to a real NAICS or " +
			"PSC code. If the term doesn't match anything in the local reference table, the " +
			"command returns suggestions and exits non-zero — it refuses to guess so an agent " +
			"won't silently query downstream APIs with an invented code.",
		Example: "  pubsec-tech-pp-cli code resolve \"cloud\" --kind naics --agent",
	}
	cmd.AddCommand(newCodeResolveCmd(flags))
	cmd.AddCommand(newCodeListCmd(flags))
	return cmd
}

func newCodeResolveCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	cmd := &cobra.Command{
		Use:   "resolve <term-or-code>",
		Short: "Resolve a term or code to canonical NAICS/PSC entries; refuses to guess",
		Long: "If <term-or-code> matches a code exactly (e.g. \"541512\"), returns that one " +
			"row. Otherwise searches the title field and returns the top N nearest matches. " +
			"On zero matches, exits with code 3 (not found) so callers can detect the miss " +
			"rather than silently producing an empty downstream query.",
		Example:     "  pubsec-tech-pp-cli code resolve \"computer systems design\" --kind naics\n  pubsec-tech-pp-cli code resolve D310 --kind psc --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureCodesSeeded(ctx, s); err != nil {
				return err
			}
			k := "naics"
			if kind == "psc" {
				k = "psc"
			}
			results, err := s.LookupCode(ctx, k, args[0], limit)
			if err != nil {
				return err
			}
			type row struct {
				Code     string `json:"code"`
				Title    string `json:"title"`
				Kind     string `json:"kind"`
				Category string `json:"category,omitempty"`
				Parent   string `json:"parent_code,omitempty"`
				Depth    int    `json:"depth,omitempty"`
			}
			rows := make([]row, 0, len(results))
			for _, r := range results {
				rows = append(rows, row{Code: r.Code, Title: r.Title, Kind: k, Category: r.Category, Parent: r.Parent, Depth: r.Depth})
			}
			if len(rows) == 0 {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"matches": []row{},
						"kind":    k,
						"term":    args[0],
						"reason":  "no matches; refusing to guess",
					})
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "No %s code matches %q. Try a broader term or list the catalog with `code list --kind %s`.\n", k, args[0], k)
				}
				return notFoundErr(fmt.Errorf("no %s match for %q", k, args[0]))
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CODE\tTITLE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\n", r.Code, r.Title)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "naics", "Code kind: naics or psc")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum matches to return when searching by title")
	return cmd
}

func newCodeListCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var minDepth int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List every NAICS or PSC code in the local reference table",
		Example:     "  pubsec-tech-pp-cli code list --kind naics --json\n  pubsec-tech-pp-cli code list --kind naics --depth 6",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureCodesSeeded(ctx, s); err != nil {
				return err
			}
			k := "naics"
			if kind == "psc" {
				k = "psc"
			}
			results, err := s.ListCodes(ctx, k, minDepth)
			if err != nil {
				return err
			}
			type row struct {
				Code  string `json:"code"`
				Title string `json:"title"`
				Depth int    `json:"depth,omitempty"`
			}
			rows := make([]row, 0, len(results))
			for _, r := range results {
				rows = append(rows, row{Code: r.Code, Title: r.Title, Depth: r.Depth})
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CODE\tTITLE")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\n", r.Code, r.Title)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "naics", "Code kind: naics or psc")
	cmd.Flags().IntVar(&minDepth, "depth", 0, "(NAICS only) Minimum depth (2=sector, 6=full code)")
	return cmd
}
