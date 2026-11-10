package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

type recordedSyncRequest struct {
	path   string
	params map[string]string
}

type recordingSyncClient struct {
	requests []recordedSyncRequest
}

func (c *recordingSyncClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	c.requests = append(c.requests, recordedSyncRequest{path: path, params: copied})
	return json.RawMessage(`{"data":[]}`), nil
}

func (c *recordingSyncClient) RateLimit() float64 { return 0 }

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestKlaviyoSyncDescriptorsControlRequestParams(t *testing.T) {
	cases := []struct {
		resource       string
		wantPath       string
		wantPageSize   string
		forbidPageSize bool
	}{
		{resource: "accounts", wantPath: "/api/accounts", forbidPageSize: true},
		{resource: "custom-metrics", wantPath: "/api/custom-metrics", forbidPageSize: true},
		{resource: "catalog-category-bulk-create-jobs", wantPath: "/api/catalog-category-bulk-create-jobs", forbidPageSize: true},
		{resource: "flows", wantPath: "/api/flows", wantPageSize: "50"},
		{resource: "lists", wantPath: "/api/lists", wantPageSize: "10"},
	}

	for _, tc := range cases {
		t.Run(tc.resource, func(t *testing.T) {
			db := openTestStore(t)
			c := &recordingSyncClient{}
			res := syncResource(context.Background(), c, db, tc.resource, "2026-07-01T00:00:00Z", false, 1, false, false, nil, nil)
			if res.Err != nil || res.Warn != nil {
				t.Fatalf("syncResource returned err=%v warn=%v", res.Err, res.Warn)
			}
			if len(c.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(c.requests))
			}
			req := c.requests[0]
			if req.path != tc.wantPath {
				t.Fatalf("path = %q, want %q", req.path, tc.wantPath)
			}
			if _, ok := req.params["since"]; ok {
				t.Fatalf("unexpected since param on %s: %#v", tc.resource, req.params)
			}
			if tc.forbidPageSize {
				if _, ok := req.params["page[size]"]; ok {
					t.Fatalf("unexpected page[size] on %s: %#v", tc.resource, req.params)
				}
			} else if got := req.params["page[size]"]; got != tc.wantPageSize {
				t.Fatalf("page[size] = %q, want %q (params %#v)", got, tc.wantPageSize, req.params)
			}
		})
	}
}

func TestKlaviyoCampaignSyncUsesUnfilteredContainerCollection(t *testing.T) {
	db := openTestStore(t)
	c := &recordingSyncClient{}
	res := syncResource(context.Background(), c, db, "campaigns", "", false, 1, false, false, nil, nil)
	if res.Err != nil || res.Warn != nil {
		t.Fatalf("syncResource returned err=%v warn=%v", res.Err, res.Warn)
	}
	if len(c.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(c.requests))
	}
	if req := c.requests[0]; req.params["page[size]"] != "100" || req.params["filter"] != "" {
		t.Fatalf("campaign params = %#v", req.params)
	}
	cursor, lastSynced, count, err := db.GetSyncState("campaigns")
	if err != nil {
		t.Fatalf("get campaign sync state: %v", err)
	}
	if cursor != "" {
		t.Fatalf("campaign cursor = %q, want empty completed cursor", cursor)
	}
	if lastSynced.IsZero() {
		t.Fatalf("campaign sync state was not saved")
	}
	if count != 0 {
		t.Fatalf("campaign sync count = %d, want 0 for empty mock pages", count)
	}
}

func TestKlaviyoCouponCodesSkipWithoutSyncedCoupons(t *testing.T) {
	db := openTestStore(t)
	c := &recordingSyncClient{}
	res := syncResource(context.Background(), c, db, "coupon-codes", "", false, 1, false, false, nil, nil)
	if res.Err != nil {
		t.Fatalf("syncResource error = %v", res.Err)
	}
	if res.Warn == nil {
		t.Fatalf("expected warning when coupon-codes has no synced coupon parents")
	}
	if len(c.requests) != 0 {
		t.Fatalf("requests = %d, want 0", len(c.requests))
	}
}

func TestKlaviyoDefaultSyncResourcesExcludeClientEndpoint(t *testing.T) {
	for _, resource := range defaultSyncResources() {
		if resource == "client" {
			t.Fatalf("client endpoint must not be in default sync resources")
		}
	}
}

func TestAuthAccountsSwitchEffectiveConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KLAVIYO_API_KEY", "")
	t.Setenv("KLAVIYO_CONFIG", "")

	root := RootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"auth", "accounts", "add", "shop-a", "pk_one", "--agent"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add shop-a: %v", err)
	}

	cfg, err := loadEffectiveKlaviyoConfig("")
	if err != nil {
		t.Fatalf("load effective config: %v", err)
	}
	if cfg.KlaviyoApiKey != "pk_one" {
		t.Fatalf("active api key = %q, want pk_one", cfg.KlaviyoApiKey)
	}

	root = RootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"auth", "accounts", "add", "shop-b", "pk_two", "--use=false", "--agent"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add shop-b: %v", err)
	}
	cfg, err = loadEffectiveKlaviyoConfig("")
	if err != nil {
		t.Fatalf("load effective config after non-selected add: %v", err)
	}
	if cfg.KlaviyoApiKey != "pk_one" {
		t.Fatalf("active api key after non-selected add = %q, want pk_one", cfg.KlaviyoApiKey)
	}

	root = RootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"auth", "accounts", "use", "shop-b", "--agent"})
	if err := root.Execute(); err != nil {
		t.Fatalf("use shop-b: %v", err)
	}
	cfg, err = loadEffectiveKlaviyoConfig("")
	if err != nil {
		t.Fatalf("load effective config after switch: %v", err)
	}
	if cfg.KlaviyoApiKey != "pk_two" {
		t.Fatalf("active api key after switch = %q, want pk_two", cfg.KlaviyoApiKey)
	}

	overridePath := filepath.Join(t.TempDir(), "override.toml")
	override, err := config.Load(overridePath)
	if err != nil {
		t.Fatalf("load override config: %v", err)
	}
	if err := override.SaveCredential("pk_override"); err != nil {
		t.Fatalf("save override credential: %v", err)
	}
	t.Setenv("KLAVIYO_CONFIG", overridePath)
	cfg, err = loadEffectiveKlaviyoConfig("")
	if err != nil {
		t.Fatalf("load env override config: %v", err)
	}
	if cfg.KlaviyoApiKey != "pk_override" {
		t.Fatalf("env override api key = %q, want pk_override", cfg.KlaviyoApiKey)
	}
	os.Unsetenv("KLAVIYO_CONFIG")

	cfg, err = loadEffectiveKlaviyoConfig(overridePath)
	if err != nil {
		t.Fatalf("load flag override config: %v", err)
	}
	if cfg.KlaviyoApiKey != "pk_override" {
		t.Fatalf("flag override api key = %q, want pk_override", cfg.KlaviyoApiKey)
	}
}

func TestAuthAccountsMissingNamedConfigErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KLAVIYO_API_KEY", "")
	t.Setenv("KLAVIYO_CONFIG", "")

	root := RootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"auth", "accounts", "show", "missing", "--agent"})
	if err := root.Execute(); err == nil {
		t.Fatalf("show missing account succeeded, want error")
	}

	if err := writeCurrentKlaviyoAccountName("missing"); err != nil {
		t.Fatalf("write active account: %v", err)
	}
	if _, err := loadEffectiveKlaviyoConfig(""); err == nil {
		t.Fatalf("load missing active account succeeded, want error")
	}
}
