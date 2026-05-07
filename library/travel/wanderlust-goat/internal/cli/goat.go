package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// `goat` is the no-LLM compound. It uses the same Fanout engine as `near`
// but explicitly relies on the static criteria→tag and criteria→reddit-keyword
// maps in `internal/criteria/`. The brief mandates a heuristic-only path
// so the CLI works standalone without an agent caller.
func newGoatCmd(flags *rootFlags) *cobra.Command {
	var (
		criteria string
		identity string
		minutes  int
	)
	cmd := &cobra.Command{
		Use:   "goat <anchor>",
		Short: "Same fanout as 'near' but with no LLM in the runtime path — criteria-to-source mapping uses static lookup tables so the CLI works standalone.",
		Long: `Heuristic GOAT compound. Identical fanout to 'near' but explicitly
guarantees no LLM is invoked at runtime: criteria phrases are translated to
OSM tag filters and Reddit body keywords through static lookup tables in
internal/criteria/. Use 'goat' from shell pipelines, cron jobs, and any
context where you cannot or should not delegate to an agent.`,
		Example: strings.Trim(`
  # Lat,lng anchor with no agent in the loop
  wanderlust-goat-pp-cli goat 35.6895,139.6917 \
    --criteria "vintage clothing, vinyl, hidden" --minutes 20

  # CSV mode for cron-driven shortlists
  wanderlust-goat-pp-cli goat "Marais, Paris" \
    --criteria "natural wine, no scene" --minutes 15 --csv`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			anchor := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := resolveAnchor(ctx, anchor)
			if err != nil {
				return err
			}
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			out := Fanout(ctx, res, criteria, identity, minutes, store)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&criteria, "criteria", "", "Free-text criteria (e.g. \"hand-pulled noodles, locals only\").")
	cmd.Flags().StringVar(&identity, "identity", "", "Free-text identity — informs ranking but does not invoke an LLM.")
	cmd.Flags().IntVar(&minutes, "minutes", 15, "Walking-time radius in minutes.")
	return cmd
}
