package govdata

import (
	"context"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestPeerPartialFailureAndCredentialIsolation(t *testing.T) {
	c := New("secret")
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "ca.gov") {
			if r.Header.Get("X-Api-Key") != "" {
				t.Fatal("federal key leaked to California")
			}
			return response(`{"success":true,"result":{"count":1,"results":[{"id":"ca1","title":"Water","resources":[{"id":"r","url":"https://agency.gov/file.csv","format":"CSV"}]}]}}`, 200), nil
		}
		return response(`denied`, 403), nil
	})
	r, e := c.Find(context.Background(), "water", "both", "", "", 5, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Results) != 1 || len(r.FetchFailures) != 1 || r.Complete {
		t.Fatalf("bad partial shape: %+v", r)
	}
}
func TestNormalizeFederalFactsAreMetadataOnly(t *testing.T) {
	m := map[string]any{"slug": "record", "dcat": map[string]any{"title": "Test", "publisher": map[string]any{"name": "Agency"}, "distribution": []any{map[string]any{"accessURL": "https://agency.gov/api", "mediaType": "application/json"}}}}
	d := Normalize("federal", m)
	if d.ID != "record" || d.Publisher != "Agency" || len(d.Resources) != 1 || d.Resources[0].URL != "https://agency.gov/api" {
		t.Fatalf("normalization %+v", d)
	}
}
func TestRateLimitTyped(t *testing.T) {
	c := New("")
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { return response("", 429), nil })
	_, e := c.Fetch(context.Background(), "california", "/package_show", nil)
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) {
		t.Fatalf("expected typed rate error: %v", e)
	}
}
func TestSourceValidationBeforeNetwork(t *testing.T) {
	c := New("")
	if _, e := c.Find(context.Background(), "water", "wrong", "", "", 10, 1); e == nil {
		t.Fatal("expected invalid source")
	}
	if _, e := c.Find(context.Background(), "water", "california", "", "", 0, 1); e == nil {
		t.Fatal("expected bound validation")
	}
}
func TestDownloadLimit(t *testing.T) {
	c := New("")
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Api-Key") != "" {
			t.Fatal("credential in download")
		}
		return response("123456", 200), nil
	})
	n, e := c.Download(context.Background(), "https://agency.gov/a", io.Discard, 3)
	if e == nil || n != 4 {
		t.Fatalf("expected bounded overflow, got %d %v", n, e)
	}
}

func TestCaliforniaFilterAndExactMetadata(t *testing.T) {
	c := New("")
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("fq"); got != `res_format:"CSV" AND organization:"water"` {
			t.Fatalf("bad Solr filter %q", got)
		}
		return response(`{"success":true,"result":{"count":100,"results":[{"id":"one","title":"Water","large":9007199254740993}]}}`, 200), nil
	})
	r, err := c.Find(context.Background(), "water", "california", "CSV", "water", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete || r.Next["california"] != "1" {
		t.Fatalf("scan incorrectly complete: %+v", r)
	}
	if !strings.Contains(string(r.Results[0].Raw), "9007199254740993") {
		t.Fatalf("metadata lost precision: %s", r.Results[0].Raw)
	}
}
func TestSourceRateLimitSurvivesFind(t *testing.T) {
	for _, source := range []string{"california", "federal"} {
		c := New("fixture")
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { return response("", 429), nil })
		_, err := c.Find(context.Background(), "water", source, "", "", 1, 1)
		var rate *cliutil.RateLimitError
		if !errors.As(err, &rate) {
			t.Fatalf("%s lost typed throttle: %v", source, err)
		}
	}
}
func TestDownloadRejectsUserinfo(t *testing.T) {
	c := New("")
	_, err := c.Download(context.Background(), "https://user:password@agency.gov/file", io.Discard, 10)
	if err == nil {
		t.Fatal("accepted credential-bearing URL")
	}
}
