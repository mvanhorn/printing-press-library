// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

func learningTestSession(t *testing.T, dbPath string) *platform.Session {
	t.Helper()
	session := learningProfileTestSession(t, "learning-fixture")
	stateDir, err := cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	session.Paths.DataFile = dbPath
	session.Paths.StateDir = stateDir
	session.Paths.ReceiptsDir = filepath.Join(stateDir, "receipts")
	return session
}

func learningProfileTestSession(t *testing.T, name string) *platform.Session {
	t.Helper()
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: name, Sources: map[string]platform.SourceProfile{
		"klaviyo": {ExpectedAccountID: "acct-" + name, ExpectedOrgName: name},
	}}
	session, err := platform.PrepareProfileSourceFromConfig(profile, "klaviyo-pp-cli", "klaviyo")
	if err != nil {
		t.Fatal(err)
	}
	session.GateOutcome, session.GateCheckedAt = platform.GateVerified, time.Now().UTC()
	session.ObservedIdentity = session.ExpectedIdentity
	session.CredentialFingerprint = name
	return session
}

func openLearningTestStore(t *testing.T, dbPath string) (*store.Store, error) {
	t.Helper()
	return store.OpenVerifiedWithContext(context.Background(), dbPath, learningTestSession(t, dbPath))
}

func runTSECCommand(t *testing.T, session *platform.Session, args ...string) (string, error) {
	t.Helper()
	flags := &rootFlags{platformSession: session}
	root := newRootCmd(flags)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	previousState, stateErr := learn.StateDir()
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	executed, err := root.ExecuteC()
	if stateDir, stateErr := learn.StateDir(); stateErr != nil || stateDir != previousState {
		t.Errorf("command retained learning state binding: %s %v", stateDir, stateErr)
	}
	journalInvocation(flags, root, executed, err, "", "")
	if stateDir, stateErr := learn.StateDir(); stateErr != nil || stateDir != previousState {
		t.Errorf("journal retained learning state binding: %s %v", stateDir, stateErr)
	}
	finalizePlatformInvocation(flags, &err)
	return stdout.String(), err
}

func TestTSECLearningCommandsRequireTenantGate(t *testing.T) {
	raCLIEnvironment(t)
	registeredPlatformSource.Adapter = platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "other-tenant"}}
	for _, args := range [][]string{
		{"teach"}, {"teach-pattern"}, {"teach-lookup"}, {"teach-playbook"}, {"recall", "widgets"},
		{"learnings", "list"}, {"learnings", "list", "--warnings"}, {"learnings", "forget", "widgets", "--all"},
		{"learnings", "stats"}, {"learnings", "candidates"}, {"learnings", "confirm", "1"},
		{"learnings", "reject", "1"}, {"learnings", "purge"}, {"playbook", "list"}, {"playbook", "amend"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := runTSECCommand(t, nil, append(args, "--client-profile", "tenant-a")...)
			if err == nil || !strings.Contains(err.Error(), "tenant gate mismatch") {
				t.Fatalf("command bypassed tenant verification: %v", err)
			}
		})
	}
	session, err := platform.PrepareProfileSource("tenant-a", "klaviyo-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{session.Paths.DataFile, filepath.Join(session.Paths.StateDir, "teach.log"), filepath.Join(session.Paths.StateDir, "learnings.jsonl"), filepath.Join(session.Paths.StateDir, "learn")} {
		if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed tenant gate touched learning artifact %s: %v", filename, err)
		}
	}
}

