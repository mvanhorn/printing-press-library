// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: environment variable drift across sites/contexts.

package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

type envDriftRow struct {
	Key         string   `json:"key"`
	PresentOn   []string `json:"present_on"`
	MissingOn   []string `json:"missing_on"`
	Contexts    []string `json:"contexts"`
	MissingProd bool     `json:"missing_production_value"`
}

type envDriftView struct {
	Owners      []string      `json:"owners"`
	KeysChecked int           `json:"keys_checked"`
	Drift       []envDriftRow `json:"drift"`
}

// pp:data-source local
func newNovelEnvDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "env-drift",
		Short: "Find env vars set on some sites but missing or context-incomplete on others.",
		Long: `Compare environment variables across every synced owner (site or account) and
report keys that are not present everywhere, plus keys missing a production
context value. Run 'sync' first to populate env vars locally.`,
		Example:     "  netlify-pp-cli env-drift --json --select key,missing_on",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if mirrorMissing(dbPath) {
				return noMirror(cmd, flags, dbPath, "sites,accounts-env", envDriftView{
					Owners: []string{},
					Drift:  []envDriftRow{},
				})
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			vars := loadTyped(st, "accounts-env", "sites-env", "env", "environment-variables")

			// key -> owner set; key -> context set; key -> has production value
			ownerSet := map[string]bool{}
			keyOwners := map[string]map[string]bool{}
			keyContexts := map[string]map[string]bool{}
			keyProd := map[string]bool{}

			for _, v := range vars {
				key := firstStr(v, "key", "name")
				if key == "" {
					continue
				}
				owner := firstStr(v, "site_id", "siteId", "account_id", "accountId")
				if owner == "" {
					owner = "account"
				}
				ownerSet[owner] = true
				if keyOwners[key] == nil {
					keyOwners[key] = map[string]bool{}
					keyContexts[key] = map[string]bool{}
				}
				keyOwners[key][owner] = true
				if vals, ok := v["values"].([]any); ok {
					for _, raw := range vals {
						if vm, ok := raw.(map[string]any); ok {
							if c := str(vm, "context"); c != "" {
								keyContexts[key][c] = true
								if c == "production" || c == "all" {
									keyProd[key] = true
								}
							}
						}
					}
				}
			}

			owners := setKeys(ownerSet)
			sort.Strings(owners)

			drift := make([]envDriftRow, 0)
			for key, owned := range keyOwners {
				missing := make([]string, 0)
				for _, o := range owners {
					if !owned[o] {
						missing = append(missing, o)
					}
				}
				missingProd := !keyProd[key]
				if len(missing) == 0 && !missingProd {
					continue // present everywhere and has a prod value: no drift
				}
				present := setKeys(owned)
				sort.Strings(present)
				ctxs := setKeys(keyContexts[key])
				sort.Strings(ctxs)
				drift = append(drift, envDriftRow{
					Key:         key,
					PresentOn:   present,
					MissingOn:   missing,
					Contexts:    ctxs,
					MissingProd: missingProd,
				})
			}
			sort.Slice(drift, func(i, j int) bool { return drift[i].Key < drift[j].Key })

			view := envDriftView{Owners: owners, KeysChecked: len(keyOwners), Drift: drift}
			if len(vars) == 0 {
				cmd.PrintErrln("no env vars in local mirror; run: netlify-pp-cli sync --resources accounts-env")
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}

func setKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
