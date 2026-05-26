// Package commands wires the CLI surface using cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/config"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/store"
	syncpkg "github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/sync"
)

// Globals shared across commands via flags.
type Globals struct {
	JSON     bool
	CSV      bool
	NoHeader bool
	Limit    int
	Wide     bool
	NoSync   bool
}

var G = &Globals{}

// Context bag passed to each command's RunE.
type Ctx struct {
	Cfg *config.Config
	S   *store.Store
	C   *client.Client
	Eng *syncpkg.Engine
}

func newCtx() (*Ctx, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s, err := store.Open(config.DBPath())
	if err != nil {
		return nil, err
	}
	c := client.New()
	return &Ctx{Cfg: cfg, S: s, C: c, Eng: syncpkg.New(c, s)}, nil
}

// withCtx wraps a RunE so each command gets a fresh Ctx and closes the store.
func withCtx(fn func(cmd *cobra.Command, args []string, cx *Ctx) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cx, err := newCtx()
		if err != nil {
			return err
		}
		defer cx.S.Close()
		return fn(cmd, args, cx)
	}
}

// ensureFresh syncs the given domains if their last sync is older than threshold,
// or if there is no data yet. Reports to stderr.
func ensureFresh(ctx context.Context, cx *Ctx, domains []string) error {
	if G.NoSync {
		return nil
	}
	stale := []string{}
	for _, d := range domains {
		s, err := cx.S.StaleBefore(d, cx.Cfg.StaleOverviewDur())
		if err != nil {
			return err
		}
		if s {
			stale = append(stale, d)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "syncing stale domains: %s\n", strings.Join(stale, ", "))
	cx.Eng.Report = func(domain, msg string) {
		fmt.Fprintf(os.Stderr, "  [%s] %s\n", domain, msg)
	}
	return cx.Eng.SyncDomains(ctx, stale)
}

func mode() format.Mode {
	return format.ParseMode(G.JSON, G.CSV)
}

// parsePeriod accepts "7d", "30d", "90d", "180d", "1y", "all".
// Returns days; 0 means "all".
func parsePeriod(p string) (int, error) {
	switch p {
	case "":
		return 30, nil
	case "all":
		return 0, nil
	case "1y":
		return 365, nil
	}
	if strings.HasSuffix(p, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(p, "d"))
		if err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("invalid period %q (use 7d, 30d, 90d, 180d, 1y, all)", p)
}

// New builds the cobra root.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "defillama-pp-cli",
		Short:         "DefiLlama Printing Press CLI (SQLite-backed, agent-native)",
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       version,
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&G.JSON, "json", false, "JSON output")
	pf.BoolVar(&G.CSV, "csv", false, "CSV output")
	pf.BoolVar(&G.NoHeader, "no-header", false, "omit column headers")
	pf.IntVar(&G.Limit, "limit", 0, "max rows (0 = command default)")
	pf.BoolVar(&G.Wide, "wide", false, "show all columns")
	pf.BoolVar(&G.NoSync, "no-sync", false, "skip auto-sync staleness check")

	root.AddCommand(
		newSyncCmd(),
		newTopCmd(),
		newCompareCmd(),
		newYieldsCmd(),
		newStablesCmd(),
		newProfileCmd(),
		newChainsCmd(),
		newSQLCmd(),
		newPriceCmd(),
		newConfigCmd(),
		newTVLCmd(),
		newFeesCmd(),
		newDexsCmd(),
		newOptionsCmd(),
		newBridgesCmd(),
		newEmissionsCmd(),
		newHacksCmd(),
		newRaisesCmd(),
		newTreasuriesCmd(),
		newETFsCmd(),
		newRWACmd(),
		newNarrativesCmd(),
		newDerivativesCmd(),
		newExportCmd(),
		newMCPCmd(),
	)
	return root
}
