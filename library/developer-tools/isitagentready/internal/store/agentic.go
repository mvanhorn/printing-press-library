// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
)

// AgenticReport is the parsed shape of a GET /api/v1/report response from the
// is-agentic.com scanner. Unlike isitagentready's fixed Level 0-5, this scanner
// reports a score 0-100 over a per-site floating denominator
// (eligible_checks), which is why the two scales are never merged.
type AgenticReport struct {
	Target         string           `json:"target"`
	DisplayTarget  string           `json:"display_target"`
	ReportURL      string           `json:"report_url"`
	Score          int              `json:"score"`
	ScoreLabel     string           `json:"score_label"`
	ScannedAt      string           `json:"scanned_at"`
	EligibleChecks int              `json:"eligible_checks"`
	ScoreBreakdown AgenticBreakdown `json:"score_breakdown"`
	Issues         []AgenticIssue   `json:"issues"`
}

// AgenticTier is one score_breakdown tier (essential or recommended). Earned
// and Available are floats — the API reports 63.5 / 17.9, not whole numbers.
type AgenticTier struct {
	Earned    float64 `json:"earned"`
	Available float64 `json:"available"`
	Passing   int     `json:"passing"`
	Total     int     `json:"total"`
}

// AgenticBonus carries the bonus-tier points; Points is a float like the
// other tiers.
type AgenticBonus struct {
	Points          float64 `json:"points"`
	PositiveSignals int     `json:"positive_signals"`
}

// AgenticBreakdown groups the essential, recommended, and bonus tiers.
type AgenticBreakdown struct {
	Essential   AgenticTier  `json:"essential"`
	Recommended AgenticTier  `json:"recommended"`
	Bonus       AgenticBonus `json:"bonus"`
}

// AgenticIssue is one check the scanner evaluated, but issues[] carries ONLY
// non-passing checks (result is "failed" or "partial"); a check absent from
// issues[] is passing.
type AgenticIssue struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Tier           string `json:"tier"`
	Result         string `json:"result"`
	Details        string `json:"details"`
	Recommendation string `json:"recommendation"`
}

// ParseAgenticReport decodes a raw GET /api/v1/report response.
func ParseAgenticReport(raw json.RawMessage) (*AgenticReport, error) {
	var r AgenticReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing is-agentic report: %w", err)
	}
	return &r, nil
}

// AgenticStatus returns a normalized status for one is-agentic check id:
// "pass", "partial" or "fail".
//
// is-agentic's issues[] carries ONLY non-passing checks. Verified against
// score_breakdown on three sites — the expected non-passing count
// (essential.total-essential.passing)+(recommended.total-recommended.passing)
// matched len(issues) exactly every time:
//
//	vercel.com  essential 8/11  recommended 17/22  -> 8 expected,  8 issues
//	stripe.com  essential 6/11  recommended 11/21  -> 15 expected, 15 issues
//	sqlite.org  essential 2/7   recommended 3/11   -> 13 expected, 13 issues
//
// So an id absent from issues[] is passing. The same numbers show the tier
// totals themselves vary per site, which is the floating denominator that
// makes the two scanners' scores incomparable.
func (r *AgenticReport) AgenticStatus(id string) string {
	for _, iss := range r.Issues {
		if iss.ID == id {
			switch iss.Result {
			case "partial":
				return "partial"
			default:
				return "fail"
			}
		}
	}
	return "pass"
}

