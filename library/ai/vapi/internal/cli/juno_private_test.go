package cli

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestBuildDialBodyRecordsByDefault(t *testing.T) {
	body, err := buildDialBody(dialBodyOptions{
		AssistantID:   "asst_123",
		PhoneNumberID: "pn_123",
		Numbers:       []string{"+15551234567"},
	})
	if err != nil {
		t.Fatalf("buildDialBody returned error: %v", err)
	}
	artifactPlan := body["assistantOverrides"].(map[string]any)["artifactPlan"].(map[string]any)
	if got := artifactPlan["recordingEnabled"]; got != true {
		t.Fatalf("recordingEnabled = %v, want true", got)
	}
	if got := artifactPlan["recordingFormat"]; got != "wav;l16" {
		t.Fatalf("recordingFormat = %v, want wav;l16", got)
	}
}

func TestBuildDialBodyNoRecordOptOut(t *testing.T) {
	body, err := buildDialBody(dialBodyOptions{
		AssistantID:   "asst_123",
		PhoneNumberID: "pn_123",
		Numbers:       []string{"+15551234567"},
		NoRecord:      true,
	})
	if err != nil {
		t.Fatalf("buildDialBody returned error: %v", err)
	}
	artifactPlan := body["assistantOverrides"].(map[string]any)["artifactPlan"].(map[string]any)
	if got := artifactPlan["recordingEnabled"]; got != false {
		t.Fatalf("recordingEnabled = %v, want false", got)
	}
}

func TestJunoCommandTreeIncludesPrivateExtensions(t *testing.T) {
	cmd := RootCmd()
	for _, args := range [][]string{
		{"dial", "--help"},
		{"juno", "followup", "--help"},
		{"juno", "status", "--help"},
		{"juno", "setup", "--help"},
		{"juno", "test-call", "--help"},
		{"juno", "report", "--help"},
		{"juno", "phone-numbers", "--help"},
		{"juno", "assistant", "--help"},
		{"juno", "assistant", "payload", "--help"},
		{"juno", "assistant", "create", "--help"},
		{"juno", "assistant", "update", "--help"},
	} {
		cmd.SetArgs(args)
		cmd.SetOut(discardWriter{})
		cmd.SetErr(discardWriter{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("RootCmd(%v) returned error: %v", args, err)
		}
	}
}

func TestDefaultJunoAssistantPayload(t *testing.T) {
	payload := defaultJunoAssistantPayload()
	if payload["name"] != "Juno" {
		t.Fatalf("name = %v, want Juno", payload["name"])
	}
	artifactPlan := payload["artifactPlan"].(map[string]any)
	if got := artifactPlan["recordingEnabled"]; got != true {
		t.Fatalf("recordingEnabled = %v, want true", got)
	}
	if got := artifactPlan["recordingFormat"]; got != "wav;l16" {
		t.Fatalf("recordingFormat = %v, want wav;l16", got)
	}
	model := payload["model"].(map[string]any)
	if got := model["provider"]; got != "openai" {
		t.Fatalf("model.provider = %v, want openai", got)
	}
}

func TestSummarizeJunoPhoneNumbers(t *testing.T) {
	items := summarizeJunoPhoneNumbers([]map[string]any{
		{"id": "pn_123", "name": "Line", "number": "+15551234567", "provider": "vapi"},
		{"id": "pn_missing", "name": "Broken"},
	})
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0]["outbound_ready"] != false {
		t.Fatalf("first item should sort by name and be not ready: %#v", items[0])
	}
	if items[1]["outbound_ready"] != true {
		t.Fatalf("second item should be ready: %#v", items[1])
	}
}

func TestBuildJunoFollowupBodyFromCallCarriesContext(t *testing.T) {
	original := map[string]any{
		"assistantId":   "asst_123",
		"phoneNumberId": "pn_123",
		"customer":      map[string]any{"number": "+15551234567"},
		"assistantOverrides": map[string]any{
			"variableValues": map[string]any{
				"juno_goal":        "Make a reservation",
				"juno_constraints": []any{"no deposit"},
			},
		},
	}
	body, err := buildJunoFollowupBodyFromCall(original, "call_123", junoCommonOptions{}, junoWorkflowOptions{
		TaskType: "followup",
		Notes:    []string{"They asked us to call back after 2pm"},
	})
	if err != nil {
		t.Fatalf("buildJunoFollowupBodyFromCall returned error: %v", err)
	}
	variables := body["assistantOverrides"].(map[string]any)["variableValues"].(map[string]any)
	if got := variables["juno_previous_call_id"]; got != "call_123" {
		t.Fatalf("juno_previous_call_id = %v, want call_123", got)
	}
	if got := variables["juno_task_type"]; got != "followup" {
		t.Fatalf("juno_task_type = %v, want followup", got)
	}
	if got := body["assistantOverrides"].(map[string]any)["artifactPlan"].(map[string]any)["recordingEnabled"]; got != true {
		t.Fatalf("recordingEnabled = %v, want true", got)
	}
}

func TestDecodePPBinary(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"_pp_binary":   true,
		"content_type": "audio/wav",
		"encoding":     "base64",
		"data":         base64.StdEncoding.EncodeToString([]byte("audio")),
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, contentType, err := decodePPBinary(raw)
	if err != nil {
		t.Fatalf("decodePPBinary returned error: %v", err)
	}
	if string(payload) != "audio" {
		t.Fatalf("payload = %q, want audio", payload)
	}
	if contentType != "audio/wav" {
		t.Fatalf("contentType = %q, want audio/wav", contentType)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
