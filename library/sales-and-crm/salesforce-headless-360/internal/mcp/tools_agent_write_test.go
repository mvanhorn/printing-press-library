package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/agent"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestAgentUpdateRequiresConfirm(t *testing.T) {
	handler := agentWriteHandler(t, "agent_update")

	result, err := handler(context.Background(), callToolRequest("agent_update", map[string]any{
		"record_id": "001000000000001AAA",
		"fields":    map[string]any{"Name": "Acme"},
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}
	payload := resultMap(t, result)
	if payload["code"] != "MCP_CONFIRM_REQUIRED" {
		t.Fatalf("code = %v, want MCP_CONFIRM_REQUIRED", payload["code"])
	}
	if payload["message"] != confirmRequiredMessage {
		t.Fatalf("message = %v, want %q", payload["message"], confirmRequiredMessage)
	}
	if payload["http_status"] != float64(http.StatusBadRequest) {
		t.Fatalf("http_status = %v, want 400", payload["http_status"])
	}
}

func TestAgentUpdateDryRunBypassesConfirm(t *testing.T) {
	withTestHome(t)
	withTestWriteConfig(t, &fakeMCPWriteClient{
		getBody: uiRecordBody("001000000000001AAA", "Account", map[string]any{"Name": "Old"}),
	})
	handler := agentWriteHandler(t, "agent_update")

	result, err := handler(context.Background(), callToolRequest("agent_update", map[string]any{
		"record_id": "001000000000001AAA",
		"fields":    map[string]any{"Name": "New"},
		"dry_run":   true,
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result.IsError = true: %s", resultText(t, result))
	}
	payload := resultMap(t, result)
	if payload["success"] != true {
		t.Fatalf("success = %v, want true", payload["success"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", payload["data"])
	}
	if data["dry_run"] != true {
		t.Fatalf("data.dry_run = %v, want true", data["dry_run"])
	}
}

func TestAgentSignPlanRequiresConfirm(t *testing.T) {
	handler := agentWriteHandler(t, "agent_sign_plan")

	result, err := handler(context.Background(), callToolRequest("agent_sign_plan", map[string]any{
		"plan": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}
	payload := resultMap(t, result)
	if payload["code"] != "MCP_CONFIRM_REQUIRED" {
		t.Fatalf("code = %v, want MCP_CONFIRM_REQUIRED", payload["code"])
	}
}

func TestAgentExecutePlanCountersignatureErrorBubbles(t *testing.T) {
	withTestHome(t)
	fakeClient := &fakeMCPWriteClient{}
	withTestWriteConfig(t, fakeClient)
	plan := buildTestPlan(t, fakeClient)
	handler := agentWriteHandler(t, "agent_execute_plan")

	result, err := handler(context.Background(), callToolRequest("agent_execute_plan", map[string]any{
		"plan":                      planAsMap(t, plan),
		"require_countersignatures": float64(1),
		"confirm":                   true,
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}
	payload := resultMap(t, result)
	if payload["code"] != "INSUFFICIENT_COUNTERSIGNATURES" {
		t.Fatalf("code = %v, want INSUFFICIENT_COUNTERSIGNATURES", payload["code"])
	}
}

func TestAgentWriteAuditListFiltersExecutedRows(t *testing.T) {
	withTestHome(t)
	db, err := store.Open(dbPath())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	rows := []store.WriteAuditRow{
		{JTI: "pending", TargetSObject: "Account", Operation: "update", ExecutionStatus: "pending", GeneratedAt: "2026-04-22T10:00:00Z"},
		{JTI: "executed", TargetSObject: "Account", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T11:00:00Z"},
	}
	for _, row := range rows {
		if err := db.InsertWriteAudit(row); err != nil {
			t.Fatalf("insert write audit %s: %v", row.JTI, err)
		}
	}
	handler := agentWriteHandler(t, "agent_write_audit_list")

	result, err := handler(context.Background(), callToolRequest("agent_write_audit_list", map[string]any{
		"status": "executed",
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result.IsError = true: %s", resultText(t, result))
	}
	var got []store.WriteAuditRow
	if err := json.Unmarshal([]byte(resultText(t, result)), &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(got) != 1 || got[0].JTI != "executed" {
		t.Fatalf("rows = %#v, want only executed row", got)
	}
}

func TestRegisterToolsAdvertisesAgentWriteTools(t *testing.T) {
	s := server.NewMCPServer("test", "1.0.0", server.WithToolCapabilities(false))
	RegisterTools(s)
	RegisterAgentTools(s)
	response := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal tools/list response: %v", err)
	}
	var decoded struct {
		Result struct {
			Tools []mcplib.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal tools/list response: %v\n%s", err, data)
	}
	registered := map[string]bool{}
	for _, tool := range decoded.Result.Tools {
		registered[tool.Name] = true
	}

	for _, name := range expectedAgentWriteToolNames() {
		if !registered[name] {
			t.Fatalf("tool %s was not registered", name)
		}
	}
}

func TestAgentWriteToolSchemaParity(t *testing.T) {
	registered := map[string]mcplib.Tool{}
	for _, spec := range agentWriteToolSpecs() {
		registered[spec.name] = toolFromSpecForTest(spec)
	}

	for toolName, expected := range expectedAgentWriteToolArgs() {
		tool, ok := registered[toolName]
		if !ok {
			t.Fatalf("tool %s was not registered", toolName)
		}
		got := mapKeys(tool.InputSchema.Properties)
		sort.Strings(got)
		sort.Strings(expected)
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("%s args = %v, want %v", toolName, got, expected)
		}
	}
}

func agentWriteHandler(t *testing.T, name string) server.ToolHandlerFunc {
	t.Helper()
	for _, spec := range agentWriteToolSpecs() {
		if spec.name == name {
			return spec.handler
		}
	}
	t.Fatalf("tool %s was not registered", name)
	return nil
}

func callToolRequest(name string, args map[string]any) mcplib.CallToolRequest {
	var req mcplib.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args
	return req
}

func resultMap(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal result: %v\ntext: %s", err, resultText(t, result))
	}
	return payload
}

func resultText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("empty tool result")
	}
	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("content[0] = %T, want TextContent", result.Content[0])
	}
	return text.Text
}

func withTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SF360_ORG", "test-org")
	t.Setenv("SF360_USER_ID", "005TEST")
	t.Setenv("SF360_HOST_FINGERPRINT", "test-host")
}

func withTestWriteConfig(t *testing.T, fakeClient *fakeMCPWriteClient) {
	t.Helper()
	signer := testSigner(t)
	previous := configureMCPAgentWriteOptions
	configureMCPAgentWriteOptions = func(includeAudit bool, useMock bool) (agent.WriteOptions, func(), error) {
		return agent.WriteOptions{
			Client:     fakeClient,
			Signer:     signer,
			OrgAlias:   "test-org",
			OrgID:      "00DTEST",
			ActingUser: "005TEST",
			Now: func() time.Time {
				return time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
			},
		}, func() {}, nil
	}
	t.Cleanup(func() { configureMCPAgentWriteOptions = previous })
}

func testSigner(t *testing.T) *trust.FileSigner {
	t.Helper()
	signer, err := trust.NewFileSignerWithIdentity("test-org", "test-host", "005TEST")
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	if err := ensureMCPLocalPlanKeyRecord("test-org", signer); err != nil {
		t.Fatalf("save key record: %v", err)
	}
	return signer
}

func buildTestPlan(t *testing.T, fakeClient *fakeMCPWriteClient) *agent.WritePlan {
	t.Helper()
	signer := testSigner(t)
	opts := agent.NewCreateWriteOptions("Account", "idem-plan", map[string]any{"Name": "Acme"})
	opts.Client = fakeClient
	opts.Signer = signer
	opts.OrgAlias = "test-org"
	opts.OrgID = "00DTEST"
	opts.ActingUser = "005TEST"
	opts.Now = time.Now
	plan, err := agent.BuildPlan(context.Background(), opts, "sf_fallthrough", "ui_api", "")
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	return plan
}

func planAsMap(t *testing.T, plan *agent.WritePlan) map[string]any {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal plan map: %v", err)
	}
	return out
}

type fakeMCPWriteClient struct {
	getBody json.RawMessage
}

func (c *fakeMCPWriteClient) GetWithResponseHeaders(string, map[string]string) (json.RawMessage, http.Header, error) {
	if len(c.getBody) == 0 {
		return json.RawMessage(`{"fields":[]}`), http.Header{}, nil
	}
	return c.getBody, http.Header{}, nil
}

func (c *fakeMCPWriteClient) Post(string, any) (json.RawMessage, int, error) {
	return json.RawMessage(`{"id":"new-record"}`), http.StatusCreated, nil
}

func (c *fakeMCPWriteClient) PatchWithResponseHeaders(string, any, map[string]string) (json.RawMessage, int, http.Header, error) {
	return json.RawMessage(`{"id":"patched-record"}`), http.StatusOK, http.Header{}, nil
}

func uiRecordBody(id, apiName string, fields map[string]any) json.RawMessage {
	payloadFields := map[string]map[string]any{}
	for name, value := range fields {
		payloadFields[name] = map[string]any{"value": value}
	}
	data, _ := json.Marshal(map[string]any{
		"id":               id,
		"apiName":          apiName,
		"lastModifiedDate": "2026-04-22T10:00:00Z",
		"fields":           payloadFields,
	})
	return data
}

func expectedAgentWriteToolNames() []string {
	names := make([]string, 0, len(expectedAgentWriteToolArgs()))
	for name := range expectedAgentWriteToolArgs() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func expectedAgentWriteToolArgs() map[string][]string {
	writeCommon := []string{"confirm", "confirm_bulk", "dry_run", "mock", "run_as_user"}
	planCommon := []string{"dry_run", "expires_in", "human_summary", "mock", "output", "run_as_user"}
	return map[string][]string{
		"agent_update":              append([]string{"fields", "force_stale", "idempotency_key", "if_last_modified", "record_id"}, writeCommon...),
		"agent_upsert":              append([]string{"fields", "idempotency_key", "sobject"}, writeCommon...),
		"agent_create":              append([]string{"fields", "idempotency_key", "sobject"}, writeCommon...),
		"agent_log_activity":        append([]string{"description", "duration", "end", "idempotency_key", "start", "subject", "type", "what", "who"}, writeCommon...),
		"agent_advance":             append([]string{"close_date", "force_stale", "idempotency_key", "opp", "stage"}, writeCommon...),
		"agent_close_case":          append([]string{"case", "force_stale", "idempotency_key", "resolution", "status"}, writeCommon...),
		"agent_note":                append([]string{"entity", "text"}, writeCommon...),
		"agent_plan_update":         append([]string{"fields", "force_stale", "idempotency_key", "if_last_modified", "record_id"}, planCommon...),
		"agent_plan_upsert":         append([]string{"fields", "idempotency_key", "sobject"}, planCommon...),
		"agent_plan_create":         append([]string{"fields", "idempotency_key", "sobject"}, planCommon...),
		"agent_plan_log_activity":   append([]string{"description", "duration", "end", "idempotency_key", "start", "subject", "type", "what", "who"}, planCommon...),
		"agent_plan_advance":        append([]string{"close_date", "force_stale", "idempotency_key", "opp", "stage"}, planCommon...),
		"agent_plan_close_case":     append([]string{"case", "force_stale", "idempotency_key", "resolution", "status"}, planCommon...),
		"agent_plan_note":           append([]string{"entity", "text"}, planCommon...),
		"agent_sign_plan":           {"confirm", "output", "plan"},
		"agent_execute_plan":        {"confirm", "confirm_bulk", "dry_run", "force_stale", "mock", "plan", "require_countersignatures"},
		"agent_write_audit_list":    {"kid", "limit", "since", "sobject", "status"},
		"agent_write_audit_inspect": {"jti"},
		"agent_write_audit_verify":  {"jti"},
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func toolFromSpecForTest(spec agentWriteToolSpec) mcplib.Tool {
	options := []mcplib.ToolOption{mcplib.WithDescription(spec.description)}
	for _, arg := range spec.args {
		options = append(options, arg.toolOption())
	}
	return mcplib.NewTool(spec.name, options...)
}
