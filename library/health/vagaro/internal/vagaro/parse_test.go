// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package vagaro

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnwrapEnvelope(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"object envelope with array d", `{"d":[{"x":1}]}`, `[{"x":1}]`},
		{"object envelope with object d", `{"d":{"a":2}}`, `{"a":2}`},
		{"json-string d", `{"d":"[{\"y\":3}]"}`, `[{"y":3}]`},
		{"bare array passthrough", `[{"z":4}]`, `[{"z":4}]`},
		{"bare object without d", `{"foo":"bar"}`, `{"foo":"bar"}`},
		{"html fragment inside d", `{"d":"<div>10:00 AM</div>"}`, `<div>10:00 AM</div>`},
		{"empty", ``, `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unwrapEnvelope([]byte(tt.in))
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestFormatAppDate(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"2026-07-24", "Fri Jul-24-2026", false},
		{"2026-01-05", "Mon Jan-05-2026", false},
		{"2026-12-25", "Fri Dec-25-2026", false},
		{" 2026-07-24 ", "Fri Jul-24-2026", false},
		{"07/24/2026", "", true},
		{"not-a-date", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := FormatAppDate(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseBusinessID(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			name: "json field mode wins over zeros",
			html: `..."BusinessID":0... "BusinessID":93458 ..."BusinessID":93458... _43931725_93458_`,
			want: "93458",
		},
		{
			name: "escaped embedded json",
			html: `foo \"BusinessId\":\"93458\" bar _43931725_93458_ _43931725_93458_`,
			want: "93458",
		},
		{
			name: "image pattern fallback when no json field",
			html: `<img src="cdn/155x155/43931725_93458_photo.jpg"> <img src="cdn/43931725_93458_x.jpg">`,
			want: "93458",
		},
		{
			name:    "no id present",
			html:    `<html><body>nothing here</body></html>`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBusinessID(tt.html)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseBusinessProfile(t *testing.T) {
	html := `<meta property="og:title" content="Central Barber Shop - Seattle WA | Vagaro" />
	<meta property="og:description" content="We are an independently owned shop." />
	<script type="application/ld+json">[{"@type":"ListItem","item":{"name":"barber in Seattle , WA"}}]</script>
	"BusinessID":93458 "BusinessID":93458 _43931725_93458_`
	p := ParseBusinessProfile(html)
	assert.Equal(t, "93458", p.BusinessID)
	assert.Equal(t, "Central Barber Shop", p.Name)
	assert.Contains(t, p.Name, "Central Barber")
	assert.Equal(t, "barber", p.Category)
	assert.Equal(t, "Seattle", p.City)
	assert.Equal(t, "WA", p.State)
}

func TestParseServices(t *testing.T) {
	d := json.RawMessage(`{"ServiceProviders":null,"Services":[{"ServiceCategoryTitle":"Haircuts and Other","ServiceList":[{"ServiceID":9433955,"ServiceTitle":"Men's Haircut","PriceText":"$52.00","Price":52.00},{"ServiceID":34098477,"ServiceTitle":"Skin Fade","PriceText":"$52.00","Price":52.00}]}]}`)
	rows := ParseServices(d)
	require.Len(t, rows, 2)
	assert.Equal(t, int64(9433955), rows[0].ServiceID)
	assert.Equal(t, "Men's Haircut", rows[0].ServiceTitle)
	assert.Equal(t, "$52.00", rows[0].PriceText)
	assert.Equal(t, 5200, rows[0].PriceCents)
	assert.Equal(t, "Haircuts and Other", rows[0].Category)
	assert.Equal(t, "Skin Fade", rows[1].ServiceTitle)
}

func TestParseServices_empty(t *testing.T) {
	assert.Empty(t, ParseServices(json.RawMessage(`{"Services":null}`)))
	assert.Empty(t, ParseServices(json.RawMessage(`not json`)))
}

func TestParseStaff(t *testing.T) {
	d := json.RawMessage(`{"ServiceProviders":[{"ServiceProviderID":43931725,"FirstName":"Ronnel","LastName":"Getz"},{"ServiceProviderID":232533768,"FirstName":"George","LastName":"Kuhar"}]}`)
	ps := ParseStaff(d)
	require.Len(t, ps, 2)
	assert.Equal(t, int64(43931725), ps[0].ServiceProviderID)
	assert.Equal(t, "Ronnel Getz", ps[0].Name)
	assert.Equal(t, "George Kuhar", ps[1].Name)
}

func TestParseReviews(t *testing.T) {
	d := json.RawMessage(`[{"Reviewer":"Yan ?","PublishedDate":"2026-06-30","AverageRank":5.0,"ServiceProviderName":"Ronnel Getz","ReviewID":1444419,"ServiceProviderReview":"Awesome haircut!","VenueReview":""}]`)
	rs := ParseReviews(d)
	require.Len(t, rs, 1)
	assert.Equal(t, int64(1444419), rs[0].ReviewID)
	assert.Equal(t, "Yan ?", rs[0].Author)
	assert.Equal(t, 5.0, rs[0].Rating)
	assert.Equal(t, "Awesome haircut!", rs[0].Text)
	assert.Equal(t, "2026-06-30", rs[0].Date)
	assert.Equal(t, "Ronnel Getz", rs[0].Provider)
}

func TestParseReviews_venueFallback(t *testing.T) {
	d := json.RawMessage(`[{"Reviewer":"Ann","AverageRank":4.0,"ServiceProviderReview":"","VenueReview":"Great shop","CreatedDateFormat":"Jun 1, 2026"}]`)
	rs := ParseReviews(d)
	require.Len(t, rs, 1)
	assert.Equal(t, "Great shop", rs[0].Text)
	assert.Equal(t, "Jun 1, 2026", rs[0].Date)
}

func TestExtractSlots_structured(t *testing.T) {
	d := json.RawMessage(`[{"BookingGroup":0,"AppDate":"24 Jul 2026","ServicepPoviderData":[{"AvailableTime":"10:00 AM,10:15 AM,10:00 AM,not a time","ServiceProviderID":43931725,"ServiceProviderName":"Ronnel Getz"}]},{"BookingGroup":1,"AppDate":"24 Jul 2026","ServicepPoviderData":[{"AvailableTime":"01:00 PM","ServiceProviderID":232533768,"ServiceProviderName":"George Kuhar"}]}]`)
	groups := ExtractSlots(d, "Fri Jul-24-2026", "43931725")
	require.Len(t, groups, 2)
	assert.Equal(t, "24 Jul 2026", groups[0].Date)
	assert.Equal(t, "Ronnel Getz", groups[0].Provider)
	assert.Equal(t, "43931725", groups[0].ProviderID)
	// de-dupes "10:00 AM" and drops "not a time"
	assert.Equal(t, []string{"10:00 AM", "10:15 AM"}, groups[0].Times)
	assert.Equal(t, []string{"01:00 PM"}, groups[1].Times)
}

func TestExtractSlots_empty(t *testing.T) {
	assert.Equal(t, []SlotGroup{}, ExtractSlots(json.RawMessage(`[]`), "Fri Jul-24-2026", ""))
	// groups present but no times -> empty, non-nil
	got := ExtractSlots(json.RawMessage(`[{"AppDate":"24 Jul 2026","ServicepPoviderData":[{"AvailableTime":""}]}]`), "Fri Jul-24-2026", "")
	assert.Equal(t, []SlotGroup{}, got)
}

func TestExtractSlots_htmlFragment(t *testing.T) {
	d := json.RawMessage(`<div class="slot">10:00 AM</div><div class="slot">1:15 PM</div><div>10:00 AM</div>`)
	// The fallback stamps the known query date (and provider) onto the group so
	// callers can require a date match instead of matching a clock label on any
	// day.
	groups := ExtractSlots(d, "Fri Jul-24-2026", "43931725")
	require.Len(t, groups, 1)
	assert.Equal(t, []string{"10:00 AM", "1:15 PM"}, groups[0].Times)
	assert.Equal(t, "Fri Jul-24-2026", groups[0].Date)
	assert.Equal(t, "43931725", groups[0].ProviderID)
	assert.Empty(t, groups[0].Provider)

	// The stamped date is parseable, so a date-aware caller resolves the day.
	dt, ok := ParseSlotDateTime(groups[0].Date, groups[0].Times[0])
	require.True(t, ok)
	assert.Equal(t, 2026, dt.Year())
	assert.Equal(t, time.July, dt.Month())
	assert.Equal(t, 24, dt.Day())

	// A CSV of providers is ambiguous, so no single provider is stamped.
	multi := ExtractSlots(d, "Fri Jul-24-2026", "43931725,99999999")
	require.Len(t, multi, 1)
	assert.Empty(t, multi[0].ProviderID)
}
