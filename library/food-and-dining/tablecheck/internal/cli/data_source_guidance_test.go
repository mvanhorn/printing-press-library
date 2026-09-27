package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/store"
)

func assertRawRecoveryGuidance(t *testing.T, text string) {
	t.Helper()
	for _, obsolete := range []string{"tablecheck-pp-cli sync", "sync --resources", "enable offline access"} {
		if strings.Contains(text, obsolete) {
			t.Fatalf("removed store-population instruction %q in %q", obsolete, text)
		}
	}
	for _, required := range []string{"--data-source live", "availability check", "availability scan", "--refresh", "separate from this raw local store"} {
		if !strings.Contains(text, required) {
			t.Fatalf("recovery guidance omits %q: %q", required, text)
		}
	}
	root := RootCmd()
	for _, command := range [][]string{{"availability", "check"}, {"availability", "scan"}} {
		leaf, args, err := root.Find(command)
		if err != nil || len(args) != 0 || leaf == root || leaf.Flags().Lookup("refresh") == nil {
			t.Fatalf("recommended alternative unavailable: %v, error=%v", command, err)
		}
	}
}

func TestRawLocalRecoveryGuidanceForMissingAndEmptyInventory(t *testing.T) {
	for _, mode := range []string{"missing_database", "empty_collection", "missing_object"} {
		t.Run(mode, func(t *testing.T) {
			testenv.Isolate(t)
			path := defaultDBPath("tablecheck-pp-cli")
			if mode != "missing_database" {
				db, err := store.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			collection := mode != "missing_object"
			requestPath := "/v2/shop_search"
			if !collection {
				requestPath = "/v2/shops/missing"
			}
			var hints bytes.Buffer
			data, prov, err := resolveLocal(context.Background(), &rootFlags{dataSource: "local", maxAge: time.Minute}, &hints, "source", collection, requestPath, nil, "user_requested")
			if err == nil || data != nil || prov.Source != "" {
				t.Fatalf("missing data became success: data=%s provenance=%v error=%v", data, prov, err)
			}
			assertRawRecoveryGuidance(t, err.Error())
		})
	}
}

type recoveryDNSFailureTransport struct {
	failure *net.DNSError
	calls   int
}

func (f *recoveryDNSFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls++
	return nil, f.failure
}

func TestRawNetworkFallbackRecoveryPreservesOriginalError(t *testing.T) {
	for _, paginated := range []bool{false, true} {
		name := "single"
		if paginated {
			name = "paginated"
		}
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			original := &net.DNSError{Err: "offline fixture", Name: "example.invalid", IsNotFound: true}
			transport := &recoveryDNSFailureTransport{failure: original}
			c := client.New(&config.Config{BaseURL: "https://example.invalid"}, time.Second, 0)
			c.NoCache = true
			c.HTTPClient = &http.Client{Transport: transport}
			flags := &rootFlags{dataSource: "auto"}
			var data json.RawMessage
			var prov DataProvenance
			var err error
			if paginated {
				data, prov, err = resolvePaginatedRead(context.Background(), c, flags, "source", "/v2/shop_search", nil, nil, false, "search_after", "cursor", "per_page", 10, "meta.search_after", "", nil)
			} else {
				data, prov, err = resolveRead(context.Background(), c, flags, "source", true, "/v2/shop_search", nil, nil, nil)
			}
			if err == nil || data != nil || prov.Source != "" {
				t.Fatalf("network failure became inventory: data=%s provenance=%v error=%v", data, prov, err)
			}
			assertRawRecoveryGuidance(t, err.Error())
			if !errors.Is(err, original) || !strings.Contains(err.Error(), "Original error:") {
				t.Fatalf("network failure wrapping changed: %v", err)
			}
			if transport.calls != 1 {
				t.Fatalf("permanent DNS failure attempts=%d, want1", transport.calls)
			}
		})
	}
}

func TestRawStaleAndEmptyHintsUseAvailablePlanningRecovery(t *testing.T) {
	for _, stale := range []bool{false, true} {
		name := "empty"
		if stale {
			name = "stale"
		}
		t.Run(name, func(t *testing.T) {
			db := newSyncHintTestStore(t)
			if stale {
				if _, err := db.DB().Exec(`INSERT INTO sync_state(resource_type,last_synced_at,total_count) VALUES(?,?,?)`, "source", time.Now().Add(-2*time.Hour), 1); err != nil {
					t.Fatal(err)
				}
			}
			var emitted bytes.Buffer
			emitSyncHints(&emitted, db, "source", 30*time.Minute)
			assertRawRecoveryGuidance(t, emitted.String())
			cmd, hints := newSyncHintTestCmd()
			var reported bool
			if stale {
				reported = hintIfStale(cmd, db, "source", 30*time.Minute)
			} else {
				reported = hintIfUnsynced(cmd, db, "source")
			}
			if !reported {
				t.Fatal("expected existing hint status to remaintrue")
			}
			assertRawRecoveryGuidance(t, hints.String())
		})
	}
}

func TestRawStaleRecoveryDoesNotChangeLocalDataOrProvenance(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(defaultDBPath("tablecheck-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"id":"known","name":"cached venue"}`)
	if err := db.Upsert("source", "known", payload); err != nil {
		t.Fatal(err)
	}
	observed := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	if _, err := db.DB().Exec(`INSERT INTO sync_state(resource_type,last_synced_at,total_count) VALUES(?,?,?)`, "source", observed, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var hints bytes.Buffer
	data, prov, err := resolveLocal(context.Background(), &rootFlags{dataSource: "local", maxAge: 30 * time.Minute}, &hints, "source", false, "/v2/shops/known", nil, "user_requested")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) || prov.Source != "local" || prov.Reason != "user_requested" || prov.ResourceType != "source" || prov.SyncedAt == nil || !prov.SyncedAt.Equal(observed) {
		t.Fatalf("raw data/provenance changed: data=%s provenance=%+v", data, prov)
	}
	assertRawRecoveryGuidance(t, hints.String())
}

func TestRawMaxAgeHelpRecommendsLiveRead(t *testing.T) {
	flag := RootCmd().PersistentFlags().Lookup("max-age")
	if flag == nil || strings.Contains(flag.Usage, "sync") || !strings.Contains(flag.Usage, "live read") {
		t.Fatalf("obsolete age help: %v", flag)
	}
}
