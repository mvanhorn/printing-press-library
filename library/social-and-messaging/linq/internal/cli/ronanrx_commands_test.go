package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInviteLinkSchemaHappyPath(t *testing.T) {
	got, err := buildInviteSchema(inviteLinkOptions{
		BaseURL:       "https://ronanrx.com",
		FrontDoorPath: "/start/text/",
		From:          "+16282893046",
		Routing:       "WELCOME_FLOW",
		Token:         "rrx_8Gk27sQp",
	})
	if err != nil {
		t.Fatalf("buildInviteSchema returned error: %v", err)
	}
	if !strings.HasPrefix(got.FrontDoorLink, "https://ronanrx.com/start/text/?") {
		t.Fatalf("unexpected front door link: %s", got.FrontDoorLink)
	}
	if !strings.HasPrefix(got.SMSURIPreview, "sms:+16282893046?&body=WELCOME_FLOW+") {
		t.Fatalf("unexpected sms uri preview: %s", got.SMSURIPreview)
	}
	if got.FirstSender != "patient" || got.OutboundSendPerformed {
		t.Fatalf("schema must be inbound-first/no-send, got first_sender=%q outbound=%v", got.FirstSender, got.OutboundSendPerformed)
	}
	if !got.PointerNotPayload || got.PHIInBody || got.PHIInURL {
		t.Fatalf("expected pointer-not-payload true with no PHI flags: %#v", got)
	}
	if !got.RequiresFrontDoorHandoff || !got.FrontDoorContract.BrowserMustBuildSMSURI {
		t.Fatalf("front-door handoff contract missing: %#v", got.FrontDoorContract)
	}
}

func TestInviteLinkGenericStartWarns(t *testing.T) {
	got, err := buildInviteSchema(inviteLinkOptions{
		BaseURL: "https://ronanrx.com/start/",
		From:    "+16282893046",
		Routing: "WELCOME_FLOW",
		Token:   "rrx_8Gk27sQp",
	})
	if err != nil {
		t.Fatalf("buildInviteSchema returned error: %v", err)
	}
	if len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "looks generic") {
		t.Fatalf("expected generic /start/ warning, got %#v", got.Warnings)
	}

	noWarn, err := buildInviteSchema(inviteLinkOptions{
		BaseURL:       "https://ronanrx.com",
		FrontDoorPath: "/start/text/",
		From:          "+16282893046",
		Routing:       "WELCOME_FLOW",
		Token:         "rrx_8Gk27sQp",
	})
	if err != nil {
		t.Fatalf("buildInviteSchema with front-door-path returned error: %v", err)
	}
	if len(noWarn.Warnings) != 0 {
		t.Fatalf("did not expect generic warning when front-door-path is explicit: %#v", noWarn.Warnings)
	}
}

func TestInviteLinkRejectsInvalidPhoneAndHTTP(t *testing.T) {
	if _, err := buildInviteSchema(inviteLinkOptions{BaseURL: "https://ronanrx.com", From: "+155****1000", Routing: "WELCOME_FLOW", Token: "rrx_8Gk27sQp"}); err == nil {
		t.Fatalf("expected invalid E.164 phone to be rejected")
	}
	if _, err := buildInviteSchema(inviteLinkOptions{BaseURL: "http://ronanrx.com", From: "+16282893046", Routing: "WELCOME_FLOW", Token: "rrx_8Gk27sQp"}); err == nil {
		t.Fatalf("expected non-HTTPS base URL to be rejected")
	}
}

func TestInviteLinkRejectsPHITokensAndBody(t *testing.T) {
	tokens := []string{"patient@example.com", "john-smith-1234", "2026-06-17", "ozempic-10mg"}
	for _, token := range tokens {
		if _, err := buildInviteSchema(inviteLinkOptions{BaseURL: "https://ronanrx.com", FrontDoorPath: "/start/text/", From: "+16282893046", Routing: "WELCOME_FLOW", Token: token}); err == nil {
			t.Fatalf("expected token %q to be rejected", token)
		}
	}
	if _, err := buildInviteSchema(inviteLinkOptions{
		BaseURL:                "https://ronanrx.com",
		FrontDoorPath:          "/start/text/",
		From:                   "+16282893046",
		Routing:                "WELCOME_FLOW",
		Token:                  "rrx_8Gk27sQp",
		PrefillBody:            "Hi Jane Smith, your Ozempic 2mg refill is ready",
		PatientAuthoredPrefill: true,
	}); err == nil {
		t.Fatalf("expected natural-language PHI prefill to be rejected")
	}
}

