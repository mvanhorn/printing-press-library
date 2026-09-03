// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var expectedNarrowTools = []string{
	"cancel_pickup",
	"cancel_shipment",
	"create_label",
	"get_rates",
	"pickup_availability",
	"schedule_pickup",
	"validate_address",
	"validate_shipment",
}

func TestRegisterToolsExposesExactNarrowSurface(t *testing.T) {
	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	registered := s.ListTools()

	got := make([]string, 0, len(registered))
	for name := range registered {
		got = append(got, name)
	}
	sort.Strings(got)
	if len(got) != len(expectedNarrowTools) {
		t.Fatalf("registered tools=%v, want exactly %v", got, expectedNarrowTools)
	}
	for i := range got {
		if got[i] != expectedNarrowTools[i] {
			t.Fatalf("registered tools=%v, want exactly %v", got, expectedNarrowTools)
		}
	}

	readOnly := map[string]bool{
		"get_rates":           true,
		"pickup_availability": true,
		"validate_address":    true,
		"validate_shipment":   true,
	}
	destructive := map[string]bool{"cancel_shipment": true, "cancel_pickup": true}
	idempotent := map[string]bool{"get_rates": true, "validate_shipment": true, "pickup_availability": true}
	for name, registeredTool := range registered {
		annotations := registeredTool.Tool.Annotations
		if annotations.ReadOnlyHint == nil || annotations.DestructiveHint == nil || annotations.IdempotentHint == nil || annotations.OpenWorldHint == nil {
			t.Fatalf("tool %q must declare all safety annotations: %#v", name, annotations)
		}
		wantReadOnly := readOnly[name]
		if *annotations.ReadOnlyHint != wantReadOnly {
			t.Errorf("tool %q readOnlyHint=%v, want %v", name, *annotations.ReadOnlyHint, wantReadOnly)
		}
		if *annotations.DestructiveHint != destructive[name] {
			t.Errorf("tool %q destructiveHint=%v, want %v", name, *annotations.DestructiveHint, destructive[name])
		}
		if *annotations.IdempotentHint != idempotent[name] {
			t.Errorf("tool %q idempotentHint=%v, want %v", name, *annotations.IdempotentHint, idempotent[name])
		}
		if !*annotations.OpenWorldHint {
			t.Errorf("tool %q openWorldHint=false, want true", name)
		}
	}
}

func TestMCPMetadataMatchesRuntimeToolSurface(t *testing.T) {
	manifestData, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(manifest.Tools))
	for _, tool := range manifest.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(expectedNarrowTools, ",") {
		t.Fatalf("tools-manifest names=%v, want %v", names, expectedNarrowTools)
	}

	metadataData, err := os.ReadFile("../../.printing-press.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		MCPToolCount int `json:"mcp_tool_count"`
	}
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.MCPToolCount != len(expectedNarrowTools) {
		t.Fatalf("mcp_tool_count=%d, want %d", metadata.MCPToolCount, len(expectedNarrowTools))
	}
}

func TestCreateLabelRequiresPreviewAndBoundConfirmation(t *testing.T) {
	calls := 0
	var gotBody []byte
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"transactionShipments":[{"masterTrackingNumber":"synthetic"}]}}`))
	}))
	t.Cleanup(api.Close)

	dataDir := filepath.Join(t.TempDir(), "fedex")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", dataDir)
	t.Setenv("FEDEX_BASE_URL", api.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-test-token")

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool, ok := s.ListTools()["create_label"]
	if !ok {
		t.Fatal("create_label tool is not registered")
	}
	request := map[string]any{
		"accountNumber": map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"serviceType": "FEDEX_GROUND",
		},
	}

	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request}))
	if err != nil {
		t.Fatalf("preview handler error: %v", err)
	}
	if preview == nil || preview.IsError {
		t.Fatalf("preview result=%#v, want success", preview)
	}
	if calls != 0 {
		t.Fatalf("preview emitted %d FedEx requests, want 0", calls)
	}
	var previewEnvelope struct {
		Status             string `json:"status"`
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, preview)), &previewEnvelope); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if previewEnvelope.Status != "pending_confirmation" || previewEnvelope.OperationID == "" || previewEnvelope.ConfirmationDigest == "" {
		t.Fatalf("invalid preview envelope: %#v", previewEnvelope)
	}

	confirmed, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request":             request,
		"confirm":             true,
		"operation_id":        previewEnvelope.OperationID,
		"confirmation_digest": previewEnvelope.ConfirmationDigest,
	}))
	if err != nil {
		t.Fatalf("confirmed handler error: %v", err)
	}
	if confirmed == nil || confirmed.IsError {
		t.Fatalf("confirmed result=%#v, want success", confirmed)
	}
	if calls != 1 {
		t.Fatalf("confirmed mutation emitted %d requests, want 1", calls)
	}
	if len(gotBody) == 0 || gotBody[0] != '{' {
		t.Fatalf("confirmed mutation body=%q, want JSON object", gotBody)
	}

	replayed, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request":             request,
		"confirm":             true,
		"operation_id":        previewEnvelope.OperationID,
		"confirmation_digest": previewEnvelope.ConfirmationDigest,
	}))
	if err != nil {
		t.Fatalf("replay handler error: %v", err)
	}
	if replayed == nil || !replayed.IsError {
		t.Fatalf("replay result=%#v, want error", replayed)
	}
	if calls != 1 {
		t.Fatalf("replay emitted %d total requests, want 1", calls)
	}
}

func TestReadOnlyRateToolCallsFedExWithoutMutationConfirmation(t *testing.T) {
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/rate/v1/rates/quotes" {
			t.Errorf("path=%q, want rate endpoint", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"rateReplyDetails":[]}}`))
	}))
	t.Cleanup(api.Close)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_BASE_URL", api.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-test-token")

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["get_rates"]
	result, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request": map[string]any{"accountNumber": map[string]any{"value": "123456789"}},
	}))
	if err != nil {
		t.Fatalf("get_rates handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("get_rates result=%#v, want success", result)
	}
	if calls != 1 {
		t.Fatalf("get_rates emitted %d requests, want 1", calls)
	}
}

