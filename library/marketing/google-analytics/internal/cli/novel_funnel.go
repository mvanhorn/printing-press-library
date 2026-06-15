package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newFunnelCmd(flags *rootFlags) *cobra.Command {
	var steps, start, end string
	c := &cobra.Command{Use: "funnel", Short: "Run GA4 v1alpha runFunnelReport for a named event step sequence", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.RunFunnelReport(context.Background(), p, funnelRequest(steps, start, end))
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
	c.Flags().StringVar(&steps, "steps", "view_item,add_to_cart,begin_checkout,purchase", "Comma-separated GA4 event names")
	c.Flags().StringVar(&start, "start", "30daysAgo", "Start date")
	c.Flags().StringVar(&end, "end", "yesterday", "End date")
	return c
}
