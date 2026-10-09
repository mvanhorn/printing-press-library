// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil"
)

// Confidence labels for a sale moment.
const (
	ConfExact     = "exact"     // official rule confirmed live in this run
	ConfAnnounced = "announced" // a published notice or listing
	ConfEstimated = "estimated" // a vague notice, or a stored rule not confirmed live
	ConfUnknown   = "unknown"
)

// Sale window states.
const (
	WindowNotYetOpen = "not_yet_open"
	WindowOpen       = "open"
	WindowEnded      = "ended"
	WindowUnknown    = "unknown"
)

// DayStatus is the source-backed status of one visit date.
type DayStatus string

const (
	DayAvailable     DayStatus = "available"
	DayFew           DayStatus = "few"
	DaySoldOut       DayStatus = "sold_out"
	DayClosed        DayStatus = "closed"
	DayNoGeneralSale DayStatus = "no_general_sale"
	DayNotReleased   DayStatus = "not_released"
	DayPast          DayStatus = "past"
	DayUnknown       DayStatus = "unknown"
)

// Day is the status of one date at one sight.
type Day struct {
	Date      string    `json:"date"`
	Status    DayStatus `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	PriceFrom *int      `json:"adult_price_from_jpy"`
}

// Slot statuses.
const (
	SlotAvailable = "available"
	SlotFew       = "few"
	SlotSoldOut   = "sold_out"
	SlotUnknown   = "unknown" // the source gave no stock count
)

// Slot is one timed-entry slot with source stock.
type Slot struct {
	Date          string `json:"date"`
	Product       string `json:"product"`
	Start         string `json:"start"`
	End           string `json:"end"`
	StartJST      string `json:"start_jst"`
	Status        string `json:"status"`
	Stock         *int   `json:"stock"`
	Capacity      *int   `json:"capacity"`
	AdultPriceJPY *int   `json:"adult_price_jpy"`
	FitsParty     *bool  `json:"fits_party,omitempty"`
}

// Release is the next calendar release a source announces.
type Release struct {
	Covers      string     `json:"covers_month,omitempty"`
	OnSale      *time.Time `json:"-"`
	EarliestDay string     `json:"earliest_day,omitempty"`
	LatestDay   string     `json:"latest_day,omitempty"`
	Confidence  string     `json:"confidence"`
	Text        string     `json:"text"`
	SourceURL   string     `json:"source_url"`
	Tentative   bool       `json:"tentative"`
}

// SaleWindow is a published sale window for a range of visit dates.
type SaleWindow struct {
	FirstVisit string     `json:"first_visit_date"`
	LastVisit  string     `json:"last_visit_date"`
	Opens      *time.Time `json:"-"`
	Closes     *time.Time `json:"-"`
	Status     string     `json:"status,omitempty"`
	Label      string     `json:"label,omitempty"`
	SourceURL  string     `json:"source_url"`
}

// SourceRef records one source read.
type SourceRef struct {
	URL       string `json:"url"`
	FetchedAt string `json:"fetched_at"`
	OK        bool   `json:"ok"`
	Note      string `json:"note,omitempty"`
}

// SightData is everything read for one sight in one command run.
type SightData struct {
	Sight         Sight
	Days          map[string]Day
	CalendarUntil *Date
	Release       *Release
	RuleConfirmed bool
	SaleOverrides map[string]time.Time // YYYY-MM -> announced on-sale moment
	Windows       []SaleWindow
	Sources       []SourceRef
	Warnings      []string
	Slots         []Slot
	// RateLimited is set when any request for this sight got HTTP 429.
	RateLimited bool
}

func newSightData(s Sight) *SightData {
	return &SightData{Sight: s, Days: map[string]Day{}, SaleOverrides: map[string]time.Time{}}
}

func (sd *SightData) addSource(url string, ok bool, note string, now time.Time) {
	sd.Sources = append(sd.Sources, SourceRef{URL: url, FetchedAt: FormatJST(now), OK: ok, Note: note})
}

// warn records a warning. Warnings can hold remote text, so control and
// format characters are removed.
func (sd *SightData) warn(msg string) { sd.Warnings = append(sd.Warnings, StripControl(msg)) }

// failed records a failed source read and flags throttling.
func (sd *SightData) failed(url string, err error, now time.Time) {
	sd.addSource(url, false, err.Error(), now)
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		sd.RateLimited = true
	}
}

// read fetches one source and records the result. It returns the error on failure.
func (sd *SightData) read(ctx context.Context, f *Fetcher, url string, asJSON bool, now func() time.Time) (body []byte, err error) {
	if asJSON {
		body, err = f.GetJSON(ctx, url)
	} else {
		body, err = f.GetHTML(ctx, url)
	}
	if err != nil {
		sd.failed(url, err, now())
		return nil, err
	}
	sd.addSource(url, true, "", now())
	return body, nil
}

// firstFailure returns the note of the first failed source, if any.
func (sd *SightData) firstFailure() string {
	for _, s := range sd.Sources {
		if !s.OK {
			return s.Note
		}
	}
	return ""
}

// DefaultReason explains a day status when the source gives no reason.
func DefaultReason(st DayStatus) string {
	switch st {
	case DayAvailable:
		return "official calendar shows tickets on sale"
	case DayFew:
		return "official calendar shows few tickets left"
	case DaySoldOut:
		return "official calendar shows sold out"
	case DayClosed:
		return "closed (official calendar)"
	}
	return ""
}

func intPtr(v int) *int { return &v }
