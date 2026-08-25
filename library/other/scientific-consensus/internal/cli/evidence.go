// Hand-authored novel command: evidence engine + pyramid. Not generated.
package cli

import (
	"fmt"
	"io"
	"math"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type evidenceLevel struct {
	Design  scengine.Design `json:"design"`
	Count   int             `json:"count"`
	Pct     float64         `json:"pct"`
	Example string          `json:"example,omitempty"`
}

type evidenceOutput struct {
	Query           string          `json:"query"`
	StudyCount      int             `json:"study_count"`
	ApexDesign      scengine.Design `json:"apex_design"`
	ClassifiedByPub int             `json:"classified_by_pubtype"`
	Pyramid         []evidenceLevel `json:"pyramid"`
	Note            string          `json:"note,omitempty"`
}

func newNovelEvidenceCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var yearFrom int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "evidence <query>",
		Short: "Classify studies by design and render the evidence pyramid for a topic",
		Long: "Fetch works for a topic, classify each by study design (meta-analysis down to\n" +
			"case report), and render the evidence pyramid. Use this to judge whether a\n" +
			"claim rests on strong designs (RCTs, meta-analyses) or weak ones (case series).\n" +
			"Do NOT use for stance/verdict; use `consensus`.",
		Example:     "  scientific-consensus-pp-cli evidence \"intermittent fasting weight loss\" --csv",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would classify evidence for the topic")
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
			filter := ""
			if yearFrom > 0 {
				filter = fmt.Sprintf("from_publication_date:%d-01-01", yearFrom)
			}
			works, _, err := fetchWorks(ctx, c, query, filter, "relevance_score:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if enrich {
				enrichPubTypes(ctx, works, 50)
			}

			cs := make([]scengine.Classification, len(works))
			byPub := 0
			exemplar := map[scengine.Design]string{}
			prog := newProgress(flags, "classifying", len(works))
			for i, wk := range works {
				cls := scengine.ClassifyDesign(wk.Title, wk.Abstract, wk.Type, wk.PubTypes)
				cs[i] = cls
				if cls.Method == scengine.MethodPubMedPubType {
					byPub++
				}
				if _, ok := exemplar[cls.Design]; !ok {
					exemplar[cls.Design] = wk.Title
				}
				prog.update(i + 1)
			}
			prog.done()
			levels := scengine.Pyramid(cs)
			out := evidenceOutput{
				Query: query, StudyCount: len(works), ApexDesign: scengine.ApexDesign(cs),
				ClassifiedByPub: byPub,
			}
			for _, lvl := range levels {
				pct := 0.0
				if len(works) > 0 {
					pct = float64(lvl.Count) * 100 / float64(len(works))
				}
				out.Pyramid = append(out.Pyramid, evidenceLevel{
					Design: lvl.Design, Count: lvl.Count, Pct: round1(pct), Example: exemplar[lvl.Design],
				})
			}
			if len(works) == 0 {
				out.Note = "no works found; try a broader query or --data-source live"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderEvidence(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "number of works to classify (max 200)")
	cmd.Flags().IntVar(&yearFrom, "year-from", 0, "only include works published from this year onward")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich classification with PubMed publication types")
	return cmd
}

func renderEvidence(w io.Writer, o evidenceOutput) {
	fmt.Fprintf(w, "Evidence base for: %s\n", o.Query)
	fmt.Fprintf(w, "  %d studies classified (apex: %s; %d via PubMed pubtype)\n\n", o.StudyCount, o.ApexDesign, o.ClassifiedByPub)
	maxCount := 0
	for _, l := range o.Pyramid {
		if l.Count > maxCount {
			maxCount = l.Count
		}
	}
	for _, l := range o.Pyramid {
		if l.Count == 0 {
			continue
		}
		fmt.Fprintf(w, "  %-28s %3d (%4.1f%%) %s\n", l.Design, l.Count, l.Pct, bar(l.Count, maxCount, 24))
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
