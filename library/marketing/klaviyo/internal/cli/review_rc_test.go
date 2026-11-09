package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func rcClearLegacyEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"KLAVIYO_CONFIG", "KLAVIYO_API_KEY", "KLAVIYO_BASE_URL", "KLAVIYO_DB", "KLAVIYO_DATABASE", "KLAVIYO_DATA_FILE", "KLAVIYO_DATA_DIR", "KLAVIYO_STATE_DIR", "KLAVIYO_CACHE_DIR", "KLAVIYO_HOME"} {
		t.Setenv(name, "")
	}
}

func rcOfflineSyncRoot(t *testing.T) (*cobra.Command, *rootFlags) {
	t.Helper()
	rcClearLegacyEnvironment(t)
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")
	previous := registeredPlatformSource
	registration := *previous
	registration.BindClient = func(c *client.Client, _ *platform.Session) error {
		c.BaseURL = "http://127.0.0.1:1"
		c.Config.KlaviyoApiKey = "fixture-key"
		return nil
	}
	registeredPlatformSource = &registration
	t.Cleanup(func() { registeredPlatformSource = previous })
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "rc-tenant", Sources: map[string]platform.SourceProfile{
		"klaviyo": {CredentialRef: "op://Test/Klaviyo/key", ExpectedAccountID: "acct-rc", ExpectedOrgName: "RC"},
	}}
	session, err := platform.PrepareProfileSourceFromConfig(profile, "klaviyo-pp-cli", "klaviyo")
	if err != nil {
		t.Fatal(err)
	}
	session.GateOutcome = platform.GateVerified
	session.ObservedIdentity = map[string]string{"account_id": "acct-rc", "org_name": "RC"}
	session.CredentialFingerprint = "fixture-fingerprint"
	flags := &rootFlags{platformSession: session}
	return newRootCmd(flags), flags
}

func rcCheckSyncRequest(t *testing.T, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 0 || r.Method != http.MethodGet {
		t.Errorf("sync request = %s %s body=%q error=%v", r.Method, r.URL, body, err)
	}
	if r.Header.Get("Authorization") != "Klaviyo-API-Key fixture-key" || r.Header.Get("revision") != "2026-04-15" {
		t.Error("sync authorization or revision header changed")
	}
}

func TestRCSyncDryRunPreservesDatabase(t *testing.T) {
	for _, fixture := range []string{"missing", "legacy", "verified", "invalid"} {
		t.Run(fixture, func(t *testing.T) {
			root, flags := rcOfflineSyncRoot(t)
			dbPath := flags.platformSession.Paths.DataFile
			if fixture == "legacy" || fixture == "verified" {
				var db *store.Store
				var err error
				if fixture == "verified" {
					db, err = store.OpenVerifiedWithContext(context.Background(), dbPath, flags.platformSession)
				} else {
					db, err = store.Open(dbPath)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := db.SaveSyncState("profiles", "resume-me", 17); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			} else if fixture == "invalid" {
				if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dbPath, []byte("not a database"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(dbPath)
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(io.Discard)
			args := []string{"sync", "--resources", "profiles", "--full", "--dry-run", "--db", dbPath, "--json"}
			withSince := fixture == "legacy" || fixture == "invalid"
			if withSince {
				args = append(args, "--since", "7d")
			}
			if fixture == "verified" {
				args = append(args, "--latest-only")
			}
			root.SetArgs(args)
			started := time.Now().Add(-7 * 24 * time.Hour).Truncate(time.Second)
			if err := root.Execute(); err != nil {
				t.Fatalf("dry run: %v", err)
			}
			var preview struct {
				DryRun     bool                `json:"dry_run"`
				Full       bool                `json:"full"`
				Resources  []string            `json:"resources"`
				Filters    map[string][]string `json:"filters"`
				Since      string              `json:"since"`
				LatestOnly bool                `json:"latest_only"`
				MaxPages   int                 `json:"max_pages"`
			}
			if err := json.Unmarshal(platformTestOutputPayload(t, output.Bytes()), &preview); err != nil {
				t.Fatalf("decode preview: %v: %s", err, output.String())
			}
			wantFilters := []string{}
			if withSince {
				cutoff, err := time.Parse(time.RFC3339, preview.Since)
				if err != nil || cutoff.Before(started) || cutoff.After(time.Now().Add(-7*24*time.Hour)) {
					t.Fatalf("preview since = %q: %v", preview.Since, err)
				}
				wantFilters = append(wantFilters, "greater-than(updated,"+preview.Since+")")
			} else if preview.Since != "" {
				t.Fatalf("preview added an unrequested cutoff: %q", preview.Since)
			}
			if !preview.DryRun || !preview.Full || !reflect.DeepEqual(preview.Resources, []string{"profiles"}) || !reflect.DeepEqual(preview.Filters, map[string][]string{"profiles": wantFilters}) {
				t.Fatalf("preview = %+v", preview)
			}
			if fixture == "verified" && (!preview.LatestOnly || preview.MaxPages != 1) {
				t.Fatalf("latest-only preview = %+v", preview)
			}
			after, err := os.ReadFile(dbPath)
			if fixture == "missing" {
				if !os.IsNotExist(err) {
					t.Fatalf("dry run created database: %v", err)
				}
			} else if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("dry run modified database: %v", err)
			}
			for _, suffix := range []string{"-wal", "-shm"} {
				if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
					t.Errorf("dry run created %s: %v", suffix, err)
				}
			}
		})
	}
}

func TestRCSyncSinceRejectsUnsupportedBeforeOpeningStore(t *testing.T) {
	for _, resource := range []string{"accounts", "catalog-items", "coupon-codes", "metrics", "tags", "push-tokens", "profile-bulk-import-jobs", "profiles,tags"} {
		t.Run(resource, func(t *testing.T) {
			root, flags := rcOfflineSyncRoot(t)
			dbPath := flags.platformSession.Paths.DataFile
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"sync", "--resources", resource, "--since", "7d", "--full", "--dry-run", "--db", dbPath})
			unsupported := resource[strings.LastIndex(resource, ",")+1:]
			want := fmt.Sprintf("--since is not supported for resource %q: endpoint has no incremental time filter", unsupported)
			if err := root.Execute(); err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("unsupported --since created database: %v", err)
			}
		})
	}
}

