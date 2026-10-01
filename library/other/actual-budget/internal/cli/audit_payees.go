// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

type payeeMergeMember struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type payeeClusterRow struct {
	TargetID     string             `json:"target_id"`
	TargetName   string             `json:"target_name"`
	TargetCount  int                `json:"target_count"`
	Merge        []payeeMergeMember `json:"merge"`
	Similarity   float64            `json:"similarity"`
	MergeCommand string             `json:"merge_command"`
}

// payeeMergeCommand renders the generated payees-merge endpoint invocation
// (POST /budgets/{budgetSyncId}/payees/merge with {targetId, mergeIds}).
func payeeMergeCommand(target string, merge []string) string {
	return fmt.Sprintf("actual-budget-pp-cli payees merge --target-id %s --merge-ids %s", target, strings.Join(merge, ","))
}

// defaultMinSimilarity is the clustering threshold when --min-similarity is unset.
const defaultMinSimilarity = 0.85

func newNovelAuditPayeesCmd(flags *rootFlags) *cobra.Command {
	var flagMinSimilarity float64
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "payees",
		Short: "Find payee spelling variants (AMZN Mktp vs Amazon Marketplace) and get a ready-to-run merge plan.",
		Long: `Use this command to find payee records that are duplicates of each other (spelling variants) and get a merge plan. Do NOT use this command to find duplicate transactions; use 'ledger duplicates' instead.

Names are normalized (lowercase; "*..." suffixes, store numbers like #123,
digits, punctuation and noise words such as inc, llc, com, mktp, marketplace,
pmts, us, www removed; for payment-processor prefixes such as "SQ *", "TST*"
and "PAYPAL *" the merchant after the "*" is kept) and grouped when the
normalized names are equal, one is a whole-word prefix of the other (4+
characters, so "Kroger" joins "Kroger Fuel" but "Apple" does not join
"Applebee's"), or their Jaro-Winkler / Levenshtein similarity reaches
--min-similarity. The most-used payee in each cluster is the
merge target; merge_command is a ready-to-run 'payees merge' call
(the sidecar's payees/merge endpoint). Reads the local mirror only.`,
		Example: strings.Trim(`
  actual-budget-pp-cli audit payees
  actual-budget-pp-cli audit payees --min-similarity 0.9 --agent
  actual-budget-pp-cli audit payees --json --select target_name,merge_command`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "audit payees"); done {
				return err
			}
			minSim := flagMinSimilarity
			if math.IsNaN(minSim) || minSim < 0 || minSim > 1 {
				return usageErr(fmt.Errorf("--min-similarity must be a number between 0 and 1, got %v", minSim))
			}
			if err := checkLimit(flagLimit); err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]payeeClusterRow, 0), func(ctx context.Context, h *mirrorHandle) error {
				payees, err := h.Ledger.Payees(ctx)
				if err != nil {
					return err
				}
				clusters, err := actual.ClusterPayeesContext(ctx, payees, minSim)
				if err != nil {
					return err
				}
				rows := make([]payeeClusterRow, 0)
				for _, c := range clusters {
					r := payeeClusterRow{TargetID: c.Target.ID, TargetName: c.Target.Name, TargetCount: c.Target.TxnCount,
						Merge: make([]payeeMergeMember, 0, len(c.Merge)), Similarity: math.Round(c.Similarity*1000) / 1000}
					ids := make([]string, 0, len(c.Merge))
					for _, m := range c.Merge {
						r.Merge = append(r.Merge, payeeMergeMember{ID: m.ID, Name: m.Name, Count: m.TxnCount})
						ids = append(ids, m.ID)
					}
					r.MergeCommand = payeeMergeCommand(c.Target.ID, ids)
					rows = append(rows, r)
					if flagLimit > 0 && len(rows) >= flagLimit {
						break
					}
				}
				return emitRows(cmd, flags, rows, func() error {
					w := cmd.OutOrStdout()
					if len(rows) == 0 {
						fmt.Fprintln(w, "No payee spelling variants found.")
						return nil
					}
					for _, r := range rows {
						fmt.Fprintf(w, "%s (%d txns) <- ", r.TargetName, r.TargetCount)
						names := make([]string, 0, len(r.Merge))
						for _, m := range r.Merge {
							names = append(names, fmt.Sprintf("%s (%d)", m.Name, m.Count))
						}
						fmt.Fprintf(w, "%s  [similarity %.2f]\n  %s\n", strings.Join(names, ", "), r.Similarity, r.MergeCommand)
					}
					return nil
				})
			})
		},
	}
	cmd.Flags().Float64Var(&flagMinSimilarity, "min-similarity", defaultMinSimilarity, "Minimum name similarity (0..1) for two payees to be clustered")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum clusters to return (0 = all)")
	return cmd
}
