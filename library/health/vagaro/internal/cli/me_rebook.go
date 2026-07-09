// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: "re-run my usual". Reads a past appointment (auth), extracts
// its business + service + provider, and lists that provider's open slots in a
// window so the user can pick one and book. Cross-source join between the
// authed myaccount appointments API and the public availability surface.
// generate --force preserves this body.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// appointmentsPath is the authed myaccount appointments endpoint.
const appointmentsPath = "https://api.vagaro.com/us02/api/v2/myaccount/purchases/appointments"

// rebookAppointment is the business/service/provider triple pulled from a past
// appointment.
type rebookAppointment struct {
	AppointmentID string `json:"appointment_id,omitempty"`
	BusinessID    string `json:"business_id,omitempty"`
	BusinessSlug  string `json:"business_slug,omitempty"`
	BusinessName  string `json:"business_name,omitempty"`
	ServiceID     string `json:"service_id,omitempty"`
	ServiceName   string `json:"service_name,omitempty"`
	ProviderID    string `json:"provider_id,omitempty"`
	ProviderName  string `json:"provider_name,omitempty"`
	Date          string `json:"date,omitempty"`
}

type rebookResult struct {
	Source     rebookAppointment  `json:"based_on_appointment"`
	Window     string             `json:"window"`
	TotalSlots int                `json:"total_slots"`
	Groups     []vagaro.SlotGroup `json:"open_slots"`
	NextStep   string             `json:"next_step,omitempty"`
	Note       string             `json:"note,omitempty"`
}