func TestRCSyncFlowsIncrementalSortAndCheckpoint(t *testing.T) {
	for _, since := range []string{"", "2026-11-02T12:00:00Z"} {
		t.Run("since="+since, func(t *testing.T) {
			db := openTestStore(t)
			if err := db.SaveSyncState("flows", "unfiltered-next", 42); err != nil {
				t.Fatal(err)
			}
			_, savedAt, _, err := db.GetSyncState("flows")
			if err != nil {
				t.Fatal(err)
			}
			c := &recordingSyncClient{}
			res := syncResource(c, db, "flows", since, false, 1)
			if res.Err != nil || res.Warn != nil {
				t.Fatalf("sync result = %+v", res)
			}
			params := map[string]string{"page[size]": "50", "page[cursor]": "unfiltered-next"}
			if since != "" {
				params = map[string]string{"page[size]": "50", "filter": "greater-or-equal(updated," + since + ")", "sort": "updated"}
			}
			want := []recordedSyncRequest{{path: "/api/flows", params: params}}
			if !reflect.DeepEqual(c.requests, want) {
				t.Fatalf("requests = %+v, want %+v", c.requests, want)
			}
			cursor, lastSynced, count, err := db.GetSyncState("flows")
			if err != nil {
				t.Fatal(err)
			}
			if since != "" && (cursor != "unfiltered-next" || !lastSynced.Equal(savedAt) || count != 42) {
				t.Fatalf("filtered completion changed checkpoint: cursor=%q time=%v count=%d", cursor, lastSynced, count)
			}
			if since == "" && cursor != "" {
				t.Fatalf("unfiltered completion did not clear cursor: %q", cursor)
			}
		})
	}
}

