// Package planner implements bounded, anonymous TableCheck planning reads.
package planner

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Result map[string]any

type Options struct {
	BaseURL, CacheDir                 string
	Refresh                           bool
	Timeout                           time.Duration
	MaxRequests, Concurrency, Retries int
	Now                               func() time.Time
	HTTPClient                        *http.Client
}

type SearchOptions struct {
	Latitude, Longitude, Radius               float64
	Cuisine, BudgetMin, BudgetMax, Date, Time string
	Party, Limit                              int
	Cursor                                    string
}
type CourseOptions struct {
	Venue, CourseID, From, To string
	Limit, Offset             int
}
type CheckOptions struct {
	Venue, Date, Time  string
	Party, Limit       int
	IncludeUnavailable bool
}
type ScanOptions struct {
	Venues             []string
	From, To, Time     string
	Party, Limit       int
	IncludeUnavailable bool
}
type HandoffOptions struct {
	Venue, Date, Time string
	Party             int
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

type PartialError struct{ Failed, Total int }

func (e *PartialError) Error() string {
	return fmt.Sprintf("%d of %d venue checks failed; remaining results are preserved", e.Failed, e.Total)
}

// SourceStatusError retains an upstream machine status without presenting it as inventory.
type SourceStatusError struct{ Status string }

func (e *SourceStatusError) Error() string {
	return fmt.Sprintf("availability_calendar source type %q is not verified success", e.Status)
}

type HTTPError struct {
	Status    int
	URL, Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("TableCheck HTTP %d for %s: %s", e.Status, e.URL, e.Body)
}

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,119}$`)
var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func invalid(format string, args ...any) error { return &ValidationError{fmt.Sprintf(format, args...)} }
func ValidateVenue(slug string) error {
	if !slugPattern.MatchString(slug) {
		return invalid("venue must be a public TableCheck slug (letters, digits, hyphens or underscores; max 120 characters)")
	}
	return nil
}
func ValidateDate(s string) error {
	t, e := time.Parse("2006-01-02", s)
	if e != nil || t.Format("2006-01-02") != s {
		return invalid("date must use YYYY-MM-DD: %q", s)
	}
	return nil
}
func ValidateTime(s string) error {
	t, e := time.Parse("15:04", s)
	if e != nil || t.Format("15:04") != s {
		return invalid("time must use HH:MM: %q", s)
	}
	return nil
}
func validateParty(n int) error {
	if n < 1 || n > 20 {
		return invalid("party must be between 1 and 20")
	}
	return nil
}
func resultLimit(n int) (int, error) {
	if n == 0 {
		return 10, nil
	}
	if n < 1 || n > 50 {
		return 0, invalid("limit must be between 1 and 50")
	}
	return n, nil
}
func ValidateSearch(o SearchOptions) error {
	if math.IsNaN(o.Latitude) || math.IsInf(o.Latitude, 0) || o.Latitude < -90 || o.Latitude > 90 {
		return invalid("lat must be finite and between -90 and 90")
	}
	if math.IsNaN(o.Longitude) || math.IsInf(o.Longitude, 0) || o.Longitude < -180 || o.Longitude > 180 {
		return invalid("lon must be finite and between -180 and 180")
	}
	if math.IsNaN(o.Radius) || math.IsInf(o.Radius, 0) || o.Radius <= 0 || o.Radius > 50000 {
		return invalid("radius must be positive and at most 50000 metres")
	}
	if _, e := resultLimit(o.Limit); e != nil {
		return e
	}
	if len(o.Cursor) > 8192 {
		return invalid("cursor exceeds 8192 characters")
	}
	if len(o.Cuisine) > 100 || strings.ContainsAny(o.Cuisine, "\r\n\x00") {
		return invalid("cuisine must be a stable cuisine key of at most 100 characters")
	}
	for _, v := range []string{o.BudgetMin, o.BudgetMax} {
		if v != "" && (!decimalPattern.MatchString(v) || len(v) > 32) {
			return invalid("budget-min and budget-max must be nonnegative decimal text")
		}
	}
	if o.BudgetMin != "" && o.BudgetMax != "" && compareDecimal(o.BudgetMin, o.BudgetMax) > 0 {
		return invalid("budget-min must not exceed budget-max")
	}
	if o.Date != "" {
		if e := ValidateDate(o.Date); e != nil {
			return e
		}
	}
	if o.Time != "" {
		if e := ValidateTime(o.Time); e != nil {
			return e
		}
		if o.Date == "" {
			return invalid("time requires date for discovery")
		}
	}
	if o.Party != 0 {
		if e := validateParty(o.Party); e != nil {
			return e
		}
	}
	return nil
}
func ValidateCourses(o CourseOptions) error {
	if e := ValidateVenue(o.Venue); e != nil {
		return e
	}
	if _, e := resultLimit(o.Limit); e != nil {
		return e
	}
	if o.Offset < 0 || o.Offset > 100000 {
		return invalid("offset must be between 0 and 100000")
	}
	if len(o.CourseID) > 120 || strings.ContainsAny(o.CourseID, "\r\n\x00") {
		return invalid("course ID is invalid")
	}
	for _, s := range []string{o.From, o.To} {
		if s != "" {
			if e := ValidateDate(s); e != nil {
				return e
			}
		}
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return invalid("from must not be after to")
	}
	return nil
}
func ValidateCheck(o CheckOptions) error {
	if e := ValidateVenue(o.Venue); e != nil {
		return e
	}
	if e := ValidateDate(o.Date); e != nil {
		return e
	}
	if e := validateParty(o.Party); e != nil {
		return e
	}
	if o.Time != "" {
		if e := ValidateTime(o.Time); e != nil {
			return e
		}
	}
	_, e := resultLimit(o.Limit)
	return e
}
func ValidateScan(o ScanOptions) error {
	if len(o.Venues) == 0 || len(o.Venues) > 5 {
		return invalid("scan requires between 1 and 5 distinct venues")
	}
	seen := map[string]bool{}
	for _, s := range o.Venues {
		if e := ValidateVenue(s); e != nil {
			return e
		}
		if seen[s] {
			return invalid("scan venues must be distinct: %s", s)
		}
		seen[s] = true
	}
	if e := ValidateDate(o.From); e != nil {
		return e
	}
	if e := ValidateDate(o.To); e != nil {
		return e
	}
	from, _ := time.Parse("2006-01-02", o.From)
	to, _ := time.Parse("2006-01-02", o.To)
	days := int(to.Sub(from)/(24*time.Hour)) + 1
	if days < 1 || days > 14 {
		return invalid("scan date range must contain 1 to 14 inclusive dates")
	}
	if e := validateParty(o.Party); e != nil {
		return e
	}
	if o.Time != "" {
		if e := ValidateTime(o.Time); e != nil {
			return e
		}
	}
	_, e := resultLimit(o.Limit)
	return e
}
func ValidateHandoff(o HandoffOptions) error {
	if e := ValidateVenue(o.Venue); e != nil {
		return e
	}
	if o.Date != "" {
		if e := ValidateDate(o.Date); e != nil {
			return e
		}
	}
	if o.Time != "" {
		if e := ValidateTime(o.Time); e != nil {
			return e
		}
		if o.Date == "" {
			return invalid("time requires date for booking handoff")
		}
	}
	if o.Party != 0 {
		if e := validateParty(o.Party); e != nil {
			return e
		}
	}
	return nil
}

// DryRunPlan validates the exact public inputs without HTTP or filesystem work.
func DryRunPlan(command string, input any) (Result, error) {
	var e error
	switch v := input.(type) {
	case SearchOptions:
		e = ValidateSearch(v)
	case CourseOptions:
		e = ValidateCourses(v)
	case CheckOptions:
		e = ValidateCheck(v)
	case ScanOptions:
		e = ValidateScan(v)
	case HandoffOptions:
		e = ValidateHandoff(v)
	case string:
		e = ValidateVenue(v)
	}
	if e != nil {
		return nil, e
	}
	return Result{"dry_run": true, "action": command, "would": "Read TableCheck planning data without creating a reservation", "command": command, "request_plan": input, "meta": map[string]any{"source": "dry-run", "requests": 0}}, nil
}

// Decimal comparison deliberately avoids binary floating-point money.
func compareDecimal(a, b string) int {
	pa := strings.SplitN(a, ".", 2)
	pb := strings.SplitN(b, ".", 2)
	if len(pa[0]) < len(pb[0]) {
		return -1
	}
	if len(pa[0]) > len(pb[0]) {
		return 1
	}
	if pa[0] < pb[0] {
		return -1
	}
	if pa[0] > pb[0] {
		return 1
	}
	fa, fb := "", ""
	if len(pa) > 1 {
		fa = pa[1]
	}
	if len(pb) > 1 {
		fb = pb[1]
	}
	n := len(fa)
	if len(fb) > n {
		n = len(fb)
	}
	fa += strings.Repeat("0", n-len(fa))
	fb += strings.Repeat("0", n-len(fb))
	return strings.Compare(fa, fb)
}

// Compile-time interface documentation for command and contract-test consumers.
type PlanningClient interface {
	Search(context.Context, SearchOptions) (Result, error)
	Venue(context.Context, string) (Result, error)
	Courses(context.Context, CourseOptions) (Result, error)
	Cuisines(context.Context, string, int, int) (Result, error)
	Check(context.Context, CheckOptions) (Result, error)
	Scan(context.Context, ScanOptions) (Result, error)
	BookingURL(context.Context, HandoffOptions) (Result, error)
}
