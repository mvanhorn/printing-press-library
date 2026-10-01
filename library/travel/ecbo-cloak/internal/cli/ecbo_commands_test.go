package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEcboExplicitTaskHomeConfinesCache(t *testing.T) {
	envCache := filepath.Join(t.TempDir(), "env-cache")
	taskRoot := t.TempDir()
	t.Setenv("ECBO_CLOAK_CACHE_DIR", envCache)
	flags := &rootFlags{homePath: taskRoot}
	if got := ecboCommandCache("", flags); got != filepath.Join(taskRoot, "cache", "ecbo-cloak-cli") {
		t.Fatal(got)
	}
	if got := ecboCommandCache("", &rootFlags{}); got != envCache {
		t.Fatal(got)
	}
	explicit := filepath.Join(taskRoot, "chosen")
	if got := ecboCommandCache(explicit, flags); got != explicit {
		t.Fatal(got)
	}
	if got := ecboCommandCache("", &rootFlags{homePath: " " + taskRoot + " "}); got != filepath.Join(taskRoot, "cache", "ecbo-cloak-cli") {
		t.Fatal("padded task home escaped", got)
	}
	if got := ecboCommandCache("", &rootFlags{homePath: "   "}); got != envCache {
		t.Fatal("empty normalized home should use environment", got)
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	if got := ecboCommandCache("", &rootFlags{homePath: "~"}); got != filepath.Join(home, "cache", "ecbo-cloak-cli") {
		t.Fatal(got)
	}
}

func TestEcboFixtureHomeOverrideIsAbsolute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory")
	t.Setenv("ECBO_CLOAK_DOGFOOD_HOME", path)
	if ecboFixtureHome() != path {
		t.Fatal(ecboFixtureHome())
	}
	t.Setenv("ECBO_CLOAK_DOGFOOD_HOME", "relative")
	if ecboFixtureHome() != ".printing-press-fixtures/inventory" {
		t.Fatal("relative override accepted")
	}
}

func TestEcboRawDryRunIsOneJSONRequestPreview(t *testing.T) {
	for _, command := range [][]string{{"source", "detail", "--id", "0c3fb1e9-ad5a-42de-bfda-3027ebe4921e"}, {"source", "nearby", "--latitude", "35.6812", "--longitude", "139.7671"}, {"source", "price", "--space-id", "0c3fb1e9-ad5a-42de-bfda-3027ebe4921e", "--from", "2026-10-03 19:00", "--to", "2026-10-03 21:00", "--reservation-items-large", "1"}} {
		root := RootCmd()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(append(command, "--dry-run", "--agent"))
		if e := root.Execute(); e != nil {
			t.Fatal(e)
		}
		var v map[string]any
		if e := json.Unmarshal(output.Bytes(), &v); e != nil {
			t.Fatal("mixed stdout", output.String(), e)
		}
		if command[1] == "nearby" {
			request := v["items"].([]any)[0].(map[string]any)
			if request["url"] != "https://search.ecbo.io/api/v1/spaces" {
				t.Fatal("incorrect request preview URL", request)
			}
		}
		if v["dry_run"] != true || len(v["items"].([]any)) != 1 || v["kind"] != "request_preview" {
			t.Fatal(v)
		}
	}
}
