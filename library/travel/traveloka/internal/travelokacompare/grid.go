package travelokacompare

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"sort"
	"strings"
	"time"
)

func list(value, flag string) ([]string, error) {
	if value == "" {
		return nil, invalid(flag + " is required")
	}
	out := strings.Split(value, ",")
	seen := map[string]bool{}
	if len(out) > 9 {
		return nil, invalid(flag + " accepts at most 9 explicit entries")
	}
	for i, v := range out {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return nil, invalid(flag + " must contain nonempty distinct entries")
		}
		out[i] = v
		seen[v] = true
	}
	return out, nil
}
func cellCap(queries []traveloka.Query, max int) error {
	if max < 1 || max > 9 {
		return invalid("--max-cells must be between 1 and 9 (CLI search bound)")
	}
	if len(queries) > max {
		return invalid(fmt.Sprintf("requested %d cells exceed --max-cells=%d; narrow the explicit dates or raise the cap up to 9", len(queries), max))
	}
	return nil
}

// FlightGrid validates every explicit Cartesian date pair before any retrieval starts.
func FlightGrid(q traveloka.Query, departDates, returnDates string, max int, now time.Time) ([]traveloka.Query, error) {
	departures, e := list(departDates, "--depart-dates")
	if e != nil {
		return nil, e
	}
	returns := []string{""}
	if returnDates != "" {
		returns, e = list(returnDates, "--return-dates")
		if e != nil {
			return nil, e
		}
	}
	out := []traveloka.Query{}
	for _, d := range departures {
		for _, r := range returns {
			cell := q
			cell.Depart = d
			cell.ReturnDate = r
			if e := traveloka.ValidateQueryAt(cell, now); e != nil {
				return nil, invalid(fmt.Sprintf("--depart-dates/--return-dates cell %s:%s: %v", d, r, e))
			}
			out = append(out, cell)
		}
	}
	if e := cellCap(out, max); e != nil {
		return nil, e
	}
	return out, nil
}

// HotelGrid requires one property and explicit equal-length stays with fixed party context.
func HotelGrid(q traveloka.Query, stays string, max int, now time.Time) ([]traveloka.Query, error) {
	entries, e := list(stays, "--stays")
	if e != nil {
		return nil, e
	}
	if q.PropertyID == "" {
		return nil, invalid("--property-id is required")
	}
	out := []traveloka.Query{}
	var length time.Duration
	for i, v := range entries {
		dates := strings.Split(v, ":")
		if len(dates) != 2 {
			return nil, invalid("--stays requires check-in:check-out pairs")
		}
		cell := q
		cell.CheckIn = dates[0]
		cell.CheckOut = dates[1]
		if e := traveloka.ValidateQueryAt(cell, now); e != nil {
			return nil, invalid(fmt.Sprintf("--stays cell %s: %v", v, e))
		}
		in, _ := time.Parse("2006-01-02", dates[0])
		outDate, _ := time.Parse("2006-01-02", dates[1])
		duration := outDate.Sub(in)
		if i == 0 {
			length = duration
		} else if duration != length {
			return nil, invalid("--stays must have equal night lengths for date comparison")
		}
		out = append(out, cell)
	}
	if e := cellCap(out, max); e != nil {
		return nil, e
	}
	return out, nil
}

type Cell struct {
	Cause                error               `json:"-"`
	Query                traveloka.Query     `json:"query"`
	Attempted            bool                `json:"attempted"`
	Status               string              `json:"status"`
	StatusCode           int                 `json:"status_code"`
	AttemptedAt          string              `json:"attempted_at"`
	CompletedAt          string              `json:"completed_at"`
	Error                *traveloka.APIError `json:"error"`
	Snapshot             *traveloka.Snapshot `json:"snapshot"`
	LowestRetrievedTotal *traveloka.Money    `json:"lowest_retrieved_total"`
}
type GridResult struct {
	ContextErr               error     `json:"-"`
	Status                   string    `json:"status"`
	Cells                    []Cell    `json:"cells"`
	Requested                int       `json:"requested_cells"`
	Attempted                int       `json:"attempted_cells"`
	NotAttempted             int       `json:"not_attempted_cells"`
	FetchFailures            []Failure `json:"fetch_failures"`
	FetchFailureCount        int       `json:"fetch_failure_count"`
	SaveFailures             []Failure `json:"save_failures"`
	SaveFailureCount         int       `json:"save_failure_count"`
	DifferentOfferIdentities bool      `json:"different_offer_identities_across_dates"`
	Note                     string    `json:"note"`
}
type Failure struct {
	CellIndex int                 `json:"cell_index"`
	Query     traveloka.Query     `json:"query"`
	Error     *traveloka.APIError `json:"error"`
	At        string              `json:"at"`
}
type Fetch func(context.Context, traveloka.Query) (*traveloka.Snapshot, error)
type Save func(context.Context, *traveloka.Snapshot) error

