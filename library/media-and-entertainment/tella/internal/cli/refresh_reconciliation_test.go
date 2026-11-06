// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshLearningDisabledOutput(t *testing.T) {
	for _, command := range [][]string{{"teach"}, {"teach-playbook"}, {"playbook", "amend"}} {
		for _, mode := range []string{"human", "json", "agent", "quiet", "dry-run"} {
			t.Run(strings.Join(command, " ")+"/"+mode, func(t *testing.T) {
				home := withTempLearnHome(t)
				db := filepath.Join(home, "unused.db")
				args := append([]string{}, command...)
				args = append(args, "--no-learn", "--db", db)
				switch mode {
				case "json":
					args = append(args, "--json")
				case "agent":
					args = append(args, "--agent")
				case "quiet":
					args = append(args, "--json", "--quiet")
				case "dry-run":
					args = append(args, "--json", "--dry-run")
				}
				stdout, stderr, err := runRootArgs(t, args...)
				if err != nil || stderr != "" {
					t.Fatalf("err=%v stderr=%q", err, stderr)
				}
				if mode == "human" || mode == "quiet" {
					if stdout != "" {
						t.Fatalf("expected silence, got %q", stdout)
					}
				} else {
					var result map[string]any
					if mode == "agent" {
						unmarshalAgentResults(t, stdout, &result)
					} else if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err, stdout)
					}
					key := "disabled"
					if mode == "dry-run" {
						key = "dry_run"
					}
					if result[key] != true {
						t.Fatalf("missing %s: %s", key, stdout)
					}
				}
				if _, err := os.Stat(db); !os.IsNotExist(err) {
					t.Fatalf("disabled learning created DB: %v", err)
				}
			})
		}
	}
}

func TestRefreshTailOutputAndDryRun(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.URL.Path != "/v1/backgrounds" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"one"},{"id":"two"}]}`)
	}))
	defer server.Close()
	for _, mode := range []string{"json", "ndjson", "dry-run"} {
		t.Run(mode, func(t *testing.T) {
			flags := transcriptReadFlags(t, server)
			flags.asJSON = mode != "ndjson"
			flags.dryRun = mode == "dry-run"
			before := requests
			stdout, err := runLessonCommand(newTailCmd(flags), "backgrounds", "--follow=false")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "ndjson" {
				lines := strings.Split(strings.TrimSpace(stdout), "\n")
				if len(lines) != 2 {
					t.Fatalf("expected two captured events: %q", stdout)
				}
				for _, line := range lines {
					var event map[string]any
					if err := json.Unmarshal([]byte(line), &event); err != nil || event["event"] != "data" {
						t.Fatalf("invalid event %q: %v", line, err)
					}
				}
			} else {
				var result map[string]any
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatal(err, stdout)
				}
				if mode == "dry-run" {
					if result["dry_run"] != true || requests != before {
						t.Fatalf("dry-run performed HTTP or omitted intent: %s", stdout)
					}
				} else if data, ok := result["data"].([]any); !ok || len(data) != 2 {
					t.Fatalf("lost list envelope: %s", stdout)
				}
			}
			if mode != "dry-run" && requests != before+1 {
				t.Fatalf("single poll made %d requests", requests-before)
			}
		})
	}
}
