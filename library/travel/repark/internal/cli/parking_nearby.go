// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newNovelParkingNearbyCmd(flags *rootFlags) *cobra.Command { return newReparkNearbyCmd(flags) }
