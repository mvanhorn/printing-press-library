package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
)

type fakeHostfullyLeadAPI struct {
	get  func(path string, params map[string]string) (json.RawMessage, error)
	post func(path string, body any) (json.RawMessage, int, error)
}

func (f *fakeHostfullyLeadAPI) GetNoRetry(path string, params map[string]string) (json.RawMessage, error) {
	return f.get(path, params)
}

func (f *fakeHostfullyLeadAPI) Post(path string, body any) (json.RawMessage, int, error) {
	return f.post(path, body)
}

func fakeLeadPage(uids ...string) json.RawMessage {
	leads := make([]map[string]string, 0, len(uids))
	for _, uid := range uids {
		leads = append(leads, map[string]string{"uid": uid})
	}
	data, _ := json.Marshal(map[string]any{"leads": leads})
	return data
}

func fakeOffset(params map[string]string) int {
	raw, _ := base64.StdEncoding.DecodeString(params["_cursor"])
	var value struct {
		Offset int `json:"offset"`
	}
	_ = json.Unmarshal(raw, &value)
	return value.Offset
}

func TestFetchAllHostfullyUnitLeadsRecoversPoisonedRESTRecord(t *testing.T) {
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		offset := fakeOffset(params)
		limit, _ := strconv.Atoi(params["_limit"])
		switch {
		case offset == 0 && limit == hostfullyLeadPageSize:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset == 0 && limit == 100:
			return fakeLeadPage("a", "b"), nil
		case offset == 100 && limit > 1:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset == 100 && limit == 1:
			return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
		case offset > 100:
			return fakeLeadPage(), nil
		default:
			return fakeLeadPage(), nil
		}
	}

	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(path string, _ any) (json.RawMessage, int, error) {
		if path != "/graphql" {
			t.Fatalf("GraphQL path = %q", path)
		}
		return json.RawMessage(`{"data":{"leads":[{"uid":"a"},{"uid":"b"},{"uid":"c"}]},"extensions":{}}`), 200, nil
	}

	result, err := fetchAllHostfullyUnitLeads(rest, graphql, "agency", "property")
	if err != nil {
		t.Fatal(err)
	}
	if result.Recovered != 1 {
		t.Fatalf("recovered = %d, want 1", result.Recovered)
	}
	got := map[string]bool{}
	for _, lead := range result.Leads {
		got[hostfullyLeadUID(lead)] = true
	}
	for _, uid := range []string{"a", "b", "c"} {
		if !got[uid] {
			t.Errorf("missing lead %q", uid)
		}
	}
}

func TestFetchAllHostfullyUnitLeadsFailsWhenGraphQLDoesNotFillGap(t *testing.T) {
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		limit, _ := strconv.Atoi(params["_limit"])
		offset := fakeOffset(params)
		if offset >= hostfullyLeadPageSize && limit == hostfullyLeadPageSize {
			return fakeLeadPage(), nil
		}
		return nil, &client.APIError{Method: "GET", Path: "/leads", StatusCode: 500}
	}
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(_ string, _ any) (json.RawMessage, int, error) {
		return json.RawMessage(`{"data":{"leads":[]},"extensions":{}}`), 200, nil
	}

	_, err := fetchAllHostfullyUnitLeads(rest, graphql, "agency", "property")
	if err == nil {
		t.Fatal("expected incomplete recovery error")
	}
}

func TestHostfullyGraphQLBaseURL(t *testing.T) {
	for input, want := range map[string]string{
		"https://platform.hostfully.com/api/v3": "https://platform.hostfully.com/api/v3.3",
		"https://example.test/api/v3.3/":        "https://example.test/api/v3.3",
	} {
		t.Run(fmt.Sprintf("%s", input), func(t *testing.T) {
			if got := hostfullyGraphQLBaseURL(input); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestDateFilteredLeadFetchUsesPropertyLocalBounds(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(_ string, params map[string]string) (json.RawMessage, error) {
		if params["checkInTo"] != "2026-08-30" || params["checkOutFrom"] != "2026-08-30" {
			t.Fatalf("date filters = %#v", params)
		}
		return json.RawMessage(`{"leads":[{"uid":"stay","status":"BOOKED","checkInLocalDateTime":"2026-08-29T15:00:00","checkOutLocalDateTime":"2026-08-31T11:00:00"}]}`), nil
	}
	graphql := &fakeHostfullyLeadAPI{post: func(string, any) (json.RawMessage, int, error) {
		t.Fatal("GraphQL should not be called for a successful REST page")
		return nil, 0, nil
	}}
	result, err := fetchHostfullyUnitLeadsForDate(rest, graphql, "agency", "property", day)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Leads) != 1 || hostfullyLeadUID(result.Leads[0]) != "stay" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRawLeadOverlapUsesInclusiveCheckoutDate(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"uid":"stay","checkInLocalDateTime":"2026-08-29T15:00:00","checkOutLocalDateTime":"2026-08-30T11:00:00"}`)
	if !rawLeadOverlapsDate(raw, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("checkout day must be included for departure reporting")
	}
	if rawLeadOverlapsDate(raw, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("stay must not overlap the day after checkout")
	}
}