func TestRCSyncSinceUsesDocumentedFilters(t *testing.T) {
	const cutoff = "2026-11-02T12:00:00Z"
	for _, tc := range []struct{ resource, filter string }{
		{"events", "greater-or-equal(datetime,%s)"},
		{"profiles", "greater-than(updated,%s)"},
		{"campaigns", "greater-or-equal(updated_at,%s)"},
		{"flows", "greater-or-equal(updated,%s)"},
		{"forms", "greater-or-equal(updated_at,%s)"},
		{"images", "greater-or-equal(updated_at,%s)"},
		{"lists", "greater-than(updated,%s)"},
		{"segments", "greater-than(updated,%s)"},
		{"reviews", "greater-or-equal(created,%s)"},
		{"templates", "greater-or-equal(updated,%s)"},
		{"template-universal-content", "greater-or-equal(updated,%s)"},
		{"web-feeds", "greater-or-equal(updated,%s)"},
	} {
		t.Run(tc.resource, func(t *testing.T) {
			var mu sync.Mutex
			var got []string
			_, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
				rcCheckSyncRequest(t, r)
				if r.URL.Path != "/api/"+tc.resource {
					t.Errorf("path = %q", r.URL.Path)
				}
				query := r.URL.Query()
				filter := query.Get("filter")
				mu.Lock()
				got = append(got, filter)
				mu.Unlock()
				delete(query, "filter")
				desc, _ := syncDescriptorForResource(tc.resource)
				want := url.Values{"page[size]": {fmt.Sprint(desc.MaxPageSize)}}
				if tc.resource == "flows" {
					want.Set("sort", "updated")
				}
				if !reflect.DeepEqual(query, want) {
					t.Errorf("query without filter = %v, want %v", query, want)
				}
				fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
			})
			c, err := flags.newClient()
			if err != nil {
				t.Fatal(err)
			}
			db := openTestStore(t)
			if err := db.SaveSyncState(tc.resource, "old-query-cursor", 42); err != nil {
				t.Fatal(err)
			}
			_, savedAt, _, err := db.GetSyncState(tc.resource)
			if err != nil {
				t.Fatal(err)
			}
			res := syncResource(c, db, tc.resource, cutoff, false, 1)
			if res.Err != nil || res.Warn != nil {
				t.Fatalf("sync error=%v warning=%v", res.Err, res.Warn)
			}
			cursor, lastSynced, count, err := db.GetSyncState(tc.resource)
			if err != nil || cursor != "old-query-cursor" || !lastSynced.Equal(savedAt) || count != 42 {
				t.Fatalf("filtered completion changed unfiltered checkpoint: cursor=%q time=%v count=%d err=%v", cursor, lastSynced, count, err)
			}
			want := []string{fmt.Sprintf(tc.filter, cutoff)}
			if tc.resource == "campaigns" {
				want = nil
				for _, channel := range []string{"email", "sms", "mobile_push"} {
					want = append(want, "and(equals(messages.channel,'"+channel+"'),"+fmt.Sprintf(tc.filter, cutoff)+")")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("filters = %q, want %q", got, want)
			}
		})
	}
}

func TestRCSyncCappedRunsResumeAndReceiptIsTruncated(t *testing.T) {
	for _, emptyFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_first_page=%v", emptyFirst), func(t *testing.T) {
			rcClearLegacyEnvironment(t)
			var mu sync.Mutex
			var cursors []string
			root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
				rcCheckSyncRequest(t, r)
				cursor := r.URL.Query().Get("page[cursor]")
				mu.Lock()
				cursors = append(cursors, cursor)
				mu.Unlock()
				want := url.Values{"page[size]": {"100"}}
				if cursor != "" {
					want.Set("page[cursor]", cursor)
				}
				if r.URL.Path != "/api/profiles" || !reflect.DeepEqual(r.URL.Query(), want) {
					t.Errorf("request = %s", r.URL.RequestURI())
				}
				if cursor == "" {
					items := `[{"type":"profile","id":"p1"}]`
					if emptyFirst {
						items = `[]`
					}
					fmt.Fprintf(w, `{"data":%s,"links":{"next":"/api/profiles?page%%5Bcursor%%5D=page2"}}`, items)
				} else if cursor == "page2" {
					fmt.Fprint(w, `{"data":[{"type":"profile","id":"p2"}],"links":{"next":null}}`)
				} else {
					t.Errorf("unexpected cursor = %q", cursor)
					http.Error(w, "unexpected cursor", http.StatusBadRequest)
				}
			})
			for run := 0; run < 2; run++ {
				if run > 0 {
					// Each invocation gets fresh output metadata while retaining its tenant store.
					profile := flags.platformSession.Profile
					session, err := platform.PrepareProfileSourceFromConfig(profile, "klaviyo-pp-cli", "klaviyo")
					if err != nil {
						t.Fatal(err)
					}
					session.GateOutcome = platform.GateVerified
					session.ObservedIdentity = flags.platformSession.ObservedIdentity
					session.CredentialFingerprint = flags.platformSession.CredentialFingerprint
					flags = &rootFlags{platformSession: session}
					root = newRootCmd(flags)
				}
				receiptPath := filepath.Join(t.TempDir(), "receipt.json")
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.SetArgs([]string{"sync", "--resources", "profiles", "--max-pages", "1", "--receipt-file", receiptPath})
				err := root.Execute()
				finalizePlatformInvocation(flags, &err)
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				var receipt platform.Receipt
				if err := json.Unmarshal(data, &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Truncated != (run == 0) {
					t.Fatalf("run %d receipt truncated = %v", run, receipt.Truncated)
				}
				if run == 0 && (len(receipt.CapsHit) != 1 || receipt.CapsHit[0].Kind != "max_pages" || receipt.CapsHit[0].ResumeCursor != "page2" || !receipt.CapsHit[0].MoreAvailable) {
					t.Fatalf("caps hit = %+v", receipt.CapsHit)
				}
				db, err := store.OpenVerifiedWithContext(context.Background(), flags.platformSession.Paths.DataFile, flags.platformSession)
				if err != nil {
					t.Fatal(err)
				}
				cursor, _, count, stateErr := db.GetSyncState("profiles")
				ids, idsErr := db.ListIDs("profiles")
				_ = db.Close()
				wantCursor := "page2"
				wantCount := 1
				wantIDs := []string{"p1"}
				if emptyFirst {
					wantCount = 0
					wantIDs = nil
				}
				if run == 1 {
					wantCursor = ""
					wantCount = 1
					wantIDs = append(wantIDs, "p2")
				}
				if stateErr != nil || cursor != wantCursor || count != wantCount {
					t.Fatalf("run %d checkpoint = %q, %d, %v", run, cursor, count, stateErr)
				}
				if idsErr != nil || !reflect.DeepEqual(ids, wantIDs) {
					t.Fatalf("run %d stored profiles = %q, want %q: %v", run, ids, wantIDs, idsErr)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(cursors, []string{"", "page2"}) {
				t.Fatalf("request cursors = %q", cursors)
			}
		})
	}
}

