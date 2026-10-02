package cli

import "testing"

func TestRootCommandPublishesCanonicalCommandTree(t *testing.T) {
	t.Parallel()
	root := NewRootCommand()
	want := map[string][]string{
		"search":     nil,
		"list":       {"category", "sort", "limit"},
		"info":       nil,
		"download":   {"variant", "output", "show"},
		"trending":   nil,
		"categories": nil,
		"random":     {"category"},
	}
	for name, flags := range want {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == root {
			t.Fatalf("command %q missing: %v", name, err)
		}
		for _, flag := range flags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("command %q missing --%s", name, flag)
			}
		}
	}
}
