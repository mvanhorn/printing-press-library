package sources

import "fmt"

// NIH RePORTER v2 — awarded NIH projects. Keyless.
const nihSearchURL = "https://api.reporter.nih.gov/v2/projects/search"

// nihMaxAmountCeiling is the upper bound sent with every amount range.
//
// RePORTER answers with an empty-body HTTP 500 when max_amount exceeds the
// signed 32-bit integer limit (2,147,483,647). Measured on 2026-08-11:
// 5,000,000,000 fails, 100,000,000 succeeds with an otherwise identical
// payload. Two billion stays safely below the limit and is far above any
// single NIH award.
const nihMaxAmountCeiling = 2000000000

// nihSearchFields lists the RePORTER fields the keyword is matched against.
//
// "terms" is deliberately excluded. RePORTER attaches a machine-generated
// concept tag list to every project, often several hundred entries long, and
// searching it produces matches unrelated to the project's subject. Measured
// on 2026-08-11 (FY2024, keyword "cancer"): including "terms" returned
// facility management and lease administration awards such as "Space and
// Facilities Management" ($20.2M) and "FY24/FY25 FFRDC LEASES Operational
// Task Order" ($17.0M), because "cancer" appears in their tag lists.
// Restricting to title and abstract dropped the total from 23,120 to 16,800
// while keeping every topically relevant award.
const nihSearchFields = "projecttitle,abstracttext"

// NIHResearchCodes are activity codes for investigator-initiated research
// awards: the grants an individual researcher competes for.
var NIHResearchCodes = []string{
	"R01", "R03", "R15", "R21", "R33", "R34", "R35", "R37", "R56", "R61",
	"DP1", "DP2", "DP5", "RF1", "K99",
}

// NIHCenterCodes are activity codes for institutional centers, program
// projects and cooperative agreements.
//
// These are real, competable grants, but they fund the running of a research
// center or a multi-site consortium rather than a single research idea, and
// they are one to two orders of magnitude larger. Measured on 2026-08-11
// (FY2024, keyword "cancer"): 16 of the top 25 awards by amount were P30
// "Cancer Center Support Grant" entries between $5.7M and $13.5M, and the
// largest award shown was a $21.3M UM1 consortium, against a research-award
// median near $417,000. Including them by default makes a typical grant look
// roughly fifty times larger than it is, so they are opt-in.
var NIHCenterCodes = []string{
	"U01", "UM1", "UG1", "UH2", "UH3", "U10", "U19", "U24", "U54",
	"P01", "P20", "P30", "P50",
}

type NIHProject struct {
	ProjectNum string `json:"project_num"`
	// CoreProjectNum identifies the grant across its support years. RePORTER
	// returns one record per fiscal year, so a project funded for a decade
	// appears ten times under project numbers that differ only in their
	// prefix and suffix (5R01CA092447-08, 2R01CA092447-06A1, 3R01CA092447-08S1).
	// The core number is the same for all of them and is the only reliable
	// dedup key — trimming the project number by character position does not
	// work, because the prefix digit and the suffix length both vary.
	CoreProjectNum string  `json:"core_project_num"`
	Title          string  `json:"project_title"`
	AwardAmount    float64 `json:"award_amount"`
	FiscalYear     int     `json:"fiscal_year"`
	PI             string  `json:"contact_pi_name"`
	ActivityCode   string  `json:"activity_code"`
	Org            struct {
		Name string `json:"org_name"`
	} `json:"organization"`
}

type nihResp struct {
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
	Results []NIHProject `json:"results"`
}

// NIHQuery describes one RePORTER search.
type NIHQuery struct {
	Keyword       string
	FiscalYear    int
	MinAmount     int64
	Limit         int
	IncludeCenter bool // add center/program/cooperative awards to the search
}

// activityCodes returns the whitelist for this query.
func (q NIHQuery) activityCodes() []string {
	codes := make([]string, 0, len(NIHResearchCodes)+len(NIHCenterCodes))
	codes = append(codes, NIHResearchCodes...)
	if q.IncludeCenter {
		codes = append(codes, NIHCenterCodes...)
	}
	return codes
}

