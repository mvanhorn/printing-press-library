package client

import "testing"

func TestExtractMarketplaceInboxPreloader(t *testing.T) {
	t.Parallel()

	shell := []byte(`{"preloaderID":"abcLSPlatformGraphQLLightspeedRequestQueryRelayPreloader_inboxxyz","queryID":"9697184873702141","variables":{"deviceId":"device-1","requestId":0,"requestPayload":{"database":1}},"queryName":"LSPlatformGraphQLLightspeedRequestQuery"}`)
	queryID, variables, err := extractMarketplaceInboxPreloader(shell)
	if err != nil {
		t.Fatalf("extractMarketplaceInboxPreloader returned error: %v", err)
	}
	if queryID != "9697184873702141" {
		t.Fatalf("queryID = %q, want %q", queryID, "9697184873702141")
	}
	if got := variables["deviceId"]; got != "device-1" {
		t.Fatalf("deviceId = %#v, want %q", got, "device-1")
	}
}

func TestExtractMarketplaceLightspeedPayload(t *testing.T) {
	t.Parallel()

	body := []byte(`{"data":{"viewer":{"lightspeed_web_request":{"payload":"hello world"}}}}`)
	payload, err := extractMarketplaceLightspeedPayload(body)
	if err != nil {
		t.Fatalf("extractMarketplaceLightspeedPayload returned error: %v", err)
	}
	if payload != "hello world" {
		t.Fatalf("payload = %q, want %q", payload, "hello world")
	}
}

func TestParseMarketplaceInboxThreadsAndContacts(t *testing.T) {
	t.Parallel()

	payload := `deleteThenInsertThread",[19,"row-1"],[19,"ignored"],"Selling electronics? See our safety tips",[9],"https:\/\/img.example\/1.jpg",[9],[19,"2"],[19,"contact-1"],[19,"5"],[19,"12"],"Marketplace"
verifyContactRowExists",[19,"contact-1"],[19,"1"],"https:\/\/img.example\/contact.jpg",[19,"user"],0,"Michael",`

	threads := parseMarketplaceInboxThreads(payload)
	if len(threads) != 1 {
		t.Fatalf("thread count = %d, want 1", len(threads))
	}
	thread := threads[0]
	if thread.ThreadKey != "contact-1" {
		t.Fatalf("thread.ThreadKey = %q, want %q", thread.ThreadKey, "contact-1")
	}
	if thread.Snippet != "Selling electronics? See our safety tips" {
		t.Fatalf("thread.Snippet = %q", thread.Snippet)
	}
	if thread.ImageURL != "https://img.example/1.jpg" {
		t.Fatalf("thread.ImageURL = %q", thread.ImageURL)
	}
	if thread.AuthorityLevel != 2 || thread.FolderCode != 5 || thread.ThreadType != 12 {
		t.Fatalf("unexpected numeric fields: %+v", thread)
	}

	contacts := parseMarketplaceInboxContacts(payload)
	contact, ok := contacts["contact-1"]
	if !ok {
		t.Fatalf("expected contact-1 to be present")
	}
	if contact.Name != "Michael" {
		t.Fatalf("contact.Name = %q, want %q", contact.Name, "Michael")
	}
	if contact.ImageURL != "https://img.example/contact.jpg" {
		t.Fatalf("contact.ImageURL = %q", contact.ImageURL)
	}
}
