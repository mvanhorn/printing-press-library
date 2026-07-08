package sfmock

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCompositeGraphSmall(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/composite/graph?fixture=acme_small", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Graphs []struct {
			GraphID       string `json:"graphId"`
			IsSuccessful  bool   `json:"isSuccessful"`
			GraphResponse struct {
				CompositeResponse []struct {
					ReferenceID string `json:"referenceId"`
					Body        struct {
						TotalSize int `json:"totalSize"`
					} `json:"body"`
				} `json:"compositeResponse"`
			} `json:"graphResponse"`
		} `json:"graphs"`
	}
	decodeJSON(t, resp.Body, &body)

	if len(body.Graphs) != 1 {
		t.Fatalf("graphs len = %d, want 1", len(body.Graphs))
	}
	if body.Graphs[0].GraphID != "acme-small" || !body.Graphs[0].IsSuccessful {
		t.Fatalf("unexpected graph header: %+v", body.Graphs[0])
	}
	if len(body.Graphs[0].GraphResponse.CompositeResponse) < 6 {
		t.Fatalf("composite responses len = %d, want at least 6", len(body.Graphs[0].GraphResponse.CompositeResponse))
	}
	if body.Graphs[0].GraphResponse.CompositeResponse[1].ReferenceID != "Contacts" {
		t.Fatalf("second response reference = %q, want Contacts", body.Graphs[0].GraphResponse.CompositeResponse[1].ReferenceID)
	}
	if body.Graphs[0].GraphResponse.CompositeResponse[1].Body.TotalSize != 6 {
		t.Fatalf("contacts totalSize = %d, want 6", body.Graphs[0].GraphResponse.CompositeResponse[1].Body.TotalSize)
	}
}

func TestUIAPIRecordAndNotFound(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/ui-api/records/003ACME0001", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		APIName string `json:"apiName"`
		Fields  map[string]struct {
			Value any `json:"value"`
		} `json:"fields"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.APIName != "Contact" {
		t.Fatalf("apiName = %q, want Contact", body.APIName)
	}
	for _, field := range []string{"Id", "FirstName", "LastName", "Email", "Salary__c"} {
		if _, ok := body.Fields[field]; !ok {
			t.Fatalf("field %s missing from UI API response", field)
		}
	}

	resp = doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/ui-api/records/XYZUNKNOWN", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown status = %d, want 404", resp.StatusCode)
	}
}

func TestRateLimitFailModeAddsHeaderWithoutChangingStatus(t *testing.T) {
	server := newHandlerServer()
	server.SetFailMode(FailRateLimit)

	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/ui-api/records/003ACME0001", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Sforce-Limit-Info"); got != "api-usage=81000/100000" {
		t.Fatalf("Sforce-Limit-Info = %q, want api-usage=81000/100000", got)
	}
}

func TestShieldFieldFailModeMarksUIAPIFieldEncrypted(t *testing.T) {
	server := newHandlerServer()
	server.SetFailMode(FailShieldField)

	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/ui-api/records/003ACME0001", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Fields map[string]map[string]any `json:"fields"`
	}
	decodeJSON(t, resp.Body, &body)
	if encrypted, _ := body.Fields["Salary__c"]["IsEncrypted"].(bool); !encrypted {
		t.Fatalf("Salary__c IsEncrypted = %v, want true", body.Fields["Salary__c"]["IsEncrypted"])
	}
}

func TestSharingRestrictedFailModeRejectsRestrictedIDs(t *testing.T) {
	server := newHandlerServer()
	server.SetFailMode(FailSharingRestricted)

	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/ui-api/records/003RESTRICTED001", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "INSUFFICIENT_ACCESS") {
		t.Fatalf("body = %s, want INSUFFICIENT_ACCESS", body)
	}
}

func TestCertificateUnavailableFailModeRejectsCertificatePost(t *testing.T) {
	server := newHandlerServer()
	server.SetFailMode(FailCertificateUnavailable)

	resp := doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/tooling/sobjects/Certificate", `{"DeveloperName":"SF360"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "INVALID_TYPE") {
		t.Fatalf("body = %s, want INVALID_TYPE", body)
	}
}

func TestWritePatchRoutes(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodPatch, apiPrefix+"/sobjects/Account/001ACME0001", `{"Name":"Acme Updated"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("LastModifiedDate"); got == "" {
		t.Fatalf("LastModifiedDate header missing")
	}
	if body := ReadAll(resp); !strings.Contains(body, `"success": true`) {
		t.Fatalf("body = %s, want success", body)
	}

	server.SetFailMode(FailStaleWrite)
	resp = doHandlerRequest(t, server, http.MethodPatch, apiPrefix+"/sobjects/Account/001ACME0001", `{"Name":"Acme Updated"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale status = %d, want 409", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "PRECONDITION_FAILED") {
		t.Fatalf("body = %s, want PRECONDITION_FAILED", body)
	}
}

