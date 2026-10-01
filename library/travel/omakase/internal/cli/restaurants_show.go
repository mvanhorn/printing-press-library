// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newRestaurantsShowCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "show [id]", Short: "Read lazy course, rule and release details; optionally fetch Japanese name"}
	return configurePlanningDetailCmd(f, cmd, "show", nil, nil)
}
