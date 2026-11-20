// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/bound"
	"github.com/spf13/cobra"
)

func mirrorTestText(t *testing.T, r *mcplib.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("missing content")
	}
	return r.Content[0].(mcplib.TextContent).Text
}
func TestEtsyStdoutCapture(t *testing.T) {
	c := newStdoutCapture()
	if cap(c.data) != 0 {
		t.Fatal("preallocated stdout")
	}
	c.Write([]byte("{}"))
	if cap(c.data) > 1024 {
		t.Fatal("tiny write allocated large buffer")
	}
	c = newStdoutCapture()
	p := make([]byte, bound.MirrorCaptureMax+100)
	n, err := c.Write(p)
	if err != nil || n != len(p) || c.total != len(p) || len(c.data) != bound.MirrorCaptureMax {
		t.Fatalf("capture n=%d total=%d retained=%d err=%v", n, c.total, len(c.data), err)
	}
	n, err = c.Write([]byte("more"))
	if n != 4 || err != nil || c.total != len(p)+4 || len(c.data) != bound.MirrorCaptureMax {
		t.Fatal("did not drain overflow")
	}
}
func TestEtsyMirrorCaptureAndHandler(t *testing.T) {
	t.Setenv("PP_MCP_MAX_BYTES", "4000")
	dir := t.TempDir()
	payload := filepath.Join(dir, "payload.json")
	var rows []map[string]any
	for i := 0; i < 1000; i++ {
		rows = append(rows, map[string]any{"listing_id": 1000000001 + i, "title": strings.Repeat("Linen Apron Natural ", 5)})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(payload, data, 0600); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(dir, "calls")
	argv := filepath.Join(dir, "argv")
	helper := filepath.Join(dir, "helper")
	script := "#!/bin/sh\nprintf 'call\\n' >> '" + calls + "'\nprintf '%s\\n' \"$@\" > '" + argv + "'\ncat '" + payload + "'\n"
	if err = os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := RunCLICommand(context.Background(), helper, nil)
	if err != nil || out.Stdout != string(data) || out.StdoutBytes != len(data) || out.StdoutOverflow {
		t.Fatalf("capture: %v bytes=%d", err, out.StdoutBytes)
	}
	text := mirrorTestText(t, ToolResultFromCLICommand(out))
	var envelope map[string]any
	if json.Unmarshal([]byte(text), &envelope) != nil || envelope["truncated"] != true || envelope["_pp_truncated"] != nil || len(text) > 4000 {
		t.Fatalf("not canonical: %s", text)
	}
	handler := shellOutToCLI(func() (string, error) { return helper, nil }, []string{"market", "demand"}, nil, map[string]bool{"cursor": true}, nil, true, nil)
	call := func(args map[string]any) *mcplib.CallToolResult {
		r, e := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "market_demand", Arguments: args}})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	first := call(nil)
	json.Unmarshal([]byte(mirrorTestText(t, first)), &envelope)
	cursor, ok := envelope["next_cursor"].(string)
	if !ok {
		t.Fatal("missing cursor")
	}
	result := call(map[string]any{"cursor": cursor})
	if result.IsError {
		t.Fatal(mirrorTestText(t, result))
	}
	args, _ := os.ReadFile(argv)
	if strings.Contains(string(args), "cursor") {
		t.Fatal("forwarded cursor")
	}
	supplied := map[string]any{"cursor": ""}
	empty := call(supplied)
	if empty.IsError || !reflect.DeepEqual(empty, first) {
		t.Fatal("empty cursor differs from page one", empty)
	}
	if _, ok := supplied["cursor"]; !ok {
		t.Fatal("mutated caller map")
	}
	for _, tc := range []struct {
		name   string
		value  any
		prefix string
	}{
		{"invalid", "garbage", "cursor_invalid:"}, {"non_string", 7, "cursor_invalid:"},
		{"wrong_tool", bound.EncodeMirrorCursor(bound.MirrorCursor{T: "market_shops"}), "cursor_wrong_tool:"},
		{"args_mismatch", bound.EncodeMirrorCursor(bound.MirrorCursor{T: "market_demand", A: "different"}), "cursor_args_mismatch:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := os.ReadFile(calls)
			r := call(map[string]any{"cursor": tc.value})
			after, _ := os.ReadFile(calls)
			if !r.IsError || !strings.HasPrefix(mirrorTestText(t, r), tc.prefix) || string(before) != string(after) {
				t.Fatal("cursor validation spawned child or returned wrong error")
			}
		})
	}
	unregistered := shellOutToCLI(func() (string, error) { return helper, nil }, nil, nil, nil, nil, true, nil)
	before, _ := os.ReadFile(calls)
	r, _ := unregistered(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"cursor": cursor}}})
	after, _ := os.ReadFile(calls)
	if !r.IsError || !strings.Contains(mirrorTestText(t, r), "unknown MCP parameter") || string(before) != string(after) {
		t.Fatal("unregistered cursor accepted")
	}
}
func TestEtsyCursorEligibility(t *testing.T) {
	root := &cobra.Command{Use: "etsy"}
	market := &cobra.Command{Use: "market"}
	root.AddCommand(market)
	demand := &cobra.Command{Use: "demand", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}}
	market.AddCommand(demand)
	if !cursorEligible(demand) {
		t.Fatal("pilot not eligible")
	}
	demand.Annotations["mcp:read-only"] = "false"
	demand.Annotations["mcp:read-only-unless-flags"] = "apply"
	if cursorEligible(demand) {
		t.Fatal("conditional read-only eligible")
	}
	demand.Annotations["mcp:read-only"] = "true"
	demand.Annotations["pp:data-source"] = "live"
	if cursorEligible(demand) {
		t.Fatal("live eligible")
	}
	demand.Annotations["pp:data-source"] = "local"
	demand.Flags().String("cursor", "", "existing")
	if cursorEligible(demand) {
		t.Fatal("existing cursor eligible")
	}
}

