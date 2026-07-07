package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRunInputsMergesInDocumentedOrder(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputFile, []byte(`{"prompt":"file","steps":1,"file_only":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := runCommandOptions{
		inputFile: inputFile,
		input:     `{"prompt":"inline","steps":2,"inline_only":"yes"}`,
		inputKV:   []string{"steps=3", "guidance=7.5", "flag=false", `nested={"a":1}`},
		prompt:    "  keep my prompt exactly  ",
	}
	defaults := map[string]any{
		"prompt":     "alias",
		"alias_only": "kept",
		"steps":      0,
	}

	got, err := readRunInputs(opts, defaults, map[string]bool{
		"input":      true,
		"input-file": true,
		"prompt":     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got["prompt"] != "  keep my prompt exactly  " {
		t.Fatalf("prompt = %#v", got["prompt"])
	}
	if got["steps"] != int64(3) {
		t.Fatalf("steps = %#v", got["steps"])
	}
	if got["guidance"] != 7.5 {
		t.Fatalf("guidance = %#v", got["guidance"])
	}
	if got["flag"] != false {
		t.Fatalf("flag = %#v", got["flag"])
	}
	if got["alias_only"] != "kept" || got["file_only"] != true || got["inline_only"] != "yes" {
		t.Fatalf("merged inputs = %#v", got)
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok || nested["a"] != int64(1) {
		t.Fatalf("nested = %#v", got["nested"])
	}
}

func TestReadRunInputsCombinesSetAndInputKV(t *testing.T) {
	opts := runCommandOptions{
		inputKV: []string{"one=1"},
		setKV:   []string{"two=2"},
	}

	got, err := readRunInputs(opts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["one"] != int64(1) || got["two"] != int64(2) {
		t.Fatalf("inputs = %#v", got)
	}
}

func TestResolveProjectModel(t *testing.T) {
	project := wavespeedProjectConfig{
		DefaultModel: "wavespeed-ai/default",
		Aliases: map[string]wavespeedProjectAlias{
			"hero": {
				Model: "wavespeed-ai/hero",
				Input: map[string]any{"prompt": "hero prompt"},
			},
		},
	}

	model, defaults, err := resolveProjectModel(project, "hero")
	if err != nil {
		t.Fatal(err)
	}
	if model != "wavespeed-ai/hero" || defaults["prompt"] != "hero prompt" {
		t.Fatalf("alias resolved to %q %#v", model, defaults)
	}

	model, defaults, err = resolveProjectModel(project, "wavespeed-ai/direct")
	if err != nil {
		t.Fatal(err)
	}
	if model != "wavespeed-ai/direct" || defaults != nil {
		t.Fatalf("direct model resolved to %q %#v", model, defaults)
	}
}

func TestRequestSchemaForModel(t *testing.T) {
	models := json.RawMessage(`{
		"data": [
			{
				"model_id": "wavespeed-ai/example",
				"api_schema": {
					"api_schemas": [
						{"request_schema": {"type": "object", "properties": {"prompt": {"type": "string"}}}}
					]
				}
			}
		]
	}`)

	raw, err := requestSchemaForModel(models, "wavespeed-ai/example")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["type"] != "object" {
		t.Fatalf("schema = %#v", schema)
	}
}

func TestModelHelpShowsResolutionEnumAndPricing(t *testing.T) {
	models := json.RawMessage(`{"data":[{"model_id":"google/nano-banana-pro/edit","name":"Nano Banana Pro Edit","type":"image-to-image","base_price":0.04,"formula":"base_price * resolution_multiplier","api_schema":{"api_schemas":[{"request_schema":{"type":"object","required":["prompt","images"],"properties":{"prompt":{"type":"string"},"images":{"type":"array","items":{"type":"string"}},"resolution":{"type":"string","enum":["1k","2k","4k"],"default":"2k"}}}}]}}]}`)
	model, ok := findModelObject(models, "google/nano-banana-pro/edit")
	if !ok {
		t.Fatal("model not found")
	}
	text := modelHelpText("google/nano-banana-pro/edit", model)
	for _, want := range []string{"price: 0.04", "resolution: 1k | 2k | 4k", "enum=1k|2k|4k"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help text missing %q:\n%s", want, text)
		}
	}
}

func TestSummarizeModelsForCapabilityReturnsModelIDAndPrice(t *testing.T) {
	models := json.RawMessage(`{"data":[{"model_id":"cheap/edit","type":"image-to-image","base_price":0.01,"api_schema":{"api_schemas":[{"request_schema":{"type":"object","properties":{"images":{"type":"array","items":{"type":"string"}},"size":{"type":"string","enum":["1024x1024","1792x1024"]}}}}]}},{"model_id":"video/t2v","type":"text-to-video","base_price":1.25}]}`)
	raw, err := summarizeModelsForCapability(models, "image-edit")
	if err != nil {
		t.Fatal(err)
	}
	var got []modelCatalogSummary
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ModelID != "cheap/edit" || got[0].Price != "0.01" {
		t.Fatalf("summary = %#v", got)
	}
	if got[0].Resolutions["size"][1] != "1792x1024" {
		t.Fatalf("resolutions = %#v", got[0].Resolutions)
	}
}

func TestDownloadOutputPath(t *testing.T) {
	if got := downloadOutputPath("./out/{index}.{ext}", "https://example.com/a/photo.png", 0, 2); got != "out/1.png" {
		t.Fatalf("templated path = %q", got)
	}
	if got := downloadOutputPath("./out/final.png", "https://example.com/a/photo.png", 1, 2); got != "out/final-2.png" {
		t.Fatalf("multi exact path = %q", got)
	}
	if got := downloadOutputPath("./out/", "https://example.com/a/photo.png", 0, 1); got != "out/photo.png" {
		t.Fatalf("directory path = %q", got)
	}
}

func TestCollectURLStringsSkipsEchoedInputs(t *testing.T) {
	raw := json.RawMessage(`{
		"data": {
			"inputs": {
				"image": "https://example.com/uploaded-input.png"
			},
			"outputs": [
				"https://example.com/generated.png",
				{"video": "https://example.com/generated.mp4"}
			],
			"urls": {
				"get": "https://api.wavespeed.ai/api/v3/predictions/pred-123/result",
				"cancel": "https://api.wavespeed.ai/api/v3/predictions/pred-123/cancel",
				"result": "https://cdn.example.com/from-result-key.mp4"
			}
		}
	}`)

	got := collectURLStrings(raw)
	want := []string{"https://example.com/generated.png", "https://example.com/generated.mp4", "https://cdn.example.com/from-result-key.mp4"}
	if len(got) != len(want) {
		t.Fatalf("urls = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("urls = %#v", got)
		}
	}
}

func TestRunWaitDownloadPrintsResultAndDownloadsCDNWithoutAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	outPath := filepath.Join(t.TempDir(), "generated.png")

	var cdnAuth string
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cdnAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer cdn.Close()

	var api *httptest.Server
	var postAuth string
	var resultAuth string
	resultRequests := 0
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/google/nano-banana-2/edit":
			postAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"data":{"id":"pred-123","status":"created"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/predictions/pred-123/result":
			resultRequests++
			resultAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"data":{"id":"pred-123","status":"completed","outputs":["` + cdn.URL + `/generated.png"],"urls":{"get":"` + api.URL + `/predictions/pred-123/result"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	stdout, stderr, err := executeRootForTest(t, []string{
		"--json",
		"--timeout", "5s",
		"run",
		"google/nano-banana-2/edit",
		"-p", "edit this",
		"--images", "https://example.com/input.png",
		"-i", "aspect_ratio=1:1",
		"--wait",
		"--poll-interval", "1ms",
		"--download", outPath,
	}, api.URL)
	if err != nil {
		t.Fatalf("run returned error: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, `"status": "completed"`) || !strings.Contains(stdout, cdn.URL+`/generated.png`) {
		t.Fatalf("stdout did not preserve completed result: %s", stdout)
	}
	if !strings.Contains(stdout, `"downloads"`) || !strings.Contains(stdout, `"path": "`+outPath+`"`) {
		t.Fatalf("stdout did not include planned download mapping: %s", stdout)
	}
	if !strings.Contains(stderr, "downloaded "+outPath) {
		t.Fatalf("stderr missing download confirmation: %s", stderr)
	}
	if cdnAuth != "" {
		t.Fatalf("cdn download received auth header %q", cdnAuth)
	}
	if postAuth != "Bearer test-key" || resultAuth != "Bearer test-key" {
		t.Fatalf("api auth headers post=%q result=%q", postAuth, resultAuth)
	}
	if resultRequests != 1 {
		t.Fatalf("prediction result endpoint was called %d times, want only polling call", resultRequests)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("png-bytes")) {
		t.Fatalf("downloaded file = %q", string(got))
	}
}

func TestRunDownloadFailureWarnsAfterPrintingCompletedResult(t *testing.T) {
	t.Chdir(t.TempDir())
	outPath := filepath.Join(t.TempDir(), "generated.png")

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer cdn.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/google/nano-banana-2/edit":
			_, _ = w.Write([]byte(`{"data":{"id":"pred-456","status":"created"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/predictions/pred-456/result":
			_, _ = w.Write([]byte(`{"data":{"id":"pred-456","status":"completed","outputs":["` + cdn.URL + `/generated.png"]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	stdout, stderr, err := executeRootForTest(t, []string{
		"--json",
		"run",
		"--model-id", "google/nano-banana-2/edit",
		"--prompt", "edit this",
		"--images", "https://example.com/input.png",
		"--set", "aspect_ratio=1:1",
		"--wait",
		"--poll-interval", "1ms",
		"--download", outPath,
	}, api.URL)
	if err != nil {
		t.Fatalf("download failure should be non-fatal, got %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, `"status": "completed"`) || !strings.Contains(stdout, cdn.URL+`/generated.png`) {
		t.Fatalf("stdout did not include completed result before warning: %s", stdout)
	}
	if !strings.Contains(stdout, `"downloads"`) || !strings.Contains(stdout, `"path": "`+outPath+`"`) {
		t.Fatalf("stdout did not include planned download mapping before warning: %s", stdout)
	}
	if !strings.Contains(stderr, "warning: download failed: downloading "+cdn.URL+"/generated.png returned HTTP 401") {
		t.Fatalf("stderr missing download warning: %s", stderr)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("download path exists after failed download: err=%v", err)
	}
}

func TestRunDownloadFailureReportsPartialSuccesses(t *testing.T) {
	t.Chdir(t.TempDir())
	outTemplate := filepath.Join(t.TempDir(), "{index}.{ext}")

	okCDN := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first"))
	}))
	defer okCDN.Close()

	failCDN := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer failCDN.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/google/nano-banana-2/edit":
			_, _ = w.Write([]byte(`{"data":{"id":"pred-789","status":"created"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/predictions/pred-789/result":
			_, _ = w.Write([]byte(`{"data":{"id":"pred-789","status":"completed","outputs":["` + okCDN.URL + `/first.png","` + failCDN.URL + `/second.png"]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	stdout, stderr, err := executeRootForTest(t, []string{
		"--json",
		"run",
		"google/nano-banana-2/edit",
		"--prompt", "edit this",
		"--wait",
		"--poll-interval", "1ms",
		"--download", outTemplate,
	}, api.URL)
	if err != nil {
		t.Fatalf("partial download failure should be non-fatal, got %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	firstPath := filepath.Join(filepath.Dir(outTemplate), "1.png")
	secondPath := filepath.Join(filepath.Dir(outTemplate), "2.png")
	if !strings.Contains(stdout, firstPath) || !strings.Contains(stdout, secondPath) {
		t.Fatalf("stdout missing planned download paths: %s", stdout)
	}
	if !strings.Contains(stderr, "downloaded "+firstPath) {
		t.Fatalf("stderr missing partial success confirmation: %s", stderr)
	}
	if !strings.Contains(stderr, "warning: download failed: downloading "+failCDN.URL+"/second.png returned HTTP 401") {
		t.Fatalf("stderr missing partial failure warning: %s", stderr)
	}
	if got, err := os.ReadFile(firstPath); err != nil || string(got) != "first" {
		t.Fatalf("first download = %q, %v", string(got), err)
	}
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Fatalf("second download path exists after failed download: err=%v", err)
	}
}

func executeRootForTest(t *testing.T, args []string, baseURL string) (string, string, error) {
	t.Helper()
	t.Setenv("WAVESPEED_API_KEY", "test-key")
	t.Setenv("WAVESPEED_BASE_URL", baseURL)
	t.Setenv("WAVESPEED_CONFIG", filepath.Join(t.TempDir(), "missing-config.toml"))

	cmd := RootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestReadRunInputsMediaConvenienceFlags(t *testing.T) {
	opts := runCommandOptions{
		prompt:    "animate this",
		image:     "@start.png",
		images:    []string{"@a.png", "@b.png"},
		refImages: []string{"@ref.png"},
		syncMode:  true,
	}

	got, err := readRunInputs(opts, nil, map[string]bool{
		"prompt":          true,
		"image":           true,
		"images":          true,
		"reference-image": true,
		"sync":            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["prompt"] != "animate this" || got["image"] != "@start.png" || got["enable_sync_mode"] != true {
		t.Fatalf("inputs = %#v", got)
	}
	images, ok := got["images"].([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("images = %#v", got["images"])
	}
	refs, ok := got["reference_images"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("reference_images = %#v", got["reference_images"])
	}
}

func TestParseInputKVFileRefCSV(t *testing.T) {
	key, value, err := parseInputKV("images=@a.png,@b.png")
	if err != nil {
		t.Fatal(err)
	}
	if key != "images" {
		t.Fatalf("key = %q", key)
	}
	items, ok := value.([]any)
	if !ok || len(items) != 2 || items[0] != "@a.png" || items[1] != "@b.png" {
		t.Fatalf("value = %#v", value)
	}
}
