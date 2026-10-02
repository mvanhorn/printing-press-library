// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newNovelParkingDetailCmd(flags *rootFlags) *cobra.Command { return newReparkDetailCmd(flags) }
