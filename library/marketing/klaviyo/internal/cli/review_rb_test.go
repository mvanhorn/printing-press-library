// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	_ "modernc.org/sqlite"
)

func TestReviewRBLegacyIdentityEvidence(t *testing.T) {
	const matching = `{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Tenant A"}}}`
	const mismatch = "legacy Klaviyo account identity did not match the verified profile"
	const incomplete = "legacy Klaviyo account identity evidence was malformed or incomplete"
	for _, test := range []struct {
		name       string
		rows       []string
		verified   bool
		wantReason string
	}{
		{name: "one matching account", rows: []string{matching}, verified: true},
		{name: "all matching rows beyond old limit", rows: []string{matching, matching, matching}, verified: true},
		{name: "no accounts", wantReason: mismatch},
		{name: "mixed sparse account", rows: []string{matching, `{"id":"acct-b"}`}, wantReason: mismatch},
		{name: "mixed sparse account beyond old limit", rows: []string{matching, matching, `{"id":"acct-b"}`}, wantReason: mismatch},
		{name: "mixed complete account beyond old limit", rows: []string{matching, matching, `{"id":"acct-b","attributes":{"contact_information":{"organization_name":"Tenant B"}}}`}, wantReason: mismatch},
		{name: "malformed row", rows: []string{matching, `{"id":`}, wantReason: incomplete},
		{name: "malformed row beyond old limit", rows: []string{matching, matching, `{"id":`}, wantReason: incomplete},
		{name: "null row", rows: []string{matching, `null`}, wantReason: incomplete},
		{name: "array row", rows: []string{matching, `[]`}, wantReason: incomplete},
		{name: "missing account ID", rows: []string{matching, `{"attributes":{"contact_information":{"organization_name":"Tenant A"}}}`}, wantReason: incomplete},
		{name: "blank account ID", rows: []string{matching, `{"id":"  ","attributes":{"contact_information":{"organization_name":"Tenant A"}}}`}, wantReason: incomplete},
		{name: "missing organization", rows: []string{matching, `{"id":"acct-a"}`}, wantReason: incomplete},
		{name: "blank organization", rows: []string{matching, `{"id":"acct-a","attributes":{"contact_information":{"organization_name":"  "}}}`}, wantReason: incomplete},
		{name: "wrong organization beyond old limit", rows: []string{matching, matching, `{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Tenant B"}}}`}, wantReason: mismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			legacyPath := filepath.Join(t.TempDir(), "legacy.db")
			db, err := sql.Open("sqlite", legacyPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TABLE resources (id TEXT PRIMARY KEY, resource_type TEXT, data BLOB)`); err != nil {
				t.Fatal(err)
			}
			for i, raw := range test.rows {
				resourceType := []string{"account", "accounts"}[i%2]
				if _, err := db.Exec(`INSERT INTO resources VALUES (?, ?, ?)`, fmt.Sprintf("%03d", i), resourceType, raw); err != nil {
					t.Fatal(err)
				}
			}
			session := &platform.Session{
				ProfileName: "tenant-a", Source: "klaviyo", GateOutcome: platform.GateVerified,
				ExpectedIdentity: map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
				ObservedIdentity: map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
				Paths:            platform.Paths{DataFile: filepath.Join(t.TempDir(), "adopted.db")},
			}
			evidence, err := inspectKlaviyoLegacyDatabase(context.Background(), db, session)
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Verified != test.verified || evidence.Method != "klaviyo-account-row" || (!test.verified && evidence.Reason != test.wantReason) {
				t.Fatalf("evidence = %+v, want verified=%t, method=klaviyo-account-row, reason=%q", evidence, test.verified, test.wantReason)
			}
			if test.verified {
				return
			}
			state := &platform.LegacyMigrationState{Database: &platform.LegacyMigrationArtifact{Path: legacyPath}}
			if err := platform.AdoptVerifiedLegacyDatabase(context.Background(), session, state, platform.LegacyDatabaseAdoptionOptions{
				ConfirmEvidence: true, InspectEvidence: inspectKlaviyoLegacyDatabase,
			}); err != nil {
				t.Fatal(err)
			}
			if state.Database.Status != "quarantined" || state.Reason != test.wantReason+"; use a clean profile-scoped sync" || state.Database.Copied || state.Database.VerifiedForCleanup {
				t.Fatalf("unsafe database adoption = %+v, database=%+v", state, state.Database)
			}
			if _, err := os.Stat(session.Paths.DataFile); !os.IsNotExist(err) {
				t.Fatalf("unsafe database was copied: %v", err)
			}
			if err := platform.VerifyTenantMetadataReadOnly(context.Background(), db, session); !errors.Is(err, platform.ErrTenantMetadataMissing) {
				t.Fatalf("source metadata error = %v, want %v", err, platform.ErrTenantMetadataMissing)
			}
		})
	}
}

func TestReviewRBLegacyIdentityScanMustFinish(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// The third row fails during iteration; LIMIT 2 would conceal the failure.
	if _, err := db.Exec(`CREATE TABLE account_evidence (id TEXT PRIMARY KEY, resource_type TEXT, data BLOB);
		INSERT INTO account_evidence VALUES
		('000', 'account', '{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Tenant A"}}}'),
		('001', 'accounts', '{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Tenant A"}}}'),
		('002', 'account', '{}');
		CREATE VIEW resources AS SELECT id, resource_type,
		CASE WHEN id = '002' THEN abs(-9223372036854775808) ELSE data END AS data FROM account_evidence;`); err != nil {
		t.Fatal(err)
	}
	session := &platform.Session{ExpectedIdentity: map[string]string{"account_id": "acct-a", "org_name": "Tenant A"}}
	evidence, err := inspectKlaviyoLegacyDatabase(context.Background(), db, session)
	if evidence.Verified || err == nil || err.Error() != "SQL logic error: integer overflow (1)" {
		t.Fatalf("incomplete scan evidence=%+v error=%v, want unverified and integer overflow", evidence, err)
	}
}

func TestReviewRBReportEndpointPacing(t *testing.T) {
	for _, endpoint := range []string{
		"campaign-values-reports", "flow-values-reports", "flow-series-reports",
		"form-values-reports", "form-series-reports", "segment-values-reports", "segment-series-reports",
	} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			path := "/api/" + endpoint + "/"
			body := json.RawMessage(fmt.Sprintf(`{"data":{"type":%q,"attributes":{"statistics":["recipients"],"timeframe":{"key":"last_7_days"}}}}`, strings.TrimSuffix(endpoint, "s")))
			arrivals := make(chan time.Time, 3)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				arrivals <- time.Now()
				if r.Method != http.MethodPost || r.URL.RequestURI() != path {
					t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.RequestURI(), path)
				}
				for name, want := range map[string]string{
					"Authorization": "Klaviyo-API-Key synthetic-key", "revision": "2026-04-15",
					"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
				} {
					if got := r.Header.Get(name); got != want {
						t.Errorf("%s = %q, want %q", name, got, want)
					}
				}
				if got, err := io.ReadAll(r.Body); err != nil || string(got) != string(body) {
					t.Errorf("body = %s, error=%v, want %s", got, err, body)
				}
				w.Header().Set("Content-Type", "application/vnd.api+json")
				fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
			}))
			defer server.Close()
			session := &platform.Session{
				ProfileName: "tenant-a", Source: "klaviyo", GateOutcome: platform.GateVerified,
				Credentials:           platform.ResolvedCredentials{"credential": []byte("synthetic-key")},
				CredentialFingerprint: "synthetic-fingerprint",
				ObservedIdentity:      map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
				Paths:                 platform.Paths{StateDir: t.TempDir()},
			}
			c := client.New(&config.Config{}, 5*time.Second, 0)
			c.HTTPClient = server.Client()
			c.NoCache = true
			if err := c.BindPlatformSession(session); err != nil {
				t.Fatal(err)
			}
			if err := bindKlaviyoPlatformClient(c, session); err != nil {
				t.Fatal(err)
			}
			c.BaseURL = server.URL
			for range 3 {
				response, status, err := c.PostWithHeaders(path, body, map[string]string{
					"Accept": "application/vnd.api+json", "Content-Type": "application/vnd.api+json",
				})
				if err != nil || status != http.StatusOK || string(response) != `{"data":[],"links":{"next":null}}` {
					t.Fatalf("report response = %s, status=%d, error=%v", response, status, err)
				}
			}
			if len(arrivals) != 3 {
				t.Fatalf("report calls = %d, want 3", len(arrivals))
			}
			first, second, third := <-arrivals, <-arrivals, <-arrivals
			// Allow scheduling jitter while distinguishing 30-second pacing from the wildcard's 4 seconds.
			if second.Sub(first) < 29*time.Second || third.Sub(second) < 29*time.Second || third.Sub(first) < 59*time.Second {
				t.Fatalf("three report calls arrived at %v and %v after the first, want 30s and 60s (2/minute, burst 1)", second.Sub(first), third.Sub(first))
			}
		})
	}
}
