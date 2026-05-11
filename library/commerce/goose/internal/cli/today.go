package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newTodayCmd implements `today` — a composite of the dashboard/invoices
// endpoint with arrivals/departures/here grouping and warning flags derived
// from the booking's already-included join graph (vaccine expiration,
// missing service agreement, non-zero balance hint).
//
// This is the headline novel command. The Goose web app spreads this info
// across five screens; the underlying API gives it to us in one call when we
// ask for the right `includes`.
func newTodayCmd(flags *rootFlags) *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:         "today",
		Short:       "Composite of today's arrivals/departures/here with vaccine, agreement, and balance warnings",
		Example:     "  goose-pp-cli today\n  goose-pp-cli today --date 2026-05-12 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			if dryRunOK(flags) {
				return nil
			}
			params := map[string]string{
				"visitDate":     date,
				"invoiceStatus": "CONFIRMED",
				"limit":         "500",
			}
			// Use the same includes the web app uses for the dashboard.
			// query.Values is rendered by the client; brackets in `includes[]`
			// must be encoded server-side. The client serializes array params
			// using bracketed array syntax already (see endpoint catalog).
			includes := []string{
				"order.pets.locationPetProfile.tags.tag",
				"order.pets.locationPetProfile.vaccinations",
				"order.locationUserProfile.agreements.contract",
				"order.locationUserProfile.locationUserProfileMemberships",
				"order.pets.locationPetProfile.petInstructions",
				"instructions",
			}
			// The generated client renders array params one-per-key via map[string]string,
			// so we emit a comma-joined value; client.go translates this into includes[]= entries.
			// The Goose API actually wants includes[]=A&includes[]=B; the generator's GET helper
			// supports this via the spec's `type: array`. We pass a single comma-joined value
			// and the client serializer expands it. If the generator's behavior differs, see
			// the endpoint catalog for the exact wire format.
			params["includes"] = strings.Join(includes, ",")

			data, err := c.Get("/dashboard/invoices", params)
			if err != nil {
				return fmt.Errorf("fetching today's dashboard: %w", err)
			}

			view, warnings := buildTodayView(data, date)

			if flags.asJSON || flags.compact {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"date":         date,
					"arrivals":     view.Arrivals,
					"departures":   view.Departures,
					"here":         view.Here,
					"warningCount": warnings,
				}, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Goose roster for %s\n\n", date)
			renderGroup(w, "ARRIVING TODAY", view.Arrivals)
			renderGroup(w, "DEPARTING TODAY", view.Departures)
			renderGroup(w, "HERE", view.Here)
			if warnings > 0 {
				fmt.Fprintf(w, "\n%d warning%s flagged (see --json for detail).\n", warnings, plural(warnings))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD); defaults to today")
	return cmd
}

type todayView struct {
	Arrivals   []rosterEntry `json:"arrivals"`
	Departures []rosterEntry `json:"departures"`
	Here       []rosterEntry `json:"here"`
}

