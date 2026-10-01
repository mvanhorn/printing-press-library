package pocket

import (
	"context"
	"time"
)

const summaryFields = `id name realTimeBooking blurb area { id name } cuisines { id name } priceRanges { min max serviceType }`
const detailFields = `localizedAddress addressHidden businessHours holidays longDescription nearestStations reservationTerms services transactionsAllowed frequentlyAskedQuestions { question answer }`
const courseFields = `id name fixedPrice fixedTitle costPerGuest serviceType summary supplementaryInformation`
const taxonomyQuery = `query Filters { areas { id name } cuisines { id name } }`
const searchQuery = `query Discover($areaIds:[ID!],$cuisines:[ID!],$date:ISO8601Date,$keyword:String,$partySize:Int,$min:Int,$max:Int,$instant:Boolean,$services:[ServiceType!],$pagination:PaginationInput) {
 venuesSearch(areaIds:$areaIds,cuisines:$cuisines,date:$date,keyword:$keyword,partySize:$partySize,minPricePerPerson:$min,maxPricePerPerson:$max,realTimeBooking:$instant,serviceTypes:$services,paginationInput:$pagination) {
 collection { ` + summaryFields + ` } metadata { currentPage limitValue totalCount totalPages }
 } }`

type Pagination struct {
	CurrentPage int `json:"currentPage"`
	LimitValue  int `json:"limitValue"`
	TotalCount  int `json:"totalCount"`
	TotalPages  int `json:"totalPages"`
}
type SearchResult struct {
	Collection []Venue     `json:"collection"`
	Metadata   *Pagination `json:"metadata"`
}

