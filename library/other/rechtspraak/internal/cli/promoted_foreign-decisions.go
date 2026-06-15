// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newForeignDecisionsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagLJN string
	var flagECLI string

	cmd := &cobra.Command{
		Use:   "foreign-decisions",
		Short: "Foreign decisions registered with LJN codes (NietNederlandseUitspraken)",
		Long:  "Bridge from foreign ECLIs (CJEU, ECHR) to old Dutch LJN codes - ~173k entries. Use --ljn or --ecli to look up a single mapping; bare invocation streams the full list (large, prefer --json).",
		Example: `  rechtspraak-pp-cli foreign-decisions --ljn AF0535
  rechtspraak-pp-cli foreign-decisions --ecli ECLI:EU:C:2002:118 --json`,
		Annotations: map[string]string{"pp:endpoint": "foreign-decisions.list", "pp:method": "GET", "pp:path": "/Waardelijst/NietNederlandseUitspraken", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if flagLJN != "" || flagECLI != "" {
				return foreignDecisionLookup(ctx, cmd, flags, flagLJN, flagECLI)
			}
			fds, err := getForeignDecisions(ctx)
			if err != nil {
				return err
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), fds)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d foreign-decision mappings (use --json for full output, or --ljn/--ecli for a single lookup)\n", len(fds))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagLJN, "ljn", "", "Look up the foreign ECLI for an old LJN code")
	cmd.Flags().StringVar(&flagECLI, "ecli", "", "Look up the old LJN code(s) for a foreign ECLI")
	return cmd
}

func foreignDecisionLookup(ctx context.Context, cmd *cobra.Command, flags *rootFlags, ljn, ecli string) error {
	fds, err := getForeignDecisions(ctx)
	if err != nil {
		return err
	}
	if ljn != "" {
		ljn = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(ljn)), "LJN:")
		for _, fd := range fds {
			for _, j := range fd.LJN {
				if strings.EqualFold(j, ljn) {
					return writeJSONOut(cmd.OutOrStdout(), fd)
				}
			}
		}
		return fmt.Errorf("no foreign decision for LJN %s", ljn)
	}
	if ecli != "" {
		for _, fd := range fds {
			if strings.EqualFold(fd.ECLI, ecli) {
				return writeJSONOut(cmd.OutOrStdout(), fd)
			}
		}
		return fmt.Errorf("no foreign decision for ECLI %s", ecli)
	}
	return nil
}

var foreignDecisionsCache []rechtspraak.ForeignDecision

func getForeignDecisions(ctx context.Context) ([]rechtspraak.ForeignDecision, error) {
	if foreignDecisionsCache != nil {
		return foreignDecisionsCache, nil
	}
	cache := vocabCachePath("foreign-decisions")
	if err := readCacheJSON(cache, &foreignDecisionsCache); err == nil {
		return foreignDecisionsCache, nil
	}
	if isVerifyEnv() {
		return nil, nil
	}
	fds, err := mustHTTP().ForeignDecisions(ctx)
	if err != nil {
		return nil, err
	}
	foreignDecisionsCache = fds
	_ = writeCacheJSON(cache, fds)
	return fds, nil
}
