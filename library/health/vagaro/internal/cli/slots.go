// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: the availability slots primitive.
// This is the core query reused by find/watch/rebook.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// slotsResult wraps the slot groups with the query context so an empty result
// still carries a note instead of failing.
type slotsResult struct {
	Slug       string             `json:"slug"`
	BusinessID string             `json:"business_id"`
	ServiceID  string             `json:"service_id"`
	ProviderID string             `json:"provider_id,omitempty"`
	Date       string             `json:"date"`
	AppDate    string             `json:"app_date"`
	TotalSlots int                `json:"total_slots"`
	Groups     []vagaro.SlotGroup `json:"groups"`
	Note       string             `json:"note,omitempty"`
}

func newSlotsCmd(flags *rootFlags) *cobra.Command {
	var service, provider, date string

	cmd := &cobra.Command{
		Use:   "slots <slug>",
		Short: "List available appointment slots for a service at a business",
		Long: `The availability primitive: given a business slug and a service, list the
open appointment times for the week starting on --date.

An empty result (no availability) is returned as an empty list with a note,
not an error.`,
		Example: "  vagaro-pp-cli slots centralbarber --service 34098477 --date 2026-07-24",
		// pp:data-source live
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slug=centralbarber;service=34098477;date=2026-07-24"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug> --service <serviceID>", cmd.CommandPath()))
			}
			service = strings.TrimSpace(service)
			if service == "" {
				return usageErr(fmt.Errorf("--service <serviceID> is required (see 'vagaro-pp-cli business services %s')", slug))
			}
			date = strings.TrimSpace(date)
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			appDate, err := vagaro.FormatAppDate(date)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			businessID, err := resolveBusinessID(ctx, c, flags, slug)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			groups, err := c.Availability(ctx, businessID, service, strings.TrimSpace(provider), appDate)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			result := slotsResult{
				Slug:       slug,
				BusinessID: businessID,
				ServiceID:  service,
				ProviderID: strings.TrimSpace(provider),
				Date:       date,
				AppDate:    appDate,
				Groups:     groups,
			}
			for _, g := range groups {
				result.TotalSlots += len(g.Times)
			}
			if result.TotalSlots == 0 {
				result.Note = fmt.Sprintf("no availability for service %s during the week of %s", service, appDate)
			}
			return emitVagaro(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&service, "service", "", "Service ID (required; maps to csvServiceID)")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider ID (optional; csvSPID, empty = any provider)")
	cmd.Flags().StringVar(&date, "date", "", "Week-start date as YYYY-MM-DD (default: today)")
	return cmd
}
