package sources

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ErrPartialPool is returned by SearchNSF when paging stopped early because an
// upstream request failed. The awards collected so far are still returned, but
// the search did not cover the intended pool, so callers must not present the
// result as a completed search. Test for it with errors.Is.
var ErrPartialPool = errors.New("NSF candidate pool is incomplete")

// NSF Awards API — awarded NSF grants. Keyless.
const nsfAwardsURL = "https://api.nsf.gov/services/v1/awards.json"

// nsfPageSize is the API's fixed maximum records per page.
const nsfPageSize = 25

// nsfPoolPages is how many pages we fetch before ranking.
//
// The API's `keyword` matches anywhere in the record, including abstractText
// (~2,900 characters on average), and it returns the NEWEST matches first, not
// the most relevant. So a single page is mostly incidental one-mention hits and
// the good awards sit further down. Filtering one page can only remove rows, it
// can never surface the good ones.
//
// Measured on live data, one page (25) vs six pages (~145), counting awards
// that contain every query term:
//
//	gene therapy             2 kept  ->  34 kept, 11 with a title hit
//	artificial intelligence 18 kept  -> 115 kept, 21 with a title hit
//	climate resilience       2 kept  ->   6 kept,  2 with a title hit
//
// Six pages is the point where correct results reliably appear ("SBIR Phase I:
// A Novel Gene Therapy Platform..." shows up only past page one) while the
// sequential fetch stays fast. Deliberately not a flag: the user should not
// have to know this workaround exists.
const nsfPoolPages = 6

type NSFAward struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	FundsObligated string `json:"fundsObligatedAmt"`
	Awardee        string `json:"awardeeName"`
	StartDate      string `json:"startDate"`
	ExpDate        string `json:"expDate"`
}

// nsfAwardRaw carries the abstract used for matching. It is decode-only: the
// abstract never reaches the output, so the shape of the award objects in
// --json stays exactly as it was (the Grantvera web app parses that array).
type nsfAwardRaw struct {
	NSFAward
	Abstract string `json:"abstractText"`
}

type nsfResp struct {
	Response struct {
		Award []nsfAwardRaw `json:"award"`
	} `json:"response"`
}

// NSFStats reports what the relevance pass actually did, so a user who gets
// six results can tell that apart from a search with genuinely six matches.
type NSFStats struct {
	Examined     int `json:"examined"`      // candidate awards fetched from the API
	Matched      int `json:"matched"`       // awards containing every query term
	TitleMatched int `json:"title_matched"` // of those, awards with a term in the title

	// PagesFetched and PagesPlanned expose how much of the candidate pool was
	// actually retrieved. A later-page request can fail after the first page
	// succeeded; the partial pool is still worth ranking, but the counts above
	// then describe less than the intended search and must not be presented as
	// if they were complete.
	PagesFetched int `json:"pages_fetched"`
	PagesPlanned int `json:"pages_planned"`

	// PoolError carries the error that stopped paging early, if any. Empty
	// when paging ended because the results ran out, which is not a failure.
	PoolError string `json:"pool_error,omitempty"`
}

// Partial reports whether the candidate pool was cut short by an upstream
// failure rather than by running out of results.
func (s NSFStats) Partial() bool {
	return s.PoolError != ""
}

// nsfStopWords are dropped from the query before matching; requiring them would
// reject good awards for no reason.
var nsfStopWords = map[string]bool{
	"and": true, "the": true, "for": true, "with": true, "from": true,
	"into": true, "onto": true, "over": true, "under": true, "about": true,
	"that": true, "this": true, "these": true, "those": true, "are": true,
	"was": true, "were": true, "how": true, "why": true, "its": true,
	"via": true, "per": true, "new": true, "using": true, "use": true,
}

// nsfSuffixes are trimmed to compare words by stem. Measured: "climate
// resilience" matches zero awards containing the literal "resilience" — the
// abstracts say "resilient". A crude suffix trim covers that without pulling in
// a stemming dependency. Longest suffixes first so "-ations" wins over "-s".
var nsfSuffixes = []string{
	"ization", "ability", "ations", "ements", "iency", "ience", "ient",
	"ence", "ies", "ing", "ed", "es", "y", "s",
}

// nsfStem trims one known suffix, but only while at least four characters
// remain, so short words are not destroyed ("gene" stays "gene").
func nsfStem(word string) string {
	w := strings.ToLower(word)
	for _, suf := range nsfSuffixes {
		if len(w) >= len(suf)+4 && strings.HasSuffix(w, suf) {
			return strings.TrimSuffix(w, suf)
		}
	}
	return w
}

// nsfTerms splits a query into the stems every award must contain.
func nsfTerms(keyword string) []string {
	var terms []string
	for _, word := range strings.FieldsFunc(keyword, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		lower := strings.ToLower(word)
		if len(lower) <= 2 || nsfStopWords[lower] {
			continue
		}
		terms = append(terms, nsfStem(lower))
	}
	return terms
}

