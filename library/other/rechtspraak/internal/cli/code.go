// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelCodeCmd(flags *rootFlags) *cobra.Command {
	var foreign bool

	cmd := &cobra.Command{
		Use:   "code <code-or-name>",
		Short: "Bidirectional offline lookup of a Dutch (or foreign) court",
		Long: `Look up a Dutch court by afkorting (HR, RBAMS), full name (Hoge Raad,
Rechtbank Amsterdam), or PSI URI. Returns code, name, type, begin/end dates,
the PSI URI, and any predecessor or successor courts the local cache can
derive from Begin/EndDate.

Use --foreign to query the InstantiesBuitenlands vocabulary instead
(~5000 EU member-state, ECHR, and CJEU entries).`,
		Example: `  rechtspraak-pp-cli code RBAMS
  rechtspraak-pp-cli code "Rechtbank Amsterdam" --json
  rechtspraak-pp-cli code ECHR --foreign`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			q := args[0]
			var idx *rechtspraak.CourtIndex
			if foreign {
				var err error
				idx, err = getForeignCourtIndex(ctx)
				if err != nil {
					return err
				}
			} else {
				var err error
				idx, err = getCourtIndex(ctx)
				if err != nil {
					return err
				}
			}
			court, ok := idx.Resolve(q)
			if !ok {
				return fmt.Errorf("no match for %q (try --foreign for non-NL courts)", q)
			}
			result := map[string]any{
				"name":         court.Name,
				"code":         court.Afkorting,
				"type":         court.Type,
				"begin_date":   court.BeginDate,
				"end_date":     court.EndDate,
				"psi_uri":      court.Identifier,
				"predecessors": courtsToBrief(idx.Predecessors(court)),
				"successors":   courtsToBrief(idx.Successors(court)),
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Name:      %s\n", court.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Code:      %s\n", orDash(court.Afkorting))
			fmt.Fprintf(cmd.OutOrStdout(), "Type:      %s\n", orDash(court.Type))
			fmt.Fprintf(cmd.OutOrStdout(), "Begin:     %s\n", orDash(court.BeginDate))
			fmt.Fprintf(cmd.OutOrStdout(), "End:       %s\n", orDash(court.EndDate))
			fmt.Fprintf(cmd.OutOrStdout(), "PSI URI:   %s\n", court.Identifier)
			if preds := idx.Predecessors(court); len(preds) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Predecessors:")
				for _, p := range preds {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", p.Name, p.Afkorting)
				}
			}
			if succs := idx.Successors(court); len(succs) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Successors:")
				for _, s := range succs {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", s.Name, s.Afkorting)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&foreign, "foreign", false, "Query the foreign courts vocabulary (InstantiesBuitenlands) instead of Dutch courts")
	return cmd
}

func courtsToBrief(cs []rechtspraak.Court) []map[string]string {
	out := make([]map[string]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]string{
			"name":    c.Name,
			"code":    c.Afkorting,
			"psi_uri": c.Identifier,
		})
	}
	return out
}

// Lazy foreign-court index (kept here rather than in helpers.go to avoid
// fetching ~200KB on every cold-start when only the Dutch index is needed).
var (
	foreignCourtIdxVal *rechtspraak.CourtIndex
)

func getForeignCourtIndex(ctx context.Context) (*rechtspraak.CourtIndex, error) {
	if foreignCourtIdxVal != nil {
		return foreignCourtIdxVal, nil
	}
	var courts []rechtspraak.Court
	cache := vocabCachePath("foreign-courts")
	if err := readCacheJSON(cache, &courts); err != nil {
		if isVerifyEnv() {
			foreignCourtIdxVal = rechtspraak.NewCourtIndex(nil)
			return foreignCourtIdxVal, nil
		}
		var err error
		courts, err = mustHTTP().ForeignCourts(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch foreign courts vocab: %w", err)
		}
		_ = writeCacheJSON(cache, courts)
	}
	foreignCourtIdxVal = rechtspraak.NewCourtIndex(courts)
	return foreignCourtIdxVal, nil
}
