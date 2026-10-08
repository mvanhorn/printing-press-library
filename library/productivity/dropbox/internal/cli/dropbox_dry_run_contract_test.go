package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/config"
)

// This guards the generated client and response helper patches across reprints.
func TestDropboxEndpointDryRunActionContract(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped during regen validation")
	}
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"files", "list-folder", "--path=", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		DryRun bool   `json:"dry_run"`
		Action string `json:"action"`
		Would  string `json:"would"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	if !result.DryRun || result.Action != "POST /files/list_folder" || result.Would != "send POST /files/list_folder to Dropbox" {
		t.Fatalf("dry-run contract = %+v", result)
	}
	if strings.Contains(out.String(), "https://") {
		t.Fatalf("action contains an absolute URL: %s", out.String())
	}
}

func TestDropboxDryRunSentinelRecognition(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped during regen validation")
	}
	for _, tc := range []struct {
		name string
		data string
		want bool
	}{
		{"legacy", `{"dry_run":true}`, true},
		{"action", `{"dry_run":true,"action":"POST /files/list_folder","would":"send POST /files/list_folder to Dropbox"}`, true},
		{"api data", `{"dry_run":true,"action":"POST /files/list_folder","entries":[]}`, false},
		{"missing action", `{"dry_run":true,"would":"send request"}`, false},
		{"empty action", `{"dry_run":true,"action":""}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDryRunResponse(true, json.RawMessage(tc.data)); got != tc.want {
				t.Fatalf("isDryRunResponse(%s) = %t, want %t", tc.data, got, tc.want)
			}
			if isDryRunResponse(false, json.RawMessage(tc.data)) {
				t.Fatal("payload treated as a plan without client dry-run mode")
			}
		})
	}
}

func TestDropboxAbsoluteEndpointDryRunOmitsHostAndAuth(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped during regen validation")
	}
	c := &client.Client{BaseURL: "https://api.dropboxapi.com/2", Config: &config.Config{AuthHeaderVal: "Bearer sample-token"}, DryRun: true}
	data, _, err := c.PostQueryWithParams(context.Background(), "https://content.dropboxapi.com/2/files/get_thumbnail_batch?access_token=sample-token", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Action string `json:"action"`
		Would  string `json:"would"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Action != "POST /files/get_thumbnail_batch" || strings.Contains(string(data), "sample-token") || strings.Contains(string(data), "https://") {
		t.Fatalf("unsafe dry-run plan: %+v", result)
	}
}
