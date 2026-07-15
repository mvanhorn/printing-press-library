package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"
)

func TestSelectedPrinterSerialRejectsInvalidDefaultAndProfileValues(t *testing.T) {
	t.Setenv("BAMBU_SERIAL", "bad serial")
	if _, err := selectedPrinterSerial(&rootFlags{}); err == nil || !strings.Contains(err.Error(), "12-20") {
		t.Fatalf("default serial error = %v", err)
	}

	configurePrinterProfileTestHome(t)
	t.Setenv("SHOP_SERIAL", "bad serial")
	profileStore := &bambuPrinterProfileStore{Profiles: map[string]bambuPrinterProfile{
		"shop": {Name: "shop", SerialEnv: "SHOP_SERIAL", AccessCodeEnv: "SHOP_CODE"},
	}}
	if err := saveBambuPrinterProfiles(profileStore); err != nil {
		t.Fatal(err)
	}
	if _, err := selectedPrinterSerial(&rootFlags{printerName: "shop"}); err == nil || !strings.Contains(err.Error(), "12-20") {
		t.Fatalf("profile serial error = %v", err)
	}
}

func TestPrinterProfileLifecycleAndValidation(t *testing.T) {
	configurePrinterProfileTestHome(t)
	flags := &rootFlags{dryRun: true}
	invalidAdd := newBambuProfileAddCmd(flags)
	if err := invalidAdd.Flags().Set("name", "shop"); err != nil {
		t.Fatal(err)
	}
	if err := invalidAdd.RunE(invalidAdd, nil); err == nil {
		t.Fatal("dry-run accepted missing environment variable names")
	}
	invalidHost := newBambuProfileAddCmd(flags)
	for flag, value := range map[string]string{"name": "shop", "host": "8.8.8.8", "serial-env": "SHOP_SERIAL", "access-code-env": "SHOP_CODE"} {
		_ = invalidHost.Flags().Set(flag, value)
	}
	if err := invalidHost.RunE(invalidHost, nil); err == nil {
		t.Fatal("dry-run accepted a public printer host")
	}

	add := newBambuProfileAddCmd(&rootFlags{})
	for flag, value := range map[string]string{"name": "shop", "host": "192.168.1.25", "serial-env": "SHOP_SERIAL", "access-code-env": "SHOP_CODE"} {
		if err := add.Flags().Set(flag, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := add.RunE(add, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadBambuPrinterProfiles()
	if err != nil || loaded.Profiles["shop"].Host != "192.168.1.25" {
		t.Fatalf("saved profile = %#v err=%v", loaded, err)
	}
	list := newBambuProfileListCmd(&rootFlags{asJSON: true})
	var listed bytes.Buffer
	list.SetOut(&listed)
	if err := list.RunE(list, nil); err != nil || !strings.Contains(listed.String(), `"name": "shop"`) {
		t.Fatalf("profile list = %s err=%v", listed.String(), err)
	}
	dryDelete := newBambuProfileDeleteCmd(&rootFlags{dryRun: true})
	_ = dryDelete.Flags().Set("name", "shop")
	if err := dryDelete.RunE(dryDelete, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadBambuPrinterProfiles()
	if err != nil || loaded.Profiles["shop"].Name != "shop" {
		t.Fatal("dry-run deleted the stored profile")
	}

	missingDelete := newBambuProfileDeleteCmd(&rootFlags{dryRun: true})
	_ = missingDelete.Flags().Set("name", "missing")
	if err := missingDelete.RunE(missingDelete, nil); err == nil {
		t.Fatal("dry-run claimed nonexistent profile deletion")
	}
	deleteCmd := newBambuProfileDeleteCmd(&rootFlags{})
	_ = deleteCmd.Flags().Set("name", "shop")
	if err := deleteCmd.RunE(deleteCmd, nil); err != nil {
		t.Fatal(err)
	}
}

func TestLocalExportNeverContactsConfiguredHTTPOrigin(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BAMBU_DATA_DIR", dataDir)
	t.Setenv("BAMBU_SERIAL", "PRINTERSERIAL001")
	t.Setenv("BAMBU_ACCESS_CODE", "LOCALCODE123")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	t.Setenv("BAMBU_BASE_URL", server.URL)

	s, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := mirroredObservationData("status", time.Now().UTC(), bambu.PrinterKey("PRINTERSERIAL001"), map[string]any{"state": "RUNNING"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertObservations(data); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := newExportCmd(&rootFlags{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.RunE(cmd, []string{"observations"}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || !strings.Contains(output.String(), `"state":"RUNNING"`) {
		t.Fatalf("requests=%d output=%s", requests, output.String())
	}
}

func TestGenericHTTPClientStripsBambuCredentials(t *testing.T) {
	t.Setenv("BAMBU_SERIAL", "PRINTERSERIAL001")
	t.Setenv("BAMBU_ACCESS_CODE", "LOCALCODE123")
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		captured = request.Header.Clone()
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	t.Setenv("BAMBU_BASE_URL", server.URL)
	c, err := (&rootFlags{}).newClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "/probe", nil); err != nil {
		t.Fatal(err)
	}
	headers, _ := json.Marshal(captured)
	if strings.Contains(string(headers), "PRINTERSERIAL001") || strings.Contains(string(headers), "LOCALCODE123") || captured.Get("X-Bambu-Access-Code") != "" {
		t.Fatalf("generic HTTP request exposed Bambu credentials: %s", headers)
	}
}

func TestObservationsCanFilterByPrinterKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	firstKey := bambu.PrinterKey("PRINTERSERIAL001")
	secondKey := bambu.PrinterKey("PRINTERSERIAL002")
	for index, key := range []string{firstKey, secondKey, firstKey} {
		data, err := mirroredObservationData("status", time.Now().UTC().Add(time.Duration(index)*time.Second), key, map[string]any{"index": index})
		if err != nil || s.UpsertObservations(data) != nil {
			t.Fatalf("seed observation %d: %v", index, err)
		}
	}
	items, err := listObservationsForPrinter(context.Background(), s, firstKey, 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("filtered observations = %d err=%v", len(items), err)
	}
}

func TestWorkflowStatusDoesNotCreateMissingStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	cmd := newWorkflowStatusCmd(&rootFlags{asJSON: true})
	var output bytes.Buffer
	cmd.SetOut(&output)
	_ = cmd.Flags().Set("db", dbPath)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("workflow status created store: %v", err)
	}
	var status map[string]int
	if err := json.Unmarshal(output.Bytes(), &status); err != nil || len(status) != 0 {
		t.Fatalf("status = %s err=%v", output.String(), err)
	}
}

func TestWorkflowStatusExplainsHowToPopulateEmptyStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newWorkflowStatusCmd(&rootFlags{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	_ = cmd.Flags().Set("db", dbPath)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Run sync") {
		t.Fatalf("empty-store instruction = %q", output.String())
	}
}

func TestLocalExportRequiresBoundedPositiveLimit(t *testing.T) {
	for _, value := range []string{"0", "-1", "10001"} {
		cmd := newExportCmd(&rootFlags{dryRun: true})
		if err := cmd.Flags().Set("limit", value); err != nil {
			t.Fatal(err)
		}
		if err := cmd.RunE(cmd, []string{"observations"}); err == nil || !strings.Contains(err.Error(), "between 1 and 10000") {
			t.Fatalf("--limit %s error = %v", value, err)
		}
	}
}

func configurePrinterProfileTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("BAMBU_CONFIG_DIR", t.TempDir())
}
