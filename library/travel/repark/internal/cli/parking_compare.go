// pp:data-source live
package cli

import "github.com/spf13/cobra"

func newNovelParkingCompareCmd(flags *rootFlags) *cobra.Command { return newReparkCompareCmd(flags) }
