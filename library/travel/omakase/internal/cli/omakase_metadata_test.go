package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"
)

type metadataFake func(*http.Request) (*http.Response, error)

func (f metadataFake) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSourceMetadataNeverCachesAndBoundsBody(t *testing.T) {
	for _, n := range []int{100, (2 << 20) + 1} {
		c := client.New(&config.Config{}, 0, 0)
		c.HTTPClient.Transport = metadataFake(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", n)))}, nil
		})
		if e := ApplyClientHooks(c); e != nil {
			t.Fatal(e)
		}
		if !c.NoCache || c.BaseURL != "https://omakase.in" {
			t.Fatal("source cache/origin guard")
		}
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://omakase.in/en/r/hc778124", nil)
		_, e := c.HTTPClient.Do(req)
		if (e != nil) != (n > 2<<20) {
			t.Fatalf("n=%d err=%v", n, e)
		}
	}
}
func TestSourceMetadataRejectsMutationAndNonProvider(t *testing.T) {
	for _, x := range []struct{ method, url string }{{"POST", "https://omakase.in/en/r"}, {"GET", "https://other.test/en/r"}, {"GET", "https://omakase.in/users/sign_in"}, {"GET", "https://omakase.in/en/r/../users"}} {
		r, _ := http.NewRequest(x.method, x.url, nil)
		if metadataRequestAllowed(r) == nil {
			t.Fatal(x)
		}
	}
}

func TestSourceMetadataDryRunIsOneJSONValue(t *testing.T) {
	for _, args := range [][]string{{"pages", "list", "--json", "--dry-run", "--no-learn"}, {"pages", "detail", "hc778124", "--json", "--dry-run", "--no-learn"}} {
		c := RootCmd()
		c.SetArgs(args)
		var out bytes.Buffer
		c.SetOut(&out)
		if e := c.Execute(); e != nil {
			t.Fatal(e)
		}
		var v map[string]any
		if e := json.Unmarshal(out.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		if v["dry_run"] != true || v["action"] == "" {
			t.Fatal(v)
		}
	}
}
