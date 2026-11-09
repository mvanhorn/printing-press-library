// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func migrationRaceSession(destination string) *Session {
	return &Session{
		ProfileName: "tenant-a", Source: "klaviyo", GateOutcome: GateVerified,
		ExpectedIdentity:      map[string]string{"account_id": "acct-a"},
		ObservedIdentity:      map[string]string{"account_id": "acct-a"},
		CredentialFingerprint: "fixture", GateCheckedAt: time.Unix(1, 0).UTC(),
		Paths: Paths{DataFile: destination},
	}
}

func migrationRaceDatabase(t *testing.T, filename string, session *Session) {
	t.Helper()
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if session != nil {
		if err := InitializeTenantMetadata(context.Background(), db, session, "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE preserved (id TEXT PRIMARY KEY, payload BLOB);
INSERT INTO preserved VALUES ('original', zeroblob(4194304))`); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentLegacyMigrationsPreserveWinner(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "legacy.db")
	destination := filepath.Join(directory, "tenant.db")
	session := migrationRaceSession(destination)
	migrationRaceDatabase(t, source, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ready := make(chan struct{}, 2)
	proceed := make(chan struct{})
	option := LegacyDatabaseAdoptionOptions{ConfirmEvidence: true, InspectEvidence: func(context.Context, *sql.DB, *Session) (LegacyDatabaseEvidence, error) {
		ready <- struct{}{}
		select {
		case <-proceed:
			return LegacyDatabaseEvidence{Verified: true, Method: "fixture"}, nil
		case <-ctx.Done():
			return LegacyDatabaseEvidence{}, ctx.Err()
		}
	}}
	type result struct {
		state LegacyMigrationState
		err   error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			state := LegacyMigrationState{Database: &LegacyMigrationArtifact{Path: source}}
			err := AdoptVerifiedLegacyDatabase(ctx, session, &state, option)
			results <- result{state, err}
		}()
	}
	for range 2 {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal("migrations did not reach the evidence barrier")
		}
	}
	close(proceed)
	copied := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Errorf("concurrent adoption: %v", result.err)
		}
		if result.state.Database.Copied {
			copied++
		} else if result.state.Database.Status != "already_adopted" {
			t.Errorf("losing migration status = %q", result.state.Database.Status)
		}
	}
	if copied != 1 {
		t.Fatalf("copied migrations = %d, want exactly one", copied)
	}
	db, err := sql.Open("sqlite", readOnlySQLiteDSN(destination))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := VerifyTenantMetadataReadOnly(ctx, db, session); err != nil {
		t.Fatal(err)
	}
	var size int
	if err := db.QueryRow(`SELECT length(payload) FROM preserved WHERE id = 'original'`).Scan(&size); err != nil || size != 4194304 {
		t.Fatalf("copied data size=%d, error=%v", size, err)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("destination permissions: %v, %v", info, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("migration left temporary artifacts: %v, %v", entries, err)
	}
}

func TestLegacyMigrationPreservesExistingDestination(t *testing.T) {
	for _, sameTenant := range []bool{true, false} {
		t.Run(map[bool]string{true: "same tenant", false: "other tenant"}[sameTenant], func(t *testing.T) {
			directory := t.TempDir()
			source := filepath.Join(directory, "legacy.db")
			destination := filepath.Join(directory, "tenant.db")
			session := migrationRaceSession(destination)
			migrationRaceDatabase(t, source, session)
			owner := migrationRaceSession(destination)
			if !sameTenant {
				owner.ProfileName = "tenant-b"
			}
			migrationRaceDatabase(t, destination, owner)
			before, err := os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			state := LegacyMigrationState{Database: &LegacyMigrationArtifact{Path: source}}
			err = AdoptVerifiedLegacyDatabase(context.Background(), session, &state)
			if sameTenant {
				if err != nil || state.Database.Status != "already_adopted" || state.Database.Copied {
					t.Fatalf("existing destination: %+v, %v", state.Database, err)
				}
			} else {
				want := "existing migration destination is not owned by the verified tenant: " + ErrTenantMetadataMismatch.Error()
				if !errors.Is(err, ErrTenantMetadataMismatch) || err.Error() != want {
					t.Fatalf("error=%v, want %q", err, want)
				}
			}
			after, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("pre-existing database changed: %v", err)
			}
		})
	}
}
