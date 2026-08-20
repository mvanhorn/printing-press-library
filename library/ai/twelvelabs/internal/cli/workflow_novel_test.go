package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func executeTestCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestUploadVideoWaitPollsToReady(t *testing.T) {
	var taskPolls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/tasks":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm: %v", err)
			}
			if got := r.MultipartForm.Value["index_id"]; len(got) != 1 || got[0] != "idx" {
				t.Fatalf("index_id form value = %v, want idx", got)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"_id":"task-1","status":"processing"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/task-1":
			taskPolls++
			_, _ = w.Write([]byte(`{"_id":"task-1","status":"ready","video_id":"video-1"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("TWELVELABS_BASE_URL", server.URL)
	t.Setenv("TWELVELABS_X_API_KEY", "test-key")
	input := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(input, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := executeTestCLI(t, "--no-cache", "upload-video", "--index-id", "idx", "--file", input, "--metadata", "source=test", "--wait", "--poll-interval", "1ms", "--wait-timeout", "1s")
	if err != nil {
		t.Fatalf("upload-video returned error: %v\n%s", err, output)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("output is invalid JSON: %v\n%s", err, output)
	}
	if got["task_id"] != "task-1" || got["video_id"] != "video-1" || got["status"] != "ready" {
		t.Fatalf("unexpected output: %#v", got)
	}
	if taskPolls != 1 {
		t.Fatalf("task polls = %d, want 1", taskPolls)
	}
}

func TestUploadVideoRequiresExactlyOneInput(t *testing.T) {
	_, err := executeTestCLI(t, "upload-video", "--index-id", "idx", "--file", "a.mp4", "--url", "https://example.com/a.mp4")
	if err == nil {
		t.Fatal("expected usage error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode = %d, want 2", got)
	}
}

func TestVideoBriefCombinesAPIResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		switch r.URL.Path {
		case "/summarize":
			switch body["type"] {
			case "chapter":
				_, _ = w.Write([]byte(`{"summarize_type":"chapter","chapters":[{"start_sec":0,"end_sec":30,"chapter_title":"Intro","chapter_summary":"Setup"}]}`))
			case "highlight":
				_, _ = w.Write([]byte(`{"summarize_type":"highlight","highlights":[{"start_sec":10,"end_sec":20,"highlight":"Strong point","highlight_summary":"Useful section"}]}`))
			default:
				t.Fatalf("unexpected summarize type %v", body["type"])
			}
		case "/gist":
			_, _ = w.Write([]byte(`{"title":"Demo title","topics":["tutorial"],"hashtags":["#demo"]}`))
		case "/analyze":
			if body["stream"] != false {
				t.Fatalf("analyze stream = %v, want false", body["stream"])
			}
			_, _ = w.Write([]byte(`{"data":{"recommended_cuts":[{"start_sec":10,"end_sec":20,"clip_title":"Clip","hook":"Watch this","why_it_matters":"It lands","editing_notes":"Cut tight","caption_seed":"Caption"}]}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("TWELVELABS_BASE_URL", server.URL)
	t.Setenv("TWELVELABS_X_API_KEY", "test-key")

	output, err := executeTestCLI(t, "--no-cache", "video-brief", "--video-id", "video-1", "--format", "json")
	if err != nil {
		t.Fatalf("video-brief returned error: %v\n%s", err, output)
	}
	var got editPlan
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("output is invalid JSON: %v\n%s", err, output)
	}
	if got.VideoID != "video-1" || got.Title != "Demo title" {
		t.Fatalf("unexpected identity fields: %#v", got)
	}
	if len(got.Chapters) != 1 || got.Chapters[0].Title != "Intro" {
		t.Fatalf("chapters = %#v", got.Chapters)
	}
	if len(got.Highlights) != 1 || got.Highlights[0].Title != "Strong point" {
		t.Fatalf("highlights = %#v", got.Highlights)
	}
	if len(got.RecommendedCuts) != 1 || got.RecommendedCuts[0].ClipTitle != "Clip" {
		t.Fatalf("recommended cuts = %#v", got.RecommendedCuts)
	}
}

func TestFindKeyBoundsNestedTraversal(t *testing.T) {
	withinLimit := map[string]any{"target": "found"}
	for i := 0; i < maxJSONSearchDepth; i++ {
		withinLimit = map[string]any{"child": withinLimit}
	}
	if got, ok := findKey(withinLimit, "target"); !ok || got != "found" {
		t.Fatalf("findKey within limit = %v, %v; want found, true", got, ok)
	}

	beyondLimit := map[string]any{"target": "found"}
	for i := 0; i <= maxJSONSearchDepth; i++ {
		beyondLimit = map[string]any{"child": beyondLimit}
	}
	if got, ok := findKey(beyondLimit, "target"); ok || got != nil {
		t.Fatalf("findKey beyond limit = %v, %v; want nil, false", got, ok)
	}
}

func TestURLPathEscapeEscapesFullPathSegment(t *testing.T) {
	got := urlPathEscape("task/1?x#frag%")
	want := "task%2F1%3Fx%23frag%25"
	if got != want {
		t.Fatalf("urlPathEscape = %q, want %q", got, want)
	}
}