// pp:data-source live
func newNovelMeRebookCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLast bool
		flagFrom string
		flagTo   string
	)

	cmd := &cobra.Command{
		Use:   "rebook [<appointment-id>]",
		Short: "Re-run your usual: read a past appointment (business + service + provider) and list that provider's open slots in a window.",
		Long: `Read your past appointments (requires auth), take the most recent one (or the
named appointment id), and list that same provider's open slots in a date
window so you can pick a time and book it.

Requires auth: run 'vagaro-pp-cli auth login --chrome' first. This command only
reads and lists slots — use 'vagaro-pp-cli book' to place the appointment.`,
		Example:     "  vagaro-pp-cli me rebook --last --from thu --to sat",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			wantID := ""
			if len(args) > 0 {
				wantID = strings.TrimSpace(args[0])
			}

			now := time.Now()
			fromDate, err := resolveDay(flagFrom, now, now)
			if err != nil {
				return usageErr(err)
			}
			toDate, err := resolveDay(flagTo, now, fromDate.AddDate(0, 0, 6))
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Read past appointments via the authed myaccount API. // pp:client-call
			gc, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"pageSize": 12, "pageNumber": 1, "pastAppointment": true,
				"myOrSharedAppointments": 1, "device": "Website", "module": "MyAccount",
				"version": "2.5.3", "brandedApp": false, "multiLocation": false,
			}
			data, status, err := gc.PostQueryWithParams(ctx, appointmentsPath, map[string]string{}, body)
			if err != nil {
				if looksLikeAuthDenied(err.Error()) || status == 401 || status == 403 {
					return authErr(rebookLoginHint(err))
				}
				return classifyAPIError(err, flags)
			}
			// A 2xx with an access-denied envelope means the session isn't authed.
			if looksLikeAuthDenied(string(data)) {
				return authErr(rebookLoginHint(fmt.Errorf("the appointments API denied access")))
			}
			appts, perr := parseAppointments(data)
			if perr != nil {
				return authErr(rebookLoginHint(fmt.Errorf("could not read appointments (status %d): %w", status, perr)))
			}
			if len(appts) == 0 {
				return emitVagaro(cmd, flags, rebookResult{
					Window: fmt.Sprintf("%s .. %s", fromDate.Format("2006-01-02"), toDate.Format("2006-01-02")),
					Groups: []vagaro.SlotGroup{},
					Note:   "no past appointments found on this account (or not logged in). Run 'vagaro-pp-cli auth login --chrome' if you expected history.",
				})
			}

			chosen := appts[0]
			if wantID != "" {
				found := false
				for _, a := range appts {
					if a.AppointmentID == wantID {
						chosen = a
						found = true
						break
					}
				}
				if !found {
					return notFoundErr(fmt.Errorf("appointment %q not found in your recent history", wantID))
				}
			}

			out := rebookResult{
				Source: chosen,
				Window: fmt.Sprintf("%s .. %s", fromDate.Format("2006-01-02"), toDate.Format("2006-01-02")),
				Groups: []vagaro.SlotGroup{},
			}
			if chosen.BusinessID == "" && chosen.BusinessSlug == "" && chosen.BusinessName == "" {
				out.Note = rebookDiagnostic(chosen, "the past appointment did not include a resolvable business id, Vagaro business URL, or business name")
				return emitVagaro(cmd, flags, out)
			}

			vc := newVagaroClient(flags)
			businessID, businessSlug, err := resolveRebookBusiness(ctx, vc, flags, chosen)
			if err != nil {
				out.Note = rebookDiagnostic(chosen, err.Error())
				return emitVagaro(cmd, flags, out)
			}
			services, err := vc.Services(ctx, businessID)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			service, err := resolveRebookService(services, chosen)
			if err != nil {
				out.Note = rebookDiagnostic(chosen, err.Error())
				return emitVagaro(cmd, flags, out)
			}
			providerID := ""
			if strings.TrimSpace(chosen.ProviderID) != "" || strings.TrimSpace(chosen.ProviderName) != "" {
				providers, err := vc.Staff(ctx, businessID)
				if err != nil {
					return classifyVagaroError(err, flags)
				}
				providerID, _, err = resolveRebookProvider(providers, chosen)
				if err != nil {
					out.Note = rebookDiagnostic(chosen, err.Error())
					return emitVagaro(cmd, flags, out)
				}
			}
			serviceID := strconv.FormatInt(service.ServiceID, 10)
			for _, weekDate := range availabilityWeekDates(fromDate, toDate) {
				groups, err := vc.Availability(ctx, businessID, serviceID, providerID, formatAvailabilityDate(weekDate))
				if err != nil {
					return classifyVagaroError(err, flags)
				}
				out.Groups = append(out.Groups, filterAvailabilityGroups(groups, providerID, dateOnly(fromDate), dateOnly(toDate))...)
			}
			for _, g := range out.Groups {
				out.TotalSlots += len(g.Times)
			}
			if out.TotalSlots == 0 {
				out.Note = "no open slots for that provider in the window — widen --from/--to"
			} else {
				svcArg := serviceID
				provArg := ""
				if providerID != "" {
					provArg = " --provider " + providerID
				}
				out.NextStep = fmt.Sprintf("book with: vagaro-pp-cli book %s --service %s%s --at <YYYY-MM-DDTHH:MM>",
					firstNonEmpty(businessSlug, chosen.BusinessName, businessID), svcArg, provArg)
			}
			return emitVagaro(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&flagLast, "last", false, "Use the most recent past appointment (default behavior)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start: weekday (mon..sun) or YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end: weekday (mon..sun) or YYYY-MM-DD (default +6 days)")
	return cmd
}

// looksLikeAuthDenied reports whether a response body or error string carries
// one of the Vagaro myaccount "not authenticated" markers (HTTP 401/403 bodies
// or the 2xx "Access denied" / responseCode 1044 envelope).
func looksLikeAuthDenied(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "access denied") ||
		strings.Contains(l, "1044") ||
		strings.Contains(l, "unauthorized") ||
		strings.Contains(l, "not authenticated")
}

// rebookLoginHint wraps an underlying cause with the standard login hint.
func rebookLoginHint(cause error) error {
	return fmt.Errorf("reading appointments requires an authenticated session; "+
		"run 'vagaro-pp-cli auth login --chrome' and retry: %w", cause)
}

// parseAppointments extracts business/service/provider triples from the authed
// appointments payload. The envelope is {status,responseCode,message,data:[...]}
// but the per-appointment shape varies, so fields are pulled defensively by
// scanning top-level and nested objects for id/name-ish keys.
func parseAppointments(data json.RawMessage) ([]rebookAppointment, error) {
	// Unwrap the envelope's data array; fall back to a bare array.
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &env); err == nil && env.Data != nil {
		raws = env.Data
	} else if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("unexpected appointments payload: %w", err)
	}
	out := make([]rebookAppointment, 0, len(raws))
	for _, r := range raws {
		var obj map[string]any
		if json.Unmarshal(r, &obj) != nil {
			continue
		}
		out = append(out, extractAppointment(obj))
	}
	return out, nil
}

