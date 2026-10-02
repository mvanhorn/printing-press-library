// pp:data-source auto
package cli

import "github.com/spf13/cobra"

func newNovelTimetableCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "timetable"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().StringVar(&o.from, "from", "", "Origin station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.to, "to", "", "Destination station ID, English name or Japanese name")
	cmd.Flags().StringVar(&o.date, "date", "", "Boarding date YYYY-MM-DD in JST; fare defaults to today")
	cmd.Flags().StringVar(&o.train, "train", "", "Optional train category: nozomi, hikari, kodama, mizuho, sakura, tsubame")
	cmd.Flags().BoolVar(&o.detail, "detail", false, "Include detailed records or fetch public product-document links")
	cmd.Flags().BoolVar(&o.offline, "offline", false, "Serve explicit2026-03-14 curated snapshot without HTTP")
	cmd.Flags().StringVar(&o.after, "after", "", "Keep example departures at/after HH:MM JST")
	cmd.Flags().IntVar(&o.limit, "limit", 5, "Maximum returned examples; allowed1–50")
	return cmd
}
