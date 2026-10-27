// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
)

// expectedReadTools and expectedWriteTools pin the exact MCP surface. A new
// tool must be added here deliberately, with the right side of the split.
var expectedReadTools = []string{
	"address_list", "address_validate", "addresses_validate", "archive",
	"availability_services", "availability_special_services", "availability_transit_times",
	"consolidations_confirm_results", "consolidations_results", "consolidations_retrieve",
	"context", "freight_pickups_availability", "globaltrade_regulatory", "locations_find",
	"manifest", "openship_results", "openship_retrieve", "openship_retrieve_package",
	"pickups_availability", "postal_validate", "rate_shop", "rates_quote", "rates_quote_freight",
	"shipments_results", "shipments_validate", "spend_report",
	"track_associated", "track_diff", "track_documents", "track_number", "track_reference", "track_tcn",
}

var expectedWriteTools = []string{
	"consolidations_add_shipment", "consolidations_confirm", "consolidations_create",
	"consolidations_delete", "consolidations_delete_shipment", "consolidations_modify",
	"endofday_close", "endofday_modify",
	"freight_pickups_cancel", "freight_pickups_create", "freight_shipments_create",
	"openship_add_package", "openship_create", "openship_delete", "openship_delete_package",
	"openship_modify", "openship_modify_package",
	"pickups_cancel", "pickups_create",
	"ship_bulk", "shipments_cancel", "shipments_create", "shipments_tag", "shipments_tag_cancel",
	"track_notifications",
}

// Local address-book writes never touch FedEx, so they are neither read-only
// nor FedEx writes.
var expectedLocalWriteTools = []string{"address_delete", "address_save"}

func expectedTools() []string {
	all := append(append(append([]string{}, expectedReadTools...), expectedWriteTools...), expectedLocalWriteTools...)
	sort.Strings(all)
	return all
}

func registeredTools(t *testing.T) map[string]*server.ServerTool {
	t.Helper()
	s := server.NewMCPServer("fedex-test", "test")
	RegisterTools(s)
	return s.ListTools()
}

