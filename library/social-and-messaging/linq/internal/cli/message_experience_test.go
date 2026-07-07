package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestComposePreviewCommandShowsNestedBody(t *testing.T) {
	out, err := runExperienceCommand(t, "compose", "preview", "--text", "Congrats!", "--effect", "screen:confetti", "--decorate", "0:8:bold", "--preferred-service", "iMessage", "--agent")
	if err != nil {
		t.Fatalf("compose preview failed: %v\n%s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if got["sendable"] != true {
		t.Fatalf("expected sendable preview: %#v", got)
	}
	body := got["body"].(map[string]any)
	if _, exists := body["parts"]; exists {
		t.Fatalf("preview must use nested message body: %#v", body)
	}
	message := body["message"].(map[string]any)
	if message["preferred_service"] != "iMessage" {
		t.Fatalf("missing preferred_service: %#v", message)
	}
}

func TestLinkPreviewAuditBlocksFirstOutbound(t *testing.T) {
	out, err := runExperienceCommand(t, "link-preview", "audit", "https://example.com", "--first-outbound", "--agent")
	if err != nil {
		t.Fatalf("link-preview audit failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "first outbound POST /v3/chats must not contain links") {
		t.Fatalf("expected first-outbound link block, got %s", out)
	}
}

func TestReactRejectsStickerOutbound(t *testing.T) {
	out, err := runExperienceCommand(t, "react", "add", "--message-id", "msg_123", "--type", "sticker", "--agent", "--dry-run")
	if err == nil {
		t.Fatalf("expected sticker reaction to be rejected, got %s", out)
	}
	if !strings.Contains(err.Error(), "sticker is inbound-only") {
		t.Fatalf("expected inbound-only sticker error, got %v", err)
	}
}

func TestTypingPulseRejectsUnboundedDwell(t *testing.T) {
	out, err := runExperienceCommand(t, "typing", "pulse", "--chat-id", "ch_123", "--dwell", "10s", "--agent", "--dry-run")
	if err == nil {
		t.Fatalf("expected dwell cap error, got %s", out)
	}
	if !strings.Contains(err.Error(), "capped") {
		t.Fatalf("expected capped dwell error, got %v", err)
	}
}

func TestLinkMetaTagAttrsAcceptsContentFirst(t *testing.T) {
	key, value := linkMetaTagAttrs(` content="Example Title" property="og:title" `)
	if key != "og:title" || value != "Example Title" {
		t.Fatalf("content-first meta attrs parsed as key=%q value=%q", key, value)
	}
}

func TestAddMessageBuilderFlagsChainsPreRunE(t *testing.T) {
	var opts linqMessageBuildOptions
	called := false
	cmd := &cobra.Command{
		Use: "test",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			called = true
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.HasReplyToPartIndex {
				t.Fatalf("expected addMessageBuilderFlags PreRunE to run")
			}
			return nil
		},
	}
	addMessageBuilderFlags(cmd, &opts)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--reply-to-part-index", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !called {
		t.Fatalf("existing PreRunE was not called")
	}
}

func TestPutAttachmentFileStreamsWithContentLength(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "attachment-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString("hello attachment"); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.ContentLength != int64(len("hello attachment")) {
			t.Fatalf("content length = %d", r.ContentLength)
		}
		if got := r.Header.Get("X-Upload-Token"); got != "abc" {
			t.Fatalf("required header = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "hello attachment" {
			t.Fatalf("body = %q", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err = putAttachmentFile(context.Background(), server.URL, map[string]any{
		"required_headers": map[string]any{"X-Upload-Token": "abc"},
	}, tmp.Name(), "text/plain")
	if err != nil {
		t.Fatalf("putAttachmentFile failed: %v", err)
	}
}

func runExperienceCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}