// nsfNormalizeTitle is the dedup key. "Collaborative Research" awards are
// registered once per participating institution: different IDs, identical
// titles. So the award ID cannot be the dedup key — three of the top five
// results for two separate measured queries were duplicate titles.
func nsfNormalizeTitle(title string) string {
	return strings.Join(strings.Fields(strings.ToLower(title)), " ")
}

// nsfScore returns the relevance score and whether the title matched. Every
// term must appear somewhere in title or abstract; ok is false otherwise.
// A title hit is worth 10 abstract hits — measured to put obviously correct
// awards on top.
func nsfScore(a nsfAwardRaw, terms []string) (score int, titleHit, ok bool) {
	if len(terms) == 0 {
		return 0, false, true
	}
	title := strings.ToLower(a.Title)
	abstract := strings.ToLower(a.Abstract)
	for _, t := range terms {
		inTitle := strings.Contains(title, t)
		inAbstract := strings.Contains(abstract, t)
		if !inTitle && !inAbstract {
			return 0, false, false
		}
		if inTitle {
			score += 10
			titleHit = true
		}
		if inAbstract {
			score++
		}
	}
	return score, titleHit, true
}

// nsfRank deduplicates, filters on "every term present", and sorts by score.
func nsfRank(pool []nsfAwardRaw, terms []string, rows int) ([]NSFAward, NSFStats) {
	stats := NSFStats{Examined: len(pool)}

	type scored struct {
		award    nsfAwardRaw
		score    int
		titleHit bool
	}
	byTitle := map[string]scored{}
	var order []string
	for _, a := range pool {
		score, titleHit, ok := nsfScore(a, terms)
		if !ok {
			continue
		}
		key := nsfNormalizeTitle(a.Title)
		// Keep the largest fundsObligatedAmt: that is the lead institution's
		// share and the most informative single row for a collaborative award.
		if prev, seen := byTitle[key]; seen {
			if ParseMoney(a.FundsObligated) <= ParseMoney(prev.award.FundsObligated) {
				continue
			}
			byTitle[key] = scored{a, score, titleHit}
			continue
		}
		byTitle[key] = scored{a, score, titleHit}
		order = append(order, key)
	}

	ranked := make([]scored, 0, len(order))
	for _, key := range order {
		s := byTitle[key]
		ranked = append(ranked, s)
		stats.Matched++
		if s.titleHit {
			stats.TitleMatched++
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if rows > 0 && len(ranked) > rows {
		ranked = ranked[:rows]
	}
	out := make([]NSFAward, 0, len(ranked))
	for _, s := range ranked {
		out = append(out, s.award.NSFAward)
	}
	return out, stats
}

// SearchNSF fetches a candidate pool of awarded NSF grants, keeps only awards
// whose title or abstract contains every query term (matched by stem),
// deduplicates collaborative re-registrations, and returns the top `rows` by
// relevance along with the counts behind that selection.
//
// A failure on the first page is fatal: there is nothing to rank. A failure on
// a later page still returns the awards already fetched, but it is reported as
// an error wrapping ErrPartialPool as well as in NSFStats — the relevance
// counts then describe only part of the intended pool, and a caller that only
// checked the error must not read that as a completed search. Callers can tell
// "nothing came back" from "some pages came back" by the returned slice.
func SearchNSF(keyword string, rows int) ([]NSFAward, NSFStats, error) {
	terms := nsfTerms(keyword)

	var pool []nsfAwardRaw
	var poolErr error
	pagesFetched := 0

	// Sequential, never parallel: one keyless client should not fan out six
	// concurrent requests at a public API.
	for page := 0; page < nsfPoolPages; page++ {
		q := url.Values{}
		q.Set("keyword", keyword)
		q.Set("rpp", fmt.Sprint(nsfPageSize))
		q.Set("offset", fmt.Sprint(page*nsfPageSize+1)) // 1-based: page two starts at 26
		q.Set("printFields", "id,title,fundsObligatedAmt,awardeeName,startDate,expDate,abstractText")
		var resp nsfResp
		if err := getJSON(nsfAwardsURL+"?"+q.Encode(), &resp); err != nil {
			if page == 0 {
				return nil, NSFStats{}, fmt.Errorf("NSF awards: %w", err)
			}
			poolErr = err
			break
		}
		pagesFetched++
		pool = append(pool, resp.Response.Award...)
		if len(resp.Response.Award) < nsfPageSize {
			break // last page reached
		}
	}

	awards, stats := nsfRank(pool, terms, rows)
	stats.PagesFetched = pagesFetched
	stats.PagesPlanned = nsfPoolPages
	if poolErr != nil {
		stats.PoolError = poolErr.Error()
		return awards, stats, fmt.Errorf("%w: %d of %d pages fetched: %w",
			ErrPartialPool, pagesFetched, nsfPoolPages, poolErr)
	}
	return awards, stats, nil
}
