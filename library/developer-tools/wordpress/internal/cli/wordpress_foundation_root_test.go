package cli

import "testing"

func TestWordPressFoundationCommandsAndFlagsAreRegistered(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, flag := range []string{"wp-fields", "embed"} {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Fatalf("persistent flag --%s is not registered", flag)
		}
	}
	for _, path := range [][]string{{"site"}, {"media", "upload"}} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("command %v not registered: cmd=%v err=%v", path, cmd, err)
		}
	}
}
