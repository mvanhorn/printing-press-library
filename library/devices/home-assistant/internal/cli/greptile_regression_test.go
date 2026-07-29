// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/home-assistant/internal/store"
)

func writeTestConfig(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("base_url = \""+baseURL+"\"\nbase_path = \"\"\ntoken = \"test-token\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestModeRunRejectsUnverifiableServiceResponse(t *testing.T) {
	withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/states":
			_, _ = w.Write([]byte(`[{"entity_id":"scene.movie_night","state":"off","attributes":{"friendly_name":"Movie Night"}}]`))
		case "/api/services/scene/turn_on":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, stderr, err := runRootArgs(t, "--config", writeTestConfig(t, server.URL), "--no-learn", "mode", "run", "scene.movie_night")
	if err == nil || !strings.Contains(err.Error(), "no changed states to verify") {
		t.Fatalf("mode run error = %v, want failed verification (stderr=%q)", err, stderr)
	}
}

func TestModeRunReportsVerifiedOnlyAfterStateRefresh(t *testing.T) {
	withTempLearnHome(t)
	stateReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/states":
			stateReads++
			if stateReads == 1 {
				_, _ = w.Write([]byte(`[{"entity_id":"scene.movie_night","state":"off","attributes":{"friendly_name":"Movie Night"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"entity_id":"light.living_room","state":"on","attributes":{}}]`))
		case "/api/services/scene/turn_on":
			_, _ = w.Write([]byte(`[{"entity_id":"light.living_room","state":"on","attributes":{}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	stdout, stderr, err := runRootArgs(t, "--config", writeTestConfig(t, server.URL), "--no-learn", "--json", "mode", "run", "scene.movie_night")
	if err != nil {
		t.Fatalf("mode run: %v (stderr=%q)", err, stderr)
	}
	var output struct {
		Verified       bool `json:"verified"`
		VerifiedStates []struct {
			EntityID string `json:"entity_id"`
			State    string `json:"state"`
		} `json:"verified_states"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode mode output: %v (%s)", err, stdout)
	}
	if !output.Verified || len(output.VerifiedStates) != 1 || output.VerifiedStates[0].EntityID != "light.living_room" || output.VerifiedStates[0].State != "on" {
		t.Fatalf("mode output did not include verified refreshed state: %s", stdout)
	}
}

func TestImportFailsOnPartialFailureResponse(t *testing.T) {
	withTempLearnHome(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/template" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"partialFailureError":{"code":3,"message":"one record rejected"}}`))
	}))
	t.Cleanup(server.Close)
	input := filepath.Join(t.TempDir(), "records.jsonl")
	if err := os.WriteFile(input, []byte(`{"template":"{{ 1 }}"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	_, stderr, err := runRootArgs(t, "--config", writeTestConfig(t, server.URL), "--no-learn", "import", "template", "--input", input)
	if err == nil || !strings.Contains(err.Error(), "1 failed record") {
		t.Fatalf("import error = %v, want partial-failure error (stderr=%q)", err, stderr)
	}
}

func TestImportDoesNotAdvertiseUnimplementedBatchSize(t *testing.T) {
	cmd := newImportCmd(&rootFlags{})
	if cmd.Flags().Lookup("batch-size") != nil {
		t.Fatal("import still exposes --batch-size without batch API support")
	}
}

func TestStatesDeleteRequiresYesBeforeClientCreation(t *testing.T) {
	cmd := newStatesDeleteCmd(&rootFlags{})
	cmd.SetArgs([]string{"sensor.temperature"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("states delete error = %v, want confirmation requirement", err)
	}
}

func TestReconcileDeletedStateRemovesSyncedRow(t *testing.T) {
	withTempLearnHome(t)
	dbPath := defaultDBPath("home-assistant-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	if err := db.Upsert("states", "sensor.temperature", []byte(`{"entity_id":"sensor.temperature","state":"20"}`)); err != nil {
		t.Fatalf("seed cached state: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seeded cache: %v", err)
	}
	if err := reconcileDeletedState(context.Background(), "sensor.temperature"); err != nil {
		t.Fatalf("reconcile deleted state: %v", err)
	}
	db, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen cache: %v", err)
	}
	defer db.Close()
	if _, err := db.Get("states", "sensor.temperature"); err != sql.ErrNoRows {
		t.Fatalf("cached deleted state lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestReconcileDeletedStateSurfacesCacheFailure(t *testing.T) {
	withTempLearnHome(t)
	dbPath := defaultDBPath("home-assistant-pp-cli")
	if err := os.MkdirAll(dbPath, 0o700); err != nil {
		t.Fatalf("create invalid cache path: %v", err)
	}
	if err := reconcileDeletedState(context.Background(), "sensor.temperature"); err == nil {
		t.Fatal("cache reconciliation succeeded with a directory in place of the database")
	}
}

func TestMutationPersistenceFailureIsReturned(t *testing.T) {
	withTempLearnHome(t)
	dbPath := defaultDBPath("home-assistant-pp-cli")
	if err := os.MkdirAll(dbPath, 0o700); err != nil {
		t.Fatalf("create invalid cache path: %v", err)
	}
	err := writeMutationResponseToStore(context.Background(), "states", []byte(`{"entity_id":"sensor.temperature","state":"21"}`), "")
	if err == nil || !strings.Contains(err.Error(), "open local store") {
		t.Fatalf("mutation persistence error = %v, want surfaced local-store failure", err)
	}
}

func TestMutationPersistenceStoresHomeAssistantStateByEntityID(t *testing.T) {
	withTempLearnHome(t)
	if err := writeMutationResponseToStore(context.Background(), "states", []byte(`{"entity_id":"sensor.temperature","state":"21"}`), ""); err != nil {
		t.Fatalf("persist mutation response: %v", err)
	}
	db, err := store.OpenWithContext(context.Background(), defaultDBPath("home-assistant-pp-cli"))
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer db.Close()
	if _, err := db.Get("states", "sensor.temperature"); err != nil {
		t.Fatalf("stored state by entity_id: %v", err)
	}
}
