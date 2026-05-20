// Daily Drop extraction. HotelTonight hides the once-a-day Daily Drop behind a
// "slide to unlock" gate in the app, and the /v6/featured_deal API endpoint is
// the account-gated unlock action (404 anonymously). But the server-rendered
// results page embeds the full deal — hotel, real price, savings — in its page
// data regardless of the unlock state. This reads that embedded object: a
// replayable HTTP + HTML extraction surface, no auth, no browser at runtime.
// Hand-authored (survives `generate --force`).
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/internal/cliutil"
)

// htFeaturedDeal is the Daily Drop as embedded in the results page. Numeric
// fields use json.Number for the same cross-market type-variance reasons as the
// inventory structs.
type htFeaturedDeal struct {
	Type          string      `json:"type"`
	State         string      `json:"state"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	HTPrice       json.Number `json:"ht_price"`
	SavingsAmount json.Number `json:"savings_amount"`
	UnlockURL     string      `json:"unlock_url"`

	// Derived during extraction, not part of the embedded JSON.
	HotelName string `json:"hotel_name"`
	HotelID   string `json:"hotel_id"`
}

// selectedHotelRe pulls the Daily Drop hotel id out of the unlock_url. The
// hotel name is then resolved from the page's embedded hotel JSON by id, which
// is stable across builds (unlike the CSS-hashed card markup).
var selectedHotelRe = regexp.MustCompile(`selected_hotel_id=(\d+)`)

// dailyDropLimiter bounds the rate of SSR page fetches to www.hoteltonight.com.
var dailyDropLimiter = cliutil.NewAdaptiveLimiter(2)

// fetchFeaturedDeal pulls the results page for a market slug and extracts the
// embedded Daily Drop. Returns ok=false (no error) when the page has no
// available featured deal. seoSlug is the market's seo_slug (e.g.
// "san-francisco-ca"); startDate is YYYY-MM-DD.
func fetchFeaturedDeal(ctx context.Context, seoSlug, startDate string) (htFeaturedDeal, bool, error) {
	if seoSlug == "" {
		return htFeaturedDeal{}, false, fmt.Errorf("no market slug available to look up the Daily Drop")
	}
	url := fmt.Sprintf("https://www.hoteltonight.com/s/%s", seoSlug)
	if startDate != "" {
		url += "?startDate=" + startDate
	}

	dailyDropLimiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return htFeaturedDeal{}, false, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html")

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return htFeaturedDeal{}, false, fmt.Errorf("fetching results page: %w", err)
	}
	defer resp.Body.Close()
	// Surface throttling as a hard error, never as an empty/absent deal —
	// empty-on-throttle is indistinguishable from "no Daily Drop today".
	if resp.StatusCode == http.StatusTooManyRequests {
		dailyDropLimiter.OnRateLimit()
		return htFeaturedDeal{}, false, &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return htFeaturedDeal{}, false, fmt.Errorf("results page returned HTTP %d", resp.StatusCode)
	}
	dailyDropLimiter.OnSuccess()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return htFeaturedDeal{}, false, fmt.Errorf("reading results page: %w", err)
	}

	return parseFeaturedDeal(string(body))
}

// parseFeaturedDeal extracts the Daily Drop from a results-page body. Pure
// (string in, deal out) so it is unit-testable against a fixture. The embedded
// featured_deal object contains nested arrays (rating_options), so the object
// is delimited by brace-counting rather than a regex.
func parseFeaturedDeal(page string) (htFeaturedDeal, bool, error) {
	obj := extractEscapedJSONObject(page, `\"featured_deal\":`)
	if obj == "" {
		return htFeaturedDeal{}, false, nil
	}
	// The embedded JSON is backslash-escaped inside the HTML; unescape the
	// quotes so it parses. /-style escapes are valid JSON and left as-is.
	clean := strings.ReplaceAll(obj, `\"`, `"`)

	var fd htFeaturedDeal
	if err := json.Unmarshal([]byte(clean), &fd); err != nil {
		return htFeaturedDeal{}, false, fmt.Errorf("parsing embedded Daily Drop: %w", err)
	}
	if !strings.EqualFold(fd.State, "available") {
		// e.g. "unavailable" or "locked-out" after the daily unlock is used.
		return fd, false, nil
	}

	// Hotel identity: the selected_hotel_id from the unlock_url, then its name
	// from the page's embedded hotel JSON.
	if mm := selectedHotelRe.FindStringSubmatch(fd.UnlockURL); len(mm) == 2 {
		fd.HotelID = mm[1]
		fd.HotelName = hotelNameByID(page, mm[1])
	}
	return fd, true, nil
}

