package cli

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/ads-intel/internal/store"
)

//go:embed audit_catalog.json
var catalogFS embed.FS

type auditCatalog struct {
	SchemaVersion       string                        `json:"schema_version"`
	SeverityMultipliers map[string]float64            `json:"severity_multipliers"`
	WasteBandsPct       map[string]float64            `json:"waste_bands_pct"`
	HealthBands         map[string]float64            `json:"health_bands"`
	Categories          map[string]map[string]float64 `json:"categories"`
	Checks              []catalogCheck                `json:"checks"`
}

type catalogCheck struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

type finding struct {
	ID         string  `json:"id"`
	Severity   string  `json:"severity"`
	Platform   string  `json:"platform"`
	Title      string  `json:"title"`
	Impact     float64 `json:"impact"`
	Action     string  `json:"action"`
	Owner      string  `json:"owner"`
	ETADays    int     `json:"eta_days"`
	FixMinutes int     `json:"fix_minutes"`
	DraftPath  string  `json:"draft_path,omitempty"`
}

type auditResult struct {
	Status         string            `json:"status"`
	Score          float64           `json:"score"`
	ScoreScale     string            `json:"score_scale"`
	Grade          string            `json:"grade"`
	CheckVerdicts  map[string]string `json:"check_verdicts"`
	Findings       []finding         `json:"findings"`
	QuickWins      []finding         `json:"quick_wins"`
	NegativeDrafts []string          `json:"negative_keyword_drafts"`
}

func loadCatalog() (auditCatalog, error) {
	var c auditCatalog
	b, err := catalogFS.ReadFile("audit_catalog.json")
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

func runAudit(d store.DataSet, home string) (auditResult, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return auditResult{}, err
	}
	findings := []finding{}
	add := func(id, severity, platform, title string, impact float64, action string, eta, mins int) {
		findings = append(findings, finding{ID: id, Severity: severity, Platform: platform, Title: title, Impact: impact, Action: action, Owner: platform + "-owner", ETADays: eta, FixMinutes: mins})
	}
	// --- Signals (computed once, drive both findings and check verdicts) ---
	totalSpend := spendByPlatform(d)
	wasted := map[string]float64{}
	for _, term := range d.SearchTerms {
		if term.Spend > 10 && term.Conversions == 0 {
			wasted[term.Platform] += term.Spend
			add(term.Platform+"_negative_keyword_candidate", "high", term.Platform, "Negative keyword draft: "+term.Term, term.Spend, "write draft artifact only; no account mutation", 1, 10)
		}
	}
	wastedPct := map[string]float64{}
	for platform, spend := range wasted {
		wastedPct[platform] = percent(spend, totalSpend[platform]) * 100
	}
	zeroConv := 0
	for _, kw := range d.Keywords {
		// Legacy BMM (broad + manual CPC) behaves like phrase match — never flag it.
		if strings.EqualFold(kw.MatchType, "BROAD") && strings.EqualFold(kw.Bidding, "MANUAL_CPC") {
			continue
		}
		if kw.Clicks > 100 && kw.Conversions == 0 {
			zeroConv++
		}
	}
	metaFatigue := false
	learningActive := false
	for _, c := range d.Campaigns {
		if c.Platform == "meta" && c.Frequency > 3 && c.CTR < .005 {
			metaFatigue = true
		}
		if c.LearningPhase {
			learningActive = true
		}
	}
	trackingMissing := len(d.Campaigns) == 0 || totalSpendAll(d) == 0

	// --- Findings (the problem list driving the report, quick-wins, drafts) ---
	for platform, pct := range wastedPct {
		if pct >= catalog.WasteBandsPct["pass_below"] {
			add(platform+"_wasted_spend_terms", "high", platform, "Wasted search-term spend exceeds pass band", pct, "review draft negative candidates", 1, 10)
		}
	}
	if zeroConv > 3 {
		add("google_zero_conversion_keywords", "critical", "google", ">3 keywords have >100 clicks and 0 conversions", float64(zeroConv), "draft search-term cleanup; verify tracking first", 2, 12)
	}
	if metaFatigue {
		add("meta_frequency_fatigue", "high", "meta", "Creative fatigue: frequency >3 and CTR <0.5%", 1, "refresh creative draft only", 3, 14)
	}
	if learningActive {
		add("meta_learning_phase_guard", "critical", "meta", "Campaign is in active learning phase", 1, "do not recommend edits until learning exits", 0, 0)
	}
	if trackingMissing {
		add("tracking_cost_rows_present", "critical", "all", "Cost rows missing; CPA/ROAS math refused", 1, "sync cost rows before optimization", 1, 0)
	}
	for i := range findings {
		if strings.Contains(findings[i].ID, "negative_keyword_candidate") {
			path, err := writeNegativeDraft(home, d, findings[i])
			if err != nil {
				return auditResult{}, err
			}
			findings[i].DraftPath = path
		}
	}

	// --- Check verdicts → weighted 0-100 health score ---
	verdicts := map[string]string{
		"google_wasted_spend_terms":       wasteVerdict(wastedPct["google"], catalog),
		"amazon_wasted_spend_terms":       wasteVerdict(wastedPct["amazon"], catalog),
		"google_zero_conversion_keywords": passFail(zeroConv <= 3),
		"meta_frequency_fatigue":          passFail(!metaFatigue),
		"tracking_cost_rows_present":      passFail(!trackingMissing),
		// Checks we cannot evaluate from available data are marked N/A and
		// excluded from the score rather than fabricated as passing.
		"google_broad_smart_bidding":   "na",
		"google_shared_negative_lists": "na",
		"meta_learning_phase_guard":    "na", // advisory guard, not a health dimension
	}
	score := healthScore(catalog, verdicts)

	sortFindings(findings)
	quick := quickWins(findings, catalog)
	drafts := []string{}
	for _, f := range findings {
		if f.DraftPath != "" {
			drafts = append(drafts, f.DraftPath)
		}
	}
	return auditResult{
		Status:         healthStatus(score, catalog),
		Score:          score,
		ScoreScale:     "0-100 health (higher is better)",
		Grade:          grade(score),
		CheckVerdicts:  verdicts,
		Findings:       findings,
		QuickWins:      quick,
		NegativeDrafts: drafts,
	}, nil
}

