package travel

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type cacheWriteMethod struct {
	name  string
	calls int
	run   func(*testing.T, *Client) error
}

func cacheWriteMethods() []cacheWriteMethod {
	return []cacheWriteMethod{
		{"areas", 1, func(t *testing.T, c *Client) error {
			r, e := c.Areas(context.Background(), "")
			if e == nil && (r.Status != StatusOK || len(r.Areas) != 1 || r.Areas[0].ID != "tokyo") {
				t.Fatalf("lost parsed areas %#v", r)
			}
			return e
		}},
		{"hotels search", 1, func(t *testing.T, c *Client) error {
			r, e := c.SearchHotels(context.Background(), HotelQuery{Query: "cache"})
			if e == nil && (r.Status != StatusOK || len(r.Hotels) != 1 || r.Hotels[0].ID != "51870") {
				t.Fatalf("lost hotel candidates %#v", r)
			}
			return e
		}},
		{"hotel both pages", 2, func(t *testing.T, c *Client) error {
			r, e := c.Hotel(context.Background(), "51870")
			if e == nil && (r.Status != StatusOK || r.Hotel.ID != "51870" || r.Hotel.Name == nil || *r.Hotel.Name != "Cache test hotel" || len(r.Hotel.Access) != 1 || r.DetailsSource.URL == "") {
				t.Fatalf("lost property details %#v", r)
			}
			return e
		}},
		{"offers explicit cache", 1, func(t *testing.T, c *Client) error {
			r, e := c.Offers(context.Background(), sampleQuery())
			if e == nil && (r.Status != StatusOK || len(r.Offers) != 1 || r.Offers[0].PlanID != "3951989" || r.Offers[0].Price.PerRoomStayJPY != 20720) {
				t.Fatalf("lost offer evidence %#v", r)
			}
			return e
		}},
	}
}
func cacheWriteResponse(r *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(r.URL.Path, "/hotelinfo/plan/"):
		return response(200, string(offerFixture(sampleQuery(), []fixtureRoom{{plan: "3951989", room: "s-double-", total: 20720}}, false))), nil
	case strings.HasSuffix(r.URL.Path, "_std.html"):
		return response(200, `<link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/51870/51870_std.html"><li data-locate="hotel-access"><dl><dt>交通アクセス</dt><dd>Station access</dd></dl></li>`), nil
	case strings.Contains(r.URL.Path, "/HOTEL/"):
		return response(200, `<link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/51870/51870.html"><div id="RthNameArea">Cache test hotel</div>`), nil
	case strings.Contains(r.URL.Path, "/keyword/"):
		return response(200, `<input name="f_query" value="cache"><div id="result"><div class="hotelBox"><h2><a href="https://travel.rakuten.co.jp/HOTEL/51870/51870.html">Cache test hotel</a></h2></div></div>`), nil
	default:
		return response(200, areaBody), nil
	}
}
func unwritableCacheDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "regular-file")
	if e := os.WriteFile(p, []byte("existing file"), 0600); e != nil {
		t.Fatal(e)
	}
	return filepath.Join(p, "entries")
}
func TestCacheWriteFailuresKeepParsedResults(t *testing.T) {
	for _, method := range cacheWriteMethods() {
		t.Run(method.name, func(t *testing.T) {
			for _, callback := range []bool{false, true} {
				t.Run(map[bool]string{false: "nil callback", true: "warning callback"}[callback], func(t *testing.T) {
					cfg := Config{CacheDir: unwritableCacheDir(t), InventoryTTL: 30 * time.Second}
					warnings := []error{}
					if callback {
						cfg.OnCacheWriteError = func(e error) { warnings = append(warnings, e) }
					}
					c := clientFor(t, cacheWriteResponse, cfg)
					if e := method.run(t, c); e != nil {
						t.Fatalf("optional cache persistence failed successful source read: %v", e)
					}
					if c.Stats().Requests != method.calls {
						t.Fatalf("extra/missing source requests %#v", c.Stats())
					}
					want := 0
					if callback {
						want = method.calls
					}
					if len(warnings) != want {
						t.Fatalf("warnings %d, want exactly one per failed save (%d)", len(warnings), want)
					}
					for _, e := range warnings {
						if !errors.Is(e, syscall.ENOTDIR) {
							t.Fatalf("warning did not preserve deterministic filesystem failure: %v", e)
						}
					}
				})
			}
		})
	}
}
func TestCacheWriteFailuresNoCacheAndDefaultInventoryNeverWarn(t *testing.T) {
	for _, method := range cacheWriteMethods() {
		t.Run(method.name, func(t *testing.T) {
			warnings := 0
			c := clientFor(t, cacheWriteResponse, Config{CacheDir: unwritableCacheDir(t), NoCache: true, InventoryTTL: 30 * time.Second, OnCacheWriteError: func(error) { warnings++ }})
			if e := method.run(t, c); e != nil || warnings != 0 {
				t.Fatalf("no-cache persisted/warned: %v warnings=%d", e, warnings)
			}
		})
	}
	warnings := 0
	c := clientFor(t, cacheWriteResponse, Config{CacheDir: unwritableCacheDir(t), OnCacheWriteError: func(error) { warnings++ }})
	if _, e := c.Offers(context.Background(), sampleQuery()); e != nil || warnings != 0 {
		t.Fatalf("default live inventory attempted persistence: %v warnings=%d", e, warnings)
	}
}
func TestBestEffortCachePreservesSourceFailures(t *testing.T) {
	for _, kind := range []string{"network", "parse"} {
		for _, method := range cacheWriteMethods() {
			t.Run(kind+"/"+method.name, func(t *testing.T) {
				warnings := 0
				c := clientFor(t, func(*http.Request) (*http.Response, error) {
					if kind == "network" {
						return response(503, "upstream unavailable"), nil
					}
					return response(200, "<html>unrecognized source layout</html>"), nil
				}, Config{CacheDir: unwritableCacheDir(t), InventoryTTL: 30 * time.Second, OnCacheWriteError: func(error) { warnings++ }})
				e := method.run(t, c)
				if e == nil {
					t.Fatal("genuine source failure disappeared")
				}
				var source *SourceError
				if !errors.As(e, &source) || source.Kind == "cache_error" {
					t.Fatalf("source failure replaced %v", e)
				}
				if kind == "network" && source.Kind != "upstream_error" {
					t.Fatalf("wrong HTTP failure %v", e)
				}
				if warnings != 0 {
					t.Fatal("unsuccessful parse attempted persistence")
				}
			})
		}
	}
	t.Run("detail upstream failure survives earlier cache warning", func(t *testing.T) {
		warnings := 0
		c := clientFor(t, func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "_std.html") {
				return response(503, "upstream unavailable"), nil
			}
			return cacheWriteResponse(r)
		}, Config{CacheDir: unwritableCacheDir(t), OnCacheWriteError: func(error) { warnings++ }})
		_, e := c.Hotel(context.Background(), "51870")
		assertKind(t, e, "upstream_error")
		if warnings != 1 {
			t.Fatalf("basic save warnings %d", warnings)
		}
	})
}
