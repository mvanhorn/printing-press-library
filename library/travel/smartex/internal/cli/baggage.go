// pp:data-source computed
package cli

import "github.com/spf13/cobra"

func newNovelBaggageCmd(flags *rootFlags) *cobra.Command {
	o := &smartOptions{adults: 1, class: "reserved", limit: 20, pieces: 1, product: "smart-ex", topic: "boarding"}
	cmd := &cobra.Command{Use: "baggage"}
	configureSmartCommand(cmd, flags, o)
	cmd.Flags().Float64Var(&o.length, "length-cm", 0, "Length of each identical piece in centimeters")
	cmd.Flags().Float64Var(&o.width, "width-cm", 0, "Width of each identical piece in centimeters")
	cmd.Flags().Float64Var(&o.height, "height-cm", 0, "Height of each identical piece in centimeters")
	cmd.Flags().Float64Var(&o.weight, "weight-kg", 0, "Weight of each identical piece in kilograms")
	cmd.Flags().IntVar(&o.pieces, "pieces", 1, "Counted identical pieces; normal maximum is two")
	cmd.Flags().StringVar(&o.class, "class", "reserved", "Seat class: reserved, unreserved or green; fare also supports all")
	cmd.Flags().StringVar(&o.special, "special", "", "Special equipment: stroller, sports or instrument; confirm with operator")
	return cmd
}
