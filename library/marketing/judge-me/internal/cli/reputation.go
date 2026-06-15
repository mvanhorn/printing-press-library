// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/judge-me/internal/client"
	"github.com/spf13/cobra"
)

type reputationMetric struct {
	Value  any    `json:"value,omitempty"`
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	Note   string `json:"note,omitempty"`
}

type reputationSummary struct {
	AllReviewsCount   reputationMetric `json:"all_reviews_count"`
	AllReviewsRating  reputationMetric `json:"all_reviews_rating"`
	ShopReviewsCount  reputationMetric `json:"shop_reviews_count"`
	ShopReviewsRating reputationMetric `json:"shop_reviews_rating"`
	ShopInfo          any              `json:"shop_info,omitempty"`
	Settings          any              `json:"settings,omitempty"`
	LocalReviewStats  *reviewStats     `json:"local_review_stats,omitempty"`
	LocalReviewNote   string           `json:"local_review_note,omitempty"`
}

type reviewStats struct {
	Count          int     `json:"count"`
	AverageRating  float64 `json:"average_rating,omitempty"`
	LowRatingCount int     `json:"low_rating_count"`
	LowRatingRate  float64 `json:"low_rating_rate,omitempty"`
	VerifiedCount  int     `json:"verified_count"`
	VerifiedRate   float64 `json:"verified_rate,omitempty"`
}

type productQualityStat struct {
	ProductKey      string   `json:"product_key"`
	ProductTitle    string   `json:"product_title,omitempty"`
	ProductHandle   string   `json:"product_handle,omitempty"`
	ExternalID      string   `json:"external_id,omitempty"`
	ReviewCount     int      `json:"review_count"`
	AverageRating   float64  `json:"average_rating,omitempty"`
	LowRatingCount  int      `json:"low_rating_count"`
	LowRatingRate   float64  `json:"low_rating_rate,omitempty"`
	VerifiedCount   int      `json:"verified_count"`
	VerifiedRate    float64  `json:"verified_rate,omitempty"`
	NewestReviewAt  string   `json:"newest_review_at,omitempty"`
	SampleReviewIDs []string `json:"sample_low_rating_review_ids,omitempty"`
}

type moderationCandidate struct {
	ID            string   `json:"id,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Title         string   `json:"title,omitempty"`
	ProductTitle  string   `json:"product_title,omitempty"`
	ProductHandle string   `json:"product_handle,omitempty"`
	Curated       string   `json:"curated,omitempty"`
	Published     *bool    `json:"published,omitempty"`
	Hidden        *bool    `json:"hidden,omitempty"`
	Verified      string   `json:"verified,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	Reasons       []string `json:"reasons"`
}