func TestInviteLinkCommandKeepsDeprecatedAlias(t *testing.T) {
	out, err := runRonanRxCommand(t, "invite-link", "--base-url", "https://ronanrx.com", "--front-door-path", "/start/text/", "--from", "+16282893046", "--routing", "WELCOME_FLOW", "--token", "rrx_8Gk27sQp", "--agent")
	if err != nil {
		t.Fatalf("invite-link command failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["first_sender"] != "patient" || got["outbound_send_performed"] != false {
		t.Fatalf("schema missing inbound-first/no-send markers: %#v", got)
	}
	if got["front_door_link"] == "" || got["sms_uri_preview"] == "" || got["invite_link"] == "" || got["invite_link_deprecated"] != true {
		t.Fatalf("schema missing front-door/sms/deprecated fields: %#v", got)
	}
}

func TestWelcomeFlowBlocksSendUntilInboundChatExists(t *testing.T) {
	out, err := runRonanRxCommand(t, "welcome-flow", "--base-url", "https://ronanrx.com", "--front-door-path", "/start/text/", "--from", "+16282893046", "--routing", "WELCOME_FLOW", "--token", "rrx_8Gk27sQp", "--secure-link", "https://secure.ronanrx.com/t/opaque-123", "--allow-host", "ronanrx.com", "--agent")
	if err != nil {
		t.Fatalf("welcome-flow command failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["would_send"] != false || got["first_real_action"] != "patient_sends_prefilled_inbound_message" {
		t.Fatalf("welcome plan should be no-send and patient-first: %#v", got)
	}
	preflight := got["send_preflight"].(map[string]any)
	if preflight["allowed"] == true {
		t.Fatalf("send preflight should be blocked without inbound chat/evidence: %#v", preflight)
	}
}

func TestSendPreflightPlaceholderAllowedOnlySynthetic(t *testing.T) {
	synthetic := evaluateSendPreflight("<chat>", "INBOUND_OK WELCOME_FLOW", "https://secure.ronanrx.com/t/opaque-123", sendPreflightOptions{
		Mode:         "synthetic",
		AllowedHosts: []string{"ronanrx.com"},
	})
	if !synthetic.Allowed {
		t.Fatalf("placeholder should be allowed in synthetic mode: %#v", synthetic)
	}

	real := evaluateSendPreflight("<chat>", "INBOUND_OK WELCOME_FLOW", "https://secure.ronanrx.com/t/opaque-123", sendPreflightOptions{
		Mode:         "real",
		AllowedHosts: []string{"ronanrx.com"},
	})
	if real.Allowed {
		t.Fatalf("placeholder must be blocked in real mode: %#v", real)
	}
	if !strings.Contains(strings.Join(real.BlockingReasons, " "), "INBOUND_OK alone is not enough") {
		t.Fatalf("real mode should explain missing inbound evidence, got %#v", real.BlockingReasons)
	}
}

func TestAuditLink(t *testing.T) {
	ok := auditLink("https://secure.ronanrx.example/opaque", []string{"ronanrx.example"})
	if ok["allowed"] != true {
		t.Fatalf("expected allowlisted HTTPS opaque link to pass: %#v", ok)
	}

	bad := auditLink("https://secure.ronanrx.example/patient/Jane-Smith?dob=2026-06-17&drug=Ozempic", []string{"ronanrx.example"})
	if bad["allowed"] == true {
		t.Fatalf("expected PHI-shaped path/query to fail: %#v", bad)
	}
	keys, _ := bad["sensitive_query_keys"].([]string)
	if len(keys) == 0 {
		t.Fatalf("expected sensitive query keys to be reported: %#v", bad)
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

func runRonanRxCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}
