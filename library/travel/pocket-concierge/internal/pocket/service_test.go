package pocket

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSourceMissingCollectionsDoNotBecomeEmpty(t *testing.T) {
	for _, body := range []string{`{"data":{"venuesSearch":null}}`, `{"data":{"venuesSearch":{"metadata":{"currentPage":1,"limitValue":10,"totalCount":0,"totalPages":0}}}}`} {
		c := New("", false, true)
		c.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})
		if _, e := c.Search(context.Background(), "en", nil, 0); ExitCode(e) != 7 {
			t.Fatal(e)
		}
	}
	if e := ValidateVenue(nil, "1"); ExitCode(e) != 3 {
		t.Fatal(e)
	}
}
