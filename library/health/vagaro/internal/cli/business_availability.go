// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: next-available summary built on the
// getavailablemultiappointments slots primitive.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// availabilitySummary is the compact next-available view for a business.
type availabilitySummary struct {
	Slug         string             `json:"slug"`
	BusinessID   string             `json:"business_id"`
	ServiceID    int64              `json:"service_id,omitempty"`
	ServiceName  string             `json:"service,omitempty"`
	ProviderID   string             `json:"provider_id,omitempty"`
	ProviderName string             `json:"provider,omitempty"`
	WeekOf       string             `json:"week_of"`
	From         string             `json:"from,omitempty"`
	To           string             `json:"to,omitempty"`
	Weeks        []string           `json:"weeks,omitempty"`
	TotalSlots   int                `json:"total_slots"`
	Groups       []vagaro.SlotGroup `json:"groups"`
	Note         string             `json:"note,omitempty"`
}

type availabilityOptions struct {
	Service  string
	Provider string
	From     string
	To       string
	Weeks    int
}

type availabilityPlan struct {
	Service      vagaro.ServiceRow
	ProviderID   string
	ProviderName string
	From         time.Time
	To           time.Time
	WeekDates    []time.Time
	ExplicitFrom bool
	ExplicitTo   bool
}

func newBusinessAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var opts availabilityOptions
	cmd := &cobra.Command{
		Use:   "availability <slug>",
		Short: "Summarize a business's next-available slots, optionally scoped by service, provider, and date window",
		Example: strings.Join([]string{
			"  vagaro-pp-cli business availability sample-shop",
			"  vagaro-pp-cli business availability sample-shop --service haircut --provider alex --from 2026-07-20 --to 2026-07-31",
			"  vagaro-pp-cli business availability sample-shop --service 12345 --weeks 3 --agent",
		}, "\n"),
		// pp:data-source live
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slug=centralbarber"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug>", cmd.CommandPath()))
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
			services, err := c.Services(ctx, businessID)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			if len(services) == 0 {
				summary := availabilitySummary{Slug: slug, BusinessID: businessID, Groups: []vagaro.SlotGroup{}, Note: "business has no bookable services"}
				return emitVagaro(cmd, flags, summary)
			}
			providers := []vagaro.Provider(nil)
			if strings.TrimSpace(opts.Provider) != "" {
				providers, err = c.Staff(ctx, businessID)
				if err != nil {
					return classifyVagaroError(err, flags)
				}
			}

			plan, err := buildAvailabilityPlan(services, providers, opts, time.Now())
			if err != nil {
				return usageErr(err)
			}
			summary := availabilitySummary{
				Slug:         slug,
				BusinessID:   businessID,
				ServiceID:    plan.Service.ServiceID,
				ServiceName:  plan.Service.ServiceTitle,
				ProviderID:   plan.ProviderID,
				ProviderName: plan.ProviderName,
				WeekOf:       formatAvailabilityDate(plan.WeekDates[0]),
				From:         plan.From.Format("2006-01-02"),
				To:           plan.To.Format("2006-01-02"),
				Groups:       []vagaro.SlotGroup{},
			}
			if len(plan.WeekDates) > 1 {
				for _, d := range plan.WeekDates {
					summary.Weeks = append(summary.Weeks, formatAvailabilityDate(d))
				}
			}

			serviceID := strconv.FormatInt(plan.Service.ServiceID, 10)
			for _, weekDate := range plan.WeekDates {
				appDate := formatAvailabilityDate(weekDate)
				groups, err := c.Availability(ctx, businessID, serviceID, plan.ProviderID, appDate)
				if err != nil {
					return classifyVagaroError(err, flags)
				}
				for _, g := range filterAvailabilityGroups(groups, plan.ProviderID, plan.From, plan.To) {
					summary.TotalSlots += len(g.Times)
					summary.Groups = append(summary.Groups, g)
				}
			}
			if summary.TotalSlots == 0 {
				if plan.ProviderID != "" || opts.Service != "" || plan.ExplicitFrom || plan.ExplicitTo || opts.Weeks != 0 {
					summary.Note = "no availability found for the requested service/provider/date window"
				} else {
					summary.Note = "no availability found for the current week"
				}
			}
			return emitVagaro(cmd, flags, summary)
		},
	}
	cmd.Flags().StringVar(&opts.Service, "service", "", "Service name or ID to query (default: first service)")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Provider/staff name or ID to require (default: any provider)")
	cmd.Flags().StringVar(&opts.From, "from", "", "Start date YYYY-MM-DD or weekday name (default: today)")
	cmd.Flags().StringVar(&opts.To, "to", "", "End date YYYY-MM-DD or weekday name (default: end of selected week window)")
	cmd.Flags().IntVar(&opts.Weeks, "weeks", 0, "Number of weekly availability windows to query from --from (default: 1; cannot be combined with --to)")
	return cmd
}