// buildNIHPayload assembles the request body for one search.
//
// search_field must always be set. Omitting it turns advanced_text_search into
// a silent no-op: the request succeeds, but the reported total equals the
// unfiltered count and the keyword is absent from most rows. Measured on
// 2026-08-11: without search_field, 21 of 25 returned rows contained no
// occurrence of the search term in any field.
func buildNIHPayload(q NIHQuery, offset, limit int, sortOrder string) map[string]any {
	criteria := map[string]any{
		"advanced_text_search": map[string]any{
			"operator":     "and",
			"search_field": nihSearchFields,
			"search_text":  q.Keyword,
		},
		"activity_codes": q.activityCodes(),
	}
	if q.FiscalYear > 0 {
		criteria["fiscal_years"] = []int{q.FiscalYear}
	}
	// The amount floor belongs in the request, not in a post-filter. Filtering
	// after the fact silently shrinks an already-truncated page: the API
	// returns the largest awards first, so a floor applied afterwards can only
	// remove rows, never pull smaller-but-matching ones into view.
	if q.MinAmount > 0 {
		criteria["award_amount_range"] = map[string]any{
			"min_amount": q.MinAmount,
			"max_amount": nihMaxAmountCeiling,
		}
	}
	return map[string]any{
		"criteria":   criteria,
		"limit":      limit,
		"offset":     offset,
		"sort_field": "award_amount",
		"sort_order": sortOrder,
	}
}

// nihOverFetch is how many records are requested per row returned.
//
// RePORTER returns one record per support year, so an amount-sorted page is
// dominated by whichever long-running projects had large years. Measured on
// 2026-08-11 (keyword "cancer", 100 records): 71 distinct projects, with one
// study contributing 11 records; in the top 15 the duplication was worse still,
// leaving only 7 distinct projects. Fetching several times the requested rows
// leaves enough material to fill the page after collapsing them.
const nihOverFetch = 6

// SearchNIH returns awarded projects for a query, largest awards first, with
// each project appearing once.
func SearchNIH(q NIHQuery) ([]NIHProject, int, error) {
	fetch := q.Limit * nihOverFetch
	if fetch < 25 {
		fetch = 25
	}
	if fetch > 500 {
		fetch = 500
	}

	var resp nihResp
	payload := buildNIHPayload(q, 0, fetch, "desc")
	if err := postJSON(nihSearchURL, payload, &resp); err != nil {
		return nil, 0, fmt.Errorf("NIH RePORTER: %w", err)
	}

	projects := dedupeNIHProjects(resp.Results)
	if q.Limit > 0 && len(projects) > q.Limit {
		projects = projects[:q.Limit]
	}
	return projects, resp.Meta.Total, nil
}

// dedupeNIHProjects collapses a project's support years into its largest one,
// preserving the incoming order otherwise (the API sorts by amount descending).
//
// The largest year is kept because it is the most informative single row for
// "how much does this kind of work get": it is the peak annual award rather
// than a ramp-up or closing year. Records without a core number are passed
// through untouched rather than grouped together.
func dedupeNIHProjects(in []NIHProject) []NIHProject {
	best := make(map[string]int, len(in))
	out := make([]NIHProject, 0, len(in))

	for _, p := range in {
		key := p.CoreProjectNum
		if key == "" {
			out = append(out, p)
			continue
		}
		if idx, seen := best[key]; seen {
			if p.AwardAmount > out[idx].AwardAmount {
				out[idx] = p
			}
			continue
		}
		best[key] = len(out)
		out = append(out, p)
	}
	return out
}

// NIHTypical describes where the middle of the matching awards sits.
//
// The reported figure is an estimate located by counting, not an exact
// order statistic. Low and High bound the interval the true median falls in;
// Estimate is its midpoint and is only as good as that interval is narrow.
type NIHTypical struct {
	Population int   // how many awards the estimate covers
	Estimate   int64 // midpoint of [Low, High]
	Low        int64 // largest probed amount with more than half the awards above it
	High       int64 // smallest probed amount with at most half the awards above it
}