func TestUIRecordPatchFLSFailMode(t *testing.T) {
	server := newHandlerServer()
	server.SetFailMode(FailFLSWriteDenied)

	resp := doHandlerRequest(t, server, http.MethodPatch, apiPrefix+"/ui-api/records/003ACME0001", `{"fields":{"Salary__c":142000}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "INVALID_FIELD_FOR_INSERT_UPDATE") {
		t.Fatalf("body = %s, want INVALID_FIELD_FOR_INSERT_UPDATE", body)
	}
}

func TestWritePostRoutesAndFailureModes(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/sobjects/Task", `{"Subject":"Follow up"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, `"id":"00TWRITE0001"`) {
		t.Fatalf("body = %s, want mock Task id", body)
	}

	server.SetFailMode(FailValidationRule)
	resp = doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/sobjects/Task", `{"Subject":"Follow up"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("validation status = %d, want 400", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "VALIDATION_RULE_VIOLATION") {
		t.Fatalf("body = %s, want VALIDATION_RULE_VIOLATION", body)
	}

	server.SetFailMode("required_field_missing")
	resp = doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/sobjects/Task", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("required-field status = %d, want 400", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "REQUIRED_FIELD_MISSING") {
		t.Fatalf("body = %s, want REQUIRED_FIELD_MISSING", body)
	}
}