func buildAvailabilityPlan(services []vagaro.ServiceRow, providers []vagaro.Provider, opts availabilityOptions, now time.Time) (availabilityPlan, error) {
	if len(services) == 0 {
		return availabilityPlan{}, fmt.Errorf("business has no bookable services")
	}
	if opts.Weeks < 0 {
		return availabilityPlan{}, fmt.Errorf("--weeks must be greater than 0")
	}
	if opts.Weeks > 0 && strings.TrimSpace(opts.To) != "" {
		return availabilityPlan{}, fmt.Errorf("use either --to or --weeks, not both")
	}

	today := dateOnly(now)
	from, err := resolveDay(opts.From, today, today)
	if err != nil {
		return availabilityPlan{}, err
	}
	from = dateOnly(from)

	weeks := opts.Weeks
	if weeks == 0 {
		weeks = 1
	}
	defaultTo := from.AddDate(0, 0, weeks*7-1)
	to, err := resolveDay(opts.To, today, defaultTo)
	if err != nil {
		return availabilityPlan{}, err
	}
	to = dateOnly(to)
	if to.Before(from) {
		return availabilityPlan{}, fmt.Errorf("--to must be on or after --from")
	}

	svc, err := resolveAvailabilityService(services, opts.Service)
	if err != nil {
		return availabilityPlan{}, err
	}
	providerID, providerName, err := resolveAvailabilityProvider(providers, opts.Provider)
	if err != nil {
		return availabilityPlan{}, err
	}

	return availabilityPlan{
		Service:      svc,
		ProviderID:   providerID,
		ProviderName: providerName,
		From:         from,
		To:           to,
		WeekDates:    availabilityWeekDates(from, to),
		ExplicitFrom: strings.TrimSpace(opts.From) != "",
		ExplicitTo:   strings.TrimSpace(opts.To) != "",
	}, nil
}

func resolveAvailabilityService(services []vagaro.ServiceRow, query string) (vagaro.ServiceRow, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return services[0], nil
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		for _, s := range services {
			if s.ServiceID == id {
				return s, nil
			}
		}
		return vagaro.ServiceRow{}, fmt.Errorf("service %q was not found for this business", query)
	}
	matches := make([]vagaro.ServiceRow, 0)
	q := strings.ToLower(query)
	for _, s := range services {
		if strings.EqualFold(s.ServiceTitle, query) {
			return s, nil
		}
		if strings.Contains(strings.ToLower(s.ServiceTitle), q) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return vagaro.ServiceRow{}, fmt.Errorf("service %q matched multiple services; pass a service ID", query)
	}
	return vagaro.ServiceRow{}, fmt.Errorf("service %q was not found for this business", query)
}

func resolveAvailabilityProvider(providers []vagaro.Provider, query string) (string, string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", "", nil
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		for _, p := range providers {
			if p.ServiceProviderID == id {
				return strconv.FormatInt(p.ServiceProviderID, 10), p.Name, nil
			}
		}
		return "", "", fmt.Errorf("provider %q was not found for this business", query)
	}
	matches := make([]vagaro.Provider, 0)
	q := strings.ToLower(query)
	for _, p := range providers {
		if strings.EqualFold(p.Name, query) {
			return strconv.FormatInt(p.ServiceProviderID, 10), p.Name, nil
		}
		if strings.Contains(strings.ToLower(p.Name), q) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return strconv.FormatInt(matches[0].ServiceProviderID, 10), matches[0].Name, nil
	}
	if len(matches) > 1 {
		return "", "", fmt.Errorf("provider %q matched multiple providers; pass a provider ID", query)
	}
	return "", "", fmt.Errorf("provider %q was not found for this business", query)
}

func availabilityWeekDates(from, to time.Time) []time.Time {
	weeks := []time.Time{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 7) {
		weeks = append(weeks, d)
	}
	return weeks
}

func filterAvailabilityGroups(groups []vagaro.SlotGroup, providerID string, from, to time.Time) []vagaro.SlotGroup {
	out := make([]vagaro.SlotGroup, 0, len(groups))
	for _, g := range groups {
		if providerID != "" && strings.TrimSpace(g.ProviderID) != providerID {
			continue
		}
		kept := g
		kept.Times = nil
		for _, slot := range g.Times {
			if dt, ok := vagaro.ParseSlotDateTime(g.Date, slot); ok {
				day := dateOnly(dt)
				if day.Before(from) || day.After(to) {
					continue
				}
			}
			kept.Times = append(kept.Times, slot)
		}
		if len(kept.Times) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

func formatAvailabilityDate(d time.Time) string {
	return d.Format("Mon Jan-02-2006")
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