func extractAppointment(obj map[string]any) rebookAppointment {
	a := rebookAppointment{
		AppointmentID: firstScalar(obj, "appointmentId", "appointmentID", "appointment_id", "appNo", "id"),
		BusinessID:    firstScalar(obj, "businessId", "businessID", "business_id", "merchantId"),
		BusinessSlug:  firstScalar(obj, "businessSlug", "business_slug", "shopSlug", "merchantSlug", "slug", "vagaroURL", "vagaroUrl", "vagaro_url"),
		BusinessName:  firstScalar(obj, "businessName", "business_name", "merchantName"),
		ServiceID:     firstScalar(obj, "serviceId", "serviceID", "service_id"),
		ServiceName:   firstScalar(obj, "serviceName", "serviceTitle", "service_name"),
		ProviderID:    firstScalar(obj, "serviceProviderId", "serviceProviderID", "providerId", "provider_id", "spId"),
		ProviderName:  firstScalar(obj, "serviceProviderName", "providerName", "provider_name", "staffName"),
		Date:          firstScalar(obj, "appointmentDate", "startDate", "date", "appDate"),
	}
	if a.ProviderName == "" {
		a.ProviderName = strings.TrimSpace(firstScalar(obj, "serviceProviderFirstName", "providerFirstName") + " " + firstScalar(obj, "serviceProviderLastName", "providerLastName"))
	}
	a.BusinessSlug = firstNonEmpty(a.BusinessSlug, slugFromAppointmentURL(firstScalar(obj, "businessUrl", "businessURL", "business_url", "shopUrl", "shopURL", "bookingUrl", "bookingURL")))
	// Descend into nested objects when flat keys were absent. Always check for a
	// nested business URL/slug because the history payload may include flat ids
	// but only expose the public Vagaro slug under business.url.
	if nested, ok := obj["business"].(map[string]any); ok {
		a.BusinessID = firstNonEmpty(a.BusinessID, firstScalar(nested, "id", "businessId", "businessID"))
		a.BusinessName = firstNonEmpty(a.BusinessName, firstScalar(nested, "name", "businessName"))
		a.BusinessSlug = firstNonEmpty(a.BusinessSlug, firstScalar(nested, "slug", "businessSlug"), slugFromAppointmentURL(firstScalar(nested, "url", "businessUrl", "bookingUrl")))
	}
	if a.ServiceID == "" || a.ServiceName == "" {
		if nested, ok := obj["service"].(map[string]any); ok {
			a.ServiceID = firstNonEmpty(a.ServiceID, firstScalar(nested, "id", "serviceId", "serviceID"))
			a.ServiceName = firstNonEmpty(a.ServiceName, firstScalar(nested, "name", "serviceName", "title"))
		}
	}
	if a.ProviderID == "" || a.ProviderName == "" {
		for _, key := range []string{"serviceProvider", "provider", "staff"} {
			if nested, ok := obj[key].(map[string]any); ok {
				a.ProviderID = firstNonEmpty(a.ProviderID, firstScalar(nested, "id", "serviceProviderId", "providerId"))
				a.ProviderName = firstNonEmpty(a.ProviderName, firstScalar(nested, "name", "providerName"))
			}
		}
	}
	return a
}

func resolveRebookBusiness(ctx context.Context, c *vagaro.Client, flags *rootFlags, a rebookAppointment) (string, string, error) {
	if a.BusinessSlug != "" {
		id, err := resolveBusinessID(ctx, c, flags, a.BusinessSlug)
		return id, a.BusinessSlug, err
	}
	if strings.TrimSpace(a.BusinessName) != "" {
		if slug, id, err := resolveRebookBusinessFromCache(ctx, a.BusinessName); err != nil {
			return "", "", err
		} else if slug != "" {
			if id != "" {
				return id, slug, nil
			}
			id, err := resolveBusinessID(ctx, c, flags, slug)
			return id, slug, err
		}
	}
	if strings.TrimSpace(a.BusinessID) != "" {
		return strings.TrimSpace(a.BusinessID), "", nil
	}
	return "", "", fmt.Errorf("business name %q was not found in the local Vagaro cache; run 'vagaro-pp-cli sync' first or pass an explicit business slug", a.BusinessName)
}

