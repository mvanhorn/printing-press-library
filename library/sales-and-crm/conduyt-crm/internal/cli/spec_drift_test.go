package cli

// (Conduyt audit CND-020) The printed CLI claims every documented public endpoint. This gate fails when the app's
// public OpenAPI document (upstream/openapi.yaml, refreshed by `make sync-spec` from https://conduyt.app/openapi.yaml)
// carries operations that spec.yaml — the print's input — does not, or vice versa. A failing gate means: refresh
// spec.yaml from upstream (keeping the x-mcp block and the x-pp-resource overrides) and reprint.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	pathLine   = regexp.MustCompile(`^  (/[^:]*):\s*$`)
	methodLine = regexp.MustCompile(`^    (get|post|put|patch|delete|head|options):\s*$`)
)

func operationSet(t *testing.T, file string) map[string]bool {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("open %s: %v", file, err)
	}
	defer f.Close()
	ops := map[string]bool{}
	path := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := pathLine.FindStringSubmatch(line); m != nil {
			path = m[1]
			continue
		}
		if m := methodLine.FindStringSubmatch(line); m != nil && path != "" {
			ops[strings.ToUpper(m[1])+" "+path] = true
		}
	}
	return ops
}

func TestSpecMatchesUpstreamPublicAPI(t *testing.T) {
	root := filepath.Join("..", "..")
	spec := operationSet(t, filepath.Join(root, "spec.yaml"))
	upstream := operationSet(t, filepath.Join(root, "upstream", "openapi.yaml"))
	if len(upstream) < 500 {
		t.Fatalf("upstream/openapi.yaml looks truncated: %d operations", len(upstream))
	}
	var missing, extra []string
	for op := range upstream {
		if !spec[op] {
			missing = append(missing, op)
		}
	}
	for op := range spec {
		if !upstream[op] {
			extra = append(extra, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("spec.yaml drifted from the public API (%d operations upstream, %d in spec):\n  missing from spec (%d): %s\n  no longer public (%d): %s\n→ refresh spec.yaml from upstream/openapi.yaml (keep x-mcp + x-pp-resource) and reprint",
			len(upstream), len(spec), len(missing), strings.Join(missing, ", "), len(extra), strings.Join(extra, ", "))
	}
}
