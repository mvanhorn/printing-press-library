// Hand-authored Google Play discovery commands (live).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/gplay"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

// pp:data-source live
func newTopCmd(flags *rootFlags) *cobra.Command {
	var collection, category string
	var limit int
	var noSnapshot bool
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show a top-chart (free, paid, or grossing) for a category",
		Long: "Fetch a Play top-chart for a collection (TOP_FREE, TOP_PAID, GROSSING) and category (GAME, GAME_PUZZLE, ...). " +
			"Each fetch is snapshotted locally so 'movers' and 'rank-history' can diff chart movement over time.",
		Example:     "  google-play-pp-cli top --collection TOP_GROSSING --category GAME_PUZZLE --country us --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch top chart")
				return nil
			}
			if _, ok := gplay.NormalizeCollection(collection); !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--collection must be one of %s", strings.Join(gplay.CollectionNames(), ", ")))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			apps, err := c.TopCharts(ctx, collection, category, limit)
			if err != nil {
				return classifyGplayErr(err)
			}
			if !noSnapshot {
				snapshotChart(cmd, collection, category, apps)
			}
			return emit(cmd, flags, apps)
		},
	}
	cmd.Flags().StringVar(&collection, "collection", "TOP_FREE", "Chart collection: TOP_FREE, TOP_PAID, or GROSSING")
	cmd.Flags().StringVar(&category, "category", "GAME", "Category (e.g. GAME, GAME_PUZZLE, APPLICATION)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum chart entries (Google caps around 660)")
	cmd.Flags().BoolVar(&noSnapshot, "no-snapshot", false, "Skip writing a local chart snapshot")
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

func snapshotChart(cmd *cobra.Command, collection, category string, apps []gplay.LiteApp) {
	if len(apps) == 0 {
		return
	}
	country, _ := localeOf(cmd)
	wire, _ := gplay.NormalizeCollection(collection)
	rows := make([]store.ChartRow, 0, len(apps))
	for i, a := range apps {
		rows = append(rows, store.ChartRow{Rank: i + 1, AppID: a.AppID, Title: a.Title, Score: a.Score})
	}
	s, err := openStoreFor(cmd.Context(), resolveDBFlag(cmd))
	if err != nil {
		return
	}
	defer s.Close()
	_ = s.InsertChartSnapshot(cmd.Context(), wire, strings.ToUpper(category), country, nowUnix(), rows)
}

// pp:data-source live
func newSearchStoreCmd(flags *rootFlags) *cobra.Command {
	var price string
	var limit int
	cmd := &cobra.Command{
		Use:   "search-store <term>",
		Short: "Search the Play Store for apps by keyword",
		Long: "Search the live Play Store for apps matching a term, with an optional price filter. " +
			"This is the live store search; use 'search' for full-text search over locally-synced data.",
		Example: "  google-play-pp-cli search-store \"merge puzzle\" --price free --limit 25 --agent",
		Args:    cobra.ArbitraryArgs,
		// Any term is valid search input; an obscure term simply returns few or
		// no results (HTTP 200), so there is no bad-input error to raise.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			if _, ok := gplay.NormalizePrice(price); !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--price must be all, free, or paid"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			apps, err := c.Search(ctx, strings.Join(args, " "), price, limit)
			if err != nil {
				return classifyGplayErr(err)
			}
			return emit(cmd, flags, apps)
		},
	}
	cmd.Flags().StringVar(&price, "price", "all", "Price filter: all, free, or paid")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum results (Google caps around 250)")
	return cmd
}

// pp:data-source live
func newSuggestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suggest <term>",
		Short:   "Get Play Store search autocomplete suggestions",
		Long:    "Fetch the store's autocomplete completions for a partial term: useful ASO keyword-seed mining.",
		Example: "  google-play-pp-cli suggest \"merge\" --agent",
		Args:    cobra.ArbitraryArgs,
		// Any term is valid; an obscure term returns no completions (HTTP 200).
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch suggestions")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a term is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			sugs, err := c.Suggest(ctx, strings.Join(args, " "))
			if err != nil {
				return classifyGplayErr(err)
			}
			if sugs == nil {
				sugs = []string{}
			}
			return emit(cmd, flags, map[string]any{"term": strings.Join(args, " "), "suggestions": sugs})
		},
	}
	return cmd
}