// FailingIssues returns the issues whose result is not passing, sorted by tier
// (essential first) then id.
func (r *AgenticReport) FailingIssues() []AgenticIssue {
	rank := map[string]int{"essential": 0, "recommended": 1}
	out := make([]AgenticIssue, 0, len(r.Issues))
	for _, iss := range r.Issues {
		if iss.Result == "pass" {
			continue
		}
		out = append(out, iss)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rank[out[i].Tier], rank[out[j].Tier]
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// AgenticOpenItems maps every non-passing issue into the shared OpenItem advice
// shape. issues[].recommendation is a copy-paste fix prompt, directly analogous
// to isitagentready's nextLevel.requirements[].prompt, so `advice` and
// `open-advice` render both scanners through one code path.
func AgenticOpenItems(r *AgenticReport) []OpenItem {
	score := r.Score // local copy so &score stays stable (never the loop var)
	out := make([]OpenItem, 0, len(r.Issues))
	for _, iss := range r.FailingIssues() {
		desc := iss.Name
		if iss.Details != "" {
			desc = iss.Name + " — " + iss.Details
		}
		out = append(out, OpenItem{
			URL:         r.Target,
			Source:      SourceIsAgentic,
			Score:       &score,
			ScoreLabel:  r.ScoreLabel,
			Check:       iss.ID,
			Description: desc,
			Prompt:      iss.Recommendation,
		})
	}
	return out
}

// CrossCheck pairs one isitagentready check id with the is-agentic check id
// that measures the same thing.
type CrossCheck struct {
	IsItAgentReady string
	IsAgentic      string
	Label          string
}

// CrossCheckPairs is a deliberately conservative overlap table. It was derived
// from the real id sets of both scanners: isitagentready's 20 ids across five
// categories, and 26 is-agentic ids observed by probing seven sites. Only these
// four pairs measure the same property, so only these four can carry an
// agree/disagree verdict.
//
// Deliberately EXCLUDED near-misses — do not "fix" these omissions:
//   - apiCatalog vs openapi-spec: apiCatalog is RFC 9727 .well-known/api-catalog,
//     a catalog *of* APIs ("API Catalog found with 2 APIs listed"); openapi-spec
//     is the OpenAPI document itself. A site can publish either without the other.
//   - robotsTxtAiRules vs bot-detection: robots.txt AI directives are not WAF blocking.
//   - robotsTxt vs agent-crawler-reachability: reachability is strictly broader.
//   - oauthProtectedResource vs scoped-permissions: RFC 9728 metadata is not scope granularity.
var CrossCheckPairs = []CrossCheck{
	{"sitemap", "sitemap", "Sitemap published"},
	{"markdownNegotiation", "markdown-negotiation-vary", "Markdown content negotiation"},
	{"mcpServerCard", "mcp-server", "MCP server manifest"},
	{"oauthDiscovery", "oauth-support", "OAuth 2.0 discovery"},
}

// CrossRefIsItAgentReady is isitagentready.com's native verdict.
type CrossRefIsItAgentReady struct {
	Level     int    `json:"level"`
	LevelName string `json:"levelName"`
	Pass      int    `json:"pass"`
	Fail      int    `json:"fail"`
	Neutral   int    `json:"neutral"`
	Total     int    `json:"total"`
	ScannedAt string `json:"scannedAt"`
	SiteError bool   `json:"siteError,omitempty"`
}

// CrossRefIsAgentic is is-agentic.com's native verdict. EligibleChecks is a
// per-site floating denominator, which is why the two scores are never merged.
type CrossRefIsAgentic struct {
	Score              int     `json:"score"`
	ScoreLabel         string  `json:"scoreLabel"`
	EligibleChecks     int     `json:"eligibleChecks"`
	EssentialPassing   int     `json:"essentialPassing"`
	EssentialTotal     int     `json:"essentialTotal"`
	RecommendedPassing int     `json:"recommendedPassing"`
	RecommendedTotal   int     `json:"recommendedTotal"`
	BonusPoints        float64 `json:"bonusPoints"`
	ScannedAt          string  `json:"scannedAt"`
}

// CrossRefAgreement is one genuinely-overlapping check seen by both scanners.
type CrossRefAgreement struct {
	Label                string `json:"label"`
	IsItAgentReadyCheck  string `json:"isitagentreadyCheck"`
	IsAgenticCheck       string `json:"isAgenticCheck"`
	IsItAgentReadyStatus string `json:"isitagentreadyStatus"`
	IsAgenticStatus      string `json:"isAgenticStatus"`
	Verdict              string `json:"verdict"` // agree | partial | disagree | unknown
}

// CrossRefResult is both scanners' native verdicts side by side plus the
// overlap table. It deliberately carries NO merged or rescaled score.
type CrossRefResult struct {
	URL            string                  `json:"url"`
	IsItAgentReady *CrossRefIsItAgentReady `json:"isitagentready"`
	IsAgentic      *CrossRefIsAgentic      `json:"isAgentic"`
	Overlap        []CrossRefAgreement     `json:"overlap"`
	Notes          []string                `json:"notes"`
}

// BuildCrossRef assembles both native verdicts and the overlap verdicts.
// Either report may be nil (that scanner had no data); the corresponding
// pointer is then nil and every overlap verdict is "unknown".
func BuildCrossRef(url string, iar *Report, ag *AgenticReport) CrossRefResult {
	res := CrossRefResult{URL: url, Overlap: []CrossRefAgreement{}}
	if iar == nil && ag == nil {
		return res
	}
	var iarStatus func(string) string
	var agStatus func(string) string
	if iar != nil {
		pass, fail, neutral, total := iar.Counts()
		cr := &CrossRefIsItAgentReady{
			Level: iar.Level, LevelName: iar.LevelName,
			Pass: pass, Fail: fail, Neutral: neutral, Total: total,
			ScannedAt: iar.ScannedAt,
		}
		if iar.SiteError != nil {
			cr.SiteError = true
		}
		res.IsItAgentReady = cr
		st := iar.statusMap()
		iarStatus = func(id string) string {
			s, ok := st[id]
			if !ok {
				return "-"
			}
			return s
		}
	}
	if ag != nil {
		cr := &CrossRefIsAgentic{
			Score: ag.Score, ScoreLabel: ag.ScoreLabel, EligibleChecks: ag.EligibleChecks,
			EssentialPassing:   ag.ScoreBreakdown.Essential.Passing,
			EssentialTotal:     ag.ScoreBreakdown.Essential.Total,
			RecommendedPassing: ag.ScoreBreakdown.Recommended.Passing,
			RecommendedTotal:   ag.ScoreBreakdown.Recommended.Total,
			BonusPoints:        ag.ScoreBreakdown.Bonus.Points,
			ScannedAt:          ag.ScannedAt,
		}
		res.IsAgentic = cr
		agStatus = func(id string) string { return ag.AgenticStatus(id) }
	}

	for _, pair := range CrossCheckPairs {
		iarS := "-"
		if iarStatus != nil {
			iarS = iarStatus(pair.IsItAgentReady)
		}
		agS := "-"
		if agStatus != nil {
			agS = agStatus(pair.IsAgentic)
		}
		verdict := "unknown"
		if iarS != "-" && agS != "-" {
			verdict = crossVerdict(iarS, agS)
		}
		res.Overlap = append(res.Overlap, CrossRefAgreement{
			Label:                pair.Label,
			IsItAgentReadyCheck:  pair.IsItAgentReady,
			IsAgenticCheck:       pair.IsAgentic,
			IsItAgentReadyStatus: iarS,
			IsAgenticStatus:      agS,
			Verdict:              verdict,
		})
	}

	// Always state the two scales are not comparable: they use different
	// denominators (fixed 5-category set vs per-site eligible checks), so only
	// the listed overlapping checks can carry a verdict.
	res.Notes = append(res.Notes, "isitagentready reports a Level 0-5 over five fixed categories; is-agentic reports a score 0-100 over a per-site floating denominator. The two scales are not comparable and are never merged; only the listed overlapping checks carry an agree/disagree verdict.")

	// Staleness guard: is-agentic reports can be arbitrarily old (sqlite.org
	// was 2 days stale). When the two scans are far apart in time, flag it so a
	// disagreement is not misread as a live regression.
	if iar != nil && ag != nil && iar.ScannedAt != "" && ag.ScannedAt != "" {
		ti := cliutil.ParseStoredTime(iar.ScannedAt)
		ta := cliutil.ParseStoredTime(ag.ScannedAt)
		if !ti.IsZero() && !ta.IsZero() {
			// Absolute gap: either scanner can be the stale one. is-agentic
			// serves cached reports (sqlite.org was 2 days old) and the local
			// isitagentready scan can equally be the older of the two.
			diff := ti.Sub(ta)
			if diff < 0 {
				diff = -diff
			}
			if diff > 24*time.Hour {
				res.Notes = append(res.Notes, fmt.Sprintf("the two scans are %s apart (isitagentready %s, is-agentic %s); they may disagree simply because they describe different points in time.", diff.Round(time.Hour), iar.ScannedAt, ag.ScannedAt))
			}
		}
	}
	return res
}

// crossVerdict ranks each side — pass=2, neutral/partial=1, fail=0 — then:
// equal rank -> agree; difference of 1 -> partial; difference of 2 -> disagree.
func crossVerdict(iarStatus, agStatus string) string {
	rank := func(s string) int {
		switch s {
		case "pass":
			return 2
		case "neutral", "partial":
			return 1
		default: // fail
			return 0
		}
	}
	d := rank(iarStatus) - rank(agStatus)
	if d < 0 {
		d = -d
	}
	switch d {
	case 0:
		return "agree"
	case 1:
		return "partial"
	default:
		return "disagree"
	}
}
