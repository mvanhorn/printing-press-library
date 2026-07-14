package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCommentsCreateAliasesAddWithBodyAndMedia(t *testing.T) {
	bodyFile := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(bodyFile, []byte("Run `pnpm test` and keep $HOME literal.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(command string) map[string]any {
		t.Helper()
		out, err := executeRootForTest("comments", command,
			"--issue", "MOB-1293",
			"--body-file", bodyFile,
			"--media", "/tmp/one.png",
			"--media", "/tmp/two.mov",
			"--media-public",
			"--dry-run", "--agent")
		if err != nil {
			t.Fatalf("comments %s failed: %v\n%s", command, err, out)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("comments %s output is not JSON: %v\n%s", command, err, out)
		}
		return payload
	}

	canonical := run("add")
	alias := run("create")
	want := map[string]any{
		"event": "would_create_comment",
		"input": map[string]any{
			"body":    "Run `pnpm test` and keep $HOME literal.\n",
			"issueId": "MOB-1293",
		},
		"media":        []any{"/tmp/one.png", "/tmp/two.mov"},
		"media_public": true,
		"mutation":     "commentCreate",
	}
	if !reflect.DeepEqual(alias, want) {
		t.Fatalf("comments create parsed an unexpected payload:\ncreate=%#v\nwant=%#v", alias, want)
	}
	if !reflect.DeepEqual(alias, canonical) {
		t.Fatalf("comments create diverged from comments add:\ncreate=%#v\nadd=%#v", alias, canonical)
	}
}

func TestIssueReadAliasesSharePositionalReadContract(t *testing.T) {
	dbPath := issueMultiTestDB(t)
	for _, alias := range []string{"get", "view", "show"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			out, err := executeRootForTest("issues", alias, "MOB-2,MOB-1,MOB-2",
				"--data-source", "local", "--db", dbPath, "--select", "identifier", "--json")
			if err != nil {
				t.Fatalf("issues %s failed: %v\n%s", alias, err, out)
			}
			var payload struct {
				Results []struct {
					Identifier string `json:"identifier"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil {
				t.Fatalf("issues %s output is not JSON: %v\n%s", alias, err, out)
			}
			got := []string{payload.Results[0].Identifier, payload.Results[1].Identifier}
			if !reflect.DeepEqual(got, []string{"MOB-2", "MOB-1"}) {
				t.Fatalf("issues %s identifiers = %v, want caller order without duplicates", alias, got)
			}
		})
	}
}

func TestDocumentReadAliasesSharePositionalReadContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"documents":{"nodes":[{"id":"doc-1","title":"Runbook","slugId":"abc123","content":"body"}]}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	for _, alias := range []string{"get", "view"} {
		alias := alias
		t.Run(alias, func(t *testing.T) {
			out, err := executeRootForTest("documents", alias, "runbook-abc123", "--agent", "--select", "id,title,content")
			if err != nil {
				t.Fatalf("documents %s failed: %v\n%s", alias, err, out)
			}
			var payload struct {
				Results struct {
					ID      string `json:"id"`
					Content string `json:"content"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil || payload.Results.ID != "doc-1" || payload.Results.Content != "body" {
				t.Fatalf("documents %s returned unexpected output: err=%v output=%s", alias, err, out)
			}
		})
	}
}

func TestDocumentsShowRemainsAPositionalReference(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	cmd, args, err := root.Find([]string{"documents", "show", "runbook-abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.CommandPath() != "linear-pp-cli documents" {
		t.Fatalf("documents show unexpectedly resolved to %q", cmd.CommandPath())
	}
	if !reflect.DeepEqual(args, []string{"show", "runbook-abc123"}) {
		t.Fatalf("documents show args = %v, want positional document references", args)
	}
}

func TestParentReportsUnknownSubcommandInAgentMode(t *testing.T) {
	out, err := executeRootForTest("comments", "frobnicate", "--agent")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("unknown comments subcommand error = %v (code %d), want code 2; output=%s", err, ExitCode(err), out)
	}
	var payload struct {
		Error            string   `json:"error"`
		ValidSubcommands []string `json:"valid_subcommands"`
	}
	if decodeErr := json.Unmarshal([]byte(out), &payload); decodeErr != nil {
		t.Fatalf("unknown-subcommand output is not JSON: %v\n%s", decodeErr, out)
	}
	if payload.Error != `unknown subcommand "frobnicate"` {
		t.Fatalf("error does not identify the invalid subcommand: %s", out)
	}
	if len(payload.ValidSubcommands) == 0 {
		t.Fatalf("error omits valid subcommands: %s", out)
	}
}
