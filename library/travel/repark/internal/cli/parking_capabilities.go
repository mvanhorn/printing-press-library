// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelParkingCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	return newReparkCapabilitiesCmd(flags)
}
