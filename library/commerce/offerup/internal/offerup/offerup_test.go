package offerup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePrice(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"40", 40},
		{"1,200", 1200},
		{"$95", 95},
		{" 12.50 ", 12.5},
		{"", 0},
		{"Free", 0},
		{"Make Offer", 0},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, ParsePrice(c.in), "ParsePrice(%q)", c.in)
	}
}

func TestListingFromMap(t *testing.T) {
	raw := map[string]any{
		"listingId":     "abc-123",
		"title":         "iPhone 13 &amp; case",
		"price":         "250",
		"locationName":  "Seattle, WA",
		"conditionText": "Like New",
		"isFirmPrice":   true,
		"flags":         []any{"LOCAL_PICKUP"},
		"image":         map[string]any{"url": "https://img/x.jpg"},
	}
	l := listingFromMap(raw, "https://offerup.com")
	assert.Equal(t, "abc-123", l.ListingID)
	assert.Equal(t, "iPhone 13 & case", l.Title) // HTML entity decoded by CleanText
	assert.Equal(t, 250.0, l.Price)
	assert.Equal(t, "Seattle, WA", l.LocationName)
	assert.True(t, l.IsFirmPrice)
	assert.Equal(t, []string{"LOCAL_PICKUP"}, l.Flags)
	assert.Equal(t, "https://img/x.jpg", l.ImageURL)
	assert.Equal(t, "https://offerup.com/item/detail/abc-123", l.URL)
}

func TestComputePriceStats(t *testing.T) {
	listings := []StoredListing{
		{Price: 100, IsFirmPrice: true},
		{Price: 200},
		{Price: 300, IsFirmPrice: true},
		{Price: 400},
		{Price: 500},
		{Price: 0}, // free/unpriced -> ignored
	}
	s := ComputePriceStats("widget", "98101", listings)
	assert.Equal(t, 5, s.Count)
	assert.Equal(t, 100.0, s.Min)
	assert.Equal(t, 500.0, s.Max)
	assert.Equal(t, 300.0, s.Median)
	assert.Equal(t, 300.0, s.Mean)
	assert.Equal(t, 2, s.FirmCount)
	assert.Equal(t, 40.0, s.FirmPercent)
	assert.Equal(t, "98101", s.Location)
}

func TestComputePriceStatsEmpty(t *testing.T) {
	s := ComputePriceStats("widget", "", nil)
	assert.Equal(t, 0, s.Count)
	assert.Equal(t, 0.0, s.Median)
}

func TestMedian(t *testing.T) {
	assert.Equal(t, 0.0, Median(nil))
	assert.Equal(t, 20.0, Median([]StoredListing{{Price: 10}, {Price: 20}, {Price: 30}}))
	assert.Equal(t, 15.0, Median([]StoredListing{{Price: 10}, {Price: 20}, {Price: 0}}))
}

func TestConditionText(t *testing.T) {
	assert.Equal(t, "Good", conditionText("Good"))
	assert.Equal(t, "Like New", conditionText(map[string]any{"conditionText": "Like New"}))
	assert.Equal(t, "", conditionText(40.0)) // numeric code is not a label
	assert.Equal(t, "", conditionText(nil))
}

func TestSchemaConditionLabel(t *testing.T) {
	body := []byte(`<meta/><script>{"itemCondition":"https://schema.org/UsedCondition"}</script>`)
	assert.Equal(t, "Used", schemaConditionLabel(body))
	assert.Equal(t, "New", schemaConditionLabel([]byte(`schema.org/NewCondition`)))
	assert.Equal(t, "", schemaConditionLabel([]byte(`no condition here`)))
}

const searchPage = `<!doctype html><html><body>
<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"searchFeedResponse":{"looseTiles":[
{"__typename":"ModularFeedTileAd","tileType":"AD_3P_GOOGLE_DISPLAY","googleDisplayAd":{}},
{"__typename":"ModularFeedTileListing","tileType":"LISTING","listing":{"listingId":"l1","title":"Couch","price":"150","locationName":"Phoenix, AZ","conditionText":null,"isFirmPrice":false,"flags":["LOCAL_PICKUP"],"image":{"url":"https://img/1.jpg"}}},
{"__typename":"ModularFeedTileListing","tileType":"LISTING","listing":{"listingId":"l2","title":"Table","price":"75","locationName":"Tempe, AZ","isFirmPrice":true}}
]}}}}</script></body></html>`

func TestSearchExtractionAndFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "couch", r.URL.Query().Get("q"))
		// Location cookie is sent when a Location is provided.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(searchPage))
	}))
	defer srv.Close()
	t.Setenv("OFFERUP_BASE_URL", srv.URL)

	c := NewClient(5*time.Second, 0)
	got, err := c.Search(context.Background(), "couch", SearchOptions{Location: &Location{Zip: "85001"}})
	require.NoError(t, err)
	require.Len(t, got, 2) // ad tile filtered out
	assert.Equal(t, "l1", got[0].ListingID)
	assert.Equal(t, 150.0, got[0].Price)
	assert.Equal(t, "https://img/1.jpg", got[0].ImageURL)

	// Price filter keeps only the cheaper one.
	got, err = c.Search(context.Background(), "couch", SearchOptions{PriceMax: 100})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "l2", got[0].ListingID)

	// Firm-only filter.
	got, err = c.Search(context.Background(), "couch", SearchOptions{FirmOnly: true})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "l2", got[0].ListingID)
}

func TestLocationCookieValue(t *testing.T) {
	loc := &Location{City: "Phoenix", State: "AZ", Zip: "85001", Lat: "33.45", Lon: "-112.07"}
	v := loc.cookieValue()
	assert.Contains(t, v, "%22zipCode%22:%2285001%22") // quotes encoded, colon left raw (matches OfferUp)
	assert.NotContains(t, v, `"`)                      // no raw double-quotes
	assert.True(t, (&Location{}).empty())
	assert.False(t, loc.empty())
}