// nihCoarseProbes bracket the median on the first pass. They are spaced to
// match how NIH awards cluster: measured on 2026-08-11 (FY2024, "cancer",
// research codes only), 4,317 awards were above $400k, 3,870 above $420k and
// only 28 above $1M, so the interesting range is narrow and low.
var nihCoarseProbes = []int64{
	50000, 100000, 200000, 300000, 400000, 500000, 600000,
	750000, 1000000, 1500000, 2500000, 5000000,
}

// nihRefineRounds is how many bisection steps run inside the coarse bracket.
//
// Without refinement the midpoint of a coarse bracket is not the median and
// must not be called one. Measured on 2026-08-11 (FY2024, "cancer"): the
// coarse bracket was $400,000-$500,000, whose midpoint is $450,000, but the
// half-population line (3,911 of 7,823) actually falls between $415,000 and
// $420,000 — the unrefined figure was 8% high. Four rounds cut a $100,000
// bracket to $6,250, at a cost of four extra count requests.
const nihRefineRounds = 4

// TypicalNIHAward estimates the middle award amount for a query.
//
// It does not sample rows. RePORTER caps a response at a few hundred entries
// while topic totals run into the thousands, so any single page is a biased
// slice: taking the largest rows overstates the typical award by an order of
// magnitude, and taking the smallest understates it by the same margin.
// Measured on 2026-08-11 (FY2024, "cancer"), the 300 smallest matching awards
// had a median of $34,465 and the largest 25 a median of $1,256,124, against a
// true median near $417,000.
//
// Instead it asks the API how many awards exceed each amount and finds where
// that count crosses half the population, first over a coarse ladder and then
// by bisecting the bracket it lands in. Only the reported total is read, so
// each probe is a single cheap request.
//
// The estimate always describes the full result set for the topic, ignoring
// any MinAmount the caller set for the listing. The question it answers is
// "how much is granted for this topic", which a user-chosen display floor must
// not distort: restricting the probes to amounts above that floor would
// describe the large awards only. Measured on 2026-08-11, doing so reported
// $750,000 for a population whose middle sits near $417,000.
func TypicalNIHAward(q NIHQuery) (NIHTypical, error) {
	base := q
	base.MinAmount = 0
	base.Limit = 1

	total, err := nihCount(base)
	if err != nil {
		return NIHTypical{}, err
	}
	if total == 0 {
		return NIHTypical{}, nil
	}
	half := total / 2

	low := int64(0)   // more than half the awards are above this
	high := int64(-1) // at most half are above this; -1 means not found yet

	for _, threshold := range nihCoarseProbes {
		above, err := nihCount(withMinAmount(base, threshold))
		if err != nil {
			return NIHTypical{}, err
		}
		if above <= half {
			high = threshold
			break
		}
		low = threshold
	}

	// Every probe left more than half above it: the median is above the
	// highest amount tested and cannot be bracketed from this ladder.
	if high < 0 {
		return NIHTypical{
			Population: total,
			Estimate:   low,
			Low:        low,
			High:       0,
		}, nil
	}

	// Bisect the bracket. Each round halves the interval the median can be in.
	for i := 0; i < nihRefineRounds; i++ {
		mid := (low + high) / 2
		if mid <= low || mid >= high {
			break
		}
		above, err := nihCount(withMinAmount(base, mid))
		if err != nil {
			return NIHTypical{}, err
		}
		if above <= half {
			high = mid
		} else {
			low = mid
		}
	}

	return NIHTypical{
		Population: total,
		Estimate:   (low + high) / 2,
		Low:        low,
		High:       high,
	}, nil
}

// withMinAmount copies a query with a different amount floor.
func withMinAmount(q NIHQuery, amount int64) NIHQuery {
	q.MinAmount = amount
	return q
}

// nihCount runs a search and returns only the reported total.
func nihCount(q NIHQuery) (int, error) {
	var resp nihResp
	payload := buildNIHPayload(q, 0, 1, "desc")
	if err := postJSON(nihSearchURL, payload, &resp); err != nil {
		return 0, fmt.Errorf("NIH RePORTER: %w", err)
	}
	return resp.Meta.Total, nil
}