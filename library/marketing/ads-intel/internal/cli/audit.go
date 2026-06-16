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
	StatusBands         map[string]float64            `json:"status_bands"`
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
	Status         string    `json:"status"`
	Score          float64   `json:"score"`
	Findings       []finding `json:"findings"`
	QuickWins      []finding `json:"quick_wins"`
	NegativeDrafts []string  `json:"negative_keyword_drafts"`
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
	totalSpend := spendByPlatform(d)
	wasted := map[string]float64{}
	for _, term := range d.SearchTerms {
		if term.Spend > 10 && term.Conversions == 0 {
			wasted[term.Platform] += term.Spend
			add(term.Platform+"_negative_keyword_candidate", "high", term.Platform, "Negative keyword draft: "+term.Term, term.Spend, "write draft artifact only; no account mutation", 1, 10)
		}
	}
	for platform, spend := range wasted {
		pct := percent(spend, totalSpend[platform])
		if pct >= .05 {
			add(platform+"_wasted_spend_terms", "high", platform, "Wasted search-term spend exceeds pass band", pct*100, "review draft negative candidates", 1, 10)
		}
	}
	zeroConv := 0
	for _, kw := range d.Keywords {
		if strings.EqualFold(kw.MatchType, "BROAD") && strings.EqualFold(kw.Bidding, "MANUAL_CPC") {
			continue
		}
		if kw.Clicks > 100 && kw.Conversions == 0 {
			zeroConv++
		}
	}
	if zeroConv > 3 {
		add("google_zero_conversion_keywords", "critical", "google", ">3 keywords have >100 clicks and 0 conversions", float64(zeroConv), "draft search-term cleanup; verify tracking first", 2, 12)
	}
	for _, c := range d.Campaigns {
		if c.Platform == "meta" && c.Frequency > 3 && c.CTR < .005 {
			add("meta_frequency_fatigue", "high", "meta", "Creative fatigue: frequency >3 and CTR <0.5%", c.Frequency, "refresh creative draft only", 3, 14)
		}
		if c.LearningPhase {
			add("meta_learning_phase_guard", "critical", c.Platform, "Campaign is in active learning phase", 1, "do not recommend edits until learning exits", 0, 0)
		}
	}
	if len(d.Campaigns) == 0 || totalSpendAll(d) == 0 {
		add("tracking_cost_rows_present", "critical", "all", "Cost rows missing; CPA/ROAS math refused", 1, "sync cost rows before optimization", 1, 0)
	}
	for i := range findings {
		if strings.Contains(findings[i].ID, "negative_keyword_candidate") {
			path, err := writeNegativeDraft(home, d.Profile, findings[i])
			if err != nil {
				return auditResult{}, err
			}
			findings[i].DraftPath = path
		}
	}
	score := scoreFindings(catalog, findings)
	sortFindings(findings)
	quick := quickWins(findings, catalog)
	drafts := []string{}
	for _, f := range findings {
		if f.DraftPath != "" {
			drafts = append(drafts, f.DraftPath)
		}
	}
	return auditResult{Status: auditStatus(score, catalog), Score: score, Findings: findings, QuickWins: quick, NegativeDrafts: drafts}, nil
}

func scoreFindings(c auditCatalog, findings []finding) float64 {
	score := 0.0
	for _, f := range findings {
		score += c.SeverityMultipliers[f.Severity] * maxf(1, f.Impact)
	}
	return score
}

func auditStatus(score float64, c auditCatalog) string {
	if score < c.StatusBands["pass_below"] {
		return "PASS"
	}
	if score < c.StatusBands["warn_below"] {
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

func writeNegativeDraft(home, profile string, f finding) (string, error) {
	dir := filepath.Join(home, "drafts", profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, f.ID+".json")
	body := map[string]any{"draft_type": "negative_keyword_candidate", "read_only": true, "finding": f, "note": "Draft artifact only. Phase 4 writes nothing to ad accounts."}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
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