func TestRegisterToolsExposesExactSurface(t *testing.T) {
	registered := registeredTools(t)
	got := make([]string, 0, len(registered))
	for name := range registered {
		got = append(got, name)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(expectedTools(), ",") {
		t.Fatalf("registered tools=%v\nwant exactly %v", got, expectedTools())
	}
	if len(got) != toolCount() {
		t.Fatalf("toolCount()=%d, registered %d", toolCount(), len(got))
	}
	for _, removed := range []string{"fedex_execute", "fedex_search", "sql", "export", "import", "tail", "track_watch", "ship_etd", "return_create"} {
		if _, ok := registered[removed]; ok {
			t.Errorf("open-ended or removed tool %q is registered", removed)
		}
	}
}

func TestToolAnnotationsMatchWriteSafety(t *testing.T) {
	registered := registeredTools(t)
	isRead := map[string]bool{}
	for _, name := range expectedReadTools {
		isRead[name] = true
	}
	isWrite := map[string]bool{}
	for _, name := range expectedWriteTools {
		isWrite[name] = true
	}
	for name, tool := range registered {
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || a.DestructiveHint == nil || a.IdempotentHint == nil || a.OpenWorldHint == nil {
			t.Fatalf("tool %q must declare all safety annotations: %#v", name, a)
		}
		_, hasAllowDuplicate := tool.Tool.InputSchema.Properties["allow_duplicate"]
		switch {
		case isRead[name]:
			if !*a.ReadOnlyHint || *a.DestructiveHint {
				t.Errorf("read tool %q annotations readOnly=%v destructive=%v", name, *a.ReadOnlyHint, *a.DestructiveHint)
			}
			if hasAllowDuplicate {
				t.Errorf("read tool %q exposes allow_duplicate", name)
			}
		case isWrite[name]:
			if *a.ReadOnlyHint || !*a.DestructiveHint || *a.IdempotentHint || !*a.OpenWorldHint {
				t.Errorf("write tool %q annotations readOnly=%v destructive=%v idempotent=%v openWorld=%v", name, *a.ReadOnlyHint, *a.DestructiveHint, *a.IdempotentHint, *a.OpenWorldHint)
			}
			if !hasAllowDuplicate {
				t.Errorf("write tool %q has no allow_duplicate override", name)
			}
		default:
			if *a.ReadOnlyHint || *a.OpenWorldHint {
				t.Errorf("local tool %q annotations readOnly=%v openWorld=%v", name, *a.ReadOnlyHint, *a.OpenWorldHint)
			}
		}
		for _, banned := range []string{"args", "config", "deliver", "yes", "data-source", "output", "input", "webhook", "close", "endpoint_id"} {
			if _, ok := tool.Tool.InputSchema.Properties[banned]; ok {
				t.Errorf("tool %q exposes open-ended parameter %q", name, banned)
			}
		}
	}
	// Endpoint annotations must agree with the allowlist the client enforces.
	for _, ep := range endpointTools {
		name := toolNameForEndpoint(ep.ID)
		if client.IsReadOnlyOperation(ep.Method, ep.Path) != isRead[name] {
			t.Errorf("endpoint tool %q read-only annotation disagrees with client allowlist", name)
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
			Name        string `json:"name"`
			Annotations struct {
				ReadOnlyHint    bool `json:"readOnlyHint"`
				DestructiveHint bool `json:"destructiveHint"`
			} `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	registered := registeredTools(t)
	names := make([]string, 0, len(manifest.Tools))
	for _, tool := range manifest.Tools {
		names = append(names, tool.Name)
		runtime, ok := registered[tool.Name]
		if !ok {
			continue
		}
		a := runtime.Tool.Annotations
		if tool.Annotations.ReadOnlyHint != *a.ReadOnlyHint || tool.Annotations.DestructiveHint != *a.DestructiveHint {
			t.Errorf("tools-manifest annotations for %q differ from runtime", tool.Name)
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(expectedTools(), ",") {
		t.Fatalf("tools-manifest names=%v\nwant %v", names, expectedTools())
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
	if metadata.MCPToolCount != len(expectedTools()) {
		t.Fatalf("mcp_tool_count=%d, want %d", metadata.MCPToolCount, len(expectedTools()))
	}
}

type fakeAPI struct {
	calls  int
	bodies [][]byte
	status int
}

func newFakeAPI(t *testing.T, status int, response string) *fakeAPI {
	t.Helper()
	fake := &fakeAPI{status: status}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.calls++
		body, _ := io.ReadAll(r.Body)
		fake.bodies = append(fake.bodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fake.status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(api.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	t.Setenv("FEDEX_BASE_URL", api.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-test-token")
	fastWritePacer(t)
	return fake
}

// fastWritePacer shrinks the shared write spacing so tests don't sleep.
func fastWritePacer(t *testing.T) {
	t.Helper()
	writePacer.mu.Lock()
	previous := writePacer.interval
	writePacer.interval = time.Millisecond
	writePacer.next = time.Time{}
	writePacer.mu.Unlock()
	t.Cleanup(func() {
		writePacer.mu.Lock()
		writePacer.interval = previous
		writePacer.next = time.Time{}
		writePacer.mu.Unlock()
	})
}

func callTool(t *testing.T, tools map[string]*server.ServerTool, name string, arguments map[string]any) (*mcplib.CallToolResult, map[string]any) {
	t.Helper()
	tool, ok := tools[name]
	if !ok {
		t.Fatalf("tool %q is not registered", name)
	}
	result, err := tool.Handler(context.Background(), toolRequest(arguments))
	if err != nil {
		t.Fatalf("%s handler error: %v", name, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(toolResultText(t, result)), &envelope); err != nil {
		t.Fatalf("decode %s result: %v", name, err)
	}
	return result, envelope
}

func TestWriteToolRunsInOneCallAndRefusesDuplicate(t *testing.T) {
	label := "JVBERi0xLjcKJSVFT0YK"
	fake := newFakeAPI(t, http.StatusOK, `{"output":{"transactionShipments":[{"masterTrackingNumber":"794600000000","serviceType":"FEDEX_GROUND","pieceResponses":[{"trackingNumber":"794600000000","packageDocuments":[{"contentType":"LABEL","docType":"PDF","encodedLabel":"`+label+`"}]}]}]}}`)
	tools := registeredTools(t)
	request := shipRequest("DUPLICATE")

	result, envelope := callTool(t, tools, "shipments_create", map[string]any{"request": request})
	if result.IsError || envelope["status"] != "succeeded" {
		t.Fatalf("shipments_create result=%v, want success", envelope)
	}
	if fake.calls != 1 || len(fake.bodies[0]) == 0 || fake.bodies[0][0] != '{' {
		t.Fatalf("shipments_create calls=%d body=%q, want one JSON object", fake.calls, fake.bodies)
	}
	text := toolResultText(t, result)
	if strings.Contains(text, label) || !strings.Contains(text, "794600000000") {
		t.Fatalf("write result must keep the tracking number and drop the encoded label: %s", text)
	}

	result, envelope = callTool(t, tools, "shipments_create", map[string]any{"request": request})
	if !result.IsError || envelope["error_class"] != "duplicate_refused" {
		t.Fatalf("identical second call=%v, want duplicate_refused", envelope)
	}
	if fake.calls != 1 {
		t.Fatalf("duplicate reached FedEx: calls=%d", fake.calls)
	}

	result, envelope = callTool(t, tools, "shipments_create", map[string]any{"request": request, "allow_duplicate": true})
	if result.IsError {
		t.Fatalf("allow_duplicate call=%v, want success", envelope)
	}
	if fake.calls != 2 {
		t.Fatalf("allow_duplicate calls=%d, want 2", fake.calls)
	}
}

func TestWrite500IsOutcomeUnknownNotRetriedAndBlocksResend(t *testing.T) {
	fake := newFakeAPI(t, http.StatusInternalServerError, `{"errors":[{"code":"SYSTEM.UNAVAILABLE"}]}`)
	tools := registeredTools(t)
	request := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-03T14:00:00-05:00",
			"customerCloseTime":  "18:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"companyName": "Origin LLC", "phoneNumber": "5551112222"},
				"address": map[string]any{"streetLines": []any{"1 Main St"}, "city": "Chicago", "stateOrProvinceCode": "IL", "postalCode": "60601", "countryCode": "US"},
			},
		},
		"totalWeight":  map[string]any{"units": "LB", "value": 3.5},
		"packageCount": 1,
		"carrierCode":  "FDXG",
	}

	result, envelope := callTool(t, tools, "pickups_create", map[string]any{"request": request, "availability_override_reason": "carrier desk confirmed the window"})
	if !result.IsError || envelope["error_class"] != "outcome_unknown" {
		t.Fatalf("pickups_create 500=%v, want outcome_unknown", envelope)
	}
	if operationID, _ := envelope["operation_id"].(string); operationID == "" {
		t.Fatalf("pickups_create 500 dropped operation_id: %v", envelope)
	}
	if fake.calls != 1 {
		t.Fatalf("write was retried: calls=%d", fake.calls)
	}
	result, envelope = callTool(t, tools, "pickups_create", map[string]any{"request": request, "availability_override_reason": "carrier desk confirmed the window"})
	if !result.IsError || envelope["error_class"] != "duplicate_refused" {
		t.Fatalf("resend after unknown outcome=%v, want duplicate_refused", envelope)
	}
	if fake.calls != 1 {
		t.Fatalf("resend reached FedEx: calls=%d", fake.calls)
	}
}

func TestUnusablePickupSuccessReturnsOperationID(t *testing.T) {
	fake := newFakeAPI(t, http.StatusOK, `{"output":{"message":"no confirmation"}}`)
	tools := registeredTools(t)
	request := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-03T14:00:00-05:00",
			"customerCloseTime":  "18:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"companyName": "Origin LLC", "phoneNumber": "5551112222"},
				"address": map[string]any{"streetLines": []any{"1 Main St"}, "city": "Chicago", "stateOrProvinceCode": "IL", "postalCode": "60601", "countryCode": "US"},
			},
		},
		"totalWeight":  map[string]any{"units": "LB", "value": 3.5},
		"packageCount": 1,
		"carrierCode":  "FDXG",
	}
	result, envelope := callTool(t, tools, "pickups_create", map[string]any{"request": request, "availability_override_reason": "carrier desk confirmed the window"})
	if !result.IsError {
		t.Fatalf("unusable pickup success=%v", envelope)
	}
	operationID, _ := envelope["operation_id"].(string)
	if operationID == "" || fake.calls != 1 {
		t.Fatalf("unusable pickup success envelope=%v calls=%d", envelope, fake.calls)
	}
}

func TestUnwiredWriteToolWorks(t *testing.T) {
	fake := newFakeAPI(t, http.StatusOK, `{"output":{"status":"CLOSED"}}`)
	tools := registeredTools(t)
	result, envelope := callTool(t, tools, "endofday_close", map[string]any{"request": map[string]any{"accountNumber": map[string]any{"value": "123456789"}}})
	if result.IsError || fake.calls != 1 {
		t.Fatalf("endofday_close=%v calls=%d, want one successful call", envelope, fake.calls)
	}
}

func TestReadOnlyToolCallsFedExAndAllowsRepeats(t *testing.T) {
	fake := newFakeAPI(t, http.StatusOK, `{"output":{"rateReplyDetails":[]}}`)
	tools := registeredTools(t)
	args := map[string]any{"request": map[string]any{
		"accountNumber": map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"shipper":                   map[string]any{"address": map[string]any{"postalCode": "60601", "countryCode": "US"}},
			"recipient":                 map[string]any{"address": map[string]any{"postalCode": "48201", "countryCode": "US"}},
			"pickupType":                "DROPOFF_AT_FEDEX_LOCATION",
			"requestedPackageLineItems": []any{map[string]any{"weight": map[string]any{"units": "LB", "value": 1}, "groupPackageCount": 1}},
		},
	}}
	for i := 0; i < 2; i++ {
		result, envelope := callTool(t, tools, "rates_quote", args)
		if result.IsError {
			t.Fatalf("rates_quote=%v", envelope)
		}
	}
	if fake.calls != 2 {
		t.Fatalf("rates_quote calls=%d, want 2", fake.calls)
	}
}

func TestShipBulkStopsAtUnknownOutcome(t *testing.T) {
	fake := newFakeAPI(t, http.StatusInternalServerError, `{}`)
	tools := registeredTools(t)
	result, envelope := callTool(t, tools, "ship_bulk", map[string]any{"shipments": []any{
		map[string]any{"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND", "ref": "A"}},
		map[string]any{"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND", "ref": "B"}},
	}})
	if !result.IsError || envelope["status"] != "stopped" {
		t.Fatalf("ship_bulk=%v, want stopped", envelope)
	}
	if fake.calls != 1 {
		t.Fatalf("ship_bulk kept sending after unknown outcome: calls=%d", fake.calls)
	}
	counts, _ := envelope["counts"].(map[string]any)
	if counts["outcome_unknown"] != float64(1) || counts["not_attempted"] != float64(1) {
		t.Fatalf("ship_bulk counts=%v", counts)
	}
}

func TestShipBulkSendsEachShipmentOnce(t *testing.T) {
	fake := newFakeAPI(t, http.StatusOK, `{"output":{"transactionShipments":[{"masterTrackingNumber":"1"}]}}`)
	tools := registeredTools(t)
	result, envelope := callTool(t, tools, "ship_bulk", map[string]any{"shipments": []any{
		map[string]any{"requestedShipment": map[string]any{"ref": "A"}},
		map[string]any{"requestedShipment": map[string]any{"ref": "B"}},
		map[string]any{"requestedShipment": map[string]any{"ref": "A"}},
	}})
	if result.IsError || envelope["status"] != "completed" {
		t.Fatalf("ship_bulk=%v", envelope)
	}
	if fake.calls != 2 {
		t.Fatalf("ship_bulk calls=%d, want 2 (third row duplicates the first)", fake.calls)
	}
}

func TestLocalCommandArgsAreFixed(t *testing.T) {
	root := cli.RootCmd()
	var archive localCommand
	for _, lc := range localCommands {
		if lc.name == "archive" {
			archive = lc
		}
	}
	cmd, _, err := root.Find(archive.path)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := localCommandArgs(archive, cmd, map[string]any{"query": "--yes", "limit": float64(5), "service": "FEDEX_GROUND --config=/etc/passwd"})
	if err != nil {
		t.Fatal(err)
	}
	want := "--json --no-input --no-color archive --limit=5 --service=FEDEX_GROUND --config=/etc/passwd -- --yes"
	if strings.Join(argv, " ") != want {
		t.Fatalf("argv=%q, want %q", strings.Join(argv, " "), want)
	}
	for _, bad := range []map[string]any{{"args": "--yes"}, {"yes": true}, {"config": "/tmp/x"}, {"deliver": "file:/tmp/x"}, {"limit": "5"}} {
		if _, err := localCommandArgs(archive, cmd, bad); err == nil {
			t.Errorf("localCommandArgs accepted %v", bad)
		}
	}
	for _, lc := range localCommands {
		cmd, _, err := root.Find(lc.path)
		if err != nil || cmd.Name() != lc.path[len(lc.path)-1] {
			t.Fatalf("local tool %q points at missing command %v", lc.name, lc.path)
		}
		for _, name := range lc.flags {
			if cmd.LocalNonPersistentFlags().Lookup(name) == nil {
				t.Errorf("local tool %q allowlists missing flag --%s", lc.name, name)
			}
		}
	}
}

func TestWriteSummaryOmitsEncodedLabelsAndPII(t *testing.T) {
	encoded := strings.Repeat("synthetic-label-bytes", 200)
	street := "SENTINEL RECIPIENT STREET"
	redacted := summarizeWriteResponse("shipments_create", map[string]any{
		"output": map[string]any{
			"encodedLabel":   encoded,
			"streetLines":    street,
			"trackingNumber": "synthetic-tracking",
			"packageDocuments": []any{map[string]any{
				"url":         "https://labels.invalid/label-url",
				"companyName": "SENTINEL COMPANY",
				"city":        "SENTINEL CITY",
				"postalCode":  "00000",
			}},
		},
	})
	serialized, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{encoded, street, "SENTINEL COMPANY", "SENTINEL CITY", "00000"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("sensitive value %q escaped redaction", secret)
		}
	}
	for _, kept := range []string{"synthetic-tracking", "label-url"} {
		if !strings.Contains(string(serialized), kept) {
			t.Fatalf("%q was lost: %s", kept, serialized)
		}
	}

	generic := omitLargeStrings(map[string]any{"docs": []any{map[string]any{"content": encoded}}, "status": "OK"})
	serialized, _ = json.Marshal(generic)
	if strings.Contains(string(serialized), encoded) || !strings.Contains(string(serialized), "OK") {
		t.Fatalf("generic write summary=%s", serialized)
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
