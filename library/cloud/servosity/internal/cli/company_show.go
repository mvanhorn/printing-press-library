// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newCompanyCmd is a singular-form parent that hosts the snapshot-style
// `company show <id>` composer. Distinct from the generator's plural
// `companies` command tree which mirrors per-endpoint operations.
func newCompanyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "company",
		Short:       "Per-company snapshot views (composer commands distinct from the typed `companies` tree)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newCompanyShowCmd(flags))
	return cmd
}

// newCompanyShowCmd composes ~7 endpoints + the cross-engine backup_facts view
// + nested notes (workaround for the broken /company-notes/?company= filter)
// into a single per-company snapshot.
//
// All calls are GETs. Read-only. Safe against PROD.
func newCompanyShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <company-id>",
		Short: "Per-company snapshot: metadata + addresses + contracts + backups (3 engines) + open issues + agent sessions",
		Long: `Single command that pulls a company's full picture into one screen by
composing:

  GET /companies/{id}/                      (core record)
  GET /companies/{id}/notes/                (nested notes — works around
                                             the broken /company-notes/?company= filter)
  GET /companies/{id}/backup-stores/        (NAS credential health)
  GET /companies/{id}/restore-queues/       (DRaaS in flight)
  GET /backups/?company={id}                (classic engine)
  GET /restic-backups/?company={id}         (restic engine)
  GET /dr-backups/?company={id}             (DR engine)
  GET /issues/?company={id}&state=ACTIVE    (open issues)
  GET /agent-sessions/?company={id}         (agent sessions)
  GET /contracts/?company={id}              (contracts)

Each section degrades gracefully — a 404 or 403 on one section just yields
an empty array for that section, never aborting the whole snapshot.`,
		Example: `  servosity-pp-cli company show 4421 --json
  servosity-pp-cli company show 4421 --json --select results.open_issues`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"results":{}}` + "\n"))
				}
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			id := args[0]
			snap := buildCompanySnapshot(ctx, c, id)
			payload, _ := json.Marshal(snap)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	return cmd
}

type companyHTTP interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func buildCompanySnapshot(_ context.Context, c companyHTTP, id string) map[string]any {
	// Helper: best-effort GET that returns null on any failure.
	getOpt := func(path string, params map[string]string) json.RawMessage {
		data, err := c.Get(path, params)
		if err != nil {
			return json.RawMessage("null")
		}
		return data
	}
	getList := func(path string, params map[string]string) []json.RawMessage {
		data := getOpt(path, params)
		if string(data) == "null" {
			return nil
		}
		// Try direct array first (some endpoints return [{...}]).
		var list []json.RawMessage
		if err := json.Unmarshal(data, &list); err == nil {
			return list
		}
		// Then paginated envelope {results: [...]}.
		var env struct {
			Results []json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(data, &env); err == nil && env.Results != nil {
			return env.Results
		}
		return nil
	}

	core := getOpt("/companies/"+id+"/", nil)
	// `found` is the disambiguation signal between "company exists but has no
	// data in any section" and "company id was bogus / 404'd / unauthorized".
	// `getOpt` returns the literal JSON null bytes on any GET failure (404,
	// 403, network), and we also treat an empty object as not-found because
	// the API can return {} for missing records.
	coreStr := string(core)
	found := coreStr != "null" && coreStr != "{}" && coreStr != ""
	notes := getList("/companies/"+id+"/notes/", nil)
	stores := getList("/companies/"+id+"/backup-stores/", nil)
	restoreQueues := getList("/companies/"+id+"/restore-queues/", nil)
	classic := getList("/backups/", map[string]string{"company": id})
	restic := getList("/restic-backups/", map[string]string{"company": id})
	dr := getList("/dr-backups/", map[string]string{"company": id})
	issues := getList("/issues/", map[string]string{"company": id, "state": "ACTIVE"})
	sessions := getList("/agent-sessions/", map[string]string{"company": id})
	contracts := getList("/contracts/", map[string]string{"company": id})

	results := map[string]any{
		"company":         core,
		"notes":           notes,
		"backup_stores":   stores,
		"restore_queues":  restoreQueues,
		"backups_classic": classic,
		"backups_restic":  restic,
		"backups_dr":      dr,
		"open_issues":     issues,
		"agent_sessions":  sessions,
		"contracts":       contracts,
	}
	return map[string]any{
		"meta": map[string]any{
			"source":     "live",
			"company_id": id,
			"sections":   10,
			"found":      found,
			"counts": map[string]int{
				"notes":           len(notes),
				"backup_stores":   len(stores),
				"restore_queues":  len(restoreQueues),
				"backups_classic": len(classic),
				"backups_restic":  len(restic),
				"backups_dr":      len(dr),
				"open_issues":     len(issues),
				"agent_sessions":  len(sessions),
				"contracts":       len(contracts),
			},
		},
		"results": results,
	}
}

// fmt import safety
var _ = fmt.Sprintf
