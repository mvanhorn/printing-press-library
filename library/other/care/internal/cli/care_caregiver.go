// Copyright 2026 beetz12. Licensed under Apache-2.0.
// `caregiver <id>` - full profile detail for one caregiver. Hand-authored;
// safe across regen. Reuses careGetCaregiver (care_outreach.go).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCareCaregiverCmd(flags *rootFlags) *cobra.Command {
	var service string
	cmd := &cobra.Command{
		Use:         "caregiver <id>",
		Short:       "Show a caregiver's full care.com profile",
		Long:        "Fetches one caregiver's full profile: experience, ages served, qualities/certifications, education, rate, and background-check status. Use the id from `find`.",
		Example:     "  care-pp-cli caregiver 44444444-4444-4444-4444-444444444444",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch caregiver %s\n", id)
				return nil
			}
			to := flags.timeout
			if to <= 0 {
				to = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), to)
			defer cancel()

			vars := map[string]any{
				"getCaregiverId":           id,
				"serviceId":                service,
				"shouldIncludeAllProfiles": true,
				"shouldGetMarkedAsHired":   false,
			}
			data, err := careGraphQL(ctx, flags, careQGetCaregiverOp, careQGetCaregiver, vars)
			if err != nil {
				return err
			}
			var wrap struct {
				G careGetCaregiver `json:"getCaregiver"`
			}
			if err := json.Unmarshal(data, &wrap); err != nil {
				return fmt.Errorf("parsing caregiver: %w", err)
			}
			g := wrap.G
			if g.Member.FirstName == "" && g.YearsOfExperience == 0 {
				return fmt.Errorf("no caregiver found for id %s (run: care-pp-cli auth refresh)", id)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), careCaregiverDetailJSON(id, g), flags)
			}
			return renderCareCaregiver(cmd, id, g)
		},
	}
	cmd.Flags().StringVar(&service, "service", "CHILD_CARE", "care.com service type")
	return cmd
}

func careCaregiverAges(g careGetCaregiver) []string {
	var ages []string
	for _, a := range g.Profiles.ChildCare.AgeGroups {
		if lbl, ok := careAgeGroupLabels[strings.ToUpper(a)]; ok {
			ages = append(ages, lbl)
		}
	}
	return ages
}

func careCaregiverCreds(g careGetCaregiver) []string {
	var creds []string
	for k, v := range g.Profiles.ChildCare.Qualities {
		if bv, ok := v.(bool); ok && bv {
			if lbl, ok := careQualityLabels[k]; ok {
				creds = append(creds, lbl)
			}
		}
	}
	sort.Strings(creds)
	return creds
}

func careCaregiverRate(g careGetCaregiver) float64 {
	if len(g.Profiles.ChildCare.Rates) > 0 {
		if v, err := strconv.ParseFloat(string(g.Profiles.ChildCare.Rates[0].HourlyRate.Amount), 64); err == nil {
			return v
		}
	}
	return 0
}

func careCaregiverDetailJSON(id string, g careGetCaregiver) map[string]any {
	out := map[string]any{
		"id":               id,
		"name":             strings.TrimSpace(g.Member.FirstName + " " + firstInitial(g.Member.LastName)),
		"years_experience": g.YearsOfExperience,
		"has_care_check":   g.HasCareCheck,
		"age_groups":       careCaregiverAges(g),
		"credentials":      careCaregiverCreds(g),
		"max_children":     g.Profiles.ChildCare.NumberOfChildren,
		"profile_url":      "https://www.care.com/app/vip/" + id,
	}
	if r := careCaregiverRate(g); r > 0 {
		out["hourly_rate"] = r
	}
	if len(g.EducationDegrees) > 0 {
		out["education"] = g.EducationDegrees[0].SchoolName
	}
	return out
}

func renderCareCaregiver(cmd *cobra.Command, id string, g careGetCaregiver) error {
	w := cmd.OutOrStdout()
	name := strings.TrimSpace(g.Member.FirstName + " " + firstInitial(g.Member.LastName))
	fmt.Fprintf(w, "%s\n", name)
	fmt.Fprintf(w, "  Experience:  %d years\n", g.YearsOfExperience)
	if ages := careCaregiverAges(g); len(ages) > 0 {
		fmt.Fprintf(w, "  Ages:        %s\n", careJoinList(ages))
	}
	if r := careCaregiverRate(g); r > 0 {
		fmt.Fprintf(w, "  Rate:        $%.0f/hr\n", r)
	}
	if g.Profiles.ChildCare.NumberOfChildren > 0 {
		fmt.Fprintf(w, "  Max kids:    %d\n", g.Profiles.ChildCare.NumberOfChildren)
	}
	if creds := careCaregiverCreds(g); len(creds) > 0 {
		fmt.Fprintf(w, "  Credentials: %s\n", careJoinList(creds))
	}
	if len(g.EducationDegrees) > 0 && g.EducationDegrees[0].SchoolName != "" {
		fmt.Fprintf(w, "  Education:   %s\n", g.EducationDegrees[0].SchoolName)
	}
	check := "not verified"
	if g.HasCareCheck {
		check = "verified"
	}
	fmt.Fprintf(w, "  Background:  %s\n", check)
	fmt.Fprintf(w, "  Profile:     https://www.care.com/app/vip/%s\n", id)
	return nil
}
