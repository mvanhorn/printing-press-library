// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: tenants audit.

package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

type tenantAuditRow struct {
	Collection   string         `json:"collection"`
	MultiTenancy bool           `json:"multi_tenancy_enabled"`
	TenantCount  int            `json:"tenant_count,omitempty"`
	ByStatus     map[string]int `json:"by_status,omitempty"`
	FetchError   string         `json:"fetch_error,omitempty"`
}

// pp:data-source live
func newNovelTenantsAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "See tenant counts and activity status across every collection in one view.",
		Example:     "  weaviate-collections-pp-cli tenants audit --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit tenants across every multi-tenant collection")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			classes, err := fetchAllClasses(ctx, flags)
			if err != nil {
				return err
			}

			mtClasses := make([]string, 0, len(classes))
			nonMT := make([]string, 0)
			for _, cls := range classes {
				name := classNameOf(cls)
				if name == "" {
					continue
				}
				mt, _ := cls["multiTenancyConfig"].(map[string]any)
				enabled, _ := mt["enabled"].(bool)
				if enabled {
					mtClasses = append(mtClasses, name)
				} else {
					nonMT = append(nonMT, name)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type fetchResult struct {
				idx int
				row tenantAuditRow
			}
			results := make(chan fetchResult, len(mtClasses))
			var wg sync.WaitGroup
			for idx, name := range mtClasses {
				wg.Add(1)
				go func(idx int, name string) {
					defer wg.Done()
					data, getErr := c.Get(ctx, replacePathParam("/schema/{className}/tenants", "className", name), nil)
					if getErr != nil {
						results <- fetchResult{idx: idx, row: tenantAuditRow{
							Collection:   name,
							MultiTenancy: true,
							FetchError:   classifyAPIError(getErr, flags).Error(),
						}}
						return
					}
					var tenants []map[string]any
					if err := json.Unmarshal(data, &tenants); err != nil {
						results <- fetchResult{idx: idx, row: tenantAuditRow{
							Collection:   name,
							MultiTenancy: true,
							FetchError:   fmt.Sprintf("parsing tenants response: %v", err),
						}}
						return
					}
					byStatus := map[string]int{}
					for _, t := range tenants {
						status, _ := t["activityStatus"].(string)
						if status == "" {
							status = "UNKNOWN"
						}
						byStatus[status]++
					}
					results <- fetchResult{idx: idx, row: tenantAuditRow{
						Collection:   name,
						MultiTenancy: true,
						TenantCount:  len(tenants),
						ByStatus:     byStatus,
					}}
				}(idx, name)
			}
			go func() {
				wg.Wait()
				close(results)
			}()

			rows := make([]tenantAuditRow, len(mtClasses))
			for r := range results {
				rows[r.idx] = r.row
			}

			fetchFailures := 0
			for _, r := range rows {
				if r.FetchError != "" {
					fetchFailures++
				}
			}
			if fetchFailures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d multi-tenant collections failed to fetch tenants; see fetch_error per row\n", fetchFailures, len(mtClasses))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no multi-tenant collections found.")
					return nil
				}
				for _, r := range rows {
					if r.FetchError != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%s: fetch failed (%s)\n", r.Collection, r.FetchError)
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %d tenant(s) %v\n", r.Collection, r.TenantCount, r.ByStatus)
				}
				if len(nonMT) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%d collection(s) without multi-tenancy enabled: %v\n", len(nonMT), nonMT)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"multi_tenant_collections": rows,
				"non_multi_tenant":         nonMT,
				"fetch_failures":           fetchFailures,
			}, flags)
		},
	}
	return cmd
}