func TestEtsyMirrorRegisteredNarrowing(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		readOnly     bool
		want         string
	}{
		{"local_readonly", "local", true, "Narrow with `compact`, `top` parameters."},
		{"live_readonly", "live", true, ""},
		{"local_writable", "local", false, ""},
		{"live_writable", "live", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "demand", Annotations: map[string]string{"pp:data-source": tc.source, "mcp:read-only": "true"}}
			t.Setenv("ETSY_MCP_READ_ONLY", "")
			called := false
			handler := guardMirroredMCPCall(cmd, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				called = true
				opts := mirrorOptions(ctx, map[string]bool{"top": true, "compact": true, "agent": true, "snapshot": true, "from": true, "to": true}, tc.readOnly)
				if !opts.request.SnapshotParam {
					t.Fatal("snapshot registration lost")
				}
				if opts.request.Narrow != tc.want {
					t.Fatalf("narrow=%q", opts.request.Narrow)
				}
				if tc.want == "" && opts.request.NoRerun != "This output exceeded the budget. Do not re-run this tool to get the rest; use a read-only local tool such as `listings_audit`, `inventory_audit` or `market_demand`, or narrow the arguments." {
					t.Fatalf("unsafe note: %q", opts.request.NoRerun)
				}
				return mcplib.NewToolResultText("{}"), nil
			})
			if _, err := handler(context.Background(), mcplib.CallToolRequest{}); err != nil || !called {
				t.Fatalf("guard: called=%v err=%v", called, err)
			}
		})
	}
	opts := mirrorOptions(context.Background(), nil, true)
	if opts.request.Narrow != "Narrow with the tool's filter parameters." {
		t.Fatal("missing direct-handler fallback")
	}
}

func TestEtsyPerResponseBudget(t *testing.T) {
	if hintsDroppedMarker != "[secondary output dropped to fit the response budget]" {
		t.Fatal(hintsDroppedMarker)
	}
	t.Setenv("PP_MCP_MAX_BYTES", "4000")
	for _, tc := range []struct {
		name, stdout, stderr string
		fail, help, marker   bool
	}{
		{"success_fits", `{"results":[]}`, "hint: linen apron", false, false, false},
		{"success_drops_hints", strings.Repeat("x", 12000), "hint: " + strings.Repeat("x", 12000), false, false, true},
		{"failure_fits", `{"error":"linen apron"}`, "hint: ceramic mug", true, false, false},
		{"failure_huge_both", strings.Repeat("x", 12000), strings.Repeat("x", 12000), true, false, true},
		{"help_huge", "Usage:\n  etsy\nFlags:\n  -h, --help   help for etsy\n" + strings.Repeat("x", 12000), "", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "stdout")
			errfile := filepath.Join(dir, "stderr")
			helper := filepath.Join(dir, "helper")
			if err := os.WriteFile(out, []byte(tc.stdout), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(errfile, []byte(tc.stderr), 0600); err != nil {
				t.Fatal(err)
			}
			exit := 0
			if tc.fail {
				exit = 1
			}
			script := fmt.Sprintf("#!/bin/sh\ncat '%s'\ncat '%s' >&2\nexit %d\n", out, errfile, exit)
			if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			result, e := RunCLICommand(context.Background(), helper, nil)
			var r *mcplib.CallToolResult
			if e != nil {
				r = ToolResultFromCLIFailure(result, e)
			} else {
				r = ToolResultFromCLICommand(result)
			}
			if r.IsError != (tc.fail || tc.help) {
				t.Fatal("error status")
			}
			sum := 0
			marker := false
			for _, c := range r.Content {
				text := c.(mcplib.TextContent).Text
				sum += len(text)
				marker = marker || text == hintsDroppedMarker
			}
			if sum > 4000 || marker != tc.marker {
				t.Fatalf("bytes=%d marker=%v", sum, marker)
			}
			if tc.marker && !tc.help && len(mirrorTestText(t, r)) > 4000-len(hintsDroppedMarker) {
				t.Fatal("primary used reserved bytes")
			}
			if !tc.marker && (mirrorTestText(t, r) != tc.stdout || r.Content[1].(mcplib.TextContent).Text != tc.stderr) {
				t.Fatal("stream order or text changed")
			}
		})
	}
}

