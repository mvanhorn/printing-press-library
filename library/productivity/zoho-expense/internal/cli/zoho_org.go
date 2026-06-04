package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/config"
)

func newOrgCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage the active Zoho organization (list available orgs, set active)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOrgListCmd(flags))
	cmd.AddCommand(newOrgUseCmd(flags))
	return cmd
}

func newOrgListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List Zoho organizations accessible to the current credentials",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  zoho-expense-pp-cli org list
  zoho-expense-pp-cli org list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), "/organizations", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Organizations []map[string]any `json:"organizations"`
			}
			if err := json.Unmarshal(raw, &env); err != nil || env.Organizations == nil {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), env.Organizations, flags)
			}
			rows := make([][]string, 0, len(env.Organizations))
			for _, o := range env.Organizations {
				rows = append(rows, []string{
					asStringOpt(o, "organization_id"),
					asStringOpt(o, "name"),
					asStringOpt(o, "country_code"),
					asStringOpt(o, "base_currency_code"),
				})
			}
			return flags.printTable(cmd, []string{"organization_id", "name", "country", "currency"}, rows)
		},
	}
}

func newOrgUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <organization_id>",
		Short: "Set the active Zoho organization id (persists to config)",
		Example: strings.Trim(`
  zoho-expense-pp-cli org use 60012345678
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			orgID := strings.TrimSpace(args[0])
			if orgID == "" {
				return usageErr(fmt.Errorf("organization_id required"))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.ZohoExpenseOrganizationId = orgID
			// Mirror into the request-headers map so the next request
			// carries the value without a fresh Load() pass.
			if cfg.Headers == nil {
				cfg.Headers = map[string]string{}
			}
			cfg.Headers["X-com-zoho-expense-organizationid"] = orgID
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"active_organization_id": orgID,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active organization: %s\n", orgID)
			return nil
		},
	}
}
