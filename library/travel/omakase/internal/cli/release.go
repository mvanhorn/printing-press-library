// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelReleaseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "release [id]", Short: "Inspect source release schedule for one restaurant ID; seats remain separate"}
	return configurePlanningDetailCmd(flags, cmd, "release", nil, nil)
}
