package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/config"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPublicSourceHeadersCopyConfigAndRestrictHost(t *testing.T) {
	for _, base := range []string{"https://travel.rakuten.co.jp", "https://kw.travel.rakuten.co.jp", "https://hotel.travel.rakuten.co.jp", "https://api.example.test", "http://travel.rakuten.co.jp"} {
		original := &config.Config{Headers: map[string]string{"accept": "application/json", "X-Fixture": "preserved"}}
		c := &client.Client{BaseURL: base, Config: original}
		if err := configurePublicSourceHeaders(c); err != nil {
			t.Fatal(err)
		}
		if original.Headers["accept"] != "application/json" || original.Headers["Accept"] != "" {
			t.Fatal("source header hook mutated original config")
		}
		approved := strings.HasPrefix(base, "https://") && !strings.Contains(base, "example.test")
		if approved {
			if c.Config == original || c.Config.Headers["Accept"] != publicSourceHTMLAccept || c.Config.Headers["X-Fixture"] != "preserved" || c.Config.Headers["accept"] != "" {
				t.Fatalf("required HTML header/copy lost: %s %#v", base, c.Config.Headers)
			}
		} else if c.Config != original {
			t.Fatalf("source header hook changed unrelated client %s", base)
		}
	}
}

func TestPublicSourceHTMLHeaderKeepsCacheRepresentationsSeparate(t *testing.T) {
	testenv.Isolate(t)
	c := client.New(&config.Config{BaseURL: "https://travel.rakuten.co.jp", Headers: map[string]string{"Accept": "application/json"}}, time.Second, 0)
	calls := 0
	c.HTTPClient.Transport = travelCLIRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		body := "<html><title>JSON representation fixture</title></html>"
		if request.Header.Get("Accept") == publicSourceHTMLAccept {
			body = "<html><title>HTML representation fixture</title></html>"
		}
		return travelHTTPResponse(request, 200, body), nil
	})
	path := "https://kw.travel.rakuten.co.jp/keyword/Search.do"
	params := map[string]string{"charset": "utf-8", "f_query": "品川", "f_max": "3", "f_next": "1"}
	headers := map[string]string{client.HTMLResponseHeader: "true"}
	first, err := c.GetWithHeaders(context.Background(), path, params, headers)
	if err != nil || !strings.Contains(string(first), "JSON representation fixture") {
		t.Fatalf("fixture cache setup: %s %v", first, err)
	}
	if err := configurePublicSourceHeaders(c); err != nil {
		t.Fatal(err)
	}
	second, err := c.GetWithHeaders(context.Background(), path, params, headers)
	if err != nil || !strings.Contains(string(second), "HTML representation fixture") || calls != 2 {
		t.Fatalf("HTML request reused JSON representation cache: calls=%d body=%s err=%v", calls, second, err)
	}
	third, err := c.GetWithHeaders(context.Background(), path, params, headers)
	if err != nil || string(third) != string(second) || calls != 2 {
		t.Fatalf("HTML representation did not cache separately: calls=%d body=%s err=%v", calls, third, err)
	}
}
