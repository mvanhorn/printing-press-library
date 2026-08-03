// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: weekly spend digest. Self-joins the local generation ledger
// across two time windows to compute spend/volume trends the generic analytics
// command cannot emit, output as an agent-shaped report.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openrouter-image/internal/store"
)

// pp:data-source local

type digestWindow struct {
	Images        int        `json:"images"`
	SpendUSD      float64    `json:"spend_usd"`
	TopModels     []topModel `json:"top_models"`
	AvgCostPerImg float64    `json:"avg_cost_per_image"`
}

type topModel struct {
	Model string  `json:"model"`
	Count int     `json:"count"`
	Cost  float64 `json:"cost_usd"`
}

type usageDigestView struct {
	Window     string       `json:"window"`
	Current    digestWindow `json:"current"`
	Previous   digestWindow `json:"previous"`
	SpendDelta float64      `json:"spend_delta_usd"`
	Note       string       `json:"note,omitempty"`
}

func newNovelUsageDigestCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince string
		dbPath    string
	)

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Period-over-period spend and volume summary: images generated, USD spent, top models",
		Long: `Summarize image-generation spend and volume from the local ledger, comparing
the current window against the previous window of equal length.

Run generate or batch first to populate the ledger. The digest reads only the
local store.

Use this command for a period-over-period spend and volume summary.
Do NOT use it to estimate a future generation's cost; use 'cost-estimate' instead.`,
		Example: strings.Trim(`
  openrouter-image-pp-cli usage digest --since 7d
  openrouter-image-pp-cli usage digest --since 30d --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute the usage digest from the local ledger")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			window := time.Hour * 24 * 7
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("--since must be a duration like 7d or 24h, got %q", flagSince))
				}
				window = d
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openrouter-image-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: openrouter-image-pp-cli generate or batch first\n", dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureOpenRouterImageTables(ctx); err != nil {
				return err
			}

			now := time.Now().UTC()
			curStart := now.Add(-window)
			prevStart := now.Add(-2 * window)

			cur, err := db.ListGenerations(ctx, curStart, 10000)
			if err != nil {
				return err
			}
			prev, err := db.ListGenerations(ctx, prevStart, 10000)
			if err != nil {
				return err
			}

			curFiltered := make([]store.GenerationEntry, 0, len(cur))
			for _, e := range cur {
				if !e.CreatedAt.Before(curStart) {
					curFiltered = append(curFiltered, e)
				}
			}
			prevFiltered := make([]store.GenerationEntry, 0, len(prev))
			for _, e := range prev {
				if !e.CreatedAt.Before(prevStart) && e.CreatedAt.Before(curStart) {
					prevFiltered = append(prevFiltered, e)
				}
			}

			view := usageDigestView{Window: formatWindow(window)}
			view.Current = summarizeWindow(curFiltered)
			view.Previous = summarizeWindow(prevFiltered)
			view.SpendDelta = view.Current.SpendUSD - view.Previous.SpendUSD
			if len(curFiltered) == 0 && len(prevFiltered) == 0 {
				view.Note = "ledger is empty; run generate or batch to populate it"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "usage digest (last %s)\n", view.Window)
			fmt.Fprintf(cmd.OutOrStdout(), "  current:  %d images, $%.2f, avg $%.4f/image\n", view.Current.Images, view.Current.SpendUSD, view.Current.AvgCostPerImg)
			fmt.Fprintf(cmd.OutOrStdout(), "  previous: %d images, $%.2f, avg $%.4f/image\n", view.Previous.Images, view.Previous.SpendUSD, view.Previous.AvgCostPerImg)
			fmt.Fprintf(cmd.OutOrStdout(), "  delta:    $%+.2f\n", view.SpendDelta)
			if len(view.Current.TopModels) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  top models:\n")
				for _, m := range view.Current.TopModels {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-40s %3d images $%.2f\n", m.Model, m.Count, m.Cost)
				}
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window length (e.g. 7d, 24h, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: platform data dir)")
	return cmd
}

func summarizeWindow(entries []store.GenerationEntry) digestWindow {
	w := digestWindow{TopModels: make([]topModel, 0)}
	byModel := map[string]*topModel{}
	for _, e := range entries {
		w.Images++
		w.SpendUSD += e.CostUSD
		m, ok := byModel[e.Model]
		if !ok {
			byModel[e.Model] = &topModel{Model: e.Model}
			m = byModel[e.Model]
		}
		m.Count++
		m.Cost += e.CostUSD
	}
	if w.Images > 0 {
		w.AvgCostPerImg = w.SpendUSD / float64(w.Images)
	}
	models := make([]topModel, 0, len(byModel))
	for _, m := range byModel {
		models = append(models, *m)
	}
	sort.SliceStable(models, func(i, j int) bool { return models[i].Cost > models[j].Cost })
	if len(models) > 5 {
		models = models[:5]
	}
	w.TopModels = models
	return w
}

func formatWindow(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days >= 1 {
		return fmt.Sprintf("%dd", days)
	}
	return d.String()
}
