// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelInventoryCmd(flags *rootFlags) *cobra.Command { return newPlanningInventoryCmd(flags) }
