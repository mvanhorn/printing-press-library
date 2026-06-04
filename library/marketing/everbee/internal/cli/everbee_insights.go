package cli

import (
	"context"
	"database/sql"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"

	"github.com/spf13/cobra"
)

type everbeeRecord struct {
	ID           string         `json:"id"`
	ResourceType string         `json:"resource_type"`
	SyncedAt     sql.NullString `json:"-"`
	Data         map[string]any `json:"data"`
	Text         string         `json:"text"`
	Score        float64        `json:"score,omitempty"`
	Reasons      []string       `json:"reasons,omitempty"`
}

func newOpportunityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "opportunity",
		Short:   "Rank EverBee product and keyword opportunities",
		Example: strings.Trim("\n  everbee-pp-cli opportunity shortlist --query=teacher-gift --json\n", "\n"),
		RunE:    parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOpportunityShortlistCmd(flags))
	return cmd
}

func newOpportunityShortlistCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "shortlist",
		Short:   "Rank product opportunities from local EverBee snapshots",
		Example: strings.Trim("\n  everbee-pp-cli opportunity shortlist --query \"teacher gift\" --limit 25 --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if query == "" {
				return cmd.Help()
			}
			ctx := cmd.Context()
			scope := research.ResearchScope{Kind: research.ScopeQuery, Value: query}
			researchOpts.limit = limit
			researchOpts.resources = []string{"product_analytics", "keyword_research"}

			db, err := openResearchDB(ctx, researchOpts.noRefresh)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			snapshotStore := researchStoreFromDB(db)
			var snapshots []research.Snapshot
			if snapshotStore != nil {
				snapshots, err = snapshotStore.List(ctx, scope, 5)
				if err != nil {
					return err
				}
			}

			runtime := researchRuntime{fetcher: liveResearchFetcher{flags: flags, snapshotStore: snapshotStore, maxAge: researchOpts.maxAge}}
			if researchOpts.noRefresh {
				runtime.fetcher = nil
			}
			result, err := runtime.resolve(ctx, scope, researchOpts, snapshots)
			if err != nil {
				return err
			}
			evidence, evidenceWarnings := opportunityEvidence(result.Snapshot)
			plan := appendResearchWarnings(result.Plan, evidenceWarnings)
			return printJSONFiltered(cmd.OutOrStdout(), research.BuildOpportunityShortlist(scope, evidence, limit, plan), flags)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Product or keyword phrase to rank")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum records to return")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func openResearchDB(ctx context.Context, localOnly bool) (*store.Store, error) {
	if localOnly {
		return openStoreForRead(ctx, "github.com/mvanhorn/printing-press-library/library/marketing/everbee")
	}
	return store.OpenWithContext(ctx, defaultDBPath("github.com/mvanhorn/printing-press-library/library/marketing/everbee"))
}

func opportunityEvidence(snapshot research.Snapshot) ([]research.EvidenceRecord, []string) {
	if len(snapshot.Evidence) > 0 {
		return snapshot.Evidence, nil
	}
	if len(snapshot.RawRecords) == 0 {
		return nil, nil
	}
	if len(snapshot.Resources) != 1 {
		return nil, []string{"skipped raw fallback normalization because snapshot contains ambiguous resources"}
	}
	evidence, _ := research.NormalizeRecords(snapshot.Resources[0], snapshot.RawRecords)
	return evidence, nil
}

func appendResearchWarnings(plan research.ResearchPlan, warnings []string) research.ResearchPlan {
	if len(warnings) == 0 {
		return plan
	}
	seen := make(map[string]struct{}, len(plan.Warnings)+len(warnings))
	for _, warning := range plan.Warnings {
		seen[warning] = struct{}{}
	}
	for _, warning := range warnings {
		if _, ok := seen[warning]; ok {
			continue
		}
		plan.Warnings = append(plan.Warnings, warning)
		seen[warning] = struct{}{}
	}
	return plan
}

func newNicheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "niche",
		Short:   "Score Etsy niches from EverBee snapshots",
		Example: strings.Trim("\n  everbee-pp-cli niche score --keyword=teacher-gift --json\n", "\n"),
		RunE:    parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNicheScoreCmd(flags))
	return cmd
}

func newNicheScoreCmd(flags *rootFlags) *cobra.Command {
	var keyword string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "score",
		Short:   "Score a niche using product and keyword history",
		Example: strings.Trim("\n  everbee-pp-cli niche score --keyword \"mother's day mug\" --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if keyword == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeKeyword, Value: keyword}
			researchOpts.limit = 100
			researchOpts.resources = []string{"product_analytics", "keyword_research"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				evidence, plan := evidenceFromSnapshot(plan, snapshot)
				return research.BuildNicheScore(scope, evidence, plan)
			})
		},
	}
	cmd.Flags().StringVar(&keyword, "keyword", "", "Keyword or niche phrase to score")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newShopInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shop",
		Short: "Analyze competitor shop gaps",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newShopGapsCmd(flags))
	return cmd
}

