package steam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil"
)

// newTestClient wires a Client at an httptest server with a tiny retryWait
// so backoff tests sleep milliseconds, not seconds.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(NewConfig())
	c.BaseURL = srv.URL
	c.doer.retryWait = 10 * time.Millisecond
	return c
}

func TestNewConfigDefaults(t *testing.T) {
	cfg := NewConfig()
	if cfg.RateLimit != DefaultRateLimit {
		t.Errorf("RateLimit = %v, want %v", cfg.RateLimit, DefaultRateLimit)
	}
	if cfg.RateLimit != 3.0 {
		t.Errorf("default sustained rate should be 3 req/sec, got %v", cfg.RateLimit)
	}
}

func TestResolveAppIDParsesStoreSearch(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("term"); got != "Elden Ring" {
			t.Errorf("storesearch term = %q, want %q", got, "Elden Ring")
		}
		fmt.Fprint(w, `{"total":2,"items":[{"type":"bundle","name":"Elden Ring Deluxe Bundle","id":111},{"type":"app","name":"Elden Ring Soundtrack","id":222},{"type":"app","name":"Elden Ring","id":1245690}]}`)
	})
	id, err := c.ResolveAppID(context.Background(), "Elden Ring")
	if err != nil {
		t.Fatalf("ResolveAppID: %v", err)
	}
	if id != 1245690 {
		t.Errorf("ResolveAppID = %d, want 1245690 (exact app-typed match preferred over bundle)", id)
	}
}

func TestResolveAppIDEmptyItemsTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total":0,"items":[]}`)
	})
	_, err := c.ResolveAppID(context.Background(), "Nonexistent Game")
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("ResolveAppID error = %v, want ErrAppNotFound", err)
	}
}

func TestReviewSummaryParsesQuerySummary(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appreviews/1245690" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"success":1,"query_summary":{"review_score_desc":"Very Positive","total_positive":371000,"total_negative":19000,"total_reviews":390000,"review_score":9}}`)
	})
	s, err := c.ReviewSummary(context.Background(), 1245690)
	if err != nil {
		t.Fatalf("ReviewSummary: %v", err)
	}
	if s.Desc != "Very Positive" || s.Total != 390000 || s.Positive != 371000 || s.Negative != 19000 {
		t.Errorf("ReviewSummary = %+v, unexpected fields", s)
	}
	if s.Score != 90 {
		t.Errorf("Score = %v, want 90 (review_score 9 × 10)", s.Score)
	}
}

func TestReviewSummarySuccessFalseTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"success":0,"query_summary":{}}`)
	})
	_, err := c.ReviewSummary(context.Background(), 1245690)
	if !errors.Is(err, ErrReviewsUnavailable) {
		t.Fatalf("ReviewSummary error = %v, want ErrReviewsUnavailable", err)
	}
}

func TestReviewSummaryRetriesOnceOn429(t *testing.T) {
	var attempts int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		fmt.Fprint(w, `{"success":1,"query_summary":{"review_score_desc":"Mostly Positive","total_positive":100,"total_negative":20,"total_reviews":120,"review_score":8}}`)
	})
	s, err := c.ReviewSummary(context.Background(), 1245690)
	if err != nil {
		t.Fatalf("ReviewSummary after 429 retry: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want exactly 2 (one retry)", got)
	}
	if s.Desc != "Mostly Positive" {
		t.Errorf("Desc = %q, want %q", s.Desc, "Mostly Positive")
	}
}

func TestReviewSummary429ExhaustedSurfacesRateLimitError(t *testing.T) {
	var attempts int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	})
	_, err := c.ReviewSummary(context.Background(), 1245690)
	var rle *cliutil.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("error = %v, want *cliutil.RateLimitError", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want exactly 2 (retry once, then classify)", got)
	}
}

func TestClientRetriesOnceOn5xx(t *testing.T) {
	var attempts int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `boom`)
			return
		}
		fmt.Fprint(w, `{"total":1,"items":[{"type":"app","name":"Hollow Knight","id":367520}]}`)
	})
	id, err := c.ResolveAppID(context.Background(), "Hollow Knight")
	if err != nil {
		t.Fatalf("ResolveAppID after 5xx retry: %v", err)
	}
	if id != 367520 {
		t.Errorf("ResolveAppID = %d, want 367520", id)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want exactly 2 (one retry)", got)
	}
}

func TestClientDoesNotRetryOther4xx(t *testing.T) {
	var attempts int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.NotFound(w, r)
	})
	_, err := c.ResolveAppID(context.Background(), "Hollow Knight")
	if err == nil {
		t.Fatal("ResolveAppID should fail on 404")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("attempts = %d, want 1 (never retry non-429 4xx)", got)
	}
	var se *statusError
	if !errors.As(err, &se) || se.status != http.StatusNotFound {
		t.Fatalf("error = %v, want statusError{404}", err)
	}
}

func TestAppDetailsDynamicKeyParsing(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"1245690":{"success":true,"data":{"name":"Elden Ring","type":"game","price_overview":{"currency":"USD","initial":5999,"final":5999,"discount_percent":0}}}}`)
	})
	det, err := c.AppDetails(context.Background(), 1245690)
	if err != nil {
		t.Fatalf("AppDetails: %v", err)
	}
	if det.Name != "Elden Ring" {
		t.Errorf("Name = %q, want %q", det.Name, "Elden Ring")
	}
	if det.Price == nil || det.Price.Final != 5999 || det.Price.Currency != "USD" {
		t.Errorf("Price = %+v, want Final=5999 USD", det.Price)
	}
}