func TestRCSyncInvalidSincePreservesCheckpoint(t *testing.T) {
	root, flags := rcOfflineSyncRoot(t)
	dbPath := flags.platformSession.Paths.DataFile
	db, err := store.OpenVerifiedWithContext(context.Background(), dbPath, flags.platformSession)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("profiles", "resume-me", 17); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"sync", "--resources", "profiles", "--full", "--since", "invalid"})
	want := `invalid --since value "invalid": expected format like 7d, 24h, 1w, or 30m`
	if err := root.Execute(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	after, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("invalid --since modified database: %v", err)
	}
}

func TestRCSyncSinceAllSkipsUnsupported(t *testing.T) {
	rcClearLegacyEnvironment(t)
	var mu sync.Mutex
	seen := map[string]int{}
	supported := map[string]bool{"events": true, "profiles": true, "campaigns": true, "flows": true, "forms": true, "images": true, "lists": true, "segments": true, "reviews": true, "templates": true, "template-universal-content": true, "web-feeds": true}
	root, _ := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
		rcCheckSyncRequest(t, r)
		resource := strings.TrimPrefix(r.URL.Path, "/api/")
		if !supported[resource] || r.URL.Query().Get("filter") == "" {
			t.Errorf("incremental request without supported filter: %s", r.URL.RequestURI())
		}
		mu.Lock()
		seen[resource]++
		mu.Unlock()
		fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
	})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"sync", "--since", "7d", "--strict"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != len(supported) {
		t.Fatalf("synced resources = %v", seen)
	}
	for resource := range supported {
		want := 1
		if resource == "campaigns" {
			want = 3
		}
		if seen[resource] != want {
			t.Errorf("%s requests = %d, want %d", resource, seen[resource], want)
		}
	}
}

type rcCappedSyncClient struct{}

func (rcCappedSyncClient) Get(string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(`{"data":[{"type":"profile","id":"p1"}],"links":{"next":"/api/profiles?page%5Bcursor%5D=filtered-next"}}`), nil
}

func (rcCappedSyncClient) RateLimit() float64 { return 0 }

func TestRCSyncCapWarningUsesCommandWriter(t *testing.T) {
	for _, human := range []bool{false, true} {
		t.Run(fmt.Sprintf("human=%v", human), func(t *testing.T) {
			previous := humanFriendly
			humanFriendly = human
			t.Cleanup(func() { humanFriendly = previous })
			var diagnostics bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetErr(&diagnostics)
			db := openTestStore(t)
			res := syncResource(rcCappedSyncClient{}, db, "profiles", "2026-11-02T12:00:00Z", false, 1, cmd.ErrOrStderr())
			if res.Err != nil || res.Warn != nil || res.Count != 1 {
				t.Fatalf("sync result = %+v", res)
			}
			want := `{"event":"sync_warning","resource":"profiles","reason":"max_pages_cap_hit","message":"reached --max-pages cap of 1; data may be truncated. Re-run with --max-pages 0 (unlimited) or higher to verify. --since cursor is not persisted; the saved unfiltered checkpoint is unchanged."}` + "\n"
			if human {
				want = "\n  profiles: reached --max-pages limit (1 pages, 1 items) --since cursor is not persisted; the saved unfiltered checkpoint is unchanged.\n"
			}
			if diagnostics.String() != want {
				t.Fatalf("command diagnostics = %q, want %q", diagnostics.String(), want)
			}
		})
	}
}

