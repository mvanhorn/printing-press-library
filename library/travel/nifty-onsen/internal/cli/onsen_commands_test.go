package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/onsen"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnsenDryRunsAndInputErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{{[]string{"bath", "show", "--dry-run", "--agent"}, 0}, {[]string{"bath", "coupons", "--dry-run", "--agent"}, 0}, {[]string{"bath", "nearby", "--dry-run", "--agent"}, 0}, {[]string{"bath", "compare", "--dry-run", "--agent"}, 0}, {[]string{"bath", "show", "--id=bad", "--agent"}, 2}, {[]string{"bath", "nearby", "--lat=0", "--lon=0", "--agent"}, 2}, {[]string{"bath", "search", "--filter=bad", "--agent"}, 2}, {[]string{"bath", "compare", "onsen012278", "onsen012278", "--agent"}, 2}, {[]string{"regions", "--data-source=live", "--agent"}, 2}} {
		var f rootFlags
		r := newRootCmd(&f)
		var out, errout bytes.Buffer
		r.SetOut(&out)
		r.SetErr(&errout)
		r.SetArgs(append(tc.args, "--home="+t.TempDir()))
		e := r.Execute()
		if ExitCode(e) != tc.code && !(e == nil && tc.code == 0) {
			t.Fatalf("%v: %v code %d", tc.args, e, ExitCode(e))
		}
		if e == nil {
			var v any
			if json.Unmarshal(out.Bytes(), &v) != nil {
				t.Fatalf("invalid JSON %v: %s", tc.args, out.String())
			}
		}
	}
}
func TestOnsenProjectionPreservesMetadata(t *testing.T) {
	for _, selectFields := range []string{"id,name", "items.id,items.name", "results.id,results.name"} {
		var f rootFlags
		r := newRootCmd(&f)
		r.SetArgs([]string{"regions", "--select=" + selectFields, "--agent"}) // Test output directly with facility-shaped domain rows.
		var out bytes.Buffer
		r.SetOut(&out)
		f.selectFields = selectFields
		f.agent = true
		f.asJSON = true
		p := onsen.Provenance{URL: onsen.Origin, Freshness: onsen.Freshness{Cache: "hit"}, Warnings: []string{}}
		if e := onsenOutput(r, &f, []map[string]any{{"id": "onsen012278", "name": "温泉A", "hours": "9:00"}}, p, map[string]any{"exhaustive": false}); e != nil {
			t.Fatal(e)
		}
		var v struct {
			Meta    map[string]any   `json:"meta"`
			Results []map[string]any `json:"results"`
		}
		if e := json.Unmarshal(out.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		if len(v.Results) != 1 || len(v.Results[0]) != 2 || v.Meta["source"] != "local" || v.Meta["coverage"] == nil {
			t.Fatal(out.String())
		}
		if bytes.Count(out.Bytes(), []byte("\n")) != 1 {
			t.Fatal("agent JSON not compact")
		}
	}
}

func TestProjectedComparisonRetainsStaleProvenance(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "cache", "parsed-onsen-v2")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"onsen012278", "onsen001483"} {
		sum := sha256.Sum256([]byte("onsen-parser-2:detail:" + id))
		record := map[string]any{"version": "2", "at": time.Now().Add(-time.Hour), "url": onsen.Origin + "/x/" + id + "/", "data": map[string]any{"id": id, "name": "fixture bath", "url": onsen.Origin + "/x/" + id + "/"}}
		b, _ := json.Marshal(record)
		if e := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var f rootFlags
	r := newRootCmd(&f)
	var out, errout bytes.Buffer
	r.SetOut(&out)
	r.SetErr(&errout)
	r.SetArgs([]string{"bath", "compare", "onsen012278", "onsen001483", "--home=" + home, "--data-source=local", "--max-age=1ns", "--select=facility.id", "--agent"})
	if e := r.Execute(); e != nil {
		t.Fatal(e, errout.String())
	}
	var v struct {
		Meta struct {
			Freshness onsen.Freshness `json:"freshness"`
			Coverage  struct {
				Items map[string]struct {
					Freshness onsen.Freshness `json:"freshness"`
					URL       string          `json:"source_url"`
				} `json:"item_provenance"`
			} `json:"coverage"`
		} `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	if !v.Meta.Freshness.Stale || v.Meta.Freshness.FetchedAt == "" || len(v.Meta.Coverage.Items) != 2 {
		t.Fatal(out.String())
	}
	for _, x := range v.Meta.Coverage.Items {
		if !x.Freshness.Stale || x.URL == "" {
			t.Fatal(out.String())
		}
	}
}

// A warm parsed cache must not satisfy the live harness's auto reads.
func TestOnsenLiveHarnessRefreshesWarmCache(t *testing.T) {
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	root := t.TempDir()
	restore, e := cliutil.SetHomeOverride(root)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	cacheDir, e := cliutil.CacheDir()
	if e != nil {
		t.Fatal(e)
	}
	dir := filepath.Join(cacheDir, "parsed-onsen-v2")
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	link, e := onsen.SearchURL(onsen.SearchOptions{Region: "tokyo", Page: 1})
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256([]byte("onsen-parser-2:" + link))
	record := map[string]any{"version": "2", "at": time.Now(), "url": link, "data": map[string]any{"items": []map[string]string{{"id": "onsen012278", "name": "cached"}}, "page": 1}}
	b, _ := json.Marshal(record)
	if e = os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	c, e := onsenClient(&rootFlags{dataSource: "auto"})
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	c.HTTP.Transport = onsenTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`<li class="shop" data-onsen-id="onsen012278"><a href="/x/onsen012278/"><p class="name">live fixture</p></a></li>`))}, nil
	})
	v, p, e := c.Search(context.Background(), onsen.SearchOptions{Region: "tokyo", Page: 1})
	if e != nil || calls != 1 || p.Cache != "miss" || v.Items[0].Name != "live fixture" {
		t.Fatalf("%+v %+v %v calls=%d", v, p, e, calls)
	}
	c, e = onsenClient(&rootFlags{dataSource: "local"})
	if e != nil {
		t.Fatal(e)
	}
	_, p, e = c.Search(context.Background(), onsen.SearchOptions{Region: "tokyo", Page: 1})
	if e != nil || p.Cache != "offline" {
		t.Fatalf("explicit local mode changed: %+v %v", p, e)
	}
}

type onsenTestTransport func(*http.Request) (*http.Response, error)

func (f onsenTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOnsenInvalidProjectionWritesNoResults(t *testing.T) {
	for _, mode := range []string{"agent", "json", "compact", "csv", "plain", "quiet"} {
		t.Run(mode, func(t *testing.T) {
			var f rootFlags
			r := newRootCmd(&f)
			f.selectFields = "results.typo"
			switch mode {
			case "agent":
				f.agent, f.asJSON = true, true
			case "json":
				f.asJSON = true
			case "compact":
				f.compact = true
			case "csv":
				f.csv = true
			case "plain":
				f.plain = true
			case "quiet":
				f.quiet = true
			}
			var out, diagnostics bytes.Buffer
			r.SetOut(&out)
			r.SetErr(&diagnostics)
			err := onsenOutput(r, &f, []map[string]any{{"id": "onsen012278", "name": "fixture bath"}}, onsen.Provenance{}, nil)
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("invalid selection must return a usage error, got %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("rejected selection emitted result data: %s", out.String())
			}
		})
	}
}
