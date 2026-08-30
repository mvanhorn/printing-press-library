// Hand-authored novel command: reproducibility-signal estimator. Not generated.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type reproSignal struct {
	Signal string `json:"signal"`
	Works  int    `json:"works"`
}

type reproOutput struct {
	Query        string        `json:"query"`
	Analyzed     int           `json:"analyzed"`
	Signals      []reproSignal `json:"signals"`
	ReproScore   float64       `json:"reproducibility_score"` // 0..1, share of works with any signal
	Note         string        `json:"note,omitempty"`
}

var reproProbes = []struct {
	name    string
	pattern string
}{
	{"replication/confirmatory", `(?i)\b(replicat|reproduc|confirmatory (study|trial|analysis)|independent (cohort|validation|replication))\b`},
	{"pre-registration", `(?i)\b(pre[- ]?regist|registered report|clinicaltrials\.gov|prospero|osf\.io|trial registration|nct\d{6,})\b`},
	{"power/sample-size reporting", `(?i)\b(power (calculation|analysis)|powered to detect|sample size (calculation|of \d)|a priori sample)\b`},
	{"large sample/nationwide", `(?i)\b(nationwide|population-based|large (cohort|sample)|n\s?=\s?\d{4,}|\d{5,}\s+(patients|participants|subjects))\b`},
	{"data/code availability", `(?i)\b(data (are|is) available|code (is )?available|open data|data sharing|supplementary data)\b`},
}

func newNovelReproducibilityCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "reproducibility <query>",
		Short: "Estimate reproducibility signals: replication, pre-registration, power, data sharing",
		Long: "Scan the abstracts of retrieved works for reproducibility signals (replication\n" +
			"studies, pre-registration, power analyses, large samples, data/code availability)\n" +
			"and report how prevalent each is. Heuristic text signals, not a reproducibility audit.",
		Example:     "  scientific-consensus-pp-cli reproducibility \"power posing\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would estimate reproducibility signals")
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
			out := reproOutput{Query: query, Analyzed: len(works)}
			counts := make([]int, len(reproProbes))
			anySignal := 0
			prog := newProgress(flags, "scanning works", len(works))
			for wi, wk := range works {
				hay := strings.ToLower(wk.Title + ". " + wk.Abstract)
				hit := false
				for i, p := range reproProbes {
					if reMatch(p.pattern, hay) {
						counts[i]++
						hit = true
					}
				}
				if hit {
					anySignal++
				}
				prog.update(wi + 1)
			}
			prog.done()
			for i, p := range reproProbes {
				out.Signals = append(out.Signals, reproSignal{Signal: p.name, Works: counts[i]})
			}
			if len(works) > 0 {
				out.ReproScore = round2f(float64(anySignal) / float64(len(works)))
			}
			if len(works) == 0 {
				out.Note = "no works found; try a broader query"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderRepro(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "number of works to scan (max 200)")
	return cmd
}

func renderRepro(w io.Writer, o reproOutput) {
	fmt.Fprintf(w, "Reproducibility signals: %s\n", o.Query)
	fmt.Fprintf(w, "  %d works scanned; reproducibility score %.2f (share with any signal)\n\n", o.Analyzed, o.ReproScore)
	for _, s := range o.Signals {
		fmt.Fprintf(w, "  %-32s %3d\n", s.Signal, s.Works)
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
