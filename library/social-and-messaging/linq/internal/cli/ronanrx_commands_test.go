package cli

import "testing"

func TestBuildInviteURL(t *testing.T) {
	u, err := buildInviteURL("https://ronanrx.example/text", "+155****1000", "WELCOME_FLOW", "opaque-token")
	if err != nil {
		t.Fatalf("buildInviteURL returned error: %v", err)
	}
	if u.Scheme != "https" || u.Host != "ronanrx.example" {
		t.Fatalf("unexpected invite URL: %s", u.String())
	}
	if got := u.Query().Get("text"); got != "WELCOME_FLOW opaque-token" {
		t.Fatalf("unexpected invite text query: %q", got)
	}
}

func TestEvaluateSendGuard(t *testing.T) {
	got := evaluateSendGuard("ch_123", "INBOUND_OK", "https://example.com/opaque")
	if !got.Allowed {
		t.Fatalf("expected guarded send preflight to allow safe pointer payload, reasons=%v checks=%v", got.Reasons, got.Checks)
	}

	cold := evaluateSendGuard("ch_123", "hello", "https://example.com/opaque")
	if cold.Allowed {
		t.Fatalf("expected cold outbound preflight to be refused")
	}
	if cold.Checks["inbound_first"] == "pass" {
		t.Fatalf("expected inbound_first check to fail, got %v", cold.Checks)
	}
}

func TestAuditLink(t *testing.T) {
	ok := auditLink("https://secure.ronanrx.example/opaque", []string{"ronanrx.example"})
	if ok["allowed"] != true {
		t.Fatalf("expected allowlisted HTTPS opaque link to pass: %#v", ok)
	}

	bad := auditLink("http://evil.example/patient@example.com", []string{"ronanrx.example"})
	if bad["allowed"] == true {
		t.Fatalf("expected non-HTTPS PHI-shaped off-domain link to fail: %#v", bad)
	}
}

func TestHumanReviewCandidatesRedacts(t *testing.T) {
	items := humanReviewCandidates([]map[string]any{{
		"chat_id":   "ch_1",
		"direction": "inbound",
		"text":      "I am upset and my email is patient@example.com",
	}})
	if len(items) != 1 {
		t.Fatalf("expected one candidate, got %d", len(items))
	}
	if snippet, _ := items[0]["redacted_snippet"].(string); snippet == "" || snippet == "I am upset and my email is patient@example.com" {
		t.Fatalf("expected redacted snippet, got %#v", items[0])
	}
}
