package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newScimDiffCmd(flags *rootFlags) *cobra.Command {
	var rosterPath string
	var apply bool

	cmd := &cobra.Command{
		Use:   "scim-diff",
		Short: "Diff a CSV/JSON HRIS roster against /Users SCIM list",
		Long: `Reads a roster file (CSV with email,name columns or JSON array of {email,name}) and
compares against /Users SCIM list. Prints add/update/remove sets. --apply runs SCIM CRUD;
default is dry-run.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if rosterPath == "" {
				return usageErr(fmt.Errorf("--roster <file> is required"))
			}
			roster, err := readRoster(rosterPath)
			if err != nil {
				return usageErr(fmt.Errorf("read roster: %w", err))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/Users", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// SCIM list shape: {Resources: [{userName, name, ...}]}
			var listResp struct {
				Resources []map[string]any `json:"Resources"`
			}
			_ = json.Unmarshal(data, &listResp)

			scim := map[string]map[string]any{}
			for _, u := range listResp.Resources {
				email := strings.ToLower(stringField(u, "userName"))
				if email == "" {
					if e, ok := u["emails"].([]any); ok && len(e) > 0 {
						if m, ok := e[0].(map[string]any); ok {
							email = strings.ToLower(stringField(m, "value"))
						}
					}
				}
				if email != "" {
					scim[email] = u
				}
			}

			toAdd, toUpdate, toRemove := []map[string]string{}, []map[string]string{}, []map[string]string{}
			rosterEmails := map[string]bool{}
			for _, r := range roster {
				email := strings.ToLower(r["email"])
				rosterEmails[email] = true
				existing, ok := scim[email]
				if !ok {
					toAdd = append(toAdd, r)
					continue
				}
				existingName := stringField(existing, "displayName")
				if existingName == "" {
					if n, ok := existing["name"].(map[string]any); ok {
						existingName = stringField(n, "formatted")
					}
				}
				if r["name"] != "" && r["name"] != existingName {
					toUpdate = append(toUpdate, map[string]string{"email": email, "name": r["name"], "id": stringField(existing, "id")})
				}
			}
			for email, u := range scim {
				if !rosterEmails[email] {
					toRemove = append(toRemove, map[string]string{"email": email, "id": stringField(u, "id")})
				}
			}

			result := map[string]any{
				"add":    toAdd,
				"update": toUpdate,
				"remove": toRemove,
			}

			if !apply || dryRunOK(flags) {
				result["dry_run"] = true
				body, _ := json.Marshal(result)
				fmt.Fprintln(cmd.OutOrStdout(), string(body))
				return nil
			}

			added, updated, removed := 0, 0, 0
			for _, r := range toAdd {
				body := map[string]any{
					"userName": r["email"],
					"emails":   []map[string]any{{"value": r["email"], "primary": true}},
					"name":     map[string]any{"formatted": r["name"]},
				}
				if _, code, err := c.Post("/Users", body); err == nil && code < 400 {
					added++
				}
			}
			for _, r := range toUpdate {
				body := map[string]any{"name": map[string]any{"formatted": r["name"]}}
				if _, code, err := c.Patch("/Users/"+r["id"], body); err == nil && code < 400 {
					updated++
				}
			}
			for _, r := range toRemove {
				if _, code, err := c.Delete("/Users/" + r["id"]); err == nil && code < 400 {
					removed++
				}
			}
			result["applied"] = true
			result["added"] = added
			result["updated"] = updated
			result["removed"] = removed
			body, _ := json.Marshal(result)
			fmt.Fprintln(cmd.OutOrStdout(), string(body))
			return nil
		},
	}
	cmd.Flags().StringVar(&rosterPath, "roster", "", "Path to CSV (email,name) or JSON array roster file (required)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply add/update/remove via SCIM CRUD (default is dry-run)")
	return cmd
}

func stringField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func readRoster(path string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		var arr []map[string]string
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	r := csv.NewReader(strings.NewReader(string(data)))
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty CSV")
	}
	header := rows[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := idx["email"]; !ok {
		return nil, fmt.Errorf("CSV must have an 'email' column")
	}
	out := []map[string]string{}
	for _, row := range rows[1:] {
		entry := map[string]string{}
		for k, i := range idx {
			if i < len(row) {
				entry[k] = row[i]
			}
		}
		out = append(out, entry)
	}
	return out, nil
}
