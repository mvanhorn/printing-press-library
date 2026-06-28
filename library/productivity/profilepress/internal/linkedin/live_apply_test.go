package linkedin

import "testing"

func TestRequireLiveApplySupportedAllowsHeadlineAndAbout(t *testing.T) {
	changes := []SectionChange{
		{Section: "headline", After: "New headline"},
		{Section: "about", After: "New about"},
	}
	if err := RequireLiveApplySupported(changes); err != nil {
		t.Fatalf("headline/about should be supported: %v", err)
	}
}

func TestRequireLiveApplySupportedRejectsExperienceBeforeWrites(t *testing.T) {
	changes := []SectionChange{{Section: "experience", After: "new experience blob"}}
	if err := RequireLiveApplySupported(changes); err == nil {
		t.Fatal("experience blob should be rejected until position-level apply is implemented")
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
