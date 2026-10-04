// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"context"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"reflect"
	"testing"
)

func TestNativeReadOnlyMirrorForcesNoLearnWithoutDisablingExplicitRecall(t *testing.T) {
	bin := writeArgvHelper(t)
	for _, tc := range []struct {
		path            []string
		readOnly, force bool
	}{{[]string{"parks", "detail"}, true, true}, {[]string{"parks", "detail"}, false, true}, {[]string{"recall"}, true, false}, {[]string{"teach"}, false, false}} {
		cmd := &cobra.Command{Use: "command <id>"}
		handler := shellOutToCLI(func() (string, error) { return bin, nil }, tc.path, map[string]bool{"args": true}, map[string]bool{"args": true, "no-learn": true}, positionalArgsForCommand(cmd, nil), tc.readOnly, nil)
		result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"args": "rvpark/1086", "no-learn": false}}})
		if err != nil || result.IsError {
			t.Fatalf("adapter err=%v result=%+v", err, result)
		}
		want := append(append([]string{}, tc.path...), "rvpark/1086")
		if tc.force {
			want = append(want, "--no-learn")
		}
		if got := decodeArgvResult(t, result); !reflect.DeepEqual(got, want) {
			t.Fatalf("argv=%#v want=%#v", got, want)
		}
	}
}