func TestAppDetailsNoPriceStillParses(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"367520":{"success":true,"data":{"name":"Hollow Knight","type":"game"}}}`)
	})
	det, err := c.AppDetails(context.Background(), 367520)
	if err != nil {
		t.Fatalf("AppDetails: %v", err)
	}
	if det.Price != nil {
		t.Errorf("Price should be nil for a free/priceless app, got %+v", det.Price)
	}
}

func TestAppDetailsSuccessFalseTypedError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"999999":{"success":false}}`)
	})
	_, err := c.AppDetails(context.Background(), 999999)
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("AppDetails error = %v, want ErrAppNotFound", err)
	}
}

func TestSteamReviewForTitleBridge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/storesearch/":
			fmt.Fprint(w, `{"total":1,"items":[{"type":"app","name":"Elden Ring","id":1245690}]}`)
		case "/appreviews/1245690":
			fmt.Fprint(w, `{"success":1,"query_summary":{"review_score_desc":"Very Positive","total_positive":371000,"total_negative":19000,"total_reviews":390000,"review_score":9}}`)
		case "/api/appdetails":
			fmt.Fprint(w, `{"1245690":{"success":true,"data":{"name":"Elden Ring","price_overview":{"currency":"USD","initial":5999,"final":5999,"discount_percent":0}}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	orig := bridgeClient
	t.Cleanup(func() { bridgeClient = orig })
	bc := New(NewConfig())
	bc.BaseURL = srv.URL
	bc.doer.retryWait = 10 * time.Millisecond
	bridgeClient = bc

	review, err := SteamReviewForTitle(context.Background(), "Elden Ring")
	if err != nil {
		t.Fatalf("SteamReviewForTitle: %v", err)
	}
	if review.AppID != 1245690 || review.Desc != "Very Positive" || review.Total != 390000 {
		t.Errorf("review = %+v, unexpected fields", review)
	}
	if review.Price == nil || review.Price.Final != 5999 {
		t.Errorf("Price = %+v, want Final=5999", review.Price)
	}
}

func TestSteamReviewForTitlePriceDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/storesearch/":
			fmt.Fprint(w, `{"total":1,"items":[{"type":"app","name":"Elden Ring","id":1245690}]}`)
		case "/appreviews/1245690":
			fmt.Fprint(w, `{"success":1,"query_summary":{"review_score_desc":"Very Positive","total_positive":371000,"total_negative":19000,"total_reviews":390000,"review_score":9}}`)
		case "/api/appdetails":
			// success:false must not sink the review block — price degrades.
			fmt.Fprint(w, `{"1245690":{"success":false}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	orig := bridgeClient
	t.Cleanup(func() { bridgeClient = orig })
	bc := New(NewConfig())
	bc.BaseURL = srv.URL
	bc.doer.retryWait = 10 * time.Millisecond
	bridgeClient = bc

	review, err := SteamReviewForTitle(context.Background(), "Elden Ring")
	if err != nil {
		t.Fatalf("SteamReviewForTitle should survive an appdetails failure: %v", err)
	}
	if review.Price != nil {
		t.Errorf("Price should degrade to nil, got %+v", review.Price)
	}
}

func TestSteamReviewForTitleUnresolvedTitleTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total":0,"items":[]}`)
	}))
	t.Cleanup(srv.Close)
	orig := bridgeClient
	t.Cleanup(func() { bridgeClient = orig })
	bc := New(NewConfig())
	bc.BaseURL = srv.URL
	bridgeClient = bc

	_, err := SteamReviewForTitle(context.Background(), "No Such Game")
	if !errors.Is(err, ErrAppNotFound) {
		t.Fatalf("SteamReviewForTitle error = %v, want ErrAppNotFound", err)
	}
}

