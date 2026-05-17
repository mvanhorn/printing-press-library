package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		noProbe bool
	)
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Cross-source mirror health: local counts vs API totalCount vs cursor age per table",
		Long: "Reports the state of the local mirror at a glance: per-resource local\n" +
			"row counts, last sync cursor age, and (when --no-probe is not set) the\n" +
			"API's reported totalCount for estimates so drift is obvious. Use as\n" +
			"pre-flight before any audit so 'reports show 0 rows' doesn't get\n" +
			"confused with 'we haven't synced today'. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli health
  servicetitan-salestech-pp-cli health --json
  servicetitan-salestech-pp-cli health --no-probe
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var probe salestech.APICountFn
			if !noProbe {
				c, err := flags.newClient()
				if err == nil {
					probe = st_apiCount(c, resolveTenant(""))
				}
			}
			report, err := salestech.Health(cmd.Context(), db, probe, 0)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			table := make([][]string, 0, len(report.Resources))
			for _, r := range report.Resources {
				api := "-"
				drift := "-"
				if r.APICount > 0 {
					api = iN(r.APICount)
					drift = iN(r.Drift)
				}
				table = append(table, []string{r.Resource, iN(r.LocalCount), api, drift, r.CursorAge, r.Status, r.Detail})
			}
			return stOutput(cmd, flags, report,
				[]string{"RESOURCE", "LOCAL", "API", "DRIFT", "CURSOR AGE", "STATUS", "DETAIL"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Skip the API totalCount probe (local-only report)")
	return cmd
}
