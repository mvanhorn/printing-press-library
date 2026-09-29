package activityjapan

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/cliutil"
)

type sitemapRoundTripFunc func(*http.Request) (*http.Response, error)

func (f sitemapRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type failingCloseBody struct{ io.Reader }

func (b failingCloseBody) Close() error { return errors.New("close failed") }

func TestLanguageSitemapPartialCoverage(t *testing.T) {
	c := newSitemapClient(t.TempDir(), true, false)
	c.http.Transport = sitemapRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "en.activityjapan.com" {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("denied")), Header: http.Header{}}, nil
		}
		xml := `<urlset><url><loc>https://activityjapan.com/publish/plan/62375</loc></url></urlset>`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(xml)), Header: http.Header{}}, nil
	})
	out, requests, err := checkLanguagesWithClient(context.Background(), "62375", c)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !out.Partial || out.EnglishChecked || !out.JapaneseChecked || !out.JapaneseIndexed || len(out.Errors) != 1 {
		t.Fatalf("partial coverage %+v, requests=%d", out, requests)
	}
}

func TestLanguageSitemapRateLimitIsHardError(t *testing.T) {
	c := newSitemapClient(t.TempDir(), true, false)
	c.http.Transport = sitemapRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("slow down")), Header: http.Header{}}, nil
	})
	_, requests, err := checkLanguagesWithClient(context.Background(), "62375", c)
	var rate *cliutil.RateLimitError
	if requests != 2 || !errors.As(err, &rate) {
		t.Fatalf("rate limit lost: requests=%d err=%v", requests, err)
	}
}

func TestRateLimitRemainsTypedWhenResponseCloseFails(t *testing.T) {
	c := newSitemapClient(t.TempDir(), true, false)
	c.http.Transport = sitemapRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: failingCloseBody{strings.NewReader("slow down")}, Header: http.Header{}}, nil
	})
	_, requests, err := checkLanguagesWithClient(context.Background(), "62375", c)
	var rate *cliutil.RateLimitError
	if requests != 1 || !errors.As(err, &rate) || !strings.Contains(err.Error(), "close failed") {
		t.Fatalf("typed rate limit or close failure lost: requests=%d err=%v", requests, err)
	}
}

func TestInvalidSitemapIsNotCached(t *testing.T) {
	c := newSitemapClient(t.TempDir(), false, false)
	requests := 0
	c.http.Transport = sitemapRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		body := "<html>verification</html>"
		if requests == 2 {
			body = `<urlset><url><loc>https://activityjapan.com/publish/plan/62375</loc></url></urlset>`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	if _, _, _, err := c.fetch(context.Background(), "ja"); err == nil {
		t.Fatal("invalid sitemap accepted")
	}
	body, status, _, err := c.fetch(context.Background(), "ja")
	if err != nil || requests != 2 || status != "miss" {
		t.Fatalf("valid retry: requests=%d status=%s err=%v", requests, status, err)
	}
	if _, err := parseSitemap(body); err != nil {
		t.Fatal(err)
	}
}

func TestSitemapLocaleCannotSelectCachePath(t *testing.T) {
	c := newSitemapClient(t.TempDir(), false, false)
	if _, _, _, err := c.fetch(context.Background(), "../other"); err == nil {
		t.Fatal("invalid locale selected a cache path")
	}
	if c.requests != 0 {
		t.Fatalf("invalid locale made %d requests", c.requests)
	}
}