type settingFinding struct {
	Key      string `json:"key"`
	Value    any    `json:"value,omitempty"`
	Impact   string `json:"impact"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

type productEvidence struct {
	Input               map[string]string `json:"input"`
	ReviewCount         any               `json:"review_count,omitempty"`
	RatingHistogram     map[string]any    `json:"rating_histogram,omitempty"`
	PreviewBadge        map[string]any    `json:"preview_badge,omitempty"`
	ProductReviewWidget map[string]any    `json:"product_review_widget,omitempty"`
	Notes               []string          `json:"notes,omitempty"`
}

func newReputationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reputation",
		Short:       "Trust and reputation dashboards built from Judge.me reviews",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReputationSummaryCmd(flags))
	cmd.AddCommand(newReputationProductsCmd(flags))
	cmd.AddCommand(newReputationModerationQueueCmd(flags))
	cmd.AddCommand(newReputationSettingsAuditCmd(flags))
	cmd.AddCommand(newReputationProductCmd(flags))
	return cmd
}

func newReputationSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "summary",
		Short:       "Summarize shop review trust signals",
		Example:     "  judge-me-pp-cli reputation summary --json\n  judge-me-pp-cli reputation summary --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out := reputationSummary{}
			out.AllReviewsCount = fetchMetric(ctx, c, "/widgets/all_reviews_count")
			out.AllReviewsRating = fetchMetric(ctx, c, "/widgets/all_reviews_rating")
			out.ShopReviewsCount = fetchMetric(ctx, c, "/widgets/shop_reviews_count")
			out.ShopReviewsRating = fetchMetric(ctx, c, "/widgets/shop_reviews_rating")
			if data, err := c.Get(ctx, "/shops/info", nil); err == nil {
				out.ShopInfo = decodeAny(data)
			}
			if data, err := c.Get(ctx, reputationSettingsPath(), nil); err == nil {
				out.Settings = decodeAny(data)
			}
			if db, err := openStoreForRead(ctx, "judge-me-pp-cli"); err != nil {
				out.LocalReviewNote = fmt.Sprintf("local review store unavailable: %v", err)
			} else if db != nil {
				defer db.Close()
				if items, err := db.List("reviews", 0); err == nil {
					st := reviewStatsFromRaw(items)
					out.LocalReviewStats = &st
				} else {
					out.LocalReviewNote = fmt.Sprintf("local review stats unavailable: %v", err)
				}
			} else {
				out.LocalReviewNote = "local review store not found; run 'judge-me-pp-cli sync --resources reviews' to include local review stats"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newReputationProductsCmd(flags *rootFlags) *cobra.Command {
	var limit, minReviews int
	cmd := &cobra.Command{
		Use:         "products",
		Short:       "Rank products by low-rating and verification risk from synced reviews",
		Example:     "  judge-me-pp-cli sync --resources reviews --max-pages 1\n  judge-me-pp-cli reputation products --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openStoreForRead(ctx, "judge-me-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local Judge.me store found; run 'judge-me-pp-cli sync reviews' first"))
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "reviews", flags.maxAge)
			items, err := db.List("reviews", 0)
			if err != nil {
				return err
			}
			stats := productQualityStatsFromReviews(items, minReviews)
			if limit > 0 && len(stats) > limit {
				stats = stats[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum products to return")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 1, "Minimum review count per product")
	return cmd
}

func newReputationModerationQueueCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var maxRating float64
	cmd := &cobra.Command{
		Use:         "moderation-queue",
		Short:       "Find low-rated or uncurated reviews that may need attention",
		Example:     "  judge-me-pp-cli reputation moderation-queue --max-rating 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openStoreForRead(ctx, "judge-me-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local Judge.me store found; run 'judge-me-pp-cli sync reviews' first"))
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "reviews", flags.maxAge)
			items, err := db.List("reviews", 0)
			if err != nil {
				return err
			}
			candidates := moderationCandidatesFromReviews(items, maxRating)
			if limit > 0 && len(candidates) > limit {
				candidates = candidates[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), candidates, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum reviews to return")
	cmd.Flags().Float64Var(&maxRating, "max-rating", 2, "Rating threshold for low-rating attention")
	return cmd
}

func newReputationSettingsAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "settings-audit",
		Short:       "Audit Judge.me settings that affect trust presentation",
		Example:     "  judge-me-pp-cli reputation settings-audit --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, reputationSettingsPath(), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), auditSettings(data), flags)
		},
	}
	return cmd
}

func newReputationProductCmd(flags *rootFlags) *cobra.Command {
	var productID, handle, externalID string
	cmd := &cobra.Command{
		Use:         "product",
		Short:       "Show product-level review count and widget evidence",
		Example:     "  judge-me-pp-cli reputation product --handle example-product --json\n  judge-me-pp-cli reputation product --product-id 123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if productID == "" && handle == "" && externalID == "" {
				return usageErr(fmt.Errorf("one of --product-id, --handle, or --external-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ev := productEvidence{Input: map[string]string{}, RatingHistogram: map[string]any{}}
			params := map[string]string{}
			if productID != "" {
				params["product_id"] = productID
				ev.Input["product_id"] = productID
			}
			widgetParams := map[string]string{}
			if productID != "" {
				widgetParams["id"] = productID
			}
			if handle != "" {
				widgetParams["handle"] = handle
				ev.Input["handle"] = handle
			}
			if externalID != "" {
				widgetParams["external_id"] = externalID
				ev.Input["external_id"] = externalID
			}
			if productID != "" {
				if data, err := c.Get(ctx, "/reviews/count", params); err == nil {
					ev.ReviewCount = decodeAny(data)
				}
				for i := 1; i <= 5; i++ {
					p := map[string]string{"product_id": productID, "rating": strconv.Itoa(i)}
					if data, err := c.Get(ctx, "/reviews/count", p); err == nil {
						ev.RatingHistogram[strconv.Itoa(i)] = decodeAny(data)
					}
				}
			} else {
				ev.Notes = append(ev.Notes, "review count histogram requires --product-id; handle/external-id widget evidence is still fetched")
			}
			if data, err := c.Get(ctx, "/widgets/preview_badge", widgetParams); err == nil {
				ev.PreviewBadge = widgetEvidence(data)
			}
			if data, err := c.Get(ctx, "/widgets/product_review", widgetParams); err == nil {
				ev.ProductReviewWidget = widgetEvidence(data)
			}
			return printJSONFiltered(cmd.OutOrStdout(), ev, flags)
		},
	}
	cmd.Flags().StringVar(&productID, "product-id", "", "Judge.me internal product ID")
	cmd.Flags().StringVar(&handle, "handle", "", "Product handle")
	cmd.Flags().StringVar(&externalID, "external-id", "", "External platform product ID")
	return cmd
}

func fetchMetric(ctx context.Context, c *client.Client, path string) reputationMetric {
	data, err := c.Get(ctx, path, nil)
	if err != nil {
		return reputationMetric{Source: "live", Path: path, Note: err.Error()}
	}
	return reputationMetric{Value: decodeAny(data), Source: "live", Path: path}
}

func decodeAny(data []byte) any {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	return v
}

func widgetEvidence(data []byte) map[string]any {
	out := map[string]any{"bytes": len(data), "available": len(data) > 0}
	var v any
	if err := json.Unmarshal(data, &v); err == nil {
		out["json"] = v
	}
	return out
}

func reviewStatsFromRaw(items []json.RawMessage) reviewStats {
	var st reviewStats
	var sum float64
	for _, raw := range flattenReviewItems(items) {
		r := reviewMap(raw)
		if len(r) == 0 {
			continue
		}
		st.Count++
		rating := numField(r, "rating")
		sum += rating
		if rating > 0 && rating <= 2 {
			st.LowRatingCount++
		}
		if isVerified(strField(r, "verified")) {
			st.VerifiedCount++
		}
	}
	if st.Count > 0 {
		st.AverageRating = round2(sum / float64(st.Count))
		st.LowRatingRate = round2(float64(st.LowRatingCount) / float64(st.Count) * 100)
		st.VerifiedRate = round2(float64(st.VerifiedCount) / float64(st.Count) * 100)
	}
	return st
}

func productQualityStatsFromReviews(items []json.RawMessage, minReviews int) []productQualityStat {
	groups := map[string]*productQualityStat{}
	sums := map[string]float64{}
	for _, raw := range flattenReviewItems(items) {
		r := reviewMap(raw)
		if len(r) == 0 {
			continue
		}
		key := firstNonEmpty(strField(r, "product_external_id"), strField(r, "product_handle"), strField(r, "product_title"), "unknown")
		g := groups[key]
		if g == nil {
			g = &productQualityStat{ProductKey: key, ProductTitle: strField(r, "product_title"), ProductHandle: strField(r, "product_handle"), ExternalID: strField(r, "product_external_id")}
			groups[key] = g
		}
		rating := numField(r, "rating")
		g.ReviewCount++
		sums[key] += rating
		if rating > 0 && rating <= 2 {
			g.LowRatingCount++
			if len(g.SampleReviewIDs) < 3 {
				g.SampleReviewIDs = append(g.SampleReviewIDs, strField(r, "id"))
			}
		}
		if isVerified(strField(r, "verified")) {
			g.VerifiedCount++
		}
		if ts := strField(r, "created_at"); ts > g.NewestReviewAt {
			g.NewestReviewAt = ts
		}
	}
	out := make([]productQualityStat, 0, len(groups))
	for k, g := range groups {
		if g.ReviewCount < minReviews {
			continue
		}
		g.AverageRating = round2(sums[k] / float64(g.ReviewCount))
		g.LowRatingRate = round2(float64(g.LowRatingCount) / float64(g.ReviewCount) * 100)
		g.VerifiedRate = round2(float64(g.VerifiedCount) / float64(g.ReviewCount) * 100)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LowRatingRate == out[j].LowRatingRate {
			return out[i].ReviewCount > out[j].ReviewCount
		}
		return out[i].LowRatingRate > out[j].LowRatingRate
	})
	return out
}

func moderationCandidatesFromReviews(items []json.RawMessage, maxRating float64) []moderationCandidate {
	var out []moderationCandidate
	for _, raw := range flattenReviewItems(items) {
		r := reviewMap(raw)
		if len(r) == 0 {
			continue
		}
		reasons := []string{}
		rating := numField(r, "rating")
		curated := strField(r, "curated")
		if rating > 0 && rating <= maxRating {
			reasons = append(reasons, "low_rating")
		}
		if curated == "not-yet" || curated == "" {
			reasons = append(reasons, "not_curated")
		}
		if curated == "spam" {
			reasons = append(reasons, "marked_spam")
		}
		if b, ok := boolField(r, "hidden"); ok && b {
			reasons = append(reasons, "hidden")
		}
		if len(reasons) == 0 {
			continue
		}
		cand := moderationCandidate{ID: strField(r, "id"), Rating: rating, Title: strField(r, "title"), ProductTitle: strField(r, "product_title"), ProductHandle: strField(r, "product_handle"), Curated: curated, Verified: strField(r, "verified"), CreatedAt: strField(r, "created_at"), Reasons: reasons}
		if b, ok := boolField(r, "published"); ok {
			cand.Published = &b
		}
		if b, ok := boolField(r, "hidden"); ok {
			cand.Hidden = &b
		}
		out = append(out, cand)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rating < out[j].Rating })
	return out
}

func auditSettings(data []byte) []settingFinding {
	obj := map[string]any{}
	_ = json.Unmarshal(data, &obj)
	if nested, ok := obj["settings"].(map[string]any); ok {
		obj = nested
	}
	findings := []settingFinding{}
	add := func(key, impact, severity string) {
		if v, ok := obj[key]; ok {
			findings = append(findings, settingFinding{Key: key, Value: v, Impact: impact, Severity: severity, Source: "/settings"})
		}
	}
	add("autopublish", "Controls whether approved reviews appear without manual publishing delay.", "info")
	add("enable_review_pictures", "Visual UGC can increase social proof when enabled.", "info")
	add("widget_star_color", "Star color affects storefront trust presentation consistency.", "info")
	add("admin_email", "Operational owner for review notifications; treat as PII.", "info")
	return findings
}

func flattenReviewItems(items []json.RawMessage) []json.RawMessage {
	var out []json.RawMessage
	for _, raw := range items {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil {
			if arr, ok := obj["reviews"]; ok {
				var rows []json.RawMessage
				if json.Unmarshal(arr, &rows) == nil {
					out = append(out, rows...)
					continue
				}
			}
		}
		out = append(out, raw)
	}
	return out
}
func reviewMap(raw json.RawMessage) map[string]any {
	var r map[string]any
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	return r
}
func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.Itoa(t)
		}
	}
	return ""
}
func numField(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return t
		case int:
			return float64(t)
		case string:
			f, _ := strconv.ParseFloat(t, 64)
			return f
		}
	}
	return 0
}
func boolField(m map[string]any, key string) (bool, bool) {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			return t == "true", t == "true" || t == "false"
		}
	}
	return false, false
}
func isVerified(v string) bool {
	v = strings.ToLower(v)
	switch v {
	case "confirmed-buyer", "buyer", "verified-purchase", "semi-verified-purchase", "admin":
		return true
	default:
		return false
	}
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func round2(f float64) float64 { n, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", f), 64); return n }

func reputationSettingsPath() string {
	return "/settings?setting_keys[]=autopublish&setting_keys[]=enable_review_pictures&setting_keys[]=widget_star_color&setting_keys[]=admin_email"
}
