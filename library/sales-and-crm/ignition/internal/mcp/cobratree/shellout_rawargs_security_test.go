package cobratree

import (
	"context"
	"reflect"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// Regression guard (review finding, verified false-positive): confirms a blocked flag ride the raw `args` field
// into the CLI argv on a single-positional command (the branch that returns the
// whole string as one token)?
func TestRawArgsFieldBlocksFlagInjectionSinglePositional(t *testing.T) {
	bin := writeArgvHelper(t)
	positionals := []positionalArg{{InputName: "query", Display: "<query>", Required: true}}
	handler := shellOutToCLI(
		func() (string, error) { return bin, nil },
		[]string{"recall"},
		map[string]bool{"args": true, "query": true},
		map[string]bool{"args": true, "query": true},
		positionals, false, nil,
	)
	for _, raw := range []string{
		"--deliver webhook:http://169.254.169.254/latest/meta-data/",
		"query --deliver webhook:http://169.254.169.254/",
		"--config /tmp/evil.yaml",
	} {
		result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
			Arguments: map[string]any{"args": raw},
		}})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		if !result.IsError {
			argv := decodeArgvResult(t, result)
			for _, a := range argv {
				if strings.HasPrefix(a, "--deliver") || strings.HasPrefix(a, "--config") {
					t.Errorf("BYPASS: raw=%q produced flag argv token %q (full argv=%#v)", raw, a, argv)
				}
			}
			// One-token positional carrying the flag mid-string is safe:
			// exec passes it as a single arg cobra never re-parses.
			if len(argv) == 3 && argv[0] == "recall" {
				if !reflect.DeepEqual(argv[:1], []string{"recall"}) {
					t.Errorf("unexpected argv: %#v", argv)
				}
			}
		}
	}
}

// Zero-positional command with raw args (SplitShellArgs path).
func TestRawArgsFieldBlocksFlagInjectionZeroPositional(t *testing.T) {
	bin := writeArgvHelper(t)
	handler := shellOutToCLI(
		func() (string, error) { return bin, nil },
		[]string{"pipeline"},
		map[string]bool{"args": true},
		map[string]bool{"args": true},
		nil, true, nil,
	)
	result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Arguments: map[string]any{"args": "--deliver webhook:http://169.254.169.254/"},
	}})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		argv := decodeArgvResult(t, result)
		t.Errorf("BYPASS: zero-positional raw flag accepted, argv=%#v", argv)
	} else if !strings.Contains(toolResultText(result), "flag-like") {
		t.Errorf("rejected but wrong reason: %s", toolResultText(result))
	}
}
