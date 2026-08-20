// Hand-authored novel command: controversy detector. Not generated.
package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type controversyOutput struct {
	Query            string      `json:"query"`
	StudyCount       int         `json:"study_count"`
	Supporting       int         `json:"supporting"`
	Refuting         int         `json:"refuting"`
	Mixed            int         `json:"mixed"`
	ControversyScore float64     `json:"controversy_score"` // 0 (settled) .. 1 (highly contested)
	Contested        bool        `json:"contested"`
	SupportingSide   []workBrief `json:"supporting_side"`
	RefutingSide     []workBrief `json:"refuting_side"`
	Note             string      `json:"note,omitempty"`
}

func newNovelControversiesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "controversies <query>",
		Short: "Surface conflicting studies and contradictory conclusions for a topic",
		Long: "Classify the stance of each retrieved study and measure how contested the topic\n" +
			"is. A high controversy score means supporting and refuting evidence are both\n" +
			"substantial. Use this to answer 'is this settled or disputed?'",
		Example:     "  scientific-consensus-pp-cli controversies \"saturated fat heart disease\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would detect controversies for the topic")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			works, _, err := fetchWorks(ctx, c, query, "", "relevance_score:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if enrich {
				enrichPubTypes(ctx, works, 50)
			}
			prog := newProgress(flags, "analyzing works", len(works))
			prog.update(len(works))
			_, stances := scoreWorks(ctx, works, query)
			prog.done()

			out := controversyOutput{Query: query, StudyCount: len(works)}
			for _, s := range stances {
				switch s.Stance {
				case scengine.StanceSupporting:
					out.Supporting++
				case scengine.StanceRefuting:
					out.Refuting++
				case scengine.StanceMixed:
					out.Mixed++
				}
			}
			directional := out.Supporting + out.Refuting
			if directional > 0 {
				minor := out.Supporting
				if out.Refuting < minor {
					minor = out.Refuting
				}
				// 2*minority/total => 1.0 when evenly split, 0 when one-sided.
				out.ControversyScore = round2f(2 * float64(minor) / float64(directional))
			}
			out.Contested = out.ControversyScore >= 0.5 && directional >= 4
			out.SupportingSide = topByStance(stances, scengine.StanceSupporting, 3)
			out.RefutingSide = topByStance(stances, scengine.StanceRefuting, 3)
			if len(works) == 0 {
				out.Note = "no works found; try a broader query"
			} else if directional < 4 {
				out.Note = "too few directional studies to assess controversy reliably"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderControversy(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "number of works to analyze (max 200)")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich classification with PubMed publication types")
	return cmd
}

func renderControversy(w io.Writer, o controversyOutput) {
	fmt.Fprintf(w, "Controversy analysis: %s\n\n", o.Query)
	verdict := "settled / one-sided"
	if o.Contested {
		verdict = "CONTESTED"
	}
	fmt.Fprintf(w, "  Controversy score: %.2f  (%s)\n", o.ControversyScore, verdict)
	fmt.Fprintf(w, "  Studies: %d  (support %d / refute %d / mixed %d)\n\n", o.StudyCount, o.Supporting, o.Refuting, o.Mixed)
	if len(o.SupportingSide) > 0 {
		fmt.Fprintln(w, "  Supporting side:")
		for _, b := range o.SupportingSide {
			fmt.Fprintf(w, "    • [%d, cites=%d] %s\n", b.Year, b.CitedBy, truncate(b.Title, 78))
		}
	}
	if len(o.RefutingSide) > 0 {
		fmt.Fprintln(w, "  Refuting side:")
		for _, b := range o.RefutingSide {
			fmt.Fprintf(w, "    • [%d, cites=%d] %s\n", b.Year, b.CitedBy, truncate(b.Title, 78))
		}
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}

func round2f(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