// RunGrid preserves every requested cell, including failed and deadline-curtailed searches.
// Only successful source retrievals can contribute a lowest retrieved total.
func RunGrid(ctx context.Context, queries []traveloka.Query, maxAttempts int, fetch Fetch, save Save) GridResult {
	result := GridResult{Cells: []Cell{}, FetchFailures: []Failure{}, SaveFailures: []Failure{}, Requested: len(queries), Note: "Lowest retrieved total covers only bounded returned source offers, not globally cheapest inventory. Failed and unattempted cells have no price; source quotes remain indicative."}
	firstIDs := ""
	haveIDs := false
	for i, q := range queries {
		cell := Cell{Query: q, Status: "not_attempted", Error: &traveloka.APIError{Code: "NOT_ATTEMPTED", Message: "search curtailed by command deadline or harness attempt cap"}}
		contextErr := ctx.Err()
		if i >= maxAttempts || contextErr != nil {
			if contextErr != nil {
				result.ContextErr = contextErr
				cell.Cause = contextErr
			}
			result.NotAttempted++
			result.Cells = append(result.Cells, cell)
			continue
		}
		cell.Attempted = true
		cell.AttemptedAt = time.Now().UTC().Format(time.RFC3339Nano)
		result.Attempted++
		s, err := fetch(ctx, q)
		cell.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err == nil && s != nil {
			expected := q
			if expected.PropertyName == "" {
				expected.PropertyName = s.Query.PropertyName
			}
			if s.Kind != q.Kind || !expected.SameContext(s.Query) {
				err = &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "source retrieval returned a different query context", Status: 200}
			}
		}
		if err == nil && s == nil {
			err = &traveloka.APIError{Code: "MALFORMED_RESPONSE", Message: "source retrieval returned no snapshot", Status: 200}
		}
		if err != nil {
			ae := &traveloka.APIError{Code: "UPSTREAM_ERROR", Message: err.Error()}
			var source *traveloka.APIError
			var rate *cliutil.RateLimitError
			if errors.As(err, &rate) {
				ae.Code = "RATE_LIMITED"
				ae.Status = 429
				ae.Retryable = true
			} else if errors.As(err, &source) {
				ae = source
			} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				ae.Code = "TIMEOUT"
				ae.Retryable = true
			}
			cell.Cause = err
			cell.Status = "fetch_failed"
			cell.Error = ae
			cell.StatusCode = ae.Status
			result.FetchFailureCount++
			result.FetchFailures = append(result.FetchFailures, Failure{i, q, ae, cell.CompletedAt})
			result.Cells = append(result.Cells, cell)
			continue
		}
		cell.Snapshot = s
		cell.Status = s.Status
		cell.StatusCode = 200
		cell.Error = nil
		ids := []string{}
		for _, o := range s.Offers {
			ids = append(ids, o.PropertyID+":"+o.RoomID+":"+o.ID)
			if o.Details["price_basis"] != basis(s.Kind) {
				continue
			}
			if (s.Kind == "rooms" || s.Kind == "hotels") && !matchingHotelParty(q, o) {
				continue
			}
			p, e := ExactTotal(o.Price.Total, q.Currency)
			if e != nil {
				continue
			}
			if p == nil {
				continue
			}
			prior, _ := ExactTotal(cell.LowestRetrievedTotal, q.Currency)
			if prior == nil || p.Cmp(prior) < 0 {
				cell.LowestRetrievedTotal = o.Price.Total
			}
		}
		sort.Strings(ids)
		key := strings.Join(ids, "|")
		if haveIDs && key != firstIDs {
			result.DifferentOfferIdentities = true
		}
		if !haveIDs {
			firstIDs = key
			haveIDs = true
		}
		if save != nil {
			if e := save(ctx, s); e != nil {
				cell.Status = "snapshot_save_failed"
				cell.Cause = e
				cell.Error = &traveloka.APIError{Code: "SNAPSHOT_SAVE_FAILED", Message: e.Error()}
				result.SaveFailureCount++
				result.SaveFailures = append(result.SaveFailures, Failure{i, q, cell.Error, cell.CompletedAt})
			}
		}
		result.Cells = append(result.Cells, cell)
	}
	result.Status = "success"
	if result.FetchFailureCount > 0 || result.SaveFailureCount > 0 || result.NotAttempted > 0 {
		result.Status = "partial"
	}
	if (result.Attempted == 0 && result.ContextErr != nil) || (result.Attempted > 0 && result.FetchFailureCount+result.SaveFailureCount == result.Attempted) {
		result.Status = "failed"
	}
	return result
}

// A fixed-party minimum requires explicit occupancy and consistent source room
// counts. Keep mismatched or unknown alternatives in the snapshot only.
func matchingHotelParty(q traveloka.Query, o traveloka.Offer) bool {
	if o.OccupancyMatch == nil || !*o.OccupancyMatch || q.Rooms <= 0 {
		return false
	}
	var counts []map[string]any
	if o.Kind == "room" {
		counts = []map[string]any{object(o.Details["rate"]), object(o.Details["room"])}
	} else if o.Kind == "hotel" {
		counts = []map[string]any{object(o.Details["inventory"])}
		if v, present := o.Details["num_charged_rooms"]; present {
			counts = append(counts, map[string]any{"numChargedRooms": v})
		}
	} else {
		return false
	}
	knownCount := false
	for _, fields := range counts {
		v, present := fields["numChargedRooms"]
		if !present {
			continue
		}
		n, ok := sourceInteger(v)
		if !ok || !n.IsInt64() || n.Int64() != int64(q.Rooms) {
			return false
		}
		knownCount = true
	}
	return knownCount
}
