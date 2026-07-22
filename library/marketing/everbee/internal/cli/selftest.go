// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

// Exit codes. `selftest` is the one command whose whole job is to be branched
// on by a caller, so its exits are typed and documented.
const (
	selftestExitDegraded = 3 // transport works, but the data is not trustworthy
	selftestExitFailed   = 4 // the research path is broken or refused
)

type selftestCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type selftestReport struct {
	Seed     string          `json:"seed"`
	Verdict  string          `json:"verdict"` // "ok" | "degraded" | "failed"
	Checks   []selftestCheck `json:"checks"`
	Relevant struct {
		Keywords string `json:"keywords"`
		Products string `json:"products"`
	} `json:"relevance"`
	Warnings []string `json:"warnings"`
}

func newNovelSelftestCmd(flags *rootFlags) *cobra.Command {
	var seed string

	cmd := &cobra.Command{
		Use:   "selftest",
		Short: "Check that the research path is not just reachable but actually returning relevant data.",
		Long: strings.Trim(`
Verify that the research path is semantically sound, not merely reachable.

An HTTP 200 proves the transport works. It does not prove the answer means
anything: EverBee's default suggestion feeds return 200 with unranked filler that
has nothing to do with the query, and mistaking that for evidence is exactly what
made the published CLI report confident nonsense.

This command runs a canonical seed through the live seeded endpoints and asserts
the properties that make an answer trustworthy: results come back, they are
actually about the seed, EverBee supplied the seed's own demand/competition
metrics, and the account has research quota left.

Exit codes:
  0  the research path is sound
  3  degraded — transport works but the data is not trustworthy
  4  failed — the research path is broken or the plan quota is exhausted

Run this first in any automated session.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli selftest --agent
  everbee-pp-cli selftest --seed "wedding sign" --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a canonical seed through the live research path and assert semantic validity")
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if strings.TrimSpace(seed) == "" {
				seed = "dad shirt"
			}
			rep := selftestReport{Seed: seed, Warnings: []string{}}

			verdict, err := research.Niche(ctx, c, research.Options{Seed: seed})
			if err != nil {
				rep.Verdict = "failed"
				detail := err.Error()
				if isPlanCapErr(err) {
					detail = "EverBee refused the research: the account's plan quota is exhausted. This is a refusal, not an empty result."
				}
				rep.Checks = append(rep.Checks, selftestCheck{
					Name: "research_path_reachable", Passed: false, Detail: detail,
				})
				_ = printJSONFiltered(cmd.OutOrStdout(), rep, flags)
				return &cliError{code: selftestExitFailed, err: fmt.Errorf("research path failed: %w", err)}
			}

			ev := verdict.Evidence

			add := func(name string, passed bool, detail string) {
				rep.Checks = append(rep.Checks, selftestCheck{Name: name, Passed: passed, Detail: detail})
			}

			add("transport_ok", true, "EverBee answered both the seeded keyword and product searches")

			gotRows := ev.KeywordsReturned > 0 || ev.ProductsReturned > 0
			add("results_returned", gotRows,
				fmt.Sprintf("%d keywords and %d listings returned", ev.KeywordsReturned, ev.ProductsReturned))

			// The check that matters: are the results actually about the seed?
			// This is what separates the seeded endpoints from the default feeds.
			kwRelevant := ev.KeywordsReturned > 0 && ev.KeywordsRelevant > 0
			add("keywords_semantically_relevant", kwRelevant,
				fmt.Sprintf("%d of %d returned keywords are about %q", ev.KeywordsRelevant, ev.KeywordsReturned, seed))

			prRelevant := ev.ProductsReturned > 0 && ev.ProductsRelevant > 0
			add("products_semantically_relevant", prRelevant,
				fmt.Sprintf("%d of %d returned listings are about %q", ev.ProductsRelevant, ev.ProductsReturned, seed))

			add("seed_metrics_present", verdict.SeedMetrics.Present,
				"EverBee supplied the seed's own demand/competition metrics (searched_keyword)")

			confSane := verdict.Confidence <= research.MaxConfidenceWithoutKeywordEvidence || ev.KeywordsRelevant > 0
			add("confidence_tied_to_evidence", confSane,
				fmt.Sprintf("confidence %.2f with %d relevant evidence rows", verdict.Confidence, ev.TotalEvidence()))

			rep.Relevant.Keywords = fmt.Sprintf("%d/%d", ev.KeywordsRelevant, ev.KeywordsReturned)
			rep.Relevant.Products = fmt.Sprintf("%d/%d", ev.ProductsRelevant, ev.ProductsReturned)

			failed := 0
			for _, ch := range rep.Checks {
				if !ch.Passed {
					failed++
				}
			}

			switch {
			case failed == 0:
				rep.Verdict = "ok"
			case kwRelevant || prRelevant:
				rep.Verdict = "degraded"
				rep.Warnings = append(rep.Warnings,
					"the research path answers, but some semantic checks failed; treat results with care.")
			default:
				// Nothing relevant came back at all. This is the #1492 failure
				// signature: transport success masking meaningless data.
				rep.Verdict = "degraded"
				rep.Warnings = append(rep.Warnings,
					fmt.Sprintf("EverBee answered but returned nothing relevant to %q. Transport is working; the data is not trustworthy.", seed))
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), rep, flags); err != nil {
				return err
			}
			if rep.Verdict != "ok" {
				return &cliError{code: selftestExitDegraded, err: fmt.Errorf("selftest verdict: %s (%d checks failed)", rep.Verdict, failed)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "dad shirt", "Seed keyword to probe the research path with")
	return cmd
}
