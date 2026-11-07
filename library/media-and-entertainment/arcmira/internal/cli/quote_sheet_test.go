package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/arcmira/internal/cliutil"
)

func TestQuoteSheetSavedSearches(t *testing.T) {
	const passage = `{"id":"p1","video_title":"A conversation","channel_name":"A channel","start_seconds":0,"text":"Exact words, with punctuation!","watch_url":"https://www.youtube.com/watch?v=example&t=0s"}`
	const raw = `{"query":"video research","chunks":[` + passage + `]}`
	const overlap = `{"query":"clip discovery","chunks":[` + passage + `]}`
	const expected = `[{"id":"p1","video_title":"A conversation","channel_name":"A channel","start_seconds":0,"text":"Exact words, with punctuation!","watch_url":"https://www.youtube.com/watch?v=example&t=0s","queries":["video research"]}]`
	for _, tc := range []struct {
		name      string
		inputs    []string
		stdin     bool
		agent     bool
		want      string
		wantError string
	}{
		{name: "raw", inputs: []string{raw}, want: expected},
		{name: "wrapped", inputs: []string{`{"results":` + raw + `}`}, want: expected},
		{name: "overlapping_queries", inputs: []string{raw, overlap, raw}, want: strings.Replace(expected, `["video research"]`, `["video research","clip discovery"]`, 1)},
		{name: "conflicting_quote", inputs: []string{raw, strings.Replace(raw, "Exact words, with punctuation!", "Different quote", 1)}, wantError: "conflicting saved versions of passage p1"},
		{name: "conflicting_timestamp", inputs: []string{raw, strings.Replace(raw, `"start_seconds":0`, `"start_seconds":3`, 1)}, wantError: "conflicting saved versions of passage p1"},
		{name: "conflicting_source", inputs: []string{raw, strings.Replace(raw, "v=example", "v=other", 1)}, wantError: "conflicting saved versions of passage p1"},
		{name: "empty", inputs: []string{`{"query":"video research","chunks":[]}`}, want: `[]`},
		{name: "stdin", inputs: []string{raw}, stdin: true, want: expected},
		{name: "agent", inputs: []string{raw}, agent: true, want: expected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			restore, err := cliutil.SetHomeOverride(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(restore)
			args := []string{"quote-sheet"}
			for i, input := range tc.inputs {
				if tc.stdin {
					args = append(args, "-")
					continue
				}
				path := filepath.Join(dir, fmt.Sprintf("search-%d.json", i))
				if err := os.WriteFile(path, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, path)
			}
			args = append(args, "--home", dir, "--no-learn", "--json")
			if tc.agent {
				args = append(args, "--agent")
			}
			root := RootCmd()
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs(args)
			if tc.stdin {
				root.SetIn(strings.NewReader(tc.inputs[0]))
			}
			err = root.Execute()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error=%v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("execute: %v; stderr=%s", err, stderr.String())
			}
			var got, want any
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("decode output %q: %v", out.String(), err)
			}
			if tc.agent {
				envelope, ok := got.(map[string]any)
				if !ok {
					t.Fatalf("missing agent envelope: %#v", got)
				}
				got = envelope["results"]
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v; want %#v", got, want)
			}
		})
	}
}
