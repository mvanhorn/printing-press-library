// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelConclusieCmd(flags *rootFlags) *cobra.Command {
	var flagFull bool

	cmd := &cobra.Command{
		Use:   "conclusie <ecli>",
		Short: "Pair a Hoge Raad decision with its A-G conclusie (bidirectional)",
		Long: `Given a Hoge Raad uitspraak ECLI, walk the dcterms:relation edges to find
the matching A-G (Advocate-General) conclusie ECLI. Given a conclusie ECLI,
walks the reverse direction to find the resulting uitspraak.

Pass --full to also fetch the paired decision's content (metadata +
summary + body).`,
		Example: `  rechtspraak-pp-cli conclusie ECLI:NL:HR:2024:1
  rechtspraak-pp-cli conclusie ECLI:NL:PHR:2023:1057 --full --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ecli := args[0]
			parsed, err := rechtspraak.ParseECLI(ecli)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			http := mustHTTP()
			d, err := http.Get(ctx, ecli, false)
			if err != nil {
				return err
			}
			pair := pickConclusiePair(d, parsed.Court)
			if pair == "" {
				// Surface the relations we DID see so the user can tell
				// whether the decision genuinely lacks a conclusie pair
				// (common for HR 81 RO non-substantive rulings) or whether
				// they passed the wrong ECLI (e.g. a Hof decision that
				// has cassatie + eerdere-aanleg edges but no PHR pair).
				seen := make([]string, 0, len(d.Relations))
				for _, rel := range d.Relations {
					if rel.Target == "" {
						continue
					}
					label := shortFromURI(rel.TypeRelatie)
					if label == "" {
						label = "relation"
					}
					seen = append(seen, fmt.Sprintf("%s→%s", label, rel.Target))
				}
				if len(seen) == 0 {
					return fmt.Errorf("no conclusie/uitspraak pair found for %s: this decision has no relations at all (try `rechtspraak-pp-cli chain %s` to confirm)", ecli, ecli)
				}
				return fmt.Errorf("no conclusie/uitspraak pair found for %s: relations present but none match the requested direction (%s). Other edges seen: %s", ecli, pairDirection(parsed.Court, d.Type), strings.Join(seen, ", "))
			}
			result := map[string]any{
				"source":      ecli,
				"source_type": d.Type,
				"paired":      pair,
				"direction":   pairDirection(parsed.Court, d.Type),
			}
			if flagFull {
				paired, err := http.Get(ctx, pair, false)
				if err == nil {
					result["paired_decision"] = paired
				} else {
					result["paired_error"] = err.Error()
				}
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Source:    %s (%s)\n", ecli, d.Type)
			fmt.Fprintf(cmd.OutOrStdout(), "Paired:    %s\n", pair)
			fmt.Fprintf(cmd.OutOrStdout(), "Direction: %s\n", pairDirection(parsed.Court, d.Type))
			if flagFull {
				if paired, ok := result["paired_decision"].(*rechtspraak.Decision); ok {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s, %s)\n", paired.ECLI, paired.Court, paired.DecisionDate)
					if paired.Summary != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", paired.Summary)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagFull, "full", false, "Also fetch the paired decision's content")
	return cmd
}

// pickConclusiePair returns the related ECLI that matches the conclusie/uitspraak
// pairing pattern. The logic:
//   - HR uitspraak → find Conclusie relation (links to PHR ECLI)
//   - PHR conclusie → find Cassatie relation (links to HR ECLI)
//
// Returns empty string when no relation explicitly matches the requested
// direction. Callers MUST treat empty as "no pair found" and surface a
// clear error — silently falling back to the first relation (e.g. an
// eerdere-aanleg edge pointing at a Hof) would return a structurally
// related but semantically wrong ECLI, which is worse than a clean miss.
func pickConclusiePair(d *rechtspraak.Decision, sourceCourt string) string {
	if d == nil {
		return ""
	}
	wantConclusie := sourceCourt == "HR" || strings.EqualFold(d.Type, "Uitspraak")
	for _, rel := range d.Relations {
		t := strings.ToLower(rel.TypeRelatie + rel.Text)
		if wantConclusie {
			if strings.Contains(t, "conclusie") {
				return rel.Target
			}
		} else {
			if strings.Contains(t, "cassatie") {
				return rel.Target
			}
		}
	}
	return ""
}

func pairDirection(sourceCourt, sourceType string) string {
	if sourceCourt == "HR" || strings.EqualFold(sourceType, "Uitspraak") {
		return "uitspraak → conclusie"
	}
	return "conclusie → uitspraak"
}
