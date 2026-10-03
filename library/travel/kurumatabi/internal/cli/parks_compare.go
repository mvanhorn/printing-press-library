package cli

// pp:data-source auto
import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
)

func newNovelParksCompareCmd(f *rootFlags) *cobra.Command {
	var db string
	cmd := &cobra.Command{Use: "compare [park-id...]", Short: "Align two to eight parks by published evidence", Long: "Use this command for side-by-side evidence across two or more parks. Do NOT use this command for evaluating one actual vehicle against published limits; use 'parks fit' instead. Member tariffs stay distinct; no dated total is calculated. Fetch failures stay explicit and are excluded from comparison rows; partial fetches emit usable evidence and exit 5.", Example: "  kurumatabi-pp-cli parks compare rvpark/1086 yypark/213 --agent", Annotations: parkAnnotation("auto", "first=rvpark/1086;second=yypark/213"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks compare")
		}
		if len(args) == 0 && !f.asJSON && !f.agent {
			return cmd.Help()
		}
		if len(args) < 2 || len(args) > 8 {
			return usageErr(fmt.Errorf("provide two to eight distinct park IDs"))
		}
		seen := map[string]bool{}
		for i, id := range args {
			x, e := parks.NormalizeID(id)
			if e != nil {
				return usageErr(e)
			}
			if seen[x] {
				return usageErr(fmt.Errorf("duplicate park %s", x))
			}
			seen[x] = true
			args[i] = x
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		ps := []parks.Park{}
		failures := []map[string]string{}
		src := "live"
		client := parkClientFactory("", f.rateLimit)
		origins := map[string]bool{}
		for _, id := range args {
			p, recordSource, e := parkReadWithClient(ctx, cmd, f, id, db, client)
			if e != nil {
				var rate *cliutil.RateLimitError
				if errors.As(e, &rate) {
					return rateLimitErr(e)
				}
				failures = append(failures, map[string]string{"id": id, "error": e.Error()})
				continue
			}
			origins[recordSource] = true
			ps = append(ps, p)
		}
		if len(ps) == 0 {
			return apiErr(fmt.Errorf("all %d park fetches failed: %v", len(args), failures))
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; comparison covers %d actual records\n", len(failures), len(args), len(ps))
		}
		if len(origins) > 1 {
			src = "mixed"
		} else if origins["local"] {
			src = "local"
		}
		out := map[string]any{"meta": map[string]any{"source": src, "requested_records": len(args), "compared_records": len(ps), "note": "Published evidence matrix; no dated totals or live vacancy.", "fetch_failures": failures}, "results": parks.Compare(ps)}
		if e := printJSONFiltered(cmd.OutOrStdout(), out, f); e != nil {
			return e
		}
		if len(failures) > 0 {
			return apiErr(fmt.Errorf("%d of %d park fetches failed; partial comparison contains %d actual records", len(failures), len(args), len(ps)))
		}
		return nil
	}}
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