func TestAppDetailsDecodesDemos(t *testing.T) {
	withDemos := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"379720":{"success":true,"data":{"name":"DOOM","type":"game","demos":[{"appid":479030,"description":""}]}}}`)
	})
	det, err := withDemos.AppDetails(context.Background(), 379720)
	if err != nil {
		t.Fatalf("AppDetails: %v", err)
	}
	if len(det.DemoAppIDs) != 1 || det.DemoAppIDs[0] != 479030 {
		t.Errorf("DemoAppIDs = %v, want [479030]", det.DemoAppIDs)
	}

	withoutDemos := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"367520":{"success":true,"data":{"name":"Hollow Knight","type":"game"}}}`)
	})
	bare, err := withoutDemos.AppDetails(context.Background(), 367520)
	if err != nil {
		t.Fatalf("AppDetails without demos: %v", err)
	}
	if bare.DemoAppIDs != nil {
		t.Errorf("DemoAppIDs = %v, want nil when appdetails has no demos", bare.DemoAppIDs)
	}
}

func TestEnrichAppIDSetsHasDemoOnlyWhenDetailsSucceed(t *testing.T) {
	const reviewBody = `{"success":1,"query_summary":{"review_score_desc":"Very Positive","total_positive":100,"total_negative":5,"total_reviews":105,"review_score":9}}`

	setup := func(t *testing.T, appdetails func(http.ResponseWriter)) (*Client, *int32, *int32) {
		t.Helper()
		var reviewReqs, detailReqs int32
		c := newTestCatalogClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/appreviews/"):
				atomic.AddInt32(&reviewReqs, 1)
				fmt.Fprint(w, reviewBody)
			case r.URL.Path == "/api/appdetails":
				atomic.AddInt32(&detailReqs, 1)
				appdetails(w)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		})
		return c, &reviewReqs, &detailReqs
	}

	t.Run("details ok with demos", func(t *testing.T) {
		c, reviews, details := setup(t, func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"1245690":{"success":true,"data":{"name":"Elden Ring","demos":[{"appid":5}]}}}`)
		})
		review, err := enrichAppID(context.Background(), c, 1245690)
		if err != nil {
			t.Fatalf("enrichAppID: %v", err)
		}
		if review.HasDemo == nil || !*review.HasDemo {
			t.Errorf("HasDemo = %v, want true when appdetails reports a demo", review.HasDemo)
		}
		if len(review.DemoAppIDs) != 1 || review.DemoAppIDs[0] != 5 {
			t.Errorf("DemoAppIDs = %v, want [5]", review.DemoAppIDs)
		}
		if got := atomic.LoadInt32(reviews) + atomic.LoadInt32(details); got != 2 {
			t.Errorf("requests = %d, want 2 (appreviews + appdetails)", got)
		}
	})

	t.Run("details ok without demos", func(t *testing.T) {
		c, reviews, details := setup(t, func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"1245690":{"success":true,"data":{"name":"Elden Ring"}}}`)
		})
		review, err := enrichAppID(context.Background(), c, 1245690)
		if err != nil {
			t.Fatalf("enrichAppID: %v", err)
		}
		if review.HasDemo == nil || *review.HasDemo {
			t.Errorf("HasDemo = %v, want false (known, no demos)", review.HasDemo)
		}
		if review.DemoAppIDs != nil {
			t.Errorf("DemoAppIDs = %v, want nil", review.DemoAppIDs)
		}
		if got := atomic.LoadInt32(reviews) + atomic.LoadInt32(details); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("details success false leaves has_demo unknown", func(t *testing.T) {
		c, reviews, details := setup(t, func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"1245690":{"success":false}}`)
		})
		review, err := enrichAppID(context.Background(), c, 1245690)
		if err != nil {
			t.Fatalf("the review must still ship when appdetails fails: %v", err)
		}
		if review.HasDemo != nil {
			t.Errorf("HasDemo = %v, want nil when appdetails failed", review.HasDemo)
		}
		if review.DemoAppIDs != nil {
			t.Errorf("DemoAppIDs = %v, want nil when appdetails failed", review.DemoAppIDs)
		}
		if got := atomic.LoadInt32(reviews) + atomic.LoadInt32(details); got != 2 {
			t.Errorf("requests = %d, want 2 (no new request added to this path)", got)
		}
	})

	t.Run("details HTTP 500 leaves has_demo unknown", func(t *testing.T) {
		c, reviews, details := setup(t, func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "boom")
		})
		review, err := enrichAppID(context.Background(), c, 1245690)
		if err != nil {
			t.Fatalf("the review must still ship when appdetails fails: %v", err)
		}
		if review.HasDemo != nil {
			t.Errorf("HasDemo = %v, want nil when appdetails failed", review.HasDemo)
		}
		if got := atomic.LoadInt32(reviews); got != 1 {
			t.Errorf("appreviews requests = %d, want 1", got)
		}
		// The 5xx retry policy is unchanged: one retry, exactly as before.
		if got := atomic.LoadInt32(details); got != 2 {
			t.Errorf("appdetails requests = %d, want 2 (one retry)", got)
		}
	})
}
