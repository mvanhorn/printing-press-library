// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelPolicyCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "policy"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.topic, "topic", "boarding", "boarding, change, refund, baggage, windows, products or all")
	return cmd
}
