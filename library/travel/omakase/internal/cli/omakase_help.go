package cli

import "github.com/spf13/cobra"

// Keep source endpoint help and dry runs accurate without editing generated files.
func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		examples := map[string]string{
			"filters":           "  omakase-pp-cli filters --agent",
			"membership":        "  omakase-pp-cli membership --refresh --agent",
			"inventory":         "  omakase-pp-cli inventory --agent",
			"inventory status":  "  omakase-pp-cli inventory status --agent",
			"inventory find":    "  omakase-pp-cli inventory find --query Konno --limit 5 --agent",
			"inventory refresh": "  omakase-pp-cli inventory refresh --pages 1 --agent",
			"pages detail":      "  omakase-pp-cli pages detail hc778124 --agent",
		}
		for path, example := range examples {
			var parts []string
			switch path {
			case "inventory status":
				parts = []string{"inventory", "status"}
			case "inventory find":
				parts = []string{"inventory", "find"}
			case "inventory refresh":
				parts = []string{"inventory", "refresh"}
			case "pages detail":
				parts = []string{"pages", "detail"}
			default:
				parts = []string{path}
			}
			c, _, e := root.Find(parts)
			if e == nil && c != root {
				c.Example = example
			}
		}
		for _, leaf := range []string{"list", "detail"} {
			c, _, e := root.Find([]string{"pages", leaf})
			if e != nil || c == root {
				continue
			}
			old := c.RunE
			action := "read public source metadata " + leaf
			c.RunE = func(cmd *cobra.Command, args []string) error {
				if dryRunOK(f) {
					return writeDryRun(cmd.OutOrStdout(), f, action)
				}
				return old(cmd, args)
			}
		}
	})
}