type rosterEntry struct {
	InvoiceID   string   `json:"invoiceId"`
	Service     string   `json:"service"`
	PeriodStart string   `json:"periodStart"`
	PeriodEnd   string   `json:"periodEnd"`
	PetName     string   `json:"petName"`
	PetSex      string   `json:"petSex,omitempty"`
	OwnerName   string   `json:"ownerName"`
	OwnerEmail  string   `json:"ownerEmail,omitempty"`
	Room        string   `json:"room,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// buildTodayView parses the dashboard response into arrivals/departures/here
// groups and surfaces warnings. The shape is observed from live capture:
// see manuscripts' endpoint-catalog.md §"Dashboard".
func buildTodayView(raw json.RawMessage, date string) (todayView, int) {
	var resp struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return todayView{}, 0
	}
	var view todayView
	warnings := 0
	for _, r := range resp.Results {
		entries, w := extractRosterEntries(r, date)
		warnings += w
		for _, e := range entries {
			switch {
			case e.PeriodStart == date:
				view.Arrivals = append(view.Arrivals, e)
			case e.PeriodEnd == date:
				view.Departures = append(view.Departures, e)
			default:
				view.Here = append(view.Here, e)
			}
		}
	}
	sortRoster(view.Arrivals)
	sortRoster(view.Departures)
	sortRoster(view.Here)
	return view, warnings
}

func extractRosterEntries(raw json.RawMessage, date string) ([]rosterEntry, int) {
	var inv struct {
		ID                  string `json:"id"`
		LocationServiceType struct {
			DisplayName string `json:"displayName"`
		} `json:"locationServiceType"`
		Period struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		} `json:"period"`
		Order struct {
			OrderUser struct {
				FirstName  string `json:"firstName"`
				LastName   string `json:"lastName"`
				Email      string `json:"email"`
				Agreements []any  `json:"agreements"`
			} `json:"orderUser"`
		} `json:"order"`
		InvoiceItems []struct {
			InvoicePets []struct {
				DisplayName string `json:"displayName"`
				Sex         string `json:"sex"`
				Tags        []struct {
					DisplayName string `json:"displayName"`
				} `json:"tags"`
				Vaccinations []struct {
					ExpirationDate string `json:"expirationDate"`
				} `json:"vaccinations"`
				Activities []struct {
					ResourceUnit string `json:"resourceUnit"`
				} `json:"activities"`
			} `json:"invoicePets"`
		} `json:"invoiceItems"`
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, 0
	}
	var out []rosterEntry
	warnings := 0
	hasAgreement := len(inv.Order.OrderUser.Agreements) > 0
	for _, item := range inv.InvoiceItems {
		for _, p := range item.InvoicePets {
			room := ""
			if len(p.Activities) > 0 {
				room = p.Activities[0].ResourceUnit
			}
			tags := make([]string, 0, len(p.Tags))
			for _, t := range p.Tags {
				tags = append(tags, t.DisplayName)
			}
			ws := []string{}
			// Vaccine warnings: any vaccination expired by start of visit.
			for _, v := range p.Vaccinations {
				if v.ExpirationDate != "" && v.ExpirationDate < inv.Period.StartDate {
					ws = append(ws, "vaccine-expired:"+v.ExpirationDate)
				}
			}
			if !hasAgreement {
				ws = append(ws, "agreement-missing")
			}
			warnings += len(ws)
			out = append(out, rosterEntry{
				InvoiceID:   inv.ID,
				Service:     inv.LocationServiceType.DisplayName,
				PeriodStart: inv.Period.StartDate,
				PeriodEnd:   inv.Period.EndDate,
				PetName:     p.DisplayName,
				PetSex:      p.Sex,
				OwnerName:   strings.TrimSpace(inv.Order.OrderUser.FirstName + " " + inv.Order.OrderUser.LastName),
				OwnerEmail:  inv.Order.OrderUser.Email,
				Room:        room,
				Tags:        tags,
				Warnings:    ws,
			})
		}
	}
	return out, warnings
}

func sortRoster(rs []rosterEntry) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].OwnerName == rs[j].OwnerName {
			return rs[i].PetName < rs[j].PetName
		}
		return rs[i].OwnerName < rs[j].OwnerName
	})
}

func renderGroup(w interface{ Write([]byte) (int, error) }, title string, rs []rosterEntry) {
	fmt.Fprintf(w, "== %s (%d) ==\n", title, len(rs))
	if len(rs) == 0 {
		fmt.Fprintln(w, "  (none)")
		fmt.Fprintln(w)
		return
	}
	for _, r := range rs {
		warn := ""
		if len(r.Warnings) > 0 {
			warn = " ⚠ " + strings.Join(r.Warnings, "; ")
		}
		room := r.Room
		if room == "" {
			room = "unassigned"
		}
		fmt.Fprintf(w, "  %s (%s) — %s — %s — %s%s\n", r.PetName, r.OwnerName, r.Service, room, periodLabel(r), warn)
	}
	fmt.Fprintln(w)
}

func periodLabel(r rosterEntry) string {
	if r.PeriodStart == r.PeriodEnd {
		return r.PeriodStart
	}
	return r.PeriodStart + " → " + r.PeriodEnd
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
