package parks

import "time"

const Origin = "https://www.kurumatabi.com"

var JST = time.FixedZone("JST", 9*60*60)
var FacilityKeys = []string{"electricity", "water", "dump_station", "black_water", "grey_water", "toilet_24h", "bath", "shower", "pets", "garbage", "laundry", "generator", "wifi", "kitchen", "dog_run", "premium_benefits"}

type Facility struct {
	Status   string   `json:"status"`
	Fee      string   `json:"fee"`
	Evidence []string `json:"evidence"`
}
type Dimensions struct {
	LengthM            *float64 `json:"length_m"`
	WidthM             *float64 `json:"width_m"`
	HeightM            *float64 `json:"height_m"`
	HeightUnrestricted bool     `json:"height_unrestricted"`
	Raw                string   `json:"raw"`
	Scope              string   `json:"scope"`
}
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Icon struct {
	Alt      string `json:"alt"`
	Source   string `json:"source"`
	Disabled bool   `json:"disabled"`
}
type Tariff struct {
	Audience    string   `json:"audience"`
	Description string   `json:"description"`
	AmountJPY   *float64 `json:"amount_jpy"`
	Currency    string   `json:"currency"`
	Kind        string   `json:"kind"`
	Raw         string   `json:"raw"`
}
type Membership struct {
	Status        string `json:"status"`
	Evidence      string `json:"evidence"`
	GeneralPolicy string `json:"general_policy"`
}
type Booking struct {
	SameDayAccepted *bool    `json:"same_day_reservations_accepted"`
	Vacancy         string   `json:"live_vacancy"`
	URLs            []string `json:"urls"`
	Phones          []string `json:"phones"`
	Emails          []string `json:"emails"`
	Terms           string   `json:"terms"`
}
type Park struct {
	AvailabilityPeriods []string `json:"availability_period_labels"`

	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	TypeLabel      string              `json:"type_label"`
	URL            string              `json:"url"`
	Address        string              `json:"address"`
	Coordinates    *Coordinates        `json:"coordinates"`
	Vehicles       []string            `json:"vehicle_categories"`
	Dimensions     Dimensions          `json:"dimensions"`
	Facilities     map[string]Facility `json:"facilities"`
	Membership     Membership          `json:"membership"`
	Tariffs        []Tariff            `json:"tariffs"`
	Booking        Booking             `json:"booking"`
	Sections       map[string]string   `json:"sections"`
	Icons          []Icon              `json:"icons"`
	Warnings       []string            `json:"warnings"`
	SourceLevel    string              `json:"source_level"`
	SourceUpdated  string              `json:"source_updated_date_jst"`
	ObservedAt     string              `json:"observed_at"`
	OutdoorCamping string              `json:"outdoor_camping_permission"`
}
type Meta struct {
	Source                string `json:"source"`
	SourceURL             string `json:"source_url"`
	ObservedAt            string `json:"observed_at"`
	ProviderTotal         *int   `json:"provider_total"`
	ScannedRecords        int    `json:"scanned_records"`
	ScannedPages          int    `json:"scanned_pages"`
	ReturnedRecords       int    `json:"returned_records"`
	ProviderPagesComplete bool   `json:"provider_pages_complete"`
	OutputTruncated       bool   `json:"output_truncated"`
	Note                  string `json:"note"`
}
type SearchResult struct {
	// Observations retains the full bounded scan for cache persistence only.
	Observations []Park `json:"-"`
	Meta         Meta   `json:"meta"`
	Results      []Park `json:"results"`
}
type Query struct {
	Prefecture string
	Type       string
	Keyword    string
	Vehicle    string
	Facilities []string
	Period     string
	MaxPages   int
	Limit      int
}

func newPark(id string, now time.Time) Park {
	fs := map[string]Facility{}
	for _, k := range FacilityKeys {
		fs[k] = Facility{Status: "unknown", Fee: "unknown", Evidence: []string{}}
	}
	return Park{AvailabilityPeriods: []string{}, ID: id, URL: Origin + "/park/" + id + ".html", Vehicles: []string{}, Facilities: fs, Membership: Membership{Status: "unknown", GeneralPolicy: "RV Park and Kurumatabi Park generally permit nonmembers; individual conditions override this policy."}, Tariffs: []Tariff{}, Booking: Booking{Vacancy: "unknown", URLs: []string{}, Phones: []string{}, Emails: []string{}}, Sections: map[string]string{}, Icons: []Icon{}, Warnings: []string{}, ObservedAt: now.In(JST).Format(time.RFC3339), OutdoorCamping: "not_inferred", Dimensions: Dimensions{Scope: "published_parking_space"}}
}
