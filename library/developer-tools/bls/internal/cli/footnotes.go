package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bls/internal/blsdata"

	"github.com/spf13/cobra"
)

// FootnoteDecodeRow is one decoded footnote.
type FootnoteDecodeRow struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

func newFootnotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "footnotes",
		Short: "Decode BLS footnote codes that ride along on observations (P, R, C, ...).",
	}
	cmd.AddCommand(newFootnotesDecodeCmd(flags))
	cmd.AddCommand(newFootnotesListCmd(flags))
	return cmd
}

func newFootnotesDecodeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decode <code>...",
		Short: "Decode one or more BLS footnote codes into plain-English text.",
		Long: `BLS observations carry footnote codes (single letters like P for preliminary, R
for revised, C for corrected) but the live API never returns the explanations.
This command joins the codes against the local footnote table.

Pass codes as separate arguments; unknown codes are reported with empty text so
you can spot newly-added BLS codes that need to be added to the local table.`,
		Example: `  bls-pp-cli footnotes decode P
  bls-pp-cli footnotes decode P R C --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rows := make([]FootnoteDecodeRow, 0, len(args))
			knownCount := 0
			for _, a := range args {
				code := strings.TrimSpace(a)
				text := blsdata.DecodeFootnote(code)
				if text != "" {
					knownCount++
				}
				rows = append(rows, FootnoteDecodeRow{Code: code, Text: text})
			}
			if knownCount == 0 {
				return usageErr(fmt.Errorf("none of the supplied codes are known BLS footnote codes; run `bls-pp-cli footnotes list` to see all codes"))
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(rows)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m = append(m, map[string]any{"code": r.Code, "text": r.Text})
				}
				return printAutoTable(cmd.OutOrStdout(), m)
			}
			for _, r := range rows {
				if r.Text == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t(unknown footnote code)\n", r.Code)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", r.Code, r.Text)
				}
			}
			return nil
		},
	}
	return cmd
}

func newFootnotesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every known BLS footnote code with its plain-English text.",
		Example: `  bls-pp-cli footnotes list
  bls-pp-cli footnotes list --json
  bls-pp-cli footnotes list --csv > footnotes.csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fns := blsdata.Footnotes()
			rows := make([]FootnoteDecodeRow, 0, len(fns))
			for _, f := range fns {
				rows = append(rows, FootnoteDecodeRow{Code: f.Code, Text: f.Text})
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(rows)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					m = append(m, map[string]any{"code": r.Code, "text": r.Text})
				}
				return printAutoTable(cmd.OutOrStdout(), m)
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", r.Code, r.Text)
			}
			return nil
		},
	}
	return cmd
}
