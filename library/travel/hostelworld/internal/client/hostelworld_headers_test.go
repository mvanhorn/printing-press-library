package client

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil"
	"io"
	"net/http"
	"strings"
	"testing"
)

type publicAppFakeTransport func(*http.Request) (*http.Response, error)

func (f publicAppFakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHostelworldPublicApplicationAndRedaction(t *testing.T) {
	for _, status := range []int{200, 401, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			key := "PLACEHOLDER_APPLICATION_ID"
			calls := 0
			tr := &hostelworldApplicationTransport{limiter: cliutil.NewAdaptiveLimiter(10000), base: publicAppFakeTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `APIGEE_KEY:"` + key + `"`
				code := 200
				if r.URL.Host == "prod.apigee.hostelworld.com" {
					code = status
					if r.Header.Get("api-key") != key {
						t.Fatal("public app header absent")
					}
					if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
						t.Fatal("session was forwarded")
					}
					body = `{"echo":"` + key + `"}`
				}
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://prod.apigee.hostelworld.com/autocomplete-service/v1/autocomplete/web?text=Osaka", nil)
			req.Header.Set("Cookie", "PLACEHOLDER_SESSION")
			req.Header.Set("Authorization", "PLACEHOLDER_AUTH")
			resp, e := tr.RoundTrip(req)
			if e != nil {
				t.Fatal(e)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if strings.Contains(string(body), key) {
				t.Fatal("application identifier escaped in response")
			}
			if req.Header.Get("api-key") != "" {
				t.Fatal("original request received transient identifier")
			}
			if calls != 2 {
				t.Fatal("unexpected bootstrap/request count")
			}
		})
	}
}
func TestHostelworldConfigDriftFailsClosed(t *testing.T) {
	for _, body := range []string{"<html>changed</html>", `APIGEE_KEY:"PLACEHOLDER_APPLICATION_ID" APIGEE_KEY:"PLACEHOLDER_APPLICATION_ID"`} {
		tr := &hostelworldApplicationTransport{limiter: cliutil.NewAdaptiveLimiter(1000), base: publicAppFakeTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if _, e := tr.app(context.Background()); e == nil {
			t.Fatal("changed/ambiguous config silently accepted")
		}
	}
}

func TestHostelworldPropertyProjectionExcludesProfiles(t *testing.T) {
	for _, body := range []string{`{"id":"67481","name":"Nui","reviews":[{"username":"PLACEHOLDER_PROFILE"}],"description":"PLACEHOLDER_EDITORIAL","thingsToNote":["source age rule"]}`, `{"id":"67481","name":"Nui","policies":["Taxes Included"]}`} {
		data, e := hostelworldPublicFacts("/legacy-hwapi-service/2.2/properties/67481/", []byte(body))
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(data), "PLACEHOLDER_PROFILE") || strings.Contains(string(data), "PLACEHOLDER_EDITORIAL") {
			t.Fatal("source profiles/editorial escaped into generic output")
		}
		if !strings.Contains(string(data), "67481") {
			t.Fatal("projection removed source ID")
		}
	}
	if e := AttachHostelworldApplication(nil); e == nil {
		t.Fatal("nil client accepted")
	}
}