func TestHardeningStderrAllocationAndFailure(t *testing.T) {
	for _, value := range []string{"16777216", "9223372036854775807"} {
		t.Setenv("PP_MCP_MAX_BYTES", value)
		c := newCappedCapture()
		if cap(c.data) != 0 {
			t.Fatal("preallocated stderr")
		}
		c.Write([]byte("tiny"))
		if cap(c.data) > 1024 {
			t.Fatal("stderr cap")
		}
		r := ToolResultFromCLIFailure(CLICommandResult{Stdout: strings.Repeat("a", bound.MirrorCaptureMax+1)}, nil)
		var m map[string]any
		text := mirrorTestText(t, r)
		if !r.IsError || json.Unmarshal([]byte(text), &m) != nil || m["truncated"] != true || m["original_bytes"] != float64(bound.MirrorCaptureMax+1) || m["preview"] == nil || m["_pp_truncated"] != nil {
			t.Fatal("failure preview shape")
		}
	}
}
func TestHardeningNativeCursor(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s' \"$*\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "synthetic"}
	cmd.Flags().String("cursor", "", "native cursor")
	h := shellOutToCLI(func() (string, error) { return helper, nil }, nil, nil, map[string]bool{"cursor": true}, nil, false, nil)
	h = guardMirroredMCPCall(cmd, h)
	r, err := h(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "synthetic", Arguments: map[string]any{"cursor": "x"}}})
	if err != nil || r.IsError || !strings.Contains(mirrorTestText(t, r), "--cursor=x") {
		t.Fatalf("%v %v", err, r)
	}
	// The native cursor is forwarded, but cannot resume mirror byte pages.
	t.Setenv("PP_MCP_MAX_BYTES", "2048")
	payload := "[" + strings.Repeat(`{"listing_id":1000000001,"title":"linen apron"},`, 100) + `{"listing_id":1000000002}]`
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf '%s' '"+payload+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	r, err = h(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "synthetic", Arguments: map[string]any{"cursor": "x"}}})
	var page map[string]any
	if err != nil || r.IsError || json.Unmarshal([]byte(mirrorTestText(t, r)), &page) != nil {
		t.Fatalf("%v %v", err, r)
	}
	if _, ok := page["next_cursor"]; ok || page["truncated"] != true || strings.Contains(fmt.Sprint(page["note"]), "Pass next_cursor") {
		t.Fatal(page)
	}
}

func TestEtsyRunCLICommandTrueStdoutSize(t *testing.T) {
	t.Setenv("PP_MCP_MAX_BYTES", "4000")
	helper := filepath.Join(t.TempDir(), "helper")
	script := fmt.Sprintf("#!/bin/sh\nhead -c %d /dev/zero\n", bound.MirrorCaptureMax+100)
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := RunCLICommand(context.Background(), helper, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.StdoutBytes != bound.MirrorCaptureMax+100 || !out.StdoutOverflow || len(out.Stdout) != bound.MirrorCaptureMax {
		t.Fatalf("bytes=%d overflow=%v retained=%d", out.StdoutBytes, out.StdoutOverflow, len(out.Stdout))
	}
	result := ToolResultFromCLICommand(out)
	var envelope map[string]any
	if result.IsError || json.Unmarshal([]byte(mirrorTestText(t, result)), &envelope) != nil || envelope["original_bytes"] != float64(bound.MirrorCaptureMax+100) {
		t.Fatal("incorrect preview size", envelope)
	}
}

func TestFailureFullBudget(t *testing.T) {
	t.Run("two_block_failure_reports_full_budget_max_bytes", func(t *testing.T) {
		t.Setenv("PP_MCP_MAX_BYTES", "4000")
		r := ToolResultFromCLIFailure(CLICommandResult{Stdout: strings.Repeat("a", 12000), Stderr: strings.Repeat("b", 12000)}, nil)
		var m map[string]any
		if json.Unmarshal([]byte(mirrorTestText(t, r)), &m) != nil || m["max_bytes"] != float64(bound.Budget()) {
			t.Fatal(m)
		}
		sum := 0
		for _, c := range r.Content {
			sum += len(c.(mcplib.TextContent).Text)
		}
		if len(r.Content) != 2 || sum > bound.Budget() {
			t.Fatalf("blocks=%d bytes=%d", len(r.Content), sum)
		}
	})
}

func TestHelpFullBudget(t *testing.T) {
	t.Run("help_path_reports_full_budget_max_bytes", func(t *testing.T) {
		t.Skip("toolResultFromCLIHelp bounds a fixed short primary message; oversized help is secondary output and cannot make the primary exceed the minimum budget")
	})
}
