package cli

// reprint637 round 4: structural checks over the whole Cobra tree, so a reprint cannot silently lose an operation or register
// a command twice.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Every sibling name (and alias) is unique at every level: cobra keeps both duplicates and Find returns the first, so a
// duplicate AddCommand is otherwise invisible.
func TestSiblingCommandNamesAreUnique(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		seen := map[string]bool{}
		for _, sub := range c.Commands() {
			for _, n := range append([]string{sub.Name()}, sub.Aliases...) {
				if seen[n] {
					t.Errorf("%s has two children named %q", c.CommandPath(), n)
				}
				seen[n] = true
			}
			walk(sub)
		}
	}
	walk(newRootCmd(&rootFlags{}))
}

// PATCH(reprint-637-users-invite): every non-OPTIONS operation of spec.yaml is reachable through exactly one command whose
// pp:method / pp:path annotation names it (promoted top-level shortcuts, which repeat a resource's annotation, are not counted).
func TestEverySpecOperationHasExactlyOneCommand(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "spec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for path, item := range doc.Paths {
		for _, m := range []string{"get", "post", "put", "patch", "delete", "head"} {
			if _, ok := item[m]; ok {
				want[strings.ToUpper(m)+" "+path] = true
			}
		}
	}
	got := map[string][]string{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if m, p := c.Annotations["pp:method"], c.Annotations["pp:path"]; m != "" && p != "" && c.Annotations["pp:promoted"] == "" {
			got[m+" "+p] = append(got[m+" "+p], c.CommandPath())
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(newRootCmd(&rootFlags{}))
	reached := 0
	var missing []string
	for op := range want {
		switch n := len(got[op]); {
		case n == 0:
			missing = append(missing, op)
		case n > 1:
			t.Errorf("%s is reachable through %d commands: %v", op, n, got[op])
		default:
			reached++
		}
	}
	sort.Strings(missing)
	for op := range got {
		if !want[op] {
			t.Errorf("%v carries %s, which spec.yaml does not have", got[op], op)
		}
	}
	t.Logf("spec operations reachable: %d/%d", reached, len(want))
	if len(missing) > 0 {
		t.Errorf("%d/%d spec operations reachable; missing: %s", reached, len(want), strings.Join(missing, ", "))
	}
}
