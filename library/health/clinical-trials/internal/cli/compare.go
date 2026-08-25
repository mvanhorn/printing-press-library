// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// compare.go implements the `compare` command: a head-to-head readout of two
// drugs across ClinicalTrials.gov. Each drug is first resolved through RxNorm
// (source.Normalize) so brand and generic names ("Keytruda" / "pembrolizumab")
// fold into one intervention query, then counted and sampled to report trial
// volume, recruiting share, phase mix, and the most active sponsors.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/source"

	"github.com/spf13/cobra"
)

// drugProfile is the per-drug side of a comparison.
type drugProfile struct {
	Query             string        `json:"query"`
	NormalizedName    string        `json:"normalized_name"`
	RxCUI             string        `json:"rxcui,omitempty"`
	Resolved          bool          `json:"resolved"`
	SearchTerms       []string      `json:"search_terms"`
	TotalTrials       int           `json:"total_trials"`
	RecruitingTrials  int           `json:"recruiting_trials"`
	RecruitingPct     int           `json:"recruiting_pct"`
	SampleSize        int           `json:"sample_size"`
	PhaseDistribution []rankedEntry `json:"phase_distribution,omitempty"`
	TopSponsors       []rankedEntry `json:"top_sponsors,omitempty"`
}

// compareView is the JSON shape `compare` emits.
type compareView struct {
	DrugA *drugProfile `json:"drug_a"`
	DrugB *drugProfile `json:"drug_b"`
	Note  string       `json:"note,omitempty"`
}

// pp:data-source live
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var sample, maxScanPages, topSponsors, maxTerms int
	cmd := &cobra.Command{
		Use:   "compare <drug-a> <drug-b>",
		Short: "Compare two drugs head-to-head across trial counts, phases, sponsors, and recruiting status.",
		Long: "Resolve two drug names through RxNorm (so brand and generic names fold together),\n" +
			"then compare them across ClinicalTrials.gov: total trial volume, active-recruiting\n" +
			"share, phase distribution, and the most active sponsors for each.",
		Example: "  clinical-trials-pp-cli compare Keytruda Opdivo --json\n" +
			"  clinical-trials-pp-cli compare pembrolizumab nivolumab --sample 400",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare two drugs across ClinicalTrials.gov")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			ctgov, err := flags.newClient()
			if err != nil {
				return err
			}
			src := flags.sourceClient()

			// A rate-limit/throttle from any leg is fatal; an unresolved drug
			// name is a soft miss handled inside buildDrugProfile.
			a, err := buildDrugProfile(ctx, ctgov, src, args[0], sample, maxScanPages, topSponsors, maxTerms)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			b, err := buildDrugProfile(ctx, ctgov, src, args[1], sample, maxScanPages, topSponsors, maxTerms)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := compareView{DrugA: a, DrugB: b}
			switch {
			case a.TotalTrials == 0 && b.TotalTrials == 0:
				view.Note = "neither drug matched any trials; check spelling or try the generic name"
			case !a.Resolved || !b.Resolved:
				view.Note = "one or both names could not be resolved via RxNorm; searched the literal term(s)"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderCompareHuman(cmd, view)
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 300, "Max trials per drug to sample for phase/sponsor breakdown")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 2, "Max ClinicalTrials.gov pages to scan per drug")
	cmd.Flags().IntVar(&topSponsors, "top-sponsors", 5, "Number of top sponsors to list per drug")
	cmd.Flags().IntVar(&maxTerms, "max-terms", 8, "Max RxNorm synonyms to fold into each drug's query")
	return cmd
}