// hotelNameByID finds the hotel object with the given id in the page's
// embedded (escaped) JSON and returns its name. Returns "" when the id or a
// nearby name field is not found.
func hotelNameByID(page, id string) string {
	if id == "" {
		return ""
	}
	idRe := regexp.MustCompile(`\\"id\\":` + regexp.QuoteMeta(id) + `\b`)
	loc := idRe.FindStringIndex(page)
	if loc == nil {
		return ""
	}
	end := loc[1] + 800
	if end > len(page) {
		end = len(page)
	}
	nm := hotelNameRe.FindStringSubmatch(page[loc[1]:end])
	if len(nm) == 2 {
		return cliutil.CleanText(strings.ReplaceAll(nm[1], `\"`, `"`))
	}
	return ""
}

var hotelNameRe = regexp.MustCompile(`\\"name\\":\\"([^\\]+)`)

// extractEscapedJSONObject returns the brace-delimited JSON object that follows
// `key` in an HTML-escaped page, counting `{`/`}` to find the matching close.
// Structural quotes appear as `\"` in the embedded JSON; the scan tracks
// in-string state on those so a `{` or `}` inside a string value is not
// counted. Returns "" if the key or a balanced object is not found.
func extractEscapedJSONObject(page, key string) string {
	ki := strings.Index(page, key)
	if ki < 0 {
		return ""
	}
	bi := strings.IndexByte(page[ki:], '{')
	if bi < 0 {
		return ""
	}
	bi += ki
	depth := 0
	inStr := false
	for i := bi; i < len(page); i++ {
		// A structural quote is the two-byte sequence `\"`. Toggle string
		// state on it and skip both bytes so braces inside a value don't count.
		if page[i] == '\\' && i+1 < len(page) && page[i+1] == '"' {
			inStr = !inStr
			i++
			continue
		}
		if inStr {
			continue
		}
		switch page[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return page[bi : i+1]
			}
		}
	}
	return ""
}

// strikethrough derives the pre-discount price from ht_price + savings_amount.
func (fd htFeaturedDeal) strikethrough() float64 {
	return numF(fd.HTPrice) + numF(fd.SavingsAmount)
}

// htDailyDrop is the flattened, agent-friendly Daily Drop view.
type htDailyDrop struct {
	Hotel       string  `json:"hotel"`
	HotelID     string  `json:"hotel_id,omitempty"`
	Price       float64 `json:"price"`
	Was         float64 `json:"was,omitempty"`
	Savings     float64 `json:"savings"`
	PctOff      int     `json:"pct_off"`
	State       string  `json:"state"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
}

// recordDailyDrop appends a Daily Drop observation to the snapshot store with
// deal_type=daily_drop so `daily-drop --history` can read it back over time.
func recordDailyDrop(ctx context.Context, db *sql.DB, marketID int64, marketName, checkIn string, dd htDailyDrop) error {
	hotelID, _ := strconv.ParseInt(dd.HotelID, 10, 64)
	_, err := db.ExecContext(ctx, snapshotInsertSQL,
		timeNow().UTC().Format(time.RFC3339), checkIn, hotelID, dd.Hotel, "",
		marketID, marketName, 0.0, 0.0, dailyDropDealType, "", dd.Price, dd.Was,
		dd.PctOff, 0, 0)
	return err
}

// view flattens the embedded featured deal into the agent-friendly shape,
// deriving the pre-discount price and percent off.
func (fd htFeaturedDeal) view() htDailyDrop {
	price := numF(fd.HTPrice)
	was := fd.strikethrough()
	return htDailyDrop{
		Hotel:       fd.HotelName,
		HotelID:     fd.HotelID,
		Price:       price,
		Was:         was,
		Savings:     numF(fd.SavingsAmount),
		PctOff:      pctOff(was, price),
		State:       fd.State,
		Title:       cliutil.CleanText(fd.Title),
		Description: cliutil.CleanText(fd.Description),
	}
}
