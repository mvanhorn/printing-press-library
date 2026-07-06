// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: export the user roster to CSV or JSON.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type rosterUser struct {
	ID         string `json:"id"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	Verified   bool   `json:"verified"`
}

// pp:data-source live
func newNovelRosterExportCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export users (id, name, email, status, verified) to CSV for reporting.",
		Long:        "Export users (id, name, email, status, verified) to CSV or JSON for reporting.\n\nUse to export the user roster.",
		Example:     "  agilix-dawn-pp-cli roster export --format csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch users and export roster")
				return nil
			}
			format := flagFormat
			if format == "" {
				format = "csv"
			}
			if format != "csv" && format != "json" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be 'csv' or 'json', got %q", format))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			search := fmt.Sprintf(`{"limit":%d}`, flagLimit)
			_, matches, err := fetchSearch(ctx, c, "user", search)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			users := make([]rosterUser, 0, len(matches))
			for _, m := range matches {
				var u rosterUser
				if json.Unmarshal(m, &u) == nil {
					users = append(users, u)
				}
			}
			if format == "json" || flags.asJSON {
				return flags.printJSON(cmd, users)
			}
			w := cmd.OutOrStdout()
			cw := csv.NewWriter(w)
			_ = cw.Write([]string{"id", "givenName", "familyName", "email", "status", "verified"})
			for _, u := range users {
				_ = cw.Write([]string{u.ID, u.GivenName, u.FamilyName, u.Email, u.Status, strconv.FormatBool(u.Verified)})
			}
			cw.Flush()
			return cw.Error()
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "csv", "Output format: csv or json")
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum users to export")
	return cmd
}
