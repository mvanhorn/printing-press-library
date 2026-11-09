package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestConcurrentFirstVerifiedOpensShareTenantMetadata(t *testing.T) {
	const openers = 12
	dbPath := filepath.Join(t.TempDir(), "profile.db")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := make(chan struct{})
	errs := make(chan error, openers)
	var workers sync.WaitGroup
	for range openers {
		workers.Go(func() {
			<-start
			owner := klaviyoStoreSession(time.Unix(1, 0).UTC())
			st, err := OpenVerifiedWithContext(ctx, dbPath, owner)
			if err != nil {
				errs <- err
				return
			}
			defer st.Close()
			if err := platform.VerifyTenantMetadataReadOnly(ctx, st.DB(), owner); err != nil {
				errs <- err
				return
			}
			var count int
			var profile, source, expected, observed, fingerprint, gateAt, provenance string
			err = st.DB().QueryRowContext(ctx, `SELECT COUNT(*), client_profile, source_slug, expected_identity,
				observed_identity, credential_fingerprint, last_gate_at, migration_provenance FROM pp_tenant_metadata`).
				Scan(&count, &profile, &source, &expected, &observed, &fingerprint, &gateAt, &provenance)
			if err == nil && (count != 1 || profile != "tenant-a" || source != "klaviyo" ||
				expected != `{"account_id":"acct-a","org_name":"Tenant A"}` || observed != expected ||
				fingerprint != "fixture-fingerprint" || gateAt != "1970-01-01T00:00:01Z" || provenance != "new-profile-store") {
				err = errors.New("first open did not preserve the single expected owner record")
			}
			errs <- err
		})
	}
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent verified open: %v", err)
		}
	}
}

func TestConcurrentDifferentOwnersCannotAdoptEachOthersStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "profile.db")
	start := make(chan struct{})
	type result struct {
		owner *platform.Session
		err   error
	}
	results := make(chan result, 2)
	for _, name := range []string{"tenant-a", "tenant-b"} {
		go func() {
			owner := klaviyoStoreSession(time.Unix(1, 0).UTC())
			owner.ProfileName = name
			<-start
			st, err := OpenVerifiedWithContext(context.Background(), dbPath, owner)
			if st != nil {
				st.Close()
			}
			results <- result{owner, err}
		}()
	}
	close(start)
	var winner *platform.Session
	successes := 0
	for range 2 {
		got := <-results
		if got.err == nil {
			successes++
			winner = got.owner
		} else if !errors.Is(got.err, platform.ErrTenantMetadataMismatch) || got.err.Error() !=
			"verifying database tenant metadata: verifying tenant metadata: database tenant metadata does not match the verified client profile" {
			t.Fatalf("losing owner error = %v", got.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful competing owners = %d, want 1", successes)
	}
	st, err := OpenVerifiedWithContext(context.Background(), dbPath, winner)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := platform.VerifyTenantMetadataReadOnly(context.Background(), st.DB(), winner); err != nil {
		t.Fatalf("winner's metadata changed: %v", err)
	}
}

func TestVerifiedOpenRefusesUnownedExistingSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE legacy_data (value TEXT); INSERT INTO legacy_data VALUES ('preserved')`); err != nil {
		t.Fatal(err)
	}
	_, err = OpenVerifiedWithContext(context.Background(), dbPath, klaviyoStoreSession(time.Now()))
	if !errors.Is(err, platform.ErrTenantMetadataMissing) || err.Error() !=
		"verifying database tenant metadata: database has no verified tenant metadata; use client migrate instead of automatic adoption" {
		t.Fatalf("unowned database error = %v", err)
	}
	var value string
	if err := db.QueryRow(`SELECT value FROM legacy_data`).Scan(&value); err != nil || value != "preserved" {
		t.Fatalf("legacy data changed: value=%q err=%v", value, err)
	}
	var metadataTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'pp_tenant_metadata'`).Scan(&metadataTables); err != nil || metadataTables != 0 {
		t.Fatalf("unowned schema acquired metadata: count=%d err=%v", metadataTables, err)
	}
}
