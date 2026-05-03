package craigslist

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
)

// ListingDetail is the typed shape returned by rapi.craigslist.org/web/v8/postings/<uuid>.
//
// rapi mixes types across fields (price is int but priceString is formatted; streetAddress
// is sometimes a string and sometimes a numeric-looking field). We use json.Number for
// dodgy positional-typed fields and convert during accessors.
type ListingDetail struct {
	UUID           string             `json:"uuid"`
	PostingID      int64              `json:"postingId,omitempty"`
	PostingUUID    string             `json:"postingUuid,omitempty"`
	Title          string             `json:"title,omitempty"`
	Name           string             `json:"name,omitempty"`
	Body           string             `json:"body"`     // HTML body
	BodyText       string             `json:"bodyText"` // HTML stripped (computed)
	Category       string             `json:"category"`
	CategoryAbbr   string             `json:"categoryAbbr"`
	CategoryID     int                `json:"categoryId"`
	HasContactInfo bool               `json:"hasContactInfo"`
	Images         []string           `json:"images"`
	Attributes     []ListingAttribute `json:"attributes"`
	Price          int                `json:"price,omitempty"`
	PriceString    string             `json:"priceString,omitempty"`
	StreetAddress  flexString         `json:"streetAddress,omitempty"`
	URL            string             `json:"url,omitempty"`
	PostedDate     int64              `json:"postedDate,omitempty"`
	UpdatedDate    int64              `json:"updatedDate,omitempty"`
}

// flexString accepts either a string or a number from JSON and stores the result
// as a Go string. rapi inconsistently types streetAddress (and a few other fields
// in older posts) so we tolerate both shapes.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(string(b))
	return nil
}

// ListingAttribute is one entry in the attributes array (condition, make, model, etc.).
type ListingAttribute struct {
	Label               string `json:"label"`
	Value               string `json:"value"`
	PostingAttributeKey string `json:"postingAttributeKey"`
	SpecialType         string `json:"specialType,omitempty"`
}

// rawRAPI is the on-the-wire wrapper.
type rawRAPI struct {
	APIVersion int         `json:"apiVersion"`
	Data       rawRAPIData `json:"data"`
	Errors     []any       `json:"errors"`
}

type rawRAPIData struct {
	Items []ListingDetail `json:"items"`
	Lang  string          `json:"lang"`
}

// GetListing hits rapi.craigslist.org/web/v8/postings/<uuid> and returns the typed detail.
func (c *Client) GetListing(ctx context.Context, uuid string) (*ListingDetail, error) {
	if uuid == "" {
		return nil, fmt.Errorf("craigslist: empty uuid")
	}
	params := url.Values{}
	params.Set("lang", "en")
	body, err := c.RawGet(ctx, HostRAPI, "/postings/"+uuid, params)
	if err != nil {
		return nil, err
	}
	var raw rawRAPI
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode rapi response: %w", err)
	}
	if len(raw.Data.Items) == 0 {
		return nil, fmt.Errorf("craigslist: rapi returned no items for uuid %s", uuid)
	}
	d := raw.Data.Items[0]
	d.UUID = uuid
	if d.Title == "" {
		d.Title = d.Name
	}
	if d.Name == "" {
		d.Name = d.Title
	}
	d.BodyText = stripHTML(d.Body)
	return &d, nil
}

var htmlTagRE = regexp.MustCompile(`<[^>]*>`)
var brTagRE = regexp.MustCompile(`(?i)<br\s*/?>`)

// stripHTML removes <br> as newlines and other HTML tags as empty. Unlike a full
// HTML parser this is tolerant of malformed Craigslist body markup, which is the
// shape we observed in probes.
func stripHTML(s string) string {
	s = brTagRE.ReplaceAllString(s, "\n")
	s = htmlTagRE.ReplaceAllString(s, "")
	return s
}