func newShopGapsCmd(flags *rootFlags) *cobra.Command {
	var shop string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "gaps",
		Short:   "Find product, pricing, and keyword gaps for a shop",
		Example: strings.Trim("\n  everbee-pp-cli shop gaps --shop competitor-shop --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if shop == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeShop, Value: shop}
			researchOpts.limit = 50
			researchOpts.resources = []string{"shops", "product_analytics", "keyword_research"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				evidence, plan := evidenceFromSnapshot(plan, snapshot)
				return research.BuildShopGaps(scope, evidence, plan)
			})
		},
	}
	cmd.Flags().StringVar(&shop, "shop", "", "Competitor shop name or handle")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newTagsInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tags",
		Short:   "Analyze EverBee tag gaps",
		Example: strings.Trim("\n  everbee-pp-cli tags gap --query=teacher-gift --json\n", "\n"),
		RunE:    parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTagsGapCmd(flags))
	return cmd
}

func newTagsGapCmd(flags *rootFlags) *cobra.Command {
	var query, shop string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "gap",
		Short:   "Compare winning tags against a target shop or query",
		Example: strings.Trim("\n  everbee-pp-cli tags gap --query candle --shop my-shop --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if query == "" && shop == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeQuery, Value: strings.TrimSpace(query + " " + shop)}
			researchOpts.limit = 75
			researchOpts.resources = []string{"product_analytics", "keyword_research", "shops"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				evidence, plan := evidenceFromSnapshot(plan, snapshot)
				return research.BuildTagGap(scope, evidence, 25, plan)
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Niche or product phrase")
	cmd.Flags().StringVar(&shop, "shop", "", "Target shop name")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newKeywordsInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Cluster EverBee keyword suggestions",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newKeywordsClusterCmd(flags))
	return cmd
}

func newKeywordsClusterCmd(flags *rootFlags) *cobra.Command {
	var seed string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Group keyword suggestions by shared terms",
		Example: strings.Trim("\n  everbee-pp-cli keywords cluster --seed \"wedding sign\" --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if seed == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeKeyword, Value: seed}
			researchOpts.limit = 100
			researchOpts.resources = []string{"product_analytics", "keyword_research"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				evidence, plan := evidenceFromSnapshot(plan, snapshot)
				return research.BuildKeywordClusters(scope, evidence, 25, plan)
			})
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "", "Seed keyword for clustering")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newTrendsInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Diff saved EverBee research snapshots",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTrendsDiffCmd(flags))
	return cmd
}

func newTrendsDiffCmd(flags *rootFlags) *cobra.Command {
	var query string
	var days int
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Show movement across saved research snapshots",
		Example: strings.Trim("\n  everbee-pp-cli trends diff --query \"teacher gift\" --days 30 --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if query == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeQuery, Value: query}
			researchOpts.limit = 100
			researchOpts.resources = []string{"product_analytics", "keyword_research", "shops"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				return research.BuildTrendsDiff(scope, snapshots, days, plan)
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Research phrase to diff")
	cmd.Flags().IntVar(&days, "days", 30, "Snapshot lookback window in days")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newCompetitorsInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "competitors",
		Short: "Track competitor shop changes",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCompetitorsWatchCmd(flags))
	return cmd
}

func newCompetitorsWatchCmd(flags *rootFlags) *cobra.Command {
	var shop string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "watch",
		Short:   "Summarize changed competitor shop signals",
		Example: strings.Trim("\n  everbee-pp-cli competitors watch --shop competitor-shop --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if shop == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeShop, Value: shop}
			researchOpts.limit = 50
			researchOpts.resources = []string{"shops", "product_analytics"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				return research.BuildCompetitorWatch(scope, snapshots, plan)
			})
		},
	}
	cmd.Flags().StringVar(&shop, "shop", "", "Competitor shop name or handle")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}

func newListingInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "listing",
		Short:   "Audit listings against EverBee research context",
		Example: strings.Trim("\n  everbee-pp-cli listing audit --listing-id=123456789 --json\n", "\n"),
		RunE:    parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newListingAuditCmd(flags))
	return cmd
}

func newListingAuditCmd(flags *rootFlags) *cobra.Command {
	var listingID string
	var researchOpts researchOptions
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Audit listing keyword and tag fit",
		Example: strings.Trim("\n  everbee-pp-cli listing audit --listing-id 123456789 --json\n", "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if listingID == "" {
				return cmd.Help()
			}
			scope := research.ResearchScope{Kind: research.ScopeListing, Value: listingID}
			researchOpts.limit = 25
			researchOpts.resources = []string{"product_analytics", "keyword_research"}
			return runResearchEnvelopeCommand(cmd, flags, scope, researchOpts, func(plan research.ResearchPlan, snapshot research.Snapshot, snapshots []research.Snapshot) research.ResponseEnvelope {
				evidence, plan := evidenceFromSnapshot(plan, snapshot)
				return research.BuildListingAudit(scope, evidence, 10, plan)
			})
		},
	}
	cmd.Flags().StringVar(&listingID, "listing-id", "", "Etsy listing ID")
	addResearchFlags(cmd, &researchOpts)
	return cmd
}
