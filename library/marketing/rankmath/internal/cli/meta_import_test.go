// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

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
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
)

func TestParseMetaCSV(t *testing.T) {
	tests := []struct {
		name, input string
		want        int
		err         string
	}{
		{"valid skips empty", "id,title,description\n101,New Title,\n", 1, ""},
		{"unknown", "id,title,bogus\n1,x,y\n", 0, "bogus"},
		{"missing id", "title\nx\n", 0, "include id"},
		{"case insensitive", "ID,Title\n1,Name\n", 1, ""},
		{"duplicate id", "id,title\n42,One\n42,Two\n", 0, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := parseMetaCSV(strings.NewReader(tt.input))
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != tt.want {
				t.Fatalf("rows=%v", rows)
			}
			if _, exists := rows[0]["description"]; exists {
				t.Fatal("empty description was included")
			}
		})
	}
}

func TestMetaImportApplyRefusedInHarness(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(cliutil.VerifyEnvVar, "1")
	file := filepath.Join(t.TempDir(), "empty.csv")
	if err := os.WriteFile(file, []byte("id,title\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newMetaImportCmd(&rootFlags{asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--file", file, "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Refused bool   `json:"refused"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.Refused || result.Action != "meta import" {
		t.Fatalf("refusal result=%+v err=%v out=%s", result, err, out.String())
	}
}

func TestMetaImportPrintsPartialBatchResult(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	t.Setenv("RANKMATH_BASE_URL", "https://rankmath.test/wp-json")
	file := filepath.Join(t.TempDir(), "meta.csv")
	if err := os.WriteFile(file, []byte("id,title\n1,One\n2,Two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	oldHooks := clientHooks
	t.Cleanup(func() { clientHooks = oldHooks })
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests++
			w := httptest.NewRecorder()
			if requests == 2 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			return w.Result(), nil
		})}
		return nil
	})
	cmd := newMetaImportCmd(&rootFlags{asJSON: true, timeout: time.Second, noCache: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--file", file, "--apply", "--batch", "1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected second batch failure")
	}
	var result struct {
		Sent    int `json:"sent"`
		Batches int `json:"batches"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("partial JSON: %v: %s", err, out.String())
	}
	if result.Sent != 1 || result.Batches != 1 || requests != 2 {
		t.Fatalf("result=%+v requests=%d", result, requests)
	}
}
