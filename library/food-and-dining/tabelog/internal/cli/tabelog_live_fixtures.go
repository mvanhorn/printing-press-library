package cli

import "github.com/spf13/cobra"

// Verification runners execute from the CLI checkout. These metadata-only
// arguments keep their saved-state samples in a prepared disposable home.
// Normal examples and runtime storage defaults remain independent of fixtures.
func init() {
	registerNovelCommand(func(root *cobra.Command, _ *rootFlags) {
		for _, parent := range root.Commands() {
			if parent.Name() != "lists" {
				continue
			}
			for _, cmd := range parent.Commands() {
				args := ""
				switch cmd.Name() {
				case "add":
					args = "list=tokyo-bars;id=13005012;--note=Ginza bar fixture"
				case "show", "compare":
					args = "list=tokyo-bars"
				case "note":
					args = "list=fixture-note;id=13005012;--note=Updated fixture note"
				case "remove":
					args = "list=fixture-remove;id=13005012;--json=true"
				case "refresh":
					args = "list=tokyo-bars;id=13005012"
				case "alternatives":
					args = "list=tokyo-bars;--for=13005012;--match=area,category;--meal=dinner;--budget-max=5000"
				case "audit":
					args = "list=tokyo-bars;--require=hours,payment,reservation,dinner_budget;--max-age=24h"
				}
				if args != "" {
					if cmd.Annotations == nil {
						cmd.Annotations = map[string]string{}
					}
					cmd.Annotations["pp:happy-args"] = args + ";--home=.printing-press-fixtures/live-home"
				}
			}
		}
	})
}
