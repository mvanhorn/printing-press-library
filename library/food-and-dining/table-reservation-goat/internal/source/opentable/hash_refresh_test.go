package opentable

import (
	"encoding/base64"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestExtractSha256Hash(t *testing.T) {
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "compact persisted-query body",
			body: `{"operationName":"RestaurantsAvailability","variables":{"rid":1389331},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"` + fresh + `"}}}`,
			want: fresh,
		},
		{
			name: "no hash present",
			body: `{"operationName":"RestaurantsAvailability","variables":{}}`,
			want: "",
		},
		{
			name: "malformed short hash",
			body: `{"extensions":{"persistedQuery":{"sha256Hash":"deadbeef"}}}`,
			want: "",
		},
		{
			name: "empty body",
			body: "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractSha256Hash(tc.body); got != tc.want {
				t.Fatalf("extractSha256Hash(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestExtractOperationName(t *testing.T) {
	body := `{"operationName":"RestaurantAvailability","variables":{},"extensions":{"persistedQuery":{"sha256Hash":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}}`
	if got := extractOperationName(body); got != "RestaurantAvailability" {
		t.Fatalf("extractOperationName() = %q, want RestaurantAvailability", got)
	}
}

func TestHashFromRequestPostDataEntries(t *testing.T) {
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	body := `{"operationName":"RestaurantAvailability","extensions":{"persistedQuery":{"sha256Hash":"` + fresh + `"}}}`
	req := &network.Request{
		URL: "https://www.opentable.com/dapi/fe/gql?optype=query&opname=RestaurantAvailability",
		PostDataEntries: []*network.PostDataEntry{
			{Bytes: base64.StdEncoding.EncodeToString([]byte(body[:40]))},
			{Bytes: base64.StdEncoding.EncodeToString([]byte(body[40:]))},
		},
	}
	if got := hashFromRequest(req); got != fresh {
		t.Fatalf("hashFromRequest() = %q, want %q", got, fresh)
	}
	if got := availabilityIdentityFromRequest(req); got.OperationName != "RestaurantAvailability" {
		t.Fatalf("availabilityIdentityFromRequest().OperationName = %q, want RestaurantAvailability", got.OperationName)
	}
}

func TestHashFromPostDataFallback(t *testing.T) {
	fresh := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	body := `{"operationName":"RestaurantAvailability","extensions":{"persistedQuery":{"sha256Hash":"` + fresh + `"}}}`
	if got := hashFromPostData([]byte(body)); got != fresh {
		t.Fatalf("hashFromPostData() = %q, want %q", got, fresh)
	}
	if got := availabilityIdentityFromPostData([]byte(body)); got.OperationName != "RestaurantAvailability" {
		t.Fatalf("availabilityIdentityFromPostData().OperationName = %q, want RestaurantAvailability", got.OperationName)
	}
}
