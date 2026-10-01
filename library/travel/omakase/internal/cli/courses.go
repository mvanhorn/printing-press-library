// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelCoursesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "courses [id]", Short: "Inspect source course prices, charges and cancellation for one restaurant ID"}
	return configurePlanningDetailCmd(flags, cmd, "courses", nil, nil)
}
