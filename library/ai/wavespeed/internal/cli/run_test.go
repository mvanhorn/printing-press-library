package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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
			]
		}
	}`)

	got := collectURLStrings(raw)
	want := []string{"https://example.com/generated.png", "https://example.com/generated.mp4"}
	if len(got) != len(want) {
		t.Fatalf("urls = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("urls = %#v", got)
		}
	}
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
