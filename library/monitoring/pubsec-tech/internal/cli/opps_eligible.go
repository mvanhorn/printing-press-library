// Set-aside eligibility filter — transcendence feature 8.
//
// Given a SAM-registered UEI, filters open opportunities to those whose
// set-aside categories the entity actually qualifies for. The entity
// socioeconomic indicators live in the SAM entity record; the opportunity
// set-aside taxonomy lives in the SAM opportunity record. Joining the two
// is the differentiator — no MCP in the ecosystem scan does this.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

func newOppsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opps",
		Short: "Opportunity workflow shortcuts (eligibility match, deadline triage)",
		Long: "Hand-built opportunity workflow commands that compose against the local SAM " +
			"opportunities and entities tables. See `opportunities` for the generator-emitted " +
			"endpoint-mirror surface.",
		Example: "  pubsec-tech-pp-cli opps eligible ABC1234567 --posted-from 14d --agent",
	}
	cmd.AddCommand(newOppsEligibleCmd(flags))
	return cmd
}

func newOppsEligibleCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "eligible <uei>",
		Short: "Filter open opportunities to those whose set-aside matches the UEI's socioeconomic indicators",
		Long: "Reads the SAM-registered entity's socioeconomic profile for the UEI from the " +
			"local entities table, then filters opportunities whose typeOfSetAsideDescription " +
			"matches one of the entity's qualifying categories (8(a), SDVOSB, WOSB, HUBZone, etc.). " +
			"Requires both `entities` and `opportunities` synced into the local store.",
		Example:     "  pubsec-tech-pp-cli opps eligible ABC1234567 --posted-from 14d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			sinceT, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			result, err := buildEligibilityFilter(ctx, s, args[0], sinceT, limit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "UEI: %s\n", result.UEI)
			fmt.Fprintf(w, "Entity name: %s\n", result.EntityName)
			fmt.Fprintf(w, "Qualifying set-asides: %s\n", strings.Join(result.QualifyingSetAsides, ", "))
			fmt.Fprintf(w, "\nMatching opportunities (%d):\n", result.MatchCount)
			for _, o := range result.Matches {
				fmt.Fprintf(w, "  - %s  set-aside=%s  %s\n", o.ID, o.SetAside, truncate(o.Title, 80))
			}
			for _, n := range result.Notes {
				fmt.Fprintf(w, "\n• %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "posted-from", "30d", "Look at opportunities posted since this window")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches to return")
	return cmd
}

// EligibilityResult is the structured response for `opps eligible <uei>`.
type EligibilityResult struct {
	UEI                 string             `json:"uei"`
	EntityName          string             `json:"entity_name,omitempty"`
	QualifyingSetAsides []string           `json:"qualifying_set_asides"`
	Matches             []eligibilityMatch `json:"matches,omitempty"`
	MatchCount          int                `json:"match_count"`
	GeneratedAt         time.Time          `json:"generated_at"`
	Notes               []string           `json:"notes,omitempty"`
}

type eligibilityMatch struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	SetAside string `json:"set_aside,omitempty"`
}

// buildEligibilityFilter loads the entity by UEI, extracts the socioeconomic
// indicator strings, maps them to the SAM set-aside codes, then scans the
// opportunities table for matches.
func buildEligibilityFilter(ctx context.Context, s *store.Store, uei string, since time.Time, limit int) (*EligibilityResult, error) {
	r := &EligibilityResult{UEI: uei, GeneratedAt: time.Now().UTC()}
	// Pull the entity record by UEI from the generic resources store.
	entityJSON, err := s.Get("entities", uei)
	if err != nil {
		r.Notes = append(r.Notes, fmt.Sprintf("entity %s not in local store; run `sync --resources entities` first", uei))
	} else if entityJSON != nil {
		var m map[string]any
		if json.Unmarshal(entityJSON, &m) == nil {
			// Entity name
			if cd, ok := m["coreData"].(map[string]any); ok {
				if e, _ := cd["entityRegistration"].(map[string]any); e != nil {
					if name, _ := e["legalBusinessName"].(string); name != "" {
						r.EntityName = name
					}
				}
			}
			// Socioeconomic indicators
			r.QualifyingSetAsides = extractQualifyingSetAsides(m)
		}
	}
	if len(r.QualifyingSetAsides) == 0 && r.Notes == nil {
		r.Notes = append(r.Notes, "no qualifying set-asides found on this entity (entity may not have a socioeconomic profile in SAM)")
	}
	// Scan opportunities for matching set-aside
	// Best-effort: ListIDs + Get each opportunity. For large stores, this
	// should be replaced with a typed query on opportunities.set_aside once
	// that column is added to the schema.
	ids, _ := s.ListIDs("opportunities")
	qualifySet := map[string]bool{}
	for _, q := range r.QualifyingSetAsides {
		qualifySet[strings.ToUpper(q)] = true
	}
	for _, id := range ids {
		data, err := s.Get("opportunities", id)
		if err != nil || data == nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		setAside, _ := m["typeOfSetAside"].(string)
		desc, _ := m["typeOfSetAsideDescription"].(string)
		if !qualifySet[strings.ToUpper(setAside)] && !qualifySet[strings.ToUpper(desc)] {
			continue
		}
		// Date filter
		posted, _ := m["postedDate"].(string)
		if posted != "" {
			if t, err := time.Parse("2006-01-02", posted); err == nil && t.Before(since) {
				continue
			}
		}
		mt := eligibilityMatch{ID: id, SetAside: setAside}
		if t, _ := m["title"].(string); t != "" {
			mt.Title = t
		}
		r.Matches = append(r.Matches, mt)
		if limit > 0 && len(r.Matches) >= limit {
			break
		}
	}
	r.MatchCount = len(r.Matches)
	if r.MatchCount == 0 && len(r.Notes) == 0 {
		r.Notes = append(r.Notes, "no matching opportunities; run `opportunities search` to populate the local store")
	}
	return r, nil
}

// extractQualifyingSetAsides reads the SAM entity's socioeconomic indicators
// and returns the set-aside category codes the entity qualifies for. SAM
// represents this in coreData.businessTypes.businessTypeList (an array of
// codes like "8A", "WOSB", "HUBZ", "SDVOSB", "VOSB", etc.) plus a few extras
// in the qualifications structure.
func extractQualifyingSetAsides(entity map[string]any) []string {
	var out []string
	cd, _ := entity["coreData"].(map[string]any)
	if cd == nil {
		return out
	}
	bt, _ := cd["businessTypes"].(map[string]any)
	if bt == nil {
		return out
	}
	if list, ok := bt["businessTypeList"].([]any); ok {
		for _, raw := range list {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if code, _ := m["businessTypeCode"].(string); code != "" {
				out = append(out, code)
			}
		}
	}
	// Map common business type strings to SAM set-aside categories
	mapping := map[string]string{
		"8A":      "8A",
		"SDB":     "SBA",
		"SDVOSB":  "SDVOSBC",
		"VOSB":    "VSA",
		"WOSB":    "WOSB",
		"EDWOSB":  "EDWOSB",
		"HUBZ":    "HZC",
		"HUBZONE": "HZC",
	}
	expanded := make([]string, 0, len(out)*2)
	seen := map[string]bool{}
	for _, c := range out {
		c = strings.ToUpper(c)
		if !seen[c] {
			seen[c] = true
			expanded = append(expanded, c)
		}
		if alt, ok := mapping[c]; ok && !seen[alt] {
			seen[alt] = true
			expanded = append(expanded, alt)
		}
	}
	return expanded
}
