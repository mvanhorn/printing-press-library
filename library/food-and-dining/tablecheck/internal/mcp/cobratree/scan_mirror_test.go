package cobratree

import (
	"context"
	"reflect"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func TestScanMirrorExposesAndMapsAdditionalVenues(t *testing.T) {
	bin := writeArgvHelper(t)
	root := &cobra.Command{Use: "tablecheck"}
	availability := &cobra.Command{Use: "availability"}
	scan := &cobra.Command{Use: "scan <slug>...", RunE: func(*cobra.Command, []string) error { return nil }}
	scan.Flags().String("from", "", "First date")
	scan.Flags().String("to", "", "Last date")
	scan.Flags().Int("party", 0, "Party size")
	availability.AddCommand(scan)
	venue := &cobra.Command{Use: "venue"}
	venue.AddCommand(&cobra.Command{Use: "get <slug>", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(availability, venue)

	s := server.NewMCPServer("test", "0.0.0")
	RegisterAll(s, root, func() (string, error) { return bin, nil })
	tools := s.ListTools()
	scanTool, ok := tools["availability_scan"]
	if !ok {
		t.Fatalf("availability scan tool missing: %#v", tools)
	}
	for key, wantType := range map[string]string{"slug": "string", "args": "string", "from": "string", "to": "string", "party": "number"} {
		property, ok := scanTool.Tool.InputSchema.Properties[key].(map[string]any)
		if !ok || property["type"] != wantType {
			t.Fatalf("scan schema %q = %#v, want %s", key, scanTool.Tool.InputSchema.Properties[key], wantType)
		}
	}
	if !strings.Contains(strings.Join(scanTool.Tool.InputSchema.Required, ","), "slug") {
		t.Fatalf("scan schema lost required first slug: %#v", scanTool.Tool.InputSchema.Required)
	}

	call := func(input map[string]any) *mcplib.CallToolResult {
		t.Helper()
		result, err := scanTool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: input}})
		if err != nil {
			t.Fatalf("scan handler transport error: %v", err)
		}
		return result
	}
	flags := map[string]any{"from": "2026-09-30", "to": "2026-10-01", "party": float64(2), "slug": "venue-a"}
	for _, tc := range []struct {
		name  string
		extra string
		want  []string
	}{
		{"one", "", []string{"availability", "scan", "--from=2026-09-30", "--party=2", "--to=2026-10-01", "venue-a"}},
		{"three", "venue-b venue-c", []string{"availability", "scan", "--from=2026-09-30", "--party=2", "--to=2026-10-01", "venue-a", "venue-b", "venue-c"}},
		{"five", "venue-b venue-c venue-d venue-e", []string{"availability", "scan", "--from=2026-09-30", "--party=2", "--to=2026-10-01", "venue-a", "venue-b", "venue-c", "venue-d", "venue-e"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := make(map[string]any, len(flags)+1)
			for key, value := range flags {
				input[key] = value
			}
			if tc.extra != "" {
				input["args"] = tc.extra
			}
			result := call(input)
			if result.IsError {
				t.Fatalf("scan handler error: %s", toolResultText(result))
			}
			if got := decodeArgvResult(t, result); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("scan argv = %#v, want %#v", got, tc.want)
			}
		})
	}
	tooMany := make(map[string]any, len(flags)+1)
	for key, value := range flags {
		tooMany[key] = value
	}
	tooMany["args"] = "venue-b venue-c venue-d venue-e venue-f"
	if result := call(tooMany); !result.IsError || !strings.Contains(toolResultText(result), "five") {
		t.Fatalf("six venue scan should fail before shell-out: %s", toolResultText(result))
	}

	getTool, ok := tools["venue_get"]
	if !ok {
		t.Fatalf("single-positional venue get missing: %#v", tools)
	}
	if _, exposed := getTool.Tool.InputSchema.Properties["args"]; exposed {
		t.Fatalf("single-positional schema unexpectedly advertises extra args: %#v", getTool.Tool.InputSchema.Properties)
	}
	getResult, err := getTool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"slug": "venue-a"}}})
	if err != nil || getResult.IsError {
		t.Fatalf("single-positional handler changed: result=%s err=%v", toolResultText(getResult), err)
	}
	if got, want := decodeArgvResult(t, getResult), []string{"venue", "get", "venue-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("single-positional argv = %#v, want %#v", got, want)
	}
}
