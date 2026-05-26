package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// The pro tier wraps DefiLlama's $300/mo pro-api endpoints (bridges,
// emissions, hacks, raises, treasuries, ETFs, RWA, narratives, derivatives).
// The gating is in place — every pro command refuses without
// DEFILLAMA_PRO_KEY — but the data layer for the pro endpoints themselves
// is not yet implemented in this release. Each command therefore returns a
// clear "not yet implemented" error when the key IS set, instead of
// silently returning empty rows from a stub table.
//
// Tracking issue: https://github.com/mvanhorn/printing-press-library/library/payments/defillama/issues
//
// To implement a pro command:
//   1. Add a syncer in internal/sync/pro.go that hits the pro endpoint.
//   2. Wire the syncer into `sync --pro` in the engine.
//   3. Replace the not-yet-implemented stub below with a real query.

func requirePro(cx *Ctx) error {
	if cx.Cfg.ResolvedProKey() == "" {
		return fmt.Errorf("this command requires DEFILLAMA_PRO_KEY (env var or `config set pro-key`)")
	}
	return nil
}

// notYetImplemented returns the standard "the gate works, the body doesn't"
// error for pro commands. Calling this *after* requirePro ensures key-less
// users still see the gating message first.
func notYetImplemented(name string) error {
	return fmt.Errorf("`%s` is a pro-tier command; the auth gate works but the data layer is not yet implemented in this release", name)
}

func newProCommand(use, short string) *cobra.Command {
	// Each pro command shares the same shape: refuse without key, otherwise
	// surface the implementation gap honestly.
	name := strings.Fields(use)[0]
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if err := requirePro(cx); err != nil {
				return err
			}
			return notYetImplemented(name)
		}),
	}
	return cmd
}

func newBridgesCmd() *cobra.Command {
	cmd := newProCommand("bridges", "Bridge volumes (pro)")
	cmd.Flags().String("chain", "", "filter by chain")
	cmd.Flags().String("period", "", "period filter (Nd)")
	return cmd
}

func newEmissionsCmd() *cobra.Command {
	cmd := newProCommand("emissions <protocol>", "Token unlock schedule (pro)")
	cmd.Args = cobra.ExactArgs(1)
	return cmd
}

func newHacksCmd() *cobra.Command {
	cmd := newProCommand("hacks", "Hack incidents (pro)")
	cmd.Flags().String("sort", "amount", "sort key")
	return cmd
}

func newRaisesCmd() *cobra.Command {
	return newProCommand("raises", "Protocol fundraising rounds (pro)")
}

func newTreasuriesCmd() *cobra.Command {
	return newProCommand("treasuries", "Protocol treasuries (pro)")
}

func newETFsCmd() *cobra.Command {
	cmd := newProCommand("etfs", "Crypto ETF AUM/flows (pro)")
	cmd.Flags().Bool("flows", false, "show flows instead of snapshot")
	return cmd
}

func newRWACmd() *cobra.Command {
	cmd := newProCommand("rwa", "Real-world asset tokenization (pro)")
	cmd.Flags().String("chain", "", "filter by chain")
	return cmd
}

func newNarrativesCmd() *cobra.Command {
	return newProCommand("narratives", "FDV performance by narrative (pro)")
}

func newDerivativesCmd() *cobra.Command {
	return newProCommand("derivatives", "Derivatives volume overview (pro)")
}

// init replaces the sync --pro stub with the same "gated but not implemented"
// pattern so users get a consistent message.
func init() {
	runSyncPro = func(cx *Ctx) error {
		if err := requirePro(cx); err != nil {
			return err
		}
		return notYetImplemented("sync --pro")
	}
}
