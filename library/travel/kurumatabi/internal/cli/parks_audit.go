package cli

// pp:data-source auto
import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
	"time"
)

func newNovelParksAuditCmd(f *rootFlags) *cobra.Command {
	var db string
	cmd := &cobra.Command{Use: "audit [park-id]", Short: "Find evidence conflicts and host-confirmation gaps", Long: "Use this command for conflicts and evidence gaps within a park observation. Do NOT use this command for comparing tariffs and facilities across parks; use 'parks compare' instead. Rules inspect disabled icons, missing dimensions, old source updates, qualified opening and scoped outdoor activities.", Example: "  kurumatabi-pp-cli parks audit rvpark/712 --agent", Annotations: parkAnnotation("auto", "park-id=rvpark/712"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks audit")
		}
		if len(args) == 0 && !f.agent && !f.asJSON {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide one park-id to audit"))
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		p, src, e := parkRead(ctx, cmd, f, args[0], db)
		if e != nil {
			return e
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]any{"source": src, "id": p.ID, "observed_at": p.ObservedAt, "source_updated_date_jst": p.SourceUpdated, "note": "Small explicit rule set; absence of issues does not certify operation, availability or permission."}, "results": parks.Audit(p, time.Now())}, f)
	}}
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
