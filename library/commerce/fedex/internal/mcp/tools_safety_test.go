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
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
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

func TestAllToolsExposeTypedWorkflowSchemas(t *testing.T) {
	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	wants := map[string][]string{
		"get_rates":           {"accountNumber", "requestedShipment"},
		"validate_address":    {"addressesToValidate"},
		"validate_shipment":   {"accountNumber", "requestedShipment"},
		"pickup_availability": {"pickupAddress", "pickupRequestType", "carriers", "countryRelationship"},
		"create_label":        {"accountNumber", "labelResponseOptions", "requestedShipment"},
		"cancel_shipment":     {"accountNumber", "trackingNumber", "deletionControl"},
		"schedule_pickup":     {"associatedAccountNumber", "originDetail", "totalWeight"},
		"cancel_pickup":       {"pickupConfirmationCode", "carrierCode", "scheduledDate"},
	}
	requiredWants := map[string][]string{
		"get_rates":           {"accountNumber", "requestedShipment"},
		"validate_address":    {"addressesToValidate"},
		"validate_shipment":   {"accountNumber", "requestedShipment"},
		"pickup_availability": {"pickupAddress", "pickupRequestType", "carriers", "countryRelationship"},
		"create_label":        {"labelResponseOptions", "accountNumber", "requestedShipment"},
		"cancel_shipment":     {"accountNumber", "senderCountryCode", "trackingNumber", "deletionControl"},
		"schedule_pickup":     {"associatedAccountNumber", "originDetail", "totalWeight", "packageCount", "carrierCode"},
		"cancel_pickup":       {"pickupConfirmationCode"},
	}
	for toolName, fields := range wants {
		registered := s.ListTools()[toolName]
		requestSchema, ok := registered.Tool.InputSchema.Properties["request"].(map[string]any)
		if !ok {
			t.Fatalf("tool %s request schema=%#v", toolName, registered.Tool.InputSchema.Properties["request"])
		}
		properties, ok := requestSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("tool %s request properties=%#v", toolName, requestSchema["properties"])
		}
		for _, field := range fields {
			if _, ok := properties[field]; !ok {
				t.Errorf("tool %s request schema missing %s", toolName, field)
			}
		}
		required, ok := requestSchema["required"].([]string)
		if !ok || strings.Join(required, ",") != strings.Join(requiredWants[toolName], ",") {
			t.Errorf("tool %s request required=%#v, want %v", toolName, requestSchema["required"], requiredWants[toolName])
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
			Name       string `json:"name"`
			Parameters struct {
				Properties map[string]any `json:"properties"`
			} `json:"parameters"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	names := make([]string, 0, len(manifest.Tools))
	for _, tool := range manifest.Tools {
		names = append(names, tool.Name)
		manifestRequest, ok := tool.Parameters.Properties["request"].(map[string]any)
		if !ok {
			t.Fatalf("tools-manifest tool %s request schema=%#v", tool.Name, tool.Parameters.Properties["request"])
		}
		runtimeRequest, ok := s.ListTools()[tool.Name].Tool.InputSchema.Properties["request"].(map[string]any)
		if !ok {
			t.Fatalf("runtime tool %s request schema=%#v", tool.Name, s.ListTools()[tool.Name].Tool.InputSchema.Properties["request"])
		}
		runtimeJSON, err := json.Marshal(runtimeRequest)
		if err != nil {
			t.Fatal(err)
		}
		var normalizedRuntime map[string]any
		if err := json.Unmarshal(runtimeJSON, &normalizedRuntime); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(manifestRequest, normalizedRuntime) {
			t.Errorf("tools-manifest tool %s request schema differs from runtime\nmanifest: %#v\nruntime: %#v", tool.Name, manifestRequest, normalizedRuntime)
		}
		if tool.Name == "schedule_pickup" {
			manifestAvailability, ok := tool.Parameters.Properties["availability_request"].(map[string]any)
			if !ok {
				t.Fatalf("tools-manifest schedule_pickup availability_request schema=%#v", tool.Parameters.Properties["availability_request"])
			}
			runtimeAvailability, ok := s.ListTools()[tool.Name].Tool.InputSchema.Properties["availability_request"].(map[string]any)
			if !ok {
				t.Fatalf("runtime schedule_pickup availability_request schema=%#v", s.ListTools()[tool.Name].Tool.InputSchema.Properties["availability_request"])
			}
			runtimeJSON, err := json.Marshal(runtimeAvailability)
			if err != nil {
				t.Fatal(err)
			}
			var normalizedRuntime map[string]any
			if err := json.Unmarshal(runtimeJSON, &normalizedRuntime); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(manifestAvailability, normalizedRuntime) {
				t.Errorf("tools-manifest schedule_pickup availability_request schema differs from runtime\nmanifest: %#v\nruntime: %#v", manifestAvailability, normalizedRuntime)
			}
		}
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

func TestReadToolSchemasExposeOperationSpecificNestedRequirements(t *testing.T) {
	rateProperties := requestSchemaProperties("get_rates")
	rateShipment := rateProperties["requestedShipment"].(map[string]any)
	if slices.Contains(rateShipment["required"].([]string), "rateRequestType") {
		t.Fatalf("rate schema incorrectly requires optional rateRequestType: %#v", rateShipment)
	}
	ratePackages := rateShipment["properties"].(map[string]any)["requestedPackageLineItems"].(map[string]any)
	ratePackage := ratePackages["items"].(map[string]any)
	groupCount := ratePackage["properties"].(map[string]any)["groupPackageCount"].(map[string]any)
	if groupCount["minimum"] != 1 || groupCount["enum"] != nil {
		t.Fatalf("rate schema does not allow positive grouped package counts: %#v", groupCount)
	}
	controls := rateProperties["rateRequestControlParameters"].(map[string]any)
	if _, ok := controls["properties"].(map[string]any)["returnTransitTimes"]; !ok {
		t.Fatalf("rate control schema remains opaque: %#v", controls)
	}

	addressProperties := requestSchemaProperties("validate_address")
	addresses := addressProperties["addressesToValidate"].(map[string]any)
	addressEntry := addresses["items"].(map[string]any)
	address := addressEntry["properties"].(map[string]any)["address"].(map[string]any)
	addressRequired := address["required"].([]string)
	if !slices.Contains(addressRequired, "streetLines") || !slices.Contains(addressRequired, "countryCode") {
		t.Fatalf("address validation common required fields=%v", addressRequired)
	}
	if slices.Contains(addressRequired, "city") || slices.Contains(addressRequired, "postalCode") {
		t.Fatalf("address validation incorrectly requires country-specific fields globally: %v", addressRequired)
	}
	addressConditionals, ok := address["allOf"].([]any)
	if !ok || len(addressConditionals) != 1 {
		t.Fatalf("address validation schema lacks country-conditional requirements: %#v", address)
	}
	addressThen := addressConditionals[0].(map[string]any)["then"].(map[string]any)
	addressAlternatives := addressThen["anyOf"].([]any)
	postalAlternative := addressAlternatives[0].(map[string]any)
	postalMinimum := postalAlternative["properties"].(map[string]any)["postalCode"].(map[string]any)["minLength"]
	postalPattern := postalAlternative["properties"].(map[string]any)["postalCode"].(map[string]any)["pattern"]
	if postalMinimum != 1 || postalPattern != "\\S" {
		t.Fatalf("US address conditional does not require a nonempty postal code: %#v", addressThen)
	}
	addressControls := addressProperties["validateAddressControlParameters"].(map[string]any)
	if _, ok := addressControls["properties"].(map[string]any)["includeResolutionTokens"]; !ok {
		t.Fatalf("address control schema remains opaque: %#v", addressControls)
	}

	shipmentProperties := requestSchemaProperties("validate_shipment")
	shipment := shipmentProperties["requestedShipment"].(map[string]any)
	shipmentRequired := shipment["required"].([]string)
	for _, field := range []string{"pickupType", "totalWeight", "shippingChargesPayment", "labelSpecification", "shipper", "recipients", "requestedPackageLineItems"} {
		if !slices.Contains(shipmentRequired, field) {
			t.Errorf("shipment validation schema does not require %s: %v", field, shipmentRequired)
		}
	}
	payment := shipment["properties"].(map[string]any)["shippingChargesPayment"].(map[string]any)
	if _, ok := payment["allOf"].([]any); !ok {
		t.Fatalf("shipment payment schema lacks conditional payor requirement: %#v", payment)
	}
	shipper := shipment["properties"].(map[string]any)["shipper"].(map[string]any)
	shipperAddress := shipper["properties"].(map[string]any)["address"].(map[string]any)
	shipmentAddressConditionals := shipperAddress["allOf"].([]any)
	shipmentAddressThen := shipmentAddressConditionals[0].(map[string]any)["then"].(map[string]any)
	conditionalAddressProperties := shipmentAddressThen["properties"].(map[string]any)
	for _, field := range []string{"stateOrProvinceCode", "postalCode"} {
		fieldSchema := conditionalAddressProperties[field].(map[string]any)
		if fieldSchema["minLength"] != 1 || fieldSchema["pattern"] != "\\S" {
			t.Errorf("US/CA/PR shipment address conditional does not require nonempty %s: %#v", field, shipmentAddressThen)
		}
	}

	pickupProperties := requestSchemaProperties("pickup_availability")
	pickupAddress := pickupProperties["pickupAddress"].(map[string]any)
	pickupAddressProperties := pickupAddress["properties"].(map[string]any)
	for _, field := range []string{"streetLines", "urbanizationCode", "city", "stateOrProvinceCode", "postalCode", "countryCode", "residential", "addressClassification"} {
		if _, ok := pickupAddressProperties[field]; !ok {
			t.Errorf("pickup address schema does not model %s", field)
		}
	}
	attributes := pickupProperties["shipmentAttributes"].(map[string]any)
	if !slices.Contains(attributes["required"].([]string), "serviceType") {
		t.Fatalf("pickup shipmentAttributes does not require serviceType when provided: %#v", attributes)
	}
	if _, ok := attributes["allOf"].([]any); !ok {
		t.Fatalf("pickup shipmentAttributes lacks YOUR_PACKAGING dimensions requirement: %#v", attributes)
	}
	dimensions := attributes["properties"].(map[string]any)["dimensions"].(map[string]any)
	dimensionRequired := dimensions["required"].([]string)
	for _, field := range []string{"length", "width", "height"} {
		if !slices.Contains(dimensionRequired, field) {
			t.Errorf("pickup dimensions do not require %s: %v", field, dimensionRequired)
		}
	}
	if slices.Contains(dimensionRequired, "units") {
		t.Fatalf("pickup dimensions incorrectly require units despite FedEx inch default: %v", dimensionRequired)
	}
	units := dimensions["properties"].(map[string]any)["units"].(map[string]any)
	unitEnum := units["enum"].([]any)
	if !slices.Contains(unitEnum, any("")) || !slices.Contains(unitEnum, any(nil)) {
		t.Fatalf("pickup dimension units do not expose blank/null inch defaults: %#v", units)
	}
	packageDetails := pickupProperties["packageDetails"].(map[string]any)
	packageItem := packageDetails["items"].(map[string]any)
	if !slices.Contains(packageItem["required"].([]string), "packageSpecialServices") {
		t.Fatalf("pickup packageDetails remains opaque: %#v", packageItem)
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
		_, _ = w.Write([]byte(`{"transactionId":"tx-create","output":{"transactionShipments":[{"masterTrackingNumber":"123456789012","serviceType":"FEDEX_GROUND","pieceResponses":[{"trackingNumber":"123456789012","packageDocuments":[{"contentType":"application/pdf","docType":"LABEL","encodedLabel":"JVBERi0xLjQKJSVFT0YK"}]}]}]}}`))
	}))
	t.Cleanup(api.Close)

	dataDir := filepath.Join(t.TempDir(), "fedex")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", dataDir)
	setMCPTestAuth(t, api.URL)

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool, ok := s.ListTools()["create_label"]
	if !ok {
		t.Fatal("create_label tool is not registered")
	}
	request := validMCPCreateLabelRequest()

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
	if strings.Contains(toolResultText(t, confirmed), "encodedLabel") {
		t.Fatal("confirmed result exposed encoded label data")
	}
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	shipment, err := ledger.GetShipmentByTracking(context.Background(), "123456789012")
	_ = ledger.Close()
	if err != nil || shipment == nil || shipment.TransactionID != "tx-create" {
		t.Fatalf("shipment=%+v err=%v", shipment, err)
	}
	if info, err := os.Stat(shipment.LabelPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("label info=%v err=%v", info, err)
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
		_, _ = w.Write([]byte(`{"output":{"rateReplyDetails":[{"serviceType":"FEDEX_GROUND","ratedShipmentDetails":[{"rateType":"PAYOR_ACCOUNT_PACKAGE","totalNetCharge":12.34,"currency":"USD"}]}]},"messages":[{"code":"UNRELATED","message":"must not escape"}],"requestEcho":{"accountNumber":"123456789"}}`))
	}))
	t.Cleanup(api.Close)

	t.Setenv("HOME", t.TempDir())
	setMCPTestAuth(t, api.URL)

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["get_rates"]
	result, err := tool.Handler(context.Background(), toolRequest(map[string]any{
		"request": map[string]any{
			"accountNumber": map[string]any{"value": "123456789"},
			"requestedShipment": map[string]any{
				"shipper":                   map[string]any{"address": map[string]any{"postalCode": "90210", "countryCode": "US"}},
				"recipient":                 map[string]any{"address": map[string]any{"postalCode": "10001", "countryCode": "US"}},
				"pickupType":                "DROPOFF_AT_FEDEX_LOCATION",
				"rateRequestType":           []any{"ACCOUNT"},
				"requestedPackageLineItems": []any{map[string]any{"weight": map[string]any{"units": "LB", "value": 1.0}}},
			},
		},
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
	text := toolResultText(t, result)
	if !strings.Contains(text, `"service_type":"FEDEX_GROUND"`) || !strings.Contains(text, `"total_net_charge":12.34`) {
		t.Fatalf("get_rates result omitted minimized rate fields: %s", text)
	}
	for _, forbidden := range []string{"must not escape", "requestEcho", "123456789"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("get_rates result exposed %q: %s", forbidden, text)
		}
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
	setMCPTestAuth(t, api.URL)

	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	tool := s.ListTools()["schedule_pickup"]
	request := validMCPSchedulePickupRequest()
	preview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request, "availability_override_reason": "synthetic test override"}))
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
		"request":                      request,
		"availability_override_reason": "synthetic test override",
		"confirm":                      true,
		"operation_id":                 pending.OperationID,
		"confirmation_digest":          pending.ConfirmationDigest,
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
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	pickup, err := ledger.GetPickupByOperationID(context.Background(), pending.OperationID)
	_ = ledger.Close()
	if err != nil || pickup == nil || pickup.Status != "outcome_unknown" {
		t.Fatalf("pickup=%+v err=%v", pickup, err)
	}
	retryPreview, err := tool.Handler(context.Background(), toolRequest(map[string]any{"request": request, "availability_override_reason": "synthetic test override"}))
	if err != nil || retryPreview == nil || !retryPreview.IsError {
		t.Fatalf("equivalent retry preview=%#v err=%v, want blocked", retryPreview, err)
	}
	retryText := toolResultText(t, retryPreview)
	if !strings.Contains(retryText, pending.OperationID) || !strings.Contains(retryText, "outcome_unknown") {
		t.Fatalf("blocked retry omitted reconciliation identity: %s", retryText)
	}
	if calls != 1 {
		t.Fatalf("blocked retry emitted %d requests, want 1 total", calls)
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