func TestRCSyncCappedSincePreservesUnfilteredCheckpoint(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprintf("full=%v", full), func(t *testing.T) {
			rcClearLegacyEnvironment(t)
			var calls atomic.Int32
			root, flags := g2Root(t, func(w http.ResponseWriter, r *http.Request) {
				rcCheckSyncRequest(t, r)
				call := calls.Add(1)
				query := r.URL.Query()
				if r.URL.Path != "/api/profiles" {
					t.Errorf("path = %q", r.URL.Path)
				}
				if call == 1 {
					filter := query.Get("filter")
					ts := strings.TrimSuffix(strings.TrimPrefix(filter, "greater-than(updated,"), ")")
					if _, err := time.Parse(time.RFC3339, ts); err != nil {
						t.Errorf("invalid incremental filter = %q", filter)
					}
					want := url.Values{"filter": {"greater-than(updated," + ts + ")"}, "page[size]": {"100"}}
					if !reflect.DeepEqual(query, want) {
						t.Errorf("filtered query = %v, want %v", query, want)
					}
					fmt.Fprint(w, `{"data":[{"type":"profile","id":"p-filtered"}],"links":{"next":"/api/profiles?page%5Bcursor%5D=filtered-next"}}`)
				} else {
					want := url.Values{"page[cursor]": {"unfiltered-next"}, "page[size]": {"100"}}
					if call != 2 || !reflect.DeepEqual(query, want) {
						t.Errorf("unfiltered query = %v, want %v (call %d)", query, want, call)
					}
					fmt.Fprint(w, `{"data":[{"type":"profile","id":"p-unfiltered"}],"links":{"next":null}}`)
				}
			})
			dbPath := flags.platformSession.Paths.DataFile
			db, err := store.OpenVerifiedWithContext(context.Background(), dbPath, flags.platformSession)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SaveSyncState("profiles", "unfiltered-next", 42); err != nil {
				t.Fatal(err)
			}
			_, savedAt, _, err := db.GetSyncState("profiles")
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			receiptPath := filepath.Join(t.TempDir(), "receipt.json")
			root.SetOut(io.Discard)
			var diagnostics bytes.Buffer
			root.SetErr(&diagnostics)
			args := []string{"sync", "--resources", "profiles", "--since", "1d", "--max-pages", "1", "--receipt-file", receiptPath}
			if full {
				args = append(args, "--full")
			}
			root.SetArgs(args)
			err = root.Execute()
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatal(err)
			}
			foundWarning := false
			for _, line := range bytes.Split(diagnostics.Bytes(), []byte{'\n'}) {
				var event struct{ Event, Reason, Message string }
				if json.Unmarshal(line, &event) == nil && event.Event == "sync_warning" && event.Reason == "max_pages_cap_hit" {
					foundWarning = true
					want := "reached --max-pages cap of 1; data may be truncated. Re-run with --max-pages 0 (unlimited) or higher to verify. --since cursor is not persisted; the saved unfiltered checkpoint is unchanged."
					if event.Message != want {
						t.Errorf("truncation warning = %q, want %q", event.Message, want)
					}
				}
			}
			if !foundWarning {
				t.Fatalf("missing truncation warning: %s", diagnostics.String())
			}
			receiptData, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			var receipt platform.Receipt
			if err := json.Unmarshal(receiptData, &receipt); err != nil {
				t.Fatal(err)
			}
			if !receipt.Truncated || len(receipt.CapsHit) != 1 || receipt.CapsHit[0].ResumeCursor != "" {
				t.Fatalf("filtered truncation receipt = %+v", receipt)
			}
			db, err = store.OpenVerifiedWithContext(context.Background(), dbPath, flags.platformSession)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			cursor, lastSynced, count, err := db.GetSyncState("profiles")
			if err != nil || cursor != "unfiltered-next" || !lastSynced.Equal(savedAt) || count != 42 {
				t.Fatalf("filtered cap changed unfiltered checkpoint: cursor=%q time=%v count=%d err=%v", cursor, lastSynced, count, err)
			}
			c, err := flags.newClient()
			if err != nil {
				t.Fatal(err)
			}
			res := syncResource(c, db, "profiles", "", false, 1)
			if res.Err != nil || res.Warn != nil || res.Count != 1 {
				t.Fatalf("plain sync result = %+v", res)
			}
			cursor, _, _, err = db.GetSyncState("profiles")
			if err != nil || cursor != "" || calls.Load() != 2 {
				t.Fatalf("plain completion: cursor=%q calls=%d err=%v", cursor, calls.Load(), err)
			}
		})
	}
}
