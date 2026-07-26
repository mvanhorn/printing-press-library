// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy-seller-dashboard/internal/analysis"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy-seller-dashboard/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newListingAnalysisCmd(flags))
		root.AddCommand(newEconomicsCmd(flags))
		root.AddCommand(newPromotionAnalysisCmd(flags))
		root.AddCommand(newAcquisitionAnalysisCmd(flags))
		root.AddCommand(newQuotaAnalysisCmd(flags))
		root.AddCommand(newGrowthAnalysisCmd(flags))
	})
}

type analysisData struct {
	insights   []analysis.Snapshot
	ads        []analysis.Snapshot
	offsite    []analysis.Snapshot
	promotions []analysis.Snapshot
}

func newListingAnalysisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	parent := &cobra.Command{
		Use: "listing", Short: "Cross-surface listing decisions",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	parent.AddCommand(&cobra.Command{
		Use: "action-queue", Short: "Rank deterministic weekly listing review actions",
		Long: "Use this command to rank listing-level review actions. " +
			"Do not use it for channel-cost reconciliation; use 'economics reconcile'.",
		Example:     "  etsy-seller-dashboard-pp-cli listing action-queue",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			return flags.printJSON(cmd, analysis.BuildActionQueue(data.insights, data.ads, data.offsite, data.promotions))
		},
	})
	parent.AddCommand(&cobra.Command{
		Use: "visibility-gap", Short: "Compare explicit search visibility with paid performance",
		Long: "Use this command for mapped listing-keyword visibility mismatches. " +
			"Do not use it for the broader queue; use 'listing action-queue'.",
		Example:     "  etsy-seller-dashboard-pp-cli listing visibility-gap",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			return flags.printJSON(cmd, analysis.VisibilityPerformanceGaps(data.insights, data.ads, data.offsite))
		},
	})
	return parent
}

func newEconomicsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	parent := &cobra.Command{
		Use: "economics", Short: "Attribution-safe marketing economics",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	parent.AddCommand(&cobra.Command{
		Use: "reconcile", Short: "Reconcile source subtotals without inventing net profit",
		Long: "Use this command to reconcile channel economics. " +
			"Do not use it to rank listing actions; use 'listing action-queue'.",
		Example:     "  etsy-seller-dashboard-pp-cli economics reconcile",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			return flags.printJSON(cmd, analysis.ReconcileEconomics(data.ads, data.offsite, data.promotions))
		},
	})
	return parent
}

func newPromotionAnalysisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	parent := &cobra.Command{
		Use: "promotion", Short: "Historical promotion analysis",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	parent.AddCommand(&cobra.Command{
		Use: "observed-lift <promotion-id>", Args: cobra.ExactArgs(1),
		Short: "Compare a promotion window with an equal-length prior window",
		Long: "Use this command for one promotion's observed during-versus-baseline measurements. " +
			"Do not use it for portfolio outliers; use 'growth anomalies'.",
		Example:     "  etsy-seller-dashboard-pp-cli promotion observed-lift 1000000000000 --db dashboard.db",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			result, err := analysis.ObservedPromotionLift(args[0], data.promotions, data.ads, data.offsite)
			if errors.Is(err, analysis.ErrInsufficientHistory) {
				return flags.printJSON(cmd, map[string]any{
					"status": "insufficient-history", "promotion_id": args[0],
					"required": "promotion and equal-length baseline observations",
				})
			}
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, result)
		},
	})
	return parent
}

func newAcquisitionAnalysisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	parent := &cobra.Command{
		Use: "acquisition", Short: "Cross-channel acquisition analysis",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	parent.AddCommand(&cobra.Command{
		Use:         "channel-gap",
		Short:       "Find onsite and offsite listing-performance asymmetry",
		Example:     "  etsy-seller-dashboard-pp-cli acquisition channel-gap --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			return flags.printJSON(cmd, analysis.AcquisitionChannelGaps(data.ads, data.offsite))
		},
	})
	return parent
}

func newQuotaAnalysisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	parent := &cobra.Command{
		Use: "quota", Short: "Marketplace Insights quota planning",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	parent.AddCommand(&cobra.Command{
		Use:         "allocate",
		Short:       "Rank research gaps without consuming live keyword quota",
		Example:     "  etsy-seller-dashboard-pp-cli quota allocate --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			return flags.printJSON(cmd, analysis.AllocateResearchQuota(
				data.insights, data.ads, data.offsite, data.promotions, time.Now().UTC(),
			))
		},
	})
	return parent
}

func newGrowthAnalysisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var threshold float64
	parent := &cobra.Command{
		Use: "growth", Short: "Historical cross-surface monitoring",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	parent.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database file path")
	anomalies := &cobra.Command{
		Use:     "anomalies",
		Short:   "Detect deterministic weekly cross-surface outliers",
		Example: "  etsy-seller-dashboard-pp-cli growth anomalies --threshold 0.5 --agent",
		Long: "Use this command for portfolio-wide weekly outliers. " +
			"Do not use it for one promotion; use 'promotion observed-lift'.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, database, err := loadAnalysisData(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer database.Close()
			result, err := analysis.CrossSurfaceAnomalies(map[string][]analysis.Snapshot{
				"marketplace-insights": data.insights,
				"ads":                  data.ads, "offsite-ads": data.offsite, "promotions": data.promotions,
			}, threshold)
			if errors.Is(err, analysis.ErrInsufficientHistory) {
				return flags.printJSON(cmd, map[string]any{
					"status": "insufficient-history", "required": "at least three observed weeks for one source",
				})
			}
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, result)
		},
	}
	anomalies.Flags().Float64Var(&threshold, "threshold", 0.5, "Minimum absolute change from prior median")
	parent.AddCommand(anomalies)
	return parent
}

func loadAnalysisData(
	cmd *cobra.Command,
	flags *rootFlags,
	dbPath string,
) (analysisData, *store.Store, error) {
	if err := validateDataSourceStrategy(flags, "local"); err != nil {
		return analysisData{}, nil, err
	}
	if dbPath == "" {
		dbPath = defaultDBPath("etsy-seller-dashboard-pp-cli")
	}
	var database *store.Store
	var err error
	if _, statErr := os.Stat(dbPath); errors.Is(statErr, os.ErrNotExist) {
		database, err = store.OpenWithContext(cmd.Context(), ":memory:")
	} else {
		database, err = store.OpenReadOnlyContext(cmd.Context(), dbPath)
	}
	if err != nil {
		return analysisData{}, nil, fmt.Errorf("opening local store: %w", err)
	}
	maybeEmitSyncHints(cmd, database, "", flags.maxAge)
	resources := []string{"marketplace-insights", "ads", "offsite-ads", "promotions"}
	snapshots := make(map[string][]analysis.Snapshot, len(resources))
	for _, resource := range resources {
		rows, listErr := database.List(resource, 100000)
		if listErr != nil {
			return analysisData{}, nil, errors.Join(listErr, database.Close())
		}
		decoded, decodeErr := analysis.DecodeSnapshots(resource, rows)
		if decodeErr != nil {
			return analysisData{}, nil, errors.Join(decodeErr, database.Close())
		}
		snapshots[resource] = decoded
	}
	return analysisData{
		insights: snapshots["marketplace-insights"], ads: snapshots["ads"],
		offsite: snapshots["offsite-ads"], promotions: snapshots["promotions"],
	}, database, nil
}
