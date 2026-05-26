package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/sync"
)

// domainAlias accepts both hyphenated/underscored and historical alias names
// and returns the canonical domain (or "" if unknown).
func domainAlias(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "protocols", "tvl":
		// "tvl" is the bundle name in --domain but maps to protocols+chains; we
		// handle that case in the command itself, so this returns the leading
		// component.
		return "protocols"
	case "chains":
		return "chains"
	case "pools", "yields":
		return "pools"
	case "stablecoins", "stables":
		return "stablecoins"
	case "dexs", "dex":
		return "dexs"
	case "fees":
		return "fees"
	case "options":
		return "options"
	case "open_interest", "open-interest", "oi":
		return "open_interest"
	}
	return ""
}

// domainBundle expands "tvl" into protocols+chains; otherwise returns the
// single-element canonical domain set, or nil if unknown.
func domainBundle(name string) []string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "tvl" {
		return []string{"protocols", "chains"}
	}
	d := domainAlias(name)
	if d == "" {
		return nil
	}
	return []string{d}
}

func newSyncCmd() *cobra.Command {
	var domain, protocol, chain string
	var status, backfill, pro bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the local SQLite mirror from DefiLlama",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if status {
				return runSyncStatus(cx)
			}
			if protocol != "" {
				return runSyncProtocol(cx, protocol, backfill)
			}
			if chain != "" {
				return runSyncChain(cx, chain)
			}
			if pro {
				return runSyncPro(cx)
			}
			domains := sync.Domains
			if domain != "" {
				bundle := domainBundle(domain)
				if bundle == nil {
					return fmt.Errorf("unknown --domain %q (try tvl, yields/pools, stablecoins, dexs, fees, options, open-interest, chains)", domain)
				}
				domains = bundle
			}
			cx.Eng.Report = func(d, msg string) {
				fmt.Fprintf(os.Stderr, "[%s] %s\n", d, msg)
			}
			ctx := context.Background()
			if err := cx.Eng.SyncDomains(ctx, domains); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "sync complete: "+strings.Join(domains, ", "))
			return nil
		}),
	}
	cmd.Flags().StringVar(&domain, "domain", "", "limit to a domain: tvl, yields|pools, stablecoins, dexs, fees, options, open-interest, chains")
	cmd.Flags().BoolVar(&status, "status", false, "show last sync times per domain")
	cmd.Flags().StringVar(&protocol, "protocol", "", "sync historical detail for a single protocol (trailing 90d unless --backfill)")
	cmd.Flags().StringVar(&chain, "chain", "", "sync historical chain TVL")
	cmd.Flags().BoolVar(&backfill, "backfill", false, "with --protocol, fetch full history (not just trailing 90 days)")
	cmd.Flags().BoolVar(&pro, "pro", false, "sync pro-only tables (requires DEFILLAMA_PRO_KEY)")
	return cmd
}

func runSyncStatus(cx *Ctx) error {
	existing, err := cx.S.AllSyncMeta()
	if err != nil {
		return err
	}
	got := map[string]time.Time{}
	rowCount := map[string]int{}
	for _, m := range existing {
		got[m.Domain] = m.LastSync
		rowCount[m.Domain] = m.RowCount
	}
	// canonical free-tier domain list + chains
	all := append([]string{}, sync.Domains...)

	rec := format.NewRecorder([]string{"DOMAIN", "LAST_SYNC", "ROWS"})
	for _, d := range all {
		ts, ok := got[d]
		when := "never"
		if ok {
			when = ts.Format("2006-01-02 15:04:05")
		}
		rec.Append([]format.Cell{
			format.Str(d),
			format.Str(when),
			format.Int(int64(rowCount[d])),
		})
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{2})})
}

var runSyncProtocol = func(cx *Ctx, slug string, backfill bool) error {
	// Resolve user input to a real slug via the existing resolver so that
	// `sync --protocol aave` works the same way as compare/profile.
	resolved, _, err := resolveProtocol(cx.S, slug)
	if err != nil {
		return err
	}
	cx.Eng.Report = func(d, msg string) { fmt.Fprintf(os.Stderr, "[%s] %s\n", d, msg) }
	if err := cx.Eng.SyncProtocolDetail(context.Background(), resolved, backfill); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "synced detail for %s (backfill=%v)\n", resolved, backfill)
	return nil
}

var runSyncChain = func(cx *Ctx, chain string) error {
	cx.Eng.Report = func(d, msg string) { fmt.Fprintf(os.Stderr, "[%s] %s\n", d, msg) }
	if err := cx.Eng.SyncChainTVL(context.Background(), chain); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "synced chain tvl history for %s\n", chain)
	return nil
}

// runSyncPro filled in by P2.5.
var runSyncPro = func(cx *Ctx) error {
	return fmt.Errorf("sync --pro requires P2.5 pro tier (not yet implemented)")
}
