package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/cliutil/testenv"
)

func TestActualListCommandTreeClassifiesLocalWrites(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	for _, name := range []string{"add", "note", "remove", "refresh"} {
		t.Run(name, func(t *testing.T) {
			cmd, _, err := root.Find([]string{"lists", name})
			if err != nil || cmd == nil || !cmd.Runnable() {
				t.Fatalf("actual list command unavailable: %v", err)
			}
			if cmd.Annotations["mcp:local-write"] != "true" || cmd.Annotations["mcp:read-only"] == "true" {
				t.Fatalf("local notebook mutation has misleading classification: %v", cmd.Annotations)
			}
			if !strings.Contains(cmd.Annotations["pp:happy-args"], "--home=.printing-press-fixtures/live-home") {
				t.Fatal("live harness mutation lost its isolated fixture home")
			}
			if name == "remove" && (!strings.Contains(cmd.Annotations["pp:happy-args"], "list=fixture-remove") || !strings.Contains(cmd.Annotations["pp:happy-args"], "--json=true")) {
				t.Fatal("remove lost its dedicated membership or JSON invocation")
			}
		})
	}
	compare, _, err := root.Find([]string{"lists", "compare"})
	if err != nil || compare.Annotations["mcp:read-only"] != "true" || compare.Annotations["mcp:local-write"] == "true" {
		t.Fatalf("offline comparison is not classified read-only: %v", err)
	}
	group, _, err := root.Find([]string{"lists"})
	if err != nil || group.Runnable() || group.Annotations["pp:parent-group"] != "true" {
		t.Fatalf("list overview was made an executable workflow: %v", err)
	}
}
