package linkedin

import "testing"

func TestRequireLiveApplySupportedAllowsHeadlineAboutAndExperience(t *testing.T) {
	changes := []SectionChange{
		{Section: "headline", After: "New headline"},
		{Section: "about", After: "New about"},
		{Section: "experience", After: "New experience"},
	}
	if err := RequireLiveApplySupported(changes); err != nil {
		t.Fatalf("headline/about/experience should be supported: %v", err)
	}
}

func TestRequireLiveApplySupportedRejectsUnknownSections(t *testing.T) {
	changes := []SectionChange{{Section: "education", After: "new education blob"}}
	if err := RequireLiveApplySupported(changes); err == nil {
		t.Fatal("unknown sections should be rejected before writes")
	}
}

func TestSDUIServerRequestBodyUsesLinkedInRequestID(t *testing.T) {
	body, err := BuildSDUIServerRequest("com.linkedin.sdui.requests.profile.saveProfileAboutForm", map[string]any{"about": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if body["requestId"] != "com.linkedin.sdui.requests.profile.saveProfileAboutForm" {
		t.Fatalf("unexpected request id: %#v", body["requestId"])
	}
	args := body["requestedArguments"].(map[string]any)
	payload := args["payload"].(map[string]any)
	if payload["about"] != "hello" {
		t.Fatalf("payload not preserved: %#v", payload)
	}
	if body["isStreaming"] != false {
		t.Fatalf("server request must be non-streaming: %#v", body["isStreaming"])
	}
}