// healthScore computes a 0-100 weighted health score (higher is better) over
// the catalog checks, weighting each by severity x category weight. N/A checks
// are excluded from both numerator and denominator.
func healthScore(c auditCatalog, verdicts map[string]string) float64 {
	achieved, possible := 0.0, 0.0
	for _, chk := range c.Checks {
		v := verdicts[chk.ID]
		if v == "" || v == "na" {
			continue
		}
		w := c.SeverityMultipliers[chk.Severity] * categoryWeight(c, chk.Platform, chk.Category)
		if w <= 0 {
			continue
		}
		possible += w
		switch v {
		case "pass":
			achieved += w
		case "warn":
			achieved += w * 0.5
		}
	}
	if possible == 0 {
		return 100
	}
	return 100 * achieved / possible
}

// categoryWeight resolves a check's category weight. Platform "all" uses the
// mean of that category across the per-platform weight tables.
func categoryWeight(c auditCatalog, platform, category string) float64 {
	if platform == "all" {
		sum, n := 0.0, 0
		for _, cats := range c.Categories {
			if w, ok := cats[category]; ok {
				sum += w
				n++
			}
		}
		if n > 0 {
			return sum / float64(n)
		}
		return 0
	}
	if cats, ok := c.Categories[platform]; ok {
		return cats[category]
	}
	return 0
}

func wasteVerdict(pct float64, c auditCatalog) string {
	if pct < c.WasteBandsPct["pass_below"] {
		return "pass"
	}
	if pct < c.WasteBandsPct["warn_below"] {
		return "warn"
	}
	return "fail"
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func grade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func healthStatus(score float64, c auditCatalog) string {
	if score >= c.HealthBands["pass_min"] {
		return "PASS"
	}
	if score >= c.HealthBands["warn_min"] {
		return "WARN"
	}
	return "FAIL"
}

func quickWins(findings []finding, c auditCatalog) []finding {
	out := []finding{}
	for _, f := range findings {
		if (f.Severity == "high" || f.Severity == "critical") && f.FixMinutes < 15 && f.FixMinutes > 0 {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return c.SeverityMultipliers[out[i].Severity]*out[i].Impact > c.SeverityMultipliers[out[j].Severity]*out[j].Impact
	})
	return out
}

func writeNegativeDraft(home string, d store.DataSet, f finding) (string, error) {
	dir := filepath.Join(home, "drafts", d.Profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, f.ID+".json")
	target, evidence := buildNegativeDraftTarget(d, f)
	body := map[string]any{
		"draft_type": "negative_keyword_candidate",
		"read_only":  true,
		"finding":    f,
		"target":     target,
		"evidence":   evidence,
		"apply_constraints": map[string]any{
			"platform":              "google",
			"mutation":              "add_exact_negative_keyword",
			"allowed_match_type":    "EXACT",
			"minimum_spend":         10,
			"required_conversions":  0,
			"preferred_scope_order": []string{"ad_group", "campaign"},
		},
		"note": "Draft artifact only until an explicit apply dry-run or live approval. Live apply requires --live-approved and typed confirmation.",
	}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}

func buildNegativeDraftTarget(d store.DataSet, f finding) (map[string]any, map[string]any) {
	term := strings.TrimPrefix(f.Title, "Negative keyword draft: ")
	target := map[string]any{
		"platform":     f.Platform,
		"customer_id":  d.Account.AccountID,
		"keyword_text": term,
		"match_type":   "EXACT",
	}
	evidence := map[string]any{"source_check": "wasted_spend", "minimum_spend": 10, "required_conversions": 0}
	for _, row := range d.SearchTerms {
		if row.Platform == f.Platform && strings.EqualFold(row.Term, term) {
			target["campaign_id"] = row.CampaignID
			target["campaign_name"] = row.CampaignName
			target["ad_group_id"] = row.AdGroupID
			target["ad_group_name"] = row.AdGroupName
			evidence["spend"] = row.Spend
			evidence["conversions"] = row.Conversions
			evidence["clicks"] = row.Clicks
			break
		}
	}
	return target, evidence
}

func spendByPlatform(d store.DataSet) map[string]float64 {
	out := map[string]float64{}
	for _, c := range d.Campaigns {
		out[c.Platform] += c.Spend
	}
	return out
}

func totalSpendAll(d store.DataSet) float64 {
	total := 0.0
	for _, c := range d.Campaigns {
		total += c.Spend
	}
	return total
}

func percent(n, d float64) float64 {
	if d <= 0 {
		return 0
	}
	return n / d
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sortFindings(findings []finding) {
	sort.Slice(findings, func(i, j int) bool { return findings[i].Impact > findings[j].Impact })
}

func catalogCheckIDs() map[string]bool {
	c, err := loadCatalog()
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, check := range c.Checks {
		out[check.ID] = true
	}
	return out
}

func formatFinding(f finding) string {
	return fmt.Sprintf("%s\t%s\t%s\t%.1f\t%s", f.Platform, f.Severity, f.Title, f.Impact, f.Action)
}
