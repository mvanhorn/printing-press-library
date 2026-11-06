// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestVideoEditOperationsAcceptsEveryBatchType(t *testing.T) {
	wantTypes := []string{
		"add_blur", "add_highlight", "add_layout", "add_media_overlay",
		"add_shape_overlay", "add_sound_effect", "add_text_overlay", "add_zoom",
		"remove_blur", "remove_highlight", "remove_layout", "remove_media_overlay",
		"remove_shape_overlay", "remove_sound_effect", "remove_text_overlay", "remove_zoom",
		"update_blur", "update_highlight", "update_layout", "update_media_overlay",
		"update_shape_overlay", "update_sound_effect", "update_text_overlay", "update_zoom",
	}
	for _, typeName := range wantTypes {
		t.Run(typeName, func(t *testing.T) {
			raw := fmt.Sprintf(`[{"type":%q,"operationId":"op-1","clipId":"cl_one"}]`, typeName)
			operations, err := videoEditOperations(raw, false)
			if err != nil || len(operations) != 1 || operations[0]["type"] != typeName {
				t.Fatalf("operations = %#v, err = %v", operations, err)
			}
		})
	}
	_, err := videoEditOperations(`[{"type":"unknown","operationId":"op-1"}]`, false)
	wantError := `operation 0 has unsupported type "unknown"; allowed types: ` + strings.Join(wantTypes, ", ") + "; apply_video_edits cannot cut clips"
	if err == nil || err.Error() != wantError {
		t.Fatalf("allowed types error = %v, want %s", err, wantError)
	}
}

func TestVideoEditOperationsBatchLimits(t *testing.T) {
	for _, count := range []int{0, 1, 200, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			operations := make([]map[string]any, count)
			for i := range operations {
				operations[i] = map[string]any{"type": "remove_layout", "operationId": fmt.Sprint(i)}
			}
			raw, err := json.Marshal(operations)
			if err != nil {
				t.Fatal(err)
			}
			got, err := videoEditOperations(string(raw), false)
			if count == 0 || count == 201 {
				if err == nil || err.Error() != "operations must contain 1 to 200 items" {
					t.Fatalf("batch limit error = %v", err)
				}
			} else if err != nil || len(got) != count {
				t.Fatalf("accepted batch length = %d, err = %v", len(got), err)
			}
		})
	}
}

func TestVideosApplyEditsTypesHTTP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typeName string
		apply    bool
		wantErr  bool
	}{
		{name: "unknown_rejected_before_HTTP", typeName: "unknown", apply: true, wantErr: true},
		{name: "update_text_overlay_preview", typeName: "update_text_overlay"},
		{name: "update_text_overlay_apply", typeName: "update_text_overlay", apply: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan fixtureRequest, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.RawQuery != "" {
					t.Errorf("unexpected query = %q", request.URL.RawQuery)
				}
				requests <- fixtureRequest{Method: request.Method, Path: request.URL.Path, Body: decodeFixtureBody(t, request)}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"results":[{"operationId":"text-1","type":"update_text_overlay"}]}`)
			}))
			defer server.Close()
			t.Setenv("TELLA_BASE_URL", server.URL)
			t.Setenv("TELLA_API_KEY", "fixture")
			t.Setenv("TELLA_SESSION_COOKIE", "")
			flags := &rootFlags{noCache: true, timeout: 2 * time.Second, configPath: filepath.Join(t.TempDir(), "missing.toml")}
			cmd := newVideosApplyEditsCmd(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			raw := fmt.Sprintf(`[{"type":%q,"operationId":"text-1","clipId":"cl_one","textOverlayId":"text_one","text":"Updated text"}]`, tc.typeName)
			args := []string{"vid_one", "--operations", raw}
			if tc.apply {
				args = append(args, "--apply")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if tc.wantErr {
				var cliErr *cliError
				if !errors.As(err, &cliErr) || cliErr.code != 2 || !strings.Contains(err.Error(), `unsupported type "unknown"`) {
					t.Fatalf("usage error = %v", err)
				}
				if len(requests) != 0 {
					t.Fatalf("invalid operation sent %d requests", len(requests))
				}
				return
			}
			if err != nil {
				t.Fatalf("apply-edits: %v\n%s", err, output.String())
			}
			var operations []any
			if err := json.Unmarshal([]byte(raw), &operations); err != nil {
				t.Fatal(err)
			}
			wantBody := map[string]any{"operations": operations}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("output = %s: %v", output.String(), err)
			}
			if result["video_id"] != "vid_one" || result["applied"] != tc.apply || result["dry_run"] != !tc.apply || !reflect.DeepEqual(result["body"], wantBody) {
				t.Fatalf("output = %#v", result)
			}
			if !tc.apply {
				if len(requests) != 0 {
					t.Fatalf("preview sent %d requests", len(requests))
				}
				return
			}
			if len(requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(requests))
			}
			request := <-requests
			if request.Method != http.MethodPost || request.Path != "/v1/videos/vid_one/edits" || !reflect.DeepEqual(request.Body, wantBody) {
				t.Fatalf("request = %#v", request)
			}
			wantData := map[string]any{"results": []any{map[string]any{"operationId": "text-1", "type": "update_text_overlay"}}}
			if result["status"] != float64(http.StatusOK) || !reflect.DeepEqual(result["data"], wantData) {
				t.Fatalf("response output = %#v", result)
			}
		})
	}
}
