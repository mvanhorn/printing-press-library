// Package toyota implements public, read-only Toyota Japan rental planning.
package toyota

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Origin = "https://rent.toyota.co.jp"
const BookingPath = "/eng/reservation/index01.aspx"
const OptionsPath = "/global_eng/service/option.html"
const InsurancePath = "/global_eng/guide/insurance.html"
const EligibilityPath = "/global_eng/drive/"
const OneWayPath = "/eng/service/oneway/simulation.aspx"

var jst = time.FixedZone("JST", 9*60*60)
var shopIDRE = regexp.MustCompile(`^[0-9]{5}:[0-9A-Z]{3}$`)

type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

type SourceError struct{ Message string }

func (e *SourceError) Error() string { return e.Message }

func ParseShopID(id string) (string, string, error) {
	if !shopIDRE.MatchString(id) {
		return "", "", &InputError{fmt.Sprintf("shop ID %q must be five company digits, colon, and three uppercase branch characters (e.g. 63601:01V)", id)}
	}
	parts := strings.Split(id, ":")
	return parts[0], parts[1], nil
}

func ShopURL(id string, returnMode bool) (string, error) {
	r, e, err := ParseShopID(id)
	if err != nil {
		return "", err
	}
	mode := "0"
	if returnMode {
		mode = "1"
	}
	return Origin + BookingPath + "?" + url.Values{"shopMode": {mode}, "rShop": {r}, "eShop": {e}}.Encode(), nil
}

type Period struct {
	Pickup  time.Time `json:"pickup_jst"`
	Dropoff time.Time `json:"dropoff_jst"`
	Hours   float64   `json:"duration_hours"`
}

func ParsePeriod(pickup, dropoff string, now time.Time) (Period, error) {
	parse := func(s, flag string) (time.Time, error) {
		t, err := time.ParseInLocation("2006-01-02T15:04", s, jst)
		if err != nil {
			t, err = time.Parse(time.RFC3339, s)
		}
		if err != nil {
			return time.Time{}, &InputError{flag + " must be YYYY-MM-DDTHH:MM in JST or RFC3339 (e.g. 2026-10-20T09:00)"}
		}
		t = t.In(jst)
		if t.Second() != 0 || t.Nanosecond() != 0 || t.Minute()%30 != 0 {
			return time.Time{}, &InputError{flag + " must use a 30-minute increment with zero seconds"}
		}
		return t, nil
	}
	p, err := parse(pickup, "--pickup")
	if err != nil {
		return Period{}, err
	}
	d, err := parse(dropoff, "--dropoff")
	if err != nil {
		return Period{}, err
	}
	if !p.After(now) {
		return Period{}, &InputError{"--pickup must be in the future (JST)"}
	}
	today := time.Date(now.In(jst).Year(), now.In(jst).Month(), now.In(jst).Day(), 0, 0, 0, 0, jst)
	lastPickup := addMonthsClamped(today, 3).AddDate(0, 0, 1)
	if !p.Before(lastPickup) {
		return Period{}, &InputError{"--pickup must be within Toyota's three-month advance window"}
	}
	if !d.After(p) {
		return Period{}, &InputError{"--dropoff must be later than --pickup"}
	}
	if d.After(addMonthsClamped(p, 1)) {
		return Period{}, &InputError{"--dropoff must be within one calendar month after --pickup"}
	}
	return Period{Pickup: p, Dropoff: d, Hours: d.Sub(p).Hours()}, nil
}

func addMonthsClamped(t time.Time, months int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	last := first.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func dateWire(t time.Time) string { return t.In(jst).Format("2006_01_02_1504") }

type SearchOptions struct {
	Transmission string   `json:"transmission"`
	FourWD       bool     `json:"four_wheel_drive_requested"`
	WinterTires  bool     `json:"winter_tires_requested"`
	Seats        []string `json:"child_seats_requested"`
}

func (o SearchOptions) Validate() error {
	if o.Transmission != "AT" && o.Transmission != "MT" {
		return &InputError{"--transmission must be AT or MT"}
	}
	if len(o.Seats) > 4 {
		return &InputError{"--child-seats accepts at most four seats"}
	}
	for _, s := range o.Seats {
		if s != "child" && s != "infant" && s != "booster" {
			return &InputError{"--child-seats must be a comma-separated list of child,infant,booster"}
		}
	}
	return nil
}

type Shop struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	NameJP        string   `json:"name_jp"`
	Hours         []string `json:"operating_hours"`
	Closure       string   `json:"closure_note"`
	Phone         string   `json:"phone"`
	Address       string   `json:"address"`
	MapCode       string   `json:"map_code"`
	OneWayReturns string   `json:"oneway_returns"`
	OneWayNote    string   `json:"oneway_note"`
	Services      []string `json:"services"`
	Notice        string   `json:"notice,omitempty"`
	URL           string   `json:"source_url"`
}