// buildDrugProfile resolves one drug and computes its comparison metrics.
func buildDrugProfile(ctx context.Context, ctgov ctgovClient, src *source.Client, query string, sample, maxPages, topSponsors, maxTerms int) (*drugProfile, error) {
	drug, err := source.Normalize(ctx, src, query)
	if err != nil {
		return nil, err
	}
	terms := drug.SearchTerms()
	if maxTerms > 0 && len(terms) > maxTerms {
		terms = terms[:maxTerms]
	}
	q := buildInterventionQuery(terms)

	p := &drugProfile{
		Query:          query,
		NormalizedName: drug.Name,
		RxCUI:          drug.RxCUI,
		Resolved:       drug.Resolved,
		SearchTerms:    terms,
	}

	total, err := ctgovCount(ctx, ctgov, ctgovParams("intr", q))
	if err != nil {
		return nil, err
	}
	p.TotalTrials = total

	recParams := ctgovParams("intr", q)
	recParams["filter.overallStatus"] = "RECRUITING"
	recruiting, err := ctgovCount(ctx, ctgov, recParams)
	if err != nil {
		return nil, err
	}
	p.RecruitingTrials = recruiting
	p.RecruitingPct = percentOf(recruiting, total)

	trials, err := ctgovFetch(ctx, ctgov, ctgovParams("intr", q), sample, maxPages)
	if err != nil {
		return nil, err
	}
	p.SampleSize = len(trials)

	phase := newCounter()
	sponsor := newCounter()
	for _, t := range trials {
		for _, ph := range t.Phases {
			phase.add(phaseLabel(ph))
		}
		sponsor.add(t.Sponsor)
	}
	p.PhaseDistribution = phase.top(8)
	p.TopSponsors = sponsor.top(topSponsors)
	return p, nil
}

// buildInterventionQuery joins resolved drug names into a single Essie
// intervention query: "(Keytruda OR pembrolizumab)". Multi-word terms are
// quoted so CT.gov treats them as a phrase.
func buildInterventionQuery(terms []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		if strings.ContainsAny(t, " \t") {
			t = `"` + t + `"`
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, " OR ")
}

func renderCompareHuman(cmd *cobra.Command, v compareView) error {
	w := cmd.OutOrStdout()
	a, b := v.DrugA, v.DrugB
	labelA := compareLabel(a)
	labelB := compareLabel(b)

	fmt.Fprintf(w, "Head-to-head: %s vs %s\n\n", labelA, labelB)
	fmt.Fprintf(w, "%-28s %18s %18s\n", "", truncate(labelA, 18), truncate(labelB, 18))
	fmt.Fprintf(w, "%-28s %18d %18d\n", "Total trials", a.TotalTrials, b.TotalTrials)
	fmt.Fprintf(w, "%-28s %17d%% %17d%%\n", "Recruiting share", a.RecruitingPct, b.RecruitingPct)
	fmt.Fprintf(w, "%-28s %18d %18d\n", "Recruiting trials", a.RecruitingTrials, b.RecruitingTrials)
	fmt.Fprintf(w, "%-28s %18d %18d\n\n", "Sample analyzed", a.SampleSize, b.SampleSize)

	renderProfileDetail(w, a)
	renderProfileDetail(w, b)

	if v.Note != "" {
		fmt.Fprintf(w, "note: %s\n", v.Note)
	}
	return nil
}

func renderProfileDetail(w io.Writer, p *drugProfile) {
	fmt.Fprintf(w, "%s — phase distribution (sample):\n", compareLabel(p))
	if len(p.PhaseDistribution) == 0 {
		fmt.Fprintln(w, "  (no sampled trials)")
	}
	for _, ph := range p.PhaseDistribution {
		fmt.Fprintf(w, "  %-14s %d\n", ph.Label, ph.Count)
	}
	if len(p.TopSponsors) > 0 {
		fmt.Fprintf(w, "%s — top sponsors:\n", compareLabel(p))
		for _, s := range p.TopSponsors {
			fmt.Fprintf(w, "  %-40s %d\n", truncate(s.Label, 40), s.Count)
		}
	}
	fmt.Fprintln(w)
}

// compareLabel prefers the RxNorm-normalized name, falling back to the raw query.
func compareLabel(p *drugProfile) string {
	if p == nil {
		return "?"
	}
	if p.NormalizedName != "" {
		return p.NormalizedName
	}
	return p.Query
}
