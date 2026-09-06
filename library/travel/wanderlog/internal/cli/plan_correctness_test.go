// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/config"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type correctnessTransport func(*http.Request) (*http.Response, error)

func (f correctnessTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestInvalidPlanPreviewReturnsValidationError(t *testing.T) {
	for _, dry := range []bool{false, true} {
		c := client.New(&config.Config{BaseURL: "https://example.invalid"}, time.Second, 0)
		c.NoCache = true
		c.HTTPClient.Transport = correctnessTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Fatal("preview wrote")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"success":true,"tripPlan":{"itinerary":{"sections":[]}}}`))}, nil
		})
		flags := &rootFlags{asJSON: true, dryRun: dry}
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetContext(context.Background())
		err := runPlanEditWithClient(cmd, flags, c, planEditOptions{targetKey: "abcdefghijklmnop"}, "plan test", func(map[string]any) (planEditBuildResult, error) {
			return planEditBuildResult{}, errors.New("block not found")
		})
		var result map[string]any
		if json.Unmarshal(out.Bytes(), &result) != nil || err == nil || result["validation"] != "invalid" || result["applied"] != false {
			t.Fatalf("dry=%t output=%s err=%v", dry, out.String(), err)
		}
	}
}
func TestShareDBFrameErrorPreservesDiagnosis(t *testing.T) {
	err := shareDBFrameError("subscribe", map[string]any{"error": map[string]any{"code": "SCHEMA_INVALID", "message": "splitWith.users missing; cookie=secret"}}, "cookie=secret")
	var failure *shareDBResponseError
	if !errors.As(err, &failure) || failure.Retryable || !strings.Contains(err.Error(), "splitWith.users") || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	conflict := shareDBFrameError("apply", map[string]any{"code": "ERR_OP_VERSION_TOO_OLD", "message": "Version changed"}, "")
	if !errors.As(conflict, &failure) || !failure.Retryable {
		t.Fatal(conflict)
	}
	if err := shareDBFrameError("subscribe", map[string]any{"a": "s", "data": map[string]any{}}, ""); err != nil {
		t.Fatal(err)
	}
}
func TestCoherentScheduleChanges(t *testing.T) {
	old := map[string]any{"startTime": "09:00", "endTime": "10:30", "durationMinutes": 90}
	cases := []struct {
		name       string
		change     map[string]any
		start, end string
		duration   int
		bad        bool
	}{
		{"move start", map[string]any{"startTime": "11:00"}, "11:00", "12:30", 90, false},
		{"change duration", map[string]any{"durationMinutes": 45}, "09:00", "09:45", 45, false},
		{"change end", map[string]any{"endTime": "10:00"}, "09:00", "10:00", 60, false},
		{"overnight", map[string]any{"startTime": "23:30", "durationMinutes": 90}, "23:30", "01:00", 90, false},
		{"contradiction", map[string]any{"startTime": "09:00", "endTime": "10:00", "durationMinutes": 90}, "", "", 0, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ops, got, err := buildScheduleOps(old, []any{"block"}, tt.change, false)
			if tt.bad {
				if err == nil {
					t.Fatal("wanted error")
				}
				return
			}
			if err != nil || len(ops) == 0 || got["startTime"] != tt.start || got["endTime"] != tt.end || intAny(got["durationMinutes"]) != tt.duration {
				t.Fatalf("%v %v", got, err)
			}
			if old["startTime"] != "09:00" {
				t.Fatal("mutated input")
			}
		})
	}
	_, cleared, err := buildScheduleOps(old, []any{"block"}, nil, true)
	if err != nil || len(cleared) != 0 {
		t.Fatal(cleared, err)
	}
}
func TestVisitWindowContext(t *testing.T) {
	for _, s := range []string{"Open 09:00-17:00", "Hours: 9am-5pm", "Ferry 10:00-11:00", "Alternative: 12:00-13:00", "2026-09-06", "3-5 nights"} {
		if len(intendedVisitWindows(s)) != 0 {
			t.Errorf("false visit: %s", s)
		}
	}
	for _, s := range []string{"09:00-10:00 visit", "Visit: 9-10am", "Hours: 09:00-17:00\nVisit: 11:00-12:00"} {
		if len(intendedVisitWindows(s)) != 1 {
			t.Errorf("missing visit: %s", s)
		}
	}
}
func TestBusinessStatusClosure(t *testing.T) {
	for _, status := range []string{"CLOSED_PERMANENTLY", "CLOSED_TEMPORARILY"} {
		warning, closed := placeClosedOnDateWarning(map[string]any{"name": "Venue", "business_status": status}, "2026-09-07")
		if !closed || warning == "" {
			t.Fatal(status, warning)
		}
	}
}
func TestNoteAddMarkdownFlag(t *testing.T) {
	cmd := newNovelPlanNoteAddCmd(&rootFlags{})
	if cmd.Flags().Lookup("markdown") == nil {
		t.Fatal("missing markdown")
	}
	delta, _, err := blockNoteText("**Label**\n- item", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(delta)
	if !strings.Contains(string(raw), `"bold":true`) || !strings.Contains(string(raw), `"list":"bullet"`) {
		t.Fatal(string(raw))
	}
}

func TestShareDBAcknowledgementMatchesOwnOperation(t *testing.T) {
	for _, frame := range []map[string]any{
		{"a": "op", "seq": 1, "src": "other", "d": "target"},
		{"a": "op", "seq": 2, "src": "mine", "d": "target"},
		{"a": "op", "seq": 1, "src": "mine", "d": "other-target"},
	} {
		if shareDBAcknowledges(frame, "mine", "target") {
			t.Fatalf("accepted unrelated frame: %v", frame)
		}
	}
	if !shareDBAcknowledges(map[string]any{"a": "op", "seq": 1, "src": "mine", "d": "target"}, "mine", "target") {
		t.Fatal("rejected matching acknowledgement")
	}
}
func TestSyntheticMutationNeverClaimsApplied(t *testing.T) {
	if !syntheticMutationResponse(json.RawMessage(`{"__pp_verify_synthetic__":true}`)) || mutationValidation(json.RawMessage(`{"__pp_verify_synthetic__":true}`)) != "skipped" {
		t.Fatal("verify response claimed mutation")
	}
	if syntheticMutationResponse(json.RawMessage(`{"success":true}`)) {
		t.Fatal("live response misclassified")
	}
}

func TestShareDBUnknownOutcomeDoesNotResubmit(t *testing.T) {
	var connections atomic.Int32
	var operations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var frame map[string]any
		if conn.ReadJSON(&frame) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"a": "init", "id": "synthetic-session"})
		_ = conn.WriteJSON(map[string]any{"a": "hs"})
		if conn.ReadJSON(&frame) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"a": "s", "data": map[string]any{"v": 1, "data": map[string]any{"itinerary": map[string]any{"sections": []any{}}}}})
		if conn.ReadJSON(&frame) != nil {
			return
		}
		operations.Add(1)
		// The server applied the operation but the acknowledgement is lost.
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL, AuthHeaderVal: "synthetic=test"}, time.Second, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := applyPlanEditViaShareDBWithRetry(ctx, c, "abcdefghijklmnop", 2, 2, func(map[string]any) (planEditBuildResult, error) {
		return planEditBuildResult{Ops: []map[string]any{{"p": []any{"title"}, "oi": "New"}}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") || connections.Load() != 1 || operations.Load() != 1 {
		t.Fatalf("connections=%d ops=%d err=%v", connections.Load(), operations.Load(), err)
	}
}

func TestShortHelpExposesPreviewContract(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, path := range [][]string{{"plan", "edit"}, {"trips", "create"}, {"plan", "budget", "expense", "add"}} {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		writeShortHelp(&output, command)
		for _, flag := range []string{"--dry-run", "--agent", "--json"} {
			if !strings.Contains(output.String(), flag) {
				t.Errorf("%v missing %s", path, flag)
			}
		}
		if strings.Contains(output.String(), "--rate-limit") {
			t.Fatal("dumped all globals")
		}
	}
}

func TestPlanSectionsReadPreservesSectionsInAgentOutput(t *testing.T) {
	for _, compact := range []bool{false, true} {
		cmd := &cobra.Command{}
		var output bytes.Buffer
		cmd.SetOut(&output)
		report := planEditReport{Command: "plan sections", TargetKey: "abcdefghijklmnop", Sections: []planSectionReport{
			{Index: 0, ID: 100, Title: "Empty notes", BlockCount: 0},
			{Index: 1, ID: 101, Day: 1, Date: "2026-09-07", BlockCount: 2},
		}}
		if err := printPlanEditReport(cmd, &rootFlags{asJSON: true, compact: compact}, report); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Sections []planSectionReport `json:"sections"`
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Sections) != 2 || result.Sections[0].ID != 100 || result.Sections[1].Index != 1 || result.Sections[1].BlockCount != 2 {
			t.Fatalf("compact=%t lost section result: %s", compact, output.String())
		}
	}
}