type ClassOffer struct {
	Class                string   `json:"class"`
	Availability         string   `json:"availability"`
	SourceEstimateJPY    *int     `json:"source_estimate_jpy"`
	PriceLabel           string   `json:"source_price_label"`
	TaxIncluded          bool     `json:"tax_included"`
	Capacity             *int     `json:"capacity"`
	RepresentativeModels []string `json:"representative_models"`
	ModelGuaranteed      bool     `json:"model_guaranteed"`
	ModelSelectionFee    string   `json:"model_selection_fee,omitempty"`
}

type Metadata struct {
	Source        string `json:"source"`
	FetchedAt     string `json:"fetched_at"`
	Requests      int    `json:"upstream_requests"`
	ResponseBytes int64  `json:"response_bytes_read"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	MaxRequests   int    `json:"max_upstream_requests"`
}

type ShopsResult struct {
	Meta          Metadata `json:"meta"`
	Query         string   `json:"query"`
	SourceContext string   `json:"source_context"`
	Shops         []Shop   `json:"shops"`
	SourceCount   int      `json:"source_count"`
	Truncated     bool     `json:"truncated"`
	Note          string   `json:"note,omitempty"`
}

type QuoteResult struct {
	Meta                  Metadata                   `json:"meta"`
	PickupShop            Shop                       `json:"pickup_shop"`
	DropoffShop           Shop                       `json:"dropoff_shop"`
	Period                Period                     `json:"period"`
	Options               SearchOptions              `json:"search_options"`
	OperatingWindows      map[string]OperatingWindow `json:"source_operating_windows"`
	Category              string                     `json:"category"`
	Offers                []ClassOffer               `json:"offers"`
	SourceCount           int                        `json:"source_count"`
	Truncated             bool                       `json:"truncated"`
	ConfirmedFullTotalJPY *int                       `json:"confirmed_full_total_jpy"`
	PriceAssumptions      []string                   `json:"price_assumptions"`
	BookingURL            string                     `json:"booking_url"`
	SourceURL             string                     `json:"source_url"`
}

type Handoff struct {
	BookingURL            string        `json:"booking_url"`
	PickupShopID          string        `json:"pickup_shop_id"`
	DropoffShopID         string        `json:"dropoff_shop_id"`
	Period                Period        `json:"period"`
	Options               SearchOptions `json:"search_options"`
	Class                 string        `json:"requested_class,omitempty"`
	URLPrefills           []string      `json:"url_prefills"`
	RequiresReentry       []string      `json:"requires_reentry"`
	Checklist             []string      `json:"checklist"`
	InventoryChecked      bool          `json:"inventory_checked"`
	ConfirmedFullTotalJPY *int          `json:"confirmed_full_total_jpy"`
}

func BookingHandoff(pickupID, dropoffID string, period Period, o SearchOptions, class string) (Handoff, error) {
	u, err := ShopURL(pickupID, false)
	if err != nil {
		return Handoff{}, err
	}
	if _, _, err := ParseShopID(dropoffID); err != nil {
		return Handoff{}, err
	}
	if err := o.Validate(); err != nil {
		return Handoff{}, err
	}
	if class != "" && !regexp.MustCompile(`^(?:[A-Z]{1,4}[0-9]{1,2}|LXC|LXP)$`).MatchString(class) {
		return Handoff{}, &InputError{"--class must be a Toyota class code such as C1, W2, SUV1, LXC or LXP"}
	}
	return Handoff{BookingURL: u, PickupShopID: pickupID, DropoffShopID: dropoffID, Period: period, Options: o, Class: class,
		URLPrefills: []string{"pickup shop"}, RequiresReentry: []string{"pickup/dropoff date and time", "dropoff shop", "class and options"},
		Checklist: []string{"Re-enter the exact JST dates and both shop IDs above.", "Verify class inventory; representative models are not guaranteed.", "Confirm child seats, ETC card, tires, waiver/NOC and one-way fees.", "Check final tax-inclusive total, tolls and fuel; Toyota verifies driving documents.", "Complete customer information,terms and payment yourself on Toyota."}}, nil
}
