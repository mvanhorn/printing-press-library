package cli

// PATCH: Regression coverage for Blu-ray.com helper edge cases from final review.

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/config"
)

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDecodeLatin1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "latin1", raw: []byte{0xC9, 0x6D, 0x69, 0x6C, 0x69, 0x65}, want: "Émilie"},
		{name: "ascii", raw: []byte{0x68, 0x69}, want: "hi"},
		{name: "empty", raw: []byte{}, want: ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := decodeLatin1(tt.raw); got != tt.want {
				t.Fatalf("decodeLatin1 = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBluRayGetRejectsUnexpectedHost(t *testing.T) {
	t.Parallel()

	c := client.New(&config.Config{BaseURL: "https://www.blu-ray.com"}, 0, 0)
	_, err := bluRayGet(c, "https://cdn.example.com/something", false)
	if err == nil {
		t.Fatal("bluRayGet succeeded, want host mismatch error")
	}
	if msg := err.Error(); !strings.Contains(msg, "host") || !strings.Contains(msg, "www.blu-ray.com") {
		t.Fatalf("error = %q, want host mismatch with expected hostname", msg)
	}
}

func TestBluRayGetPreservesRepeatedQueryParameters(t *testing.T) {
	t.Parallel()

	var gotQuery string
	c := client.New(&config.Config{BaseURL: "https://www.blu-ray.com"}, 0, 0)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: testRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.RawQuery
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}
	if _, err := bluRayGet(c, "https://www.blu-ray.com/something?a=1&a=2&b=3", false); err != nil {
		t.Fatalf("bluRayGet: %v", err)
	}
	if gotQuery != "a=1&a=2&b=3" {
		t.Fatalf("RawQuery = %q, want repeated params preserved", gotQuery)
	}
}
