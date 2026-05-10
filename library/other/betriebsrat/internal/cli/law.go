package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

func newLawCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "law [paragraph]",
		Short: "Look up a BetrVG paragraph with plain-language explanation",
		Long: `Returns a plain-language explanation of a BetrVG paragraph,
including co-determination classification, topic link, and deadlines.

Use the paragraph number (e.g. 87, 102, 111) or a keyword search.`,
		Example: strings.Trim(`
  betriebsrat law 87
  betriebsrat law 102 --json
  betriebsrat law kündigung
  betriebsrat law 99 --agent`, "\n"),
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

			query := strings.Join(args, " ")
			var paragraphs []betrvg.Paragraph

			// Try numeric lookup first
			if n, err := strconv.Atoi(strings.TrimPrefix(query, "§")); err == nil {
				if p := betrvg.ByNumber(n); p != nil {
					paragraphs = []betrvg.Paragraph{*p}
				}
			}

			// Fall back to keyword search
			if len(paragraphs) == 0 {
				paragraphs = betrvg.ByKeywords(tokenize(query))
			}

			if len(paragraphs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "§ %s nicht in der Datenbank.\n\nVerfügbare Paragrafen: %s\n",
					query, listParagraphNumbers())
				return nil
			}

			type outParagraph struct {
				Number       int    `json:"paragraph"`
				Title        string `json:"title"`
				Summary      string `json:"summary"`
				CoDetermType string `json:"codetermination_type"`
				DeadlineDays int    `json:"deadline_days,omitempty"`
				TopicSlug    string `json:"topic_slug"`
				TopicURL     string `json:"topic_url"`
			}

			var out []outParagraph
			for _, p := range paragraphs {
				out = append(out, outParagraph{
					Number:       p.Number,
					Title:        p.Title,
					Summary:      p.Summary,
					CoDetermType: string(p.CoDetermType),
					DeadlineDays: p.DeadlineDays,
					TopicSlug:    p.TopicSlug,
					TopicURL:     p.TopicURL,
				})
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if len(out) == 1 {
					return enc.Encode(out[0])
				}
				return enc.Encode(out)
			}

			for _, p := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "§ %d BetrVG – %s\n\n", p.Number, p.Title)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", p.Summary)
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Mitbestimmungsart", "Co-determination type"), p.CoDetermType)
				if p.DeadlineDays > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %d %s\n", tr(flags.lang, "Frist", "Deadline"), p.DeadlineDays, tr(flags.lang, "Tage", "days"))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n\n", tr(flags.lang, "Mehr Infos", "More info"), p.TopicURL)
			}
			return nil
		},
	}
	return cmd
}

func listParagraphNumbers() string {
	var nums []string
	for _, p := range betrvg.All() {
		nums = append(nums, fmt.Sprintf("%d", p.Number))
	}
	return strings.Join(nums, ", ")
}
