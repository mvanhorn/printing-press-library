// pp:data-source local

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, name := range []string{"rate-limit", "no-cache", "client-profile"} {
			if f := root.PersistentFlags().Lookup(name); f != nil {
				f.Hidden = true
			}
		}
	})
}