func TestSObjectIdempotentPatchTracksRepeatKeys(t *testing.T) {
	server := newHandlerServer()
	path := apiPrefix + "/sobjects/Task/SF360_Idempotency_Key__c/abc123"

	resp := doHandlerRequest(t, server, http.MethodPatch, path, `{"Subject":"Follow up"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", resp.StatusCode)
	}

	resp = doHandlerRequest(t, server, http.MethodPatch, path, `{"Subject":"Follow up"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("repeat status = %d, want 204", resp.StatusCode)
	}
}

func TestApexWriteRoutesAndFailureModes(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodPost, "/services/apexrest/sf360/v1/safeWrite", `{"sobject":"Account"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("safeWrite status = %d, want 200", resp.StatusCode)
	}

	server.SetFailMode(FailFLSWriteDenied)
	resp = doHandlerRequest(t, server, http.MethodPost, "/services/apexrest/sf360/v1/safeWrite", `{"sobject":"Contact"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("safeWrite FLS status = %d, want 400", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "INVALID_FIELD_FOR_INSERT_UPDATE") {
		t.Fatalf("body = %s, want INVALID_FIELD_FOR_INSERT_UPDATE", body)
	}

	server.SetFailMode(FailApexMissing)
	resp = doHandlerRequest(t, server, http.MethodPost, "/services/apexrest/sf360/v1/safeWrite", `{"sobject":"Account"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("safeWrite missing status = %d, want 404", resp.StatusCode)
	}
}

func TestApexSafeUpsertTracksRepeatKeys(t *testing.T) {
	server := newHandlerServer()
	path := "/services/apexrest/sf360/v1/safeUpsert"

	resp := doHandlerRequest(t, server, http.MethodPost, path, `{"idempotencyKey":"abc123"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", resp.StatusCode)
	}

	resp = doHandlerRequest(t, server, http.MethodPost, path, `{"idempotencyKey":"abc123"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repeat status = %d, want 200", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, `"created": false`) {
		t.Fatalf("body = %s, want created false", body)
	}
}

func TestWriteAuditPostRoutes(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/sobjects/SF360_Write_Audit__c/", `{"TraceId__c":"trace-1"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	server.SetFailMode(FailAuditFailed)
	resp = doHandlerRequest(t, server, http.MethodPost, apiPrefix+"/sobjects/SF360_Write_Audit__c/", `{"TraceId__c":"trace-1"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("audit failure status = %d, want 500", resp.StatusCode)
	}
	if body := ReadAll(resp); !strings.Contains(body, "SERVER_UNAVAILABLE") {
		t.Fatalf("body = %s, want SERVER_UNAVAILABLE", body)
	}
}

func TestUnknownRouteReturnsSalesforceErrorEnvelope(t *testing.T) {
	server := newHandlerServer()
	resp := doHandlerRequest(t, server, http.MethodGet, apiPrefix+"/not-a-route", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}

	var body []struct {
		ErrorCode string `json:"errorCode"`
		Message   string `json:"message"`
	}
	decodeJSON(t, resp.Body, &body)
	if len(body) != 1 || body[0].ErrorCode != "NOT_FOUND" {
		t.Fatalf("error envelope = %+v, want NOT_FOUND", body)
	}
}

func TestStartRespondsToCanonicalRequests(t *testing.T) {
	server := Start(t)

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/", ""},
		{http.MethodHead, "/", ""},
		{http.MethodGet, apiPrefix + "/composite/graph?fixture=acme_large", ""},
		{http.MethodPost, apiPrefix + "/composite/graph", `{"graphs":[]}`},
		{http.MethodGet, apiPrefix + "/ui-api/records/003ACME0001", ""},
		{http.MethodPatch, apiPrefix + "/ui-api/records/003ACME0001", `{"fields":{"Title":"VP"}}`},
		{http.MethodPatch, apiPrefix + "/sobjects/Account/001ACME0001", `{"Name":"Acme Updated"}`},
		{http.MethodPost, apiPrefix + "/sobjects/Task", `{"Subject":"Follow up"}`},
		{http.MethodPatch, apiPrefix + "/sobjects/Task/SF360_Idempotency_Key__c/start-canonical", `{"Subject":"Follow up"}`},
		{http.MethodPost, "/services/apexrest/sf360/v1/safeWrite", `{"sobject":"Account"}`},
		{http.MethodPost, "/services/apexrest/sf360/v1/safeUpsert", `{"idempotencyKey":"canonical"}`},
		{http.MethodPost, apiPrefix + "/sobjects/SF360_Write_Audit__c/", `{"TraceId__c":"trace-1"}`},
		{http.MethodGet, apiPrefix + "/ui-api/records/001ACME0001", ""},
		{http.MethodGet, apiPrefix + "/tooling/query?q=SELECT+Id+FROM+FieldDefinition", ""},
		{http.MethodPost, apiPrefix + "/tooling/sobjects/Certificate", `{"DeveloperName":"SF360_Bundle_Mock"}`},
		{http.MethodGet, apiPrefix + "/query?q=SELECT+Id+FROM+SlackConversationRelation", ""},
		{http.MethodGet, apiPrefix + "/limits", ""},
		{http.MethodPost, apiPrefix + "/connect/data-cloud/oauth2/token", `{"grant_type":"urn:salesforce:data-cloud"}`},
		{http.MethodGet, apiPrefix + "/connect/data-cloud/unified-profile/003ACME0001", ""},
	}

	client := &http.Client{}
	for _, tc := range requests {
		var body io.Reader
		if tc.body != "" {
			body = bytes.NewBufferString(tc.body)
		}
		req, err := http.NewRequest(tc.method, server.URL+tc.path, body)
		if err != nil {
			t.Fatalf("%s %s: create request: %v", tc.method, tc.path, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			t.Fatalf("%s %s status = %d, want 2xx", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestDoctorMock(t *testing.T) {
	skipIfLocalBindDenied(t)

	cmd := exec.Command("go", "run", "./cmd/salesforce-headless-360-pp-cli", "doctor", "--mock")
	cmd.Dir = "../.."
	cmd.Env = cleanDoctorEnv(os.Environ())

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor --mock failed: %v\n%s", err, output)
	}
	text := string(output)
	for _, want := range []string{
		"Mock server: running",
		"Config: ok",
		"Auth: configured",
		"API: reachable",
		"Credentials: valid",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("doctor --mock output missing %q:\n%s", want, text)
		}
	}
}

func skipIfLocalBindDenied(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err == nil {
		listener.Close()
		return
	}
	if isLocalBindDenied(err) {
		t.Skipf("local listener unavailable in this environment: %v", err)
	}
	t.Fatalf("unexpected local listener error: %v", err)
}

func decodeJSON(t *testing.T, r io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(target); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func newHandlerServer() *Server {
	return &Server{}
}

func doHandlerRequest(t *testing.T, server *Server, method, path, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec.Result()
}

func cleanDoctorEnv(env []string) []string {
	unset := map[string]bool{
		"SALESFORCE_ACCESS_TOKEN":          true,
		"SALESFORCE_INSTANCE_URL":          true,
		"SALESFORCE_HEADLESS_360_BASE_URL": true,
		"SALESFORCE_360_CLIENT_ID":         true,
		"SALESFORCE_360_CLIENT_SECRET":     true,
		"SF_MOCK_FAIL":                     true,
		"SALESFORCE_HEADLESS_360_CONFIG":   true,
	}
	cleaned := make([]string, 0, len(env))
	for _, item := range env {
		key := strings.SplitN(item, "=", 2)[0]
		if unset[key] {
			continue
		}
		cleaned = append(cleaned, item)
	}
	return cleaned
}