func (c *Client) Filters(ctx context.Context, lang string) (map[string]any, error) {
	var data, jp struct {
		Areas    []Label `json:"areas"`
		Cuisines []Label `json:"cuisines"`
	}
	if err := c.ReadQuery(ctx, lang, taxonomyQuery, nil, 24*time.Hour, &data); err != nil {
		return nil, err
	}
	if data.Areas == nil || data.Cuisines == nil {
		return nil, Fail("source_schema", "filter arrays are missing")
	}
	if lang == "en" {
		if err := c.ReadQuery(ctx, "ja", taxonomyQuery, nil, 24*time.Hour, &jp); err != nil {
			return nil, err
		}
	}
	join := func(items, js []Label) []Label {
		for i := range items {
			if lang == "ja" {
				items[i].NameJA = ptr(items[i].Name)
			} else {
				for _, x := range js {
					if x.ID == items[i].ID {
						items[i].NameJA = ptr(x.Name)
					}
				}
			}
		}
		return items
	}
	return map[string]any{"areas": join(data.Areas, jp.Areas), "cuisines": join(data.Cuisines, jp.Cuisines)}, nil
}
func (c *Client) Search(ctx context.Context, lang string, vars map[string]any, ttl time.Duration) (*SearchResult, error) {
	var d struct {
		Result *SearchResult `json:"venuesSearch"`
	}
	if err := c.ReadQuery(ctx, lang, searchQuery, vars, ttl, &d); err != nil {
		return nil, err
	}
	if d.Result == nil || d.Result.Collection == nil || d.Result.Metadata == nil {
		return nil, Fail("source_schema", "search collection or pagination metadata missing")
	}
	m := d.Result.Metadata
	if m.CurrentPage < 1 || m.LimitValue < 1 || m.TotalCount < 0 || m.TotalPages < 0 {
		return nil, Fail("source_schema", "invalid search pagination")
	}
	for _, v := range d.Result.Collection {
		if v.ID == "" || v.Name == "" {
			return nil, Fail("source_schema", "search item identity missing")
		}
	}
	return d.Result, nil
}
func (c *Client) JapaneseNames(ctx context.Context, ids []string, ttl time.Duration) (map[string]Venue, error) {
	result := map[string]Venue{}
	if len(ids) == 0 {
		return result, nil
	}
	q := `query Names($ids:[ID!]) { venuesSearch(ids:$ids,paginationInput:{limit:50,page:1}) { collection { id name area { id name } cuisines { id name } } } }`
	var d struct {
		Result *SearchResult `json:"venuesSearch"`
	}
	if err := c.ReadQuery(ctx, "ja", q, map[string]any{"ids": ids}, ttl, &d); err != nil {
		return nil, err
	}
	if d.Result == nil || d.Result.Collection == nil {
		return nil, Fail("source_schema", "Japanese name collection missing")
	}
	for _, v := range d.Result.Collection {
		result[v.ID] = v
	}
	return result, nil
}
func (c *Client) Get(ctx context.Context, id, lang, fields string, ttl time.Duration) (*Venue, *Venue, error) {
	q := `query Restaurant($id:ID!) { venue(id:$id) { ` + summaryFields + ` ` + fields + ` } }`
	var d struct {
		Venue *Venue `json:"venue"`
	}
	if err := c.ReadQuery(ctx, lang, q, map[string]any{"id": id}, ttl, &d); err != nil {
		return nil, nil, err
	}
	if err := ValidateVenue(d.Venue, id); err != nil {
		return nil, nil, err
	}
	if lang == "ja" {
		return d.Venue, d.Venue, nil
	}
	jq := `query Identity($id:ID!) { venue(id:$id) { id name area { id name } cuisines { id name } courses { id name } } }`
	var jp struct {
		Venue *Venue `json:"venue"`
	}
	if err := c.ReadQuery(ctx, "ja", jq, map[string]any{"id": id}, ttl, &jp); err != nil {
		return nil, nil, err
	}
	if err := ValidateVenue(jp.Venue, id); err != nil {
		return nil, nil, err
	}
	return d.Venue, jp.Venue, nil
}
func (c *Client) Detail(ctx context.Context, id, lang string) (map[string]any, error) {
	v, jp, err := c.Get(ctx, id, lang, detailFields, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	r := Summary(*v, jp, lang)
	r["address"] = v.LocalizedAddress
	r["address_hidden"] = v.AddressHidden
	r["business_hours"] = v.BusinessHours
	r["holidays"] = v.Holidays
	r["description"] = v.LongDescription
	r["nearest_stations"] = v.NearestStations
	r["reservation_terms"] = v.ReservationTerms
	r["source_services"] = v.Services
	r["transactions_allowed"] = v.TransactionsAllowed
	r["faq"] = v.FAQ
	r["conditions"] = ConditionEvidence(v.ReservationTerms, v.Services)
	r["fee_statements"] = FeeStatements("restaurant", map[string]*string{"reservation_terms": v.ReservationTerms})
	return r, nil
}
func (c *Client) Courses(ctx context.Context, id, lang string) (*Venue, *Venue, error) {
	v, jp, err := c.Get(ctx, id, lang, `reservationTerms courses { `+courseFields+` }`, 15*time.Minute)
	if err != nil {
		return nil, nil, err
	}
	if v.Courses == nil {
		return nil, nil, Fail("source_schema", "courses array missing")
	}
	for _, x := range v.Courses {
		if x.ID == "" || x.Name == "" {
			return nil, nil, Fail("source_schema", "course identity missing")
		}
	}
	return v, jp, nil
}
func (c *Client) Dates(ctx context.Context, id, lang string, ttl time.Duration) (*Venue, *Venue, error) {
	v, jp, err := c.Get(ctx, id, lang, `availabilityCalendar { reservationDates waitlistDates }`, ttl)
	if err != nil {
		return nil, nil, err
	}
	if v.Calendar == nil || v.Calendar.ReservationDates == nil || v.Calendar.WaitlistDates == nil {
		return nil, nil, Fail("source_schema", "availability calendar missing")
	}
	for _, dates := range [][]string{v.Calendar.ReservationDates, v.Calendar.WaitlistDates} {
		for _, s := range dates {
			if ValidateDate(s) != nil {
				return nil, nil, Fail("source_schema", "calendar contains invalid date")
			}
		}
	}
	return v, jp, nil
}
func (c *Client) Slots(ctx context.Context, id, date, lang string, ttl time.Duration) (*Venue, *Venue, []Slot, error) {
	q := `query Sessions($id:ID!,$date:ISO8601Date!) { venue(id:$id) { ` + summaryFields + ` } availabilitySearch(venueId:$id,date:$date) { __typename ... on ReservableAvailability { id startTime endTime seatingType minPartySize maxPartySize course { ` + courseFields + ` } } ... on WaitlistableAvailability { startTime endTime minPartySize maxPartySize course { ` + courseFields + ` } } } }`
	var d struct {
		Venue *Venue `json:"venue"`
		Slots []Slot `json:"availabilitySearch"`
	}
	if err := c.ReadQuery(ctx, lang, q, map[string]any{"id": id, "date": date}, ttl, &d); err != nil {
		return nil, nil, nil, err
	}
	if err := ValidateVenue(d.Venue, id); err != nil {
		return nil, nil, nil, err
	}
	if d.Slots == nil {
		return nil, nil, nil, Fail("source_schema", "session collection missing")
	}
	if lang == "ja" {
		return d.Venue, d.Venue, d.Slots, nil
	}
	// Identity reads are independent of transient session inventory.
	var jp struct {
		Venue *Venue `json:"venue"`
	}
	jq := `query Identity($id:ID!) { venue(id:$id) { id name courses { id name } } }`
	if err := c.ReadQuery(ctx, "ja", jq, map[string]any{"id": id}, 15*time.Minute, &jp); err != nil {
		return nil, nil, nil, err
	}
	if err := ValidateVenue(jp.Venue, id); err != nil {
		return nil, nil, nil, err
	}
	return d.Venue, jp.Venue, d.Slots, nil
}