func resolveRebookBusinessFromCache(ctx context.Context, name string) (string, string, error) {
	db, err := openStoreForRead(ctx, "vagaro-pp-cli")
	if err != nil || db == nil {
		return "", "", nil
	}
	defer db.Close()
	businesses, err := db.ListBusinesses(ctx)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return "", "", nil
		}
		return "", "", err
	}
	want := normalizeRebookBusinessName(name)
	var exactSlug, exactID string
	for _, b := range businesses {
		if normalizeRebookBusinessName(b.Name) == want {
			if exactSlug != "" {
				return "", "", fmt.Errorf("business name %q matched multiple cached businesses; pass an explicit business slug", name)
			}
			exactSlug, exactID = b.Slug, b.BusinessID
		}
	}
	if exactSlug != "" {
		return exactSlug, exactID, nil
	}
	var matchSlug, matchID string
	for _, b := range businesses {
		have := normalizeRebookBusinessName(b.Name)
		if have == "" || (want != "" && !strings.Contains(want, have) && !strings.Contains(have, want)) {
			continue
		}
		if matchSlug != "" {
			return "", "", fmt.Errorf("business name %q matched multiple cached businesses; pass an explicit business slug", name)
		}
		matchSlug, matchID = b.Slug, b.BusinessID
	}
	return matchSlug, matchID, nil
}

func normalizeRebookBusinessName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func resolveRebookService(services []vagaro.ServiceRow, a rebookAppointment) (vagaro.ServiceRow, error) {
	if id, err := strconv.ParseInt(strings.TrimSpace(a.ServiceID), 10, 64); err == nil {
		for _, s := range services {
			if s.ServiceID == id {
				return s, nil
			}
		}
	}
	if strings.TrimSpace(a.ServiceName) != "" {
		return resolveAvailabilityService(services, a.ServiceName)
	}
	if strings.TrimSpace(a.ServiceID) != "" {
		return vagaro.ServiceRow{}, fmt.Errorf("appointment service id could not be resolved to a public availability service; no service name was available for fallback")
	}
	return vagaro.ServiceRow{}, fmt.Errorf("the past appointment did not include a service id or service name")
}

func resolveRebookProvider(providers []vagaro.Provider, a rebookAppointment) (string, string, error) {
	if id, err := strconv.ParseInt(strings.TrimSpace(a.ProviderID), 10, 64); err == nil {
		for _, p := range providers {
			if p.ServiceProviderID == id {
				return strconv.FormatInt(p.ServiceProviderID, 10), p.Name, nil
			}
		}
	}
	if strings.TrimSpace(a.ProviderName) != "" {
		return resolveAvailabilityProvider(providers, a.ProviderName)
	}
	if strings.TrimSpace(a.ProviderID) != "" {
		return "", "", fmt.Errorf("appointment provider id could not be resolved to a public availability provider; no provider name was available for fallback")
	}
	return "", "", nil
}

func rebookDiagnostic(a rebookAppointment, reason string) string {
	slug := firstNonEmpty(a.BusinessSlug, "<business-slug>")
	service := firstNonEmpty(a.ServiceName, "<service-name>")
	provider := firstNonEmpty(a.ProviderName, "<provider-name>")
	return fmt.Sprintf("%s. Try the explicit query: vagaro-pp-cli business availability %s --service %q --provider %q --from <YYYY-MM-DD> --to <YYYY-MM-DD> --agent", reason, slug, service, provider)
}

func slugFromAppointmentURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimLeft(raw, "/")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "" && host != "vagaro.com" && !strings.HasSuffix(host, ".vagaro.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.EqualFold(p, "users") || strings.EqualFold(p, "book-now") {
			continue
		}
		return p
	}
	return ""
}

// firstScalar returns the first present key's value rendered as a string.
// Numeric values are rendered without a trailing ".0" so ids stay clean.
func firstScalar(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		for objKey, v := range obj {
			if !strings.EqualFold(objKey, k) {
				continue
			}
			switch val := v.(type) {
			case string:
				if s := strings.TrimSpace(val); s != "" {
					return s
				}
			case float64:
				if val != 0 {
					return strings.TrimSuffix(fmt.Sprintf("%.0f", val), ".0")
				}
			case json.Number:
				if s := val.String(); s != "" && s != "0" {
					return s
				}
			}
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
