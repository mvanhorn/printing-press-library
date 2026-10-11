package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInboxSinceUsesV33MessagesEndpoint(t *testing.T) {
	var gotCreatedAt string
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/api/v3.3/messages" {
			t.Errorf("path = %q, want /api/v3.3/messages", r.URL.Path)
		}
		if got := r.URL.Query().Get("agencyUid"); got != "agency-1" {
			t.Errorf("agencyUid = %q, want agency-1", got)
		}
		gotCreatedAt = r.URL.Query().Get("createdAt")
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("_cursor") == "next-page" {
			_, _ = io.WriteString(w, `{"messages":[{"uid":"message-2","content":{"text":"two"}}],"_paging":{"_limit":25,"_nextCursor":null}}`)
			return
		}
		_, _ = io.WriteString(w, `{"messages":[{"uid":"message-1","content":{"text":"one"}}],"_paging":{"_limit":25,"_nextCursor":"next-page"}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOSTFULLY_BASE_URL", server.URL+"/api/v3")
	t.Setenv("HOSTFULLY_API_KEY", "test-key")
	t.Setenv("HOSTFULLY_AGENCY_ID", "agency-1")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"inbox", "--since", "10m", "--json", "--no-cache"})
	before := time.Now().UTC().Add(-10 * time.Minute)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	after := time.Now().UTC().Add(-10 * time.Minute)
	parsed, err := time.Parse(time.RFC3339, gotCreatedAt)
	if err != nil {
		t.Fatalf("createdAt %q is not RFC3339: %v", gotCreatedAt, err)
	}
	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("createdAt = %s, want approximately ten minutes ago (%s..%s)", parsed, before, after)
	}
	if requestCount != 2 {
		t.Errorf("request count = %d, want 2 so --since follows pagination", requestCount)
	}
}

func TestInboxReplyResolvesLeadAndPostsMessage(t *testing.T) {
	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3.3/threads/thread-1":
			_, _ = io.WriteString(w, `{"thread":{"uid":"thread-1","participants":[{"participantType":"LEAD","participantUid":"lead-1"}]}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3.3/messages":
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Errorf("decode post body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"message":{"uid":"message-1","status":"CREATED"}}`)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOSTFULLY_BASE_URL", server.URL+"/api/v3")
	t.Setenv("HOSTFULLY_API_KEY", "test-key")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"inbox", "reply", "thread-1", "Your", "room", "is", "ready.", "--yes", "--json", "--no-cache"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := posted["leadUid"]; got != "lead-1" {
		t.Errorf("leadUid = %#v, want lead-1", got)
	}
	if got := posted["threadUid"]; got != "thread-1" {
		t.Errorf("threadUid = %#v, want thread-1", got)
	}
	if got := posted["type"]; got != "DIRECT_MESSAGE" {
		t.Errorf("type = %#v, want DIRECT_MESSAGE", got)
	}
	content, _ := posted["content"].(map[string]any)
	if got := content["text"]; got != "Your room is ready." {
		t.Errorf("content.text = %#v, want complete reply", got)
	}
}

func TestInboxReplyRequiresConfirmation(t *testing.T) {
	t.Setenv("HOSTFULLY_API_KEY", "test-key")
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"inbox", "reply", "thread-1", "hello", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want confirmation error")
	}
}

func TestInboxCreatedAtSince(t *testing.T) {
	now := time.Date(2026, time.August, 28, 22, 0, 0, 0, time.FixedZone("PDT", -7*60*60))
	got, err := inboxCreatedAtSince("10m", now)
	if err != nil {
		t.Fatalf("inboxCreatedAtSince() error = %v", err)
	}
	if want := "2026-08-29T04:50:00Z"; got != want {
		t.Errorf("inboxCreatedAtSince() = %q, want %q", got, want)
	}
	if _, err := inboxCreatedAtSince("yesterday", now); err == nil {
		t.Fatal("inboxCreatedAtSince(yesterday) error = nil, want usage error")
	}
}

func TestThreadReadStateUsesAgencyParticipant(t *testing.T) {
	thread := inboxThread{
		Participants: []inboxParticipant{
			{ParticipantType: "AGENCY", ParticipantUID: "agency-participant"},
			{ParticipantType: "LEAD", ParticipantUID: "lead-participant"},
		},
		ParticipantsReadStatuses: []inboxReadStatus{
			{ParticipantUID: "agency-participant", ReadStatus: "READ"},
			{ParticipantUID: "lead-participant", ReadStatus: "UNREAD"},
		},
	}
	if got := threadReadState(thread); got != "read" {
		t.Errorf("threadReadState() = %q, want read; guest unread status must not mark the agency inbox unread", got)
	}
}
