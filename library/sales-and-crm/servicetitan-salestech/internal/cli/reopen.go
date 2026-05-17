package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// newReopenEstimateCmd wraps Estimates_Update with status=Open to reverse a
// prior dismiss (or to reset a Sold estimate back to Open if needed). The
// ST EstimateStatus enum has only Open/Sold/Dismissed; PUT /estimates/{id}
// with {"status":"Open"} is the supported reopen path.
func newReopenEstimateCmd(flags *rootFlags) *cobra.Command {
	var tenant string
	cmd := &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a Dismissed (or Sold) estimate — PUT /estimates/{id} with status=Open",
		Long: "Wraps Estimates_Update with the single field { status: \"Open\" }, the\n" +
			"sanctioned way to reverse a prior dismiss in the ST API. Sold estimates\n" +
			"can also be Open-reset though `estimates unsell` is the more idiomatic\n" +
			"call for that case. Use --dry-run to preview the PUT before sending.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli estimates reopen 78421 --dry-run
  servicetitan-salestech-pp-cli estimates reopen 78421 --tenant $ST_TENANT_ID
`, "\n"),
		Annotations: map[string]string{"pp:endpoint": "estimates.reopen", "pp:method": "PUT", "pp:path": "/tenant/{tenant}/estimates/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("estimate id must be a positive integer, got %q", args[0])
			}
			t := resolveTenant(tenant)
			if t == "" {
				return fmt.Errorf("tenant is required (pass --tenant or set ST_TENANT_ID)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/tenant/%s/estimates/%d", t, id)
			body := map[string]any{"status": "Open"}
			data, status, err := c.Put(path, body)
			if err != nil {
				return err
			}
			_ = status
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant id (defaults to $ST_TENANT_ID)")
	return cmd
}

// _ = ensures jsonencode is referenced when reopen body is logged in --json
var _ = json.Marshal