func TestConfirmedMutation500IsOutcomeUnknownAndNotRetried(t *testing.T) {
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "synthetic server failure", http.StatusInternalServerError)
	}))
	t.Cleanup(api.Close)

	dataDir := filepath.Join(t.TempDir(), "fedex")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", dataDir)
	t.Setenv("FEDEX_BASE_URL", api.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-test-token")

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["schedule_pickup"]
	request := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             "FDXG",
		"packageCount":            1,
	}
	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request}))
	if err != nil || preview == nil || preview.IsError {
		t.Fatalf("preview result=%#v err=%v", preview, err)
	}
	var pending struct {
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, preview)), &pending); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	result, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request":             request,
		"confirm":             true,
		"operation_id":        pending.OperationID,
		"confirmation_digest": pending.ConfirmationDigest,
	}))
	if err != nil {
		t.Fatalf("confirmed handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("confirmed result=%#v, want outcome-unknown error", result)
	}
	var failure struct {
		ErrorClass      string `json:"error_class"`
		OperationID     string `json:"operation_id"`
		OperationStatus string `json:"operation_status"`
	}
	if err := json.Unmarshal([]byte(toolResultText(t, result)), &failure); err != nil {
		t.Fatalf("decode outcome-unknown result: %v", err)
	}
	if failure.ErrorClass != "outcome_unknown" {
		t.Fatalf("error_class=%q, want outcome_unknown", failure.ErrorClass)
	}
	if failure.OperationID != pending.OperationID || failure.OperationStatus != "outcome_unknown" {
		t.Fatalf("outcome-unknown identity/status not retained: %#v", failure)
	}
	if calls != 1 {
		t.Fatalf("confirmed mutation emitted %d requests, want 1", calls)
	}

	recordData, err := os.ReadFile(filepath.Join(dataDir, "pending", pending.OperationID+".json"))
	if err != nil {
		t.Fatalf("read pending record: %v", err)
	}
	var record struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(recordData, &record); err != nil {
		t.Fatalf("decode pending record: %v", err)
	}
	if record.Status != "outcome_unknown" {
		t.Fatalf("pending record status=%q, want outcome_unknown", record.Status)
	}
}

func TestMutationResultOmitsEncodedLabelsAndPII(t *testing.T) {
	encoded := strings.Repeat("synthetic-label-bytes", 50)
	shortEncoded := "short-label-payload"
	street := "SENTINEL RECIPIENT STREET"
	redacted := summarizeMutationResponse("create_label", map[string]any{
		"output": map[string]any{
			"encodedLabel":   encoded,
			"label":          shortEncoded,
			"streetLines":    street,
			"trackingNumber": "synthetic-tracking",
			"packageDocuments": []any{map[string]any{
				"url":         "https://labels.invalid/sentinel-label-url",
				"companyName": "SENTINEL COMPANY",
				"city":        "SENTINEL CITY",
				"postalCode":  "00000",
			}},
		},
	})
	serialized, err := json.Marshal(redacted)
	if err != nil {
		t.Fatalf("marshal redacted result: %v", err)
	}
	for _, secret := range []string{encoded, shortEncoded, street, "sentinel-label-url", "SENTINEL COMPANY", "SENTINEL CITY", "00000"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("sensitive value %q escaped redaction", secret)
		}
	}

	if !strings.Contains(string(serialized), "synthetic-tracking") {
		t.Fatalf("non-label metadata was lost: %s", serialized)
	}
}

func toolRequest(arguments map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: arguments}}
}

func toolResultText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("unexpected tool result content: %#v", result)
	}
	content, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("unexpected tool result content type: %T", result.Content[0])
	}
	return content.Text
}
