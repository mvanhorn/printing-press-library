// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command contact search: free-text search over synced contacts.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

type contactSearchHit struct {
	ID        int    `json:"id"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Status    string `json:"status,omitempty"`
}

// pp:data-source local

func newNovelContactSearchCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "search",
		Short:       "Free-text search across synced contacts without hitting the API.",
		Long:        "Searches the local contact mirror by name, email, or phone. Run 'respondio-pp-cli sync --resources contact' first.",
		Example:     "  respondio-pp-cli contact search --query acme --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contact search")
			}
			if flagQuery == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]contactSearchHit, 0), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			q := strings.ToLower(strings.TrimSpace(flagQuery))
			results := make([]contactSearchHit, 0)
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				needle := strings.ToLower(fmt.Sprintf("%v %v %v %v %v", c["firstName"], c["lastName"], c["email"], c["phone"], str(c["tags"])))
				if q != "" && !strings.Contains(needle, q) {
					continue
				}
				hit := contactSearchHit{
					ID: intNum(c["id"]), FirstName: str(c["firstName"]), LastName: str(c["lastName"]),
					Email: str(c["email"]), Phone: str(c["phone"]), Status: str(c["status"]),
				}
				results = append(results, hit)
				if limit > 0 && len(results) >= limit {
					break
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, h := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%d %v %v\n", h.ID, h.FirstName, h.Email)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "search terms (matches name, email, phone)")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
