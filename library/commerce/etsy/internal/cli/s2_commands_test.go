package cli

import (
	"strings"
	"testing"
)

var s2Commands = []struct {
	path     []string
	example  string // a realistic synthetic value the help must show
	readOnly string
}{
	{[]string{"listings", "bulk-edit"}, "1000000001", "false"},
	{[]string{"listings", "copy"}, "1000000001", "false"},
	{[]string{"listings", "snapshot"}, "1000000001", "false"},
	{[]string{"inventory", "set-qty"}, "SKU-LINEN-APRON-NAT", "false"},
	{[]string{"inventory", "low-stock"}, "--threshold 3", ""},
	{[]string{"orders", "unshipped"}, "orders unshipped", "true"},
	{[]string{"money", "summary"}, "--since 30d", "true"},
}

func TestSlice2CommandsAreWiredAndDocumented(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	for _, tc := range s2Commands {
		cmd, _, err := root.Find(tc.path)
		if err != nil || cmd == nil || cmd.Name() != tc.path[len(tc.path)-1] {
			t.Fatalf("%v not reachable: %v", tc.path, err)
		}
		if isNovelScaffoldCommand(cmd) {
			t.Fatalf("%v is still a scaffold", tc.path)
		}
		if cmd.Annotations["mcp:read-only"] != tc.readOnly {
			t.Errorf("%v mcp:read-only = %q, want %s", tc.path, cmd.Annotations["mcp:read-only"], tc.readOnly)
		}
		if cmd.Annotations["pp:data-source"] == "" {
			t.Errorf("%v has no pp:data-source", tc.path)
		}
		if !strings.Contains(cmd.Example, tc.example) {
			t.Errorf("%v example lacks %q:\n%s", tc.path, tc.example, cmd.Example)
		}
		if !strings.HasPrefix(cmd.Example, "  etsy-pp-cli ") {
			t.Errorf("%v example is not indented/rooted: %q", tc.path, cmd.Example)
		}
		if strings.Contains(strings.ToLower(cmd.Example+cmd.Long), "todo") {
			t.Errorf("%v help mentions TODO", tc.path)
		}
	}
	if p, _, _ := root.Find([]string{"orders"}); p == nil || p.Annotations["pp:parent-group"] != "true" {
		t.Fatal("orders parent group missing")
	}
	for _, kept := range [][]string{{"listings", "update"}, {"listings", "create-draft"}, {"inventory", "audit"}, {"money", "fees"}} {
		if c, _, err := root.Find(kept); err != nil || c == nil || c.Name() != kept[1] {
			t.Errorf("hook wiring removed existing %v", kept)
		}
	}
}

func TestSlice2CommandsDryRunExitZeroWithEnvelope(t *testing.T) {
	s2NewServer(t, nil)
	for _, tc := range s2Commands {
		args := append(append([]string{}, tc.path...), "--dry-run", "--json")
		out, _, err := s2Run(t, args...)
		if err != nil {
			t.Fatalf("%v --dry-run: %v", tc.path, err)
		}
		var env map[string]any
		s2Decode(t, out, &env)
		if env["dry_run"] != true || env["success"] != true {
			t.Errorf("%v dry-run envelope = %s", tc.path, out)
		}
	}
}

func TestSlice2HelpRendersForEveryCommand(t *testing.T) {
	for _, tc := range s2Commands {
		out, _, err := s2Run(t, append(append([]string{}, tc.path...), "--help")...)
		if err != nil || !strings.Contains(out, "Examples:") || !strings.Contains(out, tc.example) {
			t.Errorf("%v --help: err=%v out=%q", tc.path, err, out)
		}
	}
}
