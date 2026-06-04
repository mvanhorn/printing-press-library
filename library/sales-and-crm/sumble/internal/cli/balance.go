package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/cliutil"

	"github.com/spf13/cobra"
)

// balanceProbeQuery is a deliberately non-matching technology search. A
// technologies/find call that returns no matches costs 0 credits but still
// carries the credits_remaining accounting, making it the cheapest way to read
// a fresh balance.
const balanceProbeQuery = "zzqx-sumble-cli-balance-probe-no-match"

func newBalanceCmd(flags *rootFlags) *cobra.Command {
	var probe, noProbe bool

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show remaining Sumble credits and recent burn without spending any",
		Long: strings.Trim(`
Show your remaining Sumble credit balance. Sumble's REST API has no balance
endpoint, so this reads the most recent credits_remaining value recorded in the
local ledger from every billed call this CLI has made.

When the ledger has no balance yet (or --probe is passed), it makes one free
probe call (a technology search with no matches costs 0 credits) to fetch a
fresh balance. Pass --no-probe to read only the local ledger.
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli balance
  sumble-pp-cli balance --probe --json
  sumble-pp-cli balance --no-probe
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openCreditStore()
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			bal, have, err := latestBalance(db.DB())
			if err != nil {
				return apiErr(err)
			}

			shouldProbe := probe || (!have && !noProbe)
			if shouldProbe && cliutil.IsVerifyEnv() {
				shouldProbe = false
			}

			source := "ledger"
			if shouldProbe {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				raw, _, perr := c.Post(cmd.Context(), "/technologies/find", map[string]any{"query": balanceProbeQuery})
				if perr != nil {
					if !have {
						return classifyAPIError(perr, flags)
					}
				} else {
					env := parseEnvelope(raw)
					if rem := recordEnvelope(db.DB(), "technologies.find", env, "balance probe"); rem != nil {
						bal, have = *rem, true
						source = "probe"
					}
				}
			}

			if !have {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"balance_known": false})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No balance recorded yet. Run 'sumble-pp-cli balance --probe' to fetch a fresh balance.")
				return nil
			}

			var totalUsed int
			_ = db.DB().QueryRow(`SELECT COALESCE(SUM(credits_used), 0) FROM credit_ledger`).Scan(&totalUsed)

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"balance_known":      true,
					"credits_remaining":  bal,
					"credits_used_total": totalUsed,
					"source":             source,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Credits remaining: %d (source: %s)\n", bal, source)
			fmt.Fprintf(w, "Credits used (tracked by this CLI): %d\n", totalUsed)
			return nil
		},
	}

	cmd.Flags().BoolVar(&probe, "probe", false, "Make a free probe call to fetch a fresh balance")
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Read only the local ledger; never dial the API")
	return cmd
}