func TestTSECLearningProfilesIsolateDatabaseJournalAndLogs(t *testing.T) {
	home := withTempLearnHome(t)
	learnInitOnce, playbookInitOnce = sync.Once{}, sync.Once{}
	t.Cleanup(func() { learnInitOnce, playbookInitOnce = sync.Once{}, sync.Once{} })
	a, b := learningProfileTestSession(t, "tenant-a"), learningProfileTestSession(t, "tenant-b")
	legacyLog := filepath.Join(home, ".local", "share", "klaviyo-pp-cli", "teach.log")
	if err := os.MkdirAll(filepath.Dir(legacyLog), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyLog, []byte("{\"warning\":\"legacy-secret\",\"resource\":\"other-tenant-id\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyDB := defaultDBPath("klaviyo-pp-cli")
	if err := os.WriteFile(legacyDB, []byte("unverified legacy database"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(mcpBoundProfileEnv, a.ProfileName)
	if _, err := runTSECCommand(t, a, "teach", "--query", "find all widgets in inventory", "--resource", "widget-a", "--resource-type", "items",
		"--playbook-json", `{"steps":[{"cmd":"items list tenant-a"}]}`); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"learnings", "list", "--json"}, {"playbook", "list", "--json"}, {"recall", "find all widgets in inventory", "--json"}} {
		output, err := runTSECCommand(t, a, args...)
		if err != nil || (!strings.Contains(output, "widget-a") && !strings.Contains(output, "items list tenant-a")) {
			t.Fatalf("tenant A cannot read its learning: output=%s error=%v", output, err)
		}
		t.Setenv(mcpBoundProfileEnv, b.ProfileName)
		output, err = runTSECCommand(t, b, args...)
		if err != nil || strings.Contains(output, "widget-a") || strings.Contains(output, "items list tenant-a") {
			t.Fatalf("tenant B read tenant A's learning: output=%s error=%v", output, err)
		}
		t.Setenv(mcpBoundProfileEnv, a.ProfileName)
	}
	t.Setenv(mcpBoundProfileEnv, b.ProfileName)
	if output, err := runTSECCommand(t, b, "learnings", "list", "--warnings", "--json"); err != nil || strings.Contains(output, "legacy-secret") || strings.Contains(output, "other-tenant-id") {
		t.Fatalf("profile read legacy warnings: output=%s error=%v", output, err)
	}
	if _, err := runTSECCommand(t, b, "learnings", "list", "--db", a.Paths.DataFile); err == nil || !strings.Contains(err.Error(), "tenant artifact conflict") {
		t.Fatalf("cross-profile database override error=%v", err)
	}
	t.Setenv(mcpBoundProfileEnv, a.ProfileName)
	if _, err := runTSECCommand(t, a, "teach", "--query", "invalid learning"); err == nil {
		t.Fatal("invalid teach succeeded")
	}
	for _, filename := range []string{a.Paths.DataFile, b.Paths.DataFile, filepath.Join(a.Paths.StateDir, "learnings.jsonl"), filepath.Join(a.Paths.StateDir, "teach.log")} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("missing profile learning artifact %s: %v", filename, err)
		}
	}
	for _, session := range []*platform.Session{a, b} {
		segments, err := filepath.Glob(filepath.Join(session.Paths.StateDir, "learn", "journal-*.jsonl"))
		if err != nil || len(segments) != 1 {
			t.Fatalf("profile journal segments=%v error=%v", segments, err)
		}
	}
	if data, err := os.ReadFile(legacyDB); err != nil || string(data) != "unverified legacy database" {
		t.Fatalf("learning touched legacy database: %q %v", data, err)
	}
	defaultState, err := cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(defaultState, "learn")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("learning touched default journal: %v", err)
	}
	if dir, err := learn.StateDir(); err != nil || dir != defaultState {
		t.Fatalf("learning state binding was not restored: %s %v", dir, err)
	}
}

func TestTSECLearningStoreRejectsWrongTenantMetadata(t *testing.T) {
	withTempLearnHome(t)
	a, b := learningProfileTestSession(t, "tenant-a"), learningProfileTestSession(t, "tenant-b")
	s, err := store.OpenVerifiedWithContext(context.Background(), a.Paths.DataFile, a)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	b.Paths.DataFile = a.Paths.DataFile
	_, err = openLearningStore(platform.ContextWithSession(context.Background(), b), "")
	if !errors.Is(err, platform.ErrTenantMetadataMismatch) {
		t.Fatalf("wrong database tenant metadata error=%v", err)
	}
	if _, err := openLearningStore(context.Background(), a.Paths.DataFile); err == nil {
		t.Fatal("unverified learning store opened")
	}
}
