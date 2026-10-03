package cli

// pp:data-source auto
import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
)

func newNovelParksFitCmd(f *rootFlags) *cobra.Command {
	var v parks.Vehicle
	var db string
	cmd := &cobra.Command{Use: "fit [park-id]", Short: "Compare one vehicle with published park limits and membership", Long: "Use this command for one vehicle's compatibility with one park's published limits and membership conditions. Do NOT use this command for screening several parks by required facilities; use 'parks match' instead. A missing bound stays unknown; overhang or multiple-pitch permission requires host confirmation.", Example: "  kurumatabi-pp-cli parks fit rvpark/1086 --length-m 6 --width-m 2.2 --height-m 3 --vehicle van --membership nonmember --agent", Annotations: parkAnnotation("auto", "park-id=rvpark/1086;--length-m=6;--width-m=2.2;--height-m=3;--vehicle=van;--membership=nonmember"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks fit")
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !f.agent && !f.asJSON {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide one park-id and actual vehicle measurements"))
		}
		if _, e := parks.Fit(parks.Park{}, v); e != nil {
			return usageErr(e)
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		p, src, e := parkRead(ctx, cmd, f, args[0], db)
		if e != nil {
			return e
		}
		result, e := parks.Fit(p, v)
		if e != nil {
			return usageErr(e)
		}
		return parkEmit(cmd, f, src, []parks.FitResult{result}, "Compatibility is conditional on published evidence, not a host acceptance guarantee.")
	}}
	cmd.Flags().Float64Var(&v.LengthM, "length-m", 0, "Actual total vehicle length in metres, including attachments")
	cmd.Flags().Float64Var(&v.WidthM, "width-m", 0, "Actual total vehicle width in metres")
	cmd.Flags().Float64Var(&v.HeightM, "height-m", 0, "Actual total vehicle height in metres, including roof equipment")
	cmd.Flags().StringVar(&v.Category, "vehicle", "", "Vehicle category such as van, cab, bus or trailer")
	cmd.Flags().StringVar(&v.Membership, "membership", "unknown", "Traveler membership: nonmember, member, standard, premium or unknown")
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
