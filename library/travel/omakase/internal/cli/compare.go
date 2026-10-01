// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelCompareCmd(flags *rootFlags) *cobra.Command { return newPlanningCompareCmd(flags) }
