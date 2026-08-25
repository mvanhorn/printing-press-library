// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cli — intel.go is the hand-authored normalization + ClinicalTrials.gov
// query layer shared by every intelligence command (emerging, compare, risk,
// sponsors, hotspots, recruiting, phase3, report, watch, velocity). It turns the
// deeply-nested CT.gov protocolSection into one flat Trial model and centralizes
// the /studies query helpers so no command hand-rolls API paths.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// normalizedFields is the CT.gov field list every intelligence command requests.
// Keeping it narrow bounds response size (the full protocolSection is tens of KB
// per study) while covering every field the unified Trial model needs.
const normalizedFields = "NCTId,BriefTitle,OverallStatus,Phase,LeadSponsorName,LeadSponsorClass,LocationCountry,Condition,InterventionName,InterventionType,EnrollmentCount,StartDate,PrimaryCompletionDate,CompletionDate,LastUpdatePostDate,WhyStopped,HasResults,SecondaryId"

// Trial is the single normalized model every source maps into. It is the
// "Layer 2" unified structure: one shape regardless of which registry the row
// came from.
type Trial struct {
	NCTID         string   `json:"id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Phase         string   `json:"phase,omitempty"`
	Phases        []string `json:"phases,omitempty"`
	Conditions    []string `json:"conditions,omitempty"`
	Interventions []string `json:"interventions,omitempty"`
	Sponsor       string   `json:"sponsor,omitempty"`
	SponsorClass  string   `json:"sponsor_class,omitempty"`
	Countries     []string `json:"countries,omitempty"`
	StartDate     string   `json:"start_date,omitempty"`
	// PrimaryCompletionDate and CompletionDate stay omitempty so a trial with no
	// date posted yields an absent field, distinguishable from "not requested".
	PrimaryCompletionDate string        `json:"primary_completion_date,omitempty"`
	CompletionDate        string        `json:"completion_date,omitempty"`
	LastUpdate            string        `json:"last_update,omitempty"`
	Enrollment            int           `json:"enrollment"`
	WhyStopped            string        `json:"why_stopped,omitempty"`
	HasResults            bool          `json:"has_results"`
	Source                string        `json:"source"`
	SecondaryIDs          []SecondaryID `json:"secondary_ids,omitempty"`
}

// SecondaryID carries a cross-registry identifier (EU CTR, WHO, etc.) that
// CT.gov already records on each study — the free dedup key the merge engine
// uses to reconcile the same trial across sources.
type SecondaryID struct {
	ID     string `json:"id"`
	Type   string `json:"type,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// ctgovClient is the subset of *client.Client the intel layer needs. Declaring
// it as an interface keeps intel.go testable without a live API.
type ctgovClient interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// studiesEnvelope is the /studies response shape.
type studiesEnvelope struct {
	TotalCount    int               `json:"totalCount"`
	Studies       []json.RawMessage `json:"studies"`
	NextPageToken string            `json:"nextPageToken"`
}

// rawStudy mirrors the nested protocolSection fields the normalized model needs.
type rawStudy struct {
	HasResults      bool `json:"hasResults"`
	ProtocolSection struct {
		IdentificationModule struct {
			NCTID            string `json:"nctId"`
			BriefTitle       string `json:"briefTitle"`
			SecondaryIDInfos []struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Domain string `json:"domain"`
			} `json:"secondaryIdInfos"`
		} `json:"identificationModule"`
		StatusModule struct {
			OverallStatus   string `json:"overallStatus"`
			WhyStopped      string `json:"whyStopped"`
			StartDateStruct struct {
				Date string `json:"date"`
			} `json:"startDateStruct"`
			PrimaryCompletionDateStruct struct {
				Date string `json:"date"`
			} `json:"primaryCompletionDateStruct"`
			CompletionDateStruct struct {
				Date string `json:"date"`
			} `json:"completionDateStruct"`
			LastUpdatePostDateStruct struct {
				Date string `json:"date"`
			} `json:"lastUpdatePostDateStruct"`
		} `json:"statusModule"`
		SponsorCollaboratorsModule struct {
			LeadSponsor struct {
				Name  string `json:"name"`
				Class string `json:"class"`
			} `json:"leadSponsor"`
		} `json:"sponsorCollaboratorsModule"`
		DesignModule struct {
			Phases         []string `json:"phases"`
			EnrollmentInfo struct {
				Count int `json:"count"`
			} `json:"enrollmentInfo"`
		} `json:"designModule"`
		ConditionsModule struct {
			Conditions []string `json:"conditions"`
		} `json:"conditionsModule"`
		ArmsInterventionsModule struct {
			Interventions []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"interventions"`
		} `json:"armsInterventionsModule"`
		ContactsLocationsModule struct {
			Locations []struct {
				Country string `json:"country"`
			} `json:"locations"`
		} `json:"contactsLocationsModule"`
	} `json:"protocolSection"`
}

// normalizeStudy flattens one CT.gov study into the unified Trial.
func normalizeStudy(raw json.RawMessage) (Trial, bool) {
	var s rawStudy
	if err := json.Unmarshal(raw, &s); err != nil {
		return Trial{}, false
	}
	id := s.ProtocolSection.IdentificationModule.NCTID
	if id == "" {
		return Trial{}, false
	}
	t := Trial{
		NCTID:        id,
		Title:        cliutil.CleanText(s.ProtocolSection.IdentificationModule.BriefTitle),
		Status:       s.ProtocolSection.StatusModule.OverallStatus,
		Phases:       s.ProtocolSection.DesignModule.Phases,
		Phase:        strings.Join(s.ProtocolSection.DesignModule.Phases, "/"),
		Conditions:   s.ProtocolSection.ConditionsModule.Conditions,
		Sponsor:      cliutil.CleanText(s.ProtocolSection.SponsorCollaboratorsModule.LeadSponsor.Name),
		SponsorClass: s.ProtocolSection.SponsorCollaboratorsModule.LeadSponsor.Class,
		StartDate:    s.ProtocolSection.StatusModule.StartDateStruct.Date,

		PrimaryCompletionDate: s.ProtocolSection.StatusModule.PrimaryCompletionDateStruct.Date,
		CompletionDate:        s.ProtocolSection.StatusModule.CompletionDateStruct.Date,

		LastUpdate: s.ProtocolSection.StatusModule.LastUpdatePostDateStruct.Date,
		Enrollment: s.ProtocolSection.DesignModule.EnrollmentInfo.Count,
		WhyStopped: cliutil.CleanText(s.ProtocolSection.StatusModule.WhyStopped),
		HasResults: s.HasResults,
		Source:     "clinicaltrials.gov",
	}
	for _, iv := range s.ProtocolSection.ArmsInterventionsModule.Interventions {
		if iv.Name != "" {
			t.Interventions = append(t.Interventions, cliutil.CleanText(iv.Name))
		}
	}
	seenCountry := map[string]bool{}
	for _, loc := range s.ProtocolSection.ContactsLocationsModule.Locations {
		if loc.Country != "" && !seenCountry[loc.Country] {
			seenCountry[loc.Country] = true
			t.Countries = append(t.Countries, loc.Country)
		}
	}
	for _, sid := range s.ProtocolSection.IdentificationModule.SecondaryIDInfos {
		t.SecondaryIDs = append(t.SecondaryIDs, SecondaryID{ID: sid.ID, Type: sid.Type, Domain: sid.Domain})
	}
	return t, true
}

// ctgovParams builds the base query map for a term against CT.gov. termField
// selects which Essie query area the term lands in: "cond" for conditions,
// "intr" for interventions, "term" for general text, "spons" for sponsors.
func ctgovParams(termField, term string) map[string]string {
	p := map[string]string{
		"format":   "json",
		"fields":   normalizedFields,
		"pageSize": "200",
	}
	if term != "" {
		p["query."+termField] = term
	}
	return p
}

// ctgovCount returns the total number of studies matching params via the
// countTotal flag. It overrides pageSize to 1 so the count is cheap.
func ctgovCount(ctx context.Context, c ctgovClient, params map[string]string) (int, error) {
	q := map[string]string{"format": "json", "countTotal": "true", "pageSize": "1", "fields": "NCTId"}
	for k, v := range params {
		if strings.HasPrefix(k, "query.") || strings.HasPrefix(k, "filter.") {
			q[k] = v
		}
	}
	raw, err := c.Get(ctx, "/studies", q)
	if err != nil {
		return 0, err
	}
	var env studiesEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, fmt.Errorf("parsing count response: %w", err)
	}
	return env.TotalCount, nil
}

// ctgovFetch pulls up to maxStudies normalized trials matching params, paging
// through the cursor as needed. maxPages bounds scan effort independently of
// maxStudies so a command can cap both result size and API calls.
func ctgovFetch(ctx context.Context, c ctgovClient, params map[string]string, maxStudies, maxPages int) ([]Trial, error) {
	if maxPages <= 0 {
		maxPages = 1
	}
	q := map[string]string{}
	for k, v := range params {
		if v != "" {
			q[k] = v
		}
	}
	if q["format"] == "" {
		q["format"] = "json"
	}
	if q["fields"] == "" {
		q["fields"] = normalizedFields
	}
	out := make([]Trial, 0, maxStudies)
	token := ""
	for page := 0; page < maxPages; page++ {
		if token != "" {
			q["pageToken"] = token
		}
		raw, err := c.Get(ctx, "/studies", q)
		if err != nil {
			return out, err
		}
		var env studiesEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return out, fmt.Errorf("parsing studies response: %w", err)
		}
		for _, rs := range env.Studies {
			if t, ok := normalizeStudy(rs); ok {
				out = append(out, t)
				if len(out) >= maxStudies {
					return out, nil
				}
			}
		}
		if env.NextPageToken == "" {
			break
		}
		token = env.NextPageToken
	}
	return out, nil
}

// noResultsErr gives search/term commands a grep-style non-zero exit (code 3)
// when a query produced zero rows. Callers render the empty result to stdout
// first, then return this so the rendered output is preserved while the process
// still signals "no match" — matching how `risk` treats a not-found id and how
// the publish error-path gate expects a nonsense argument to fail.
func noResultsErr(term string) error {
	if strings.TrimSpace(term) == "" {
		return notFoundErr(fmt.Errorf("no results"))
	}
	return notFoundErr(fmt.Errorf("no results for %q", term))
}

// ctgovFetchOne fetches and normalizes a single study by NCT id via the
// /studies/{nctId} endpoint. The bare study object the endpoint returns has the
// same shape normalizeStudy expects, so it reuses the same flattening path. The
// bool reports whether the study parsed into a usable Trial (false on an empty
// or malformed record); the error is reserved for transport failures.
func ctgovFetchOne(ctx context.Context, c ctgovClient, nctID string) (Trial, bool, error) {
	nctID = strings.TrimSpace(nctID)
	if nctID == "" {
		return Trial{}, false, fmt.Errorf("an NCT id is required")
	}
	raw, err := c.Get(ctx, "/studies/"+nctID, map[string]string{"format": "json", "fields": normalizedFields})
	if err != nil {
		return Trial{}, false, err
	}
	t, ok := normalizeStudy(raw)
	return t, ok, nil
}

// counter is a small ordered frequency counter used by emerging/sponsors/hotspots.
type counter struct {
	counts map[string]int
}

func newCounter() *counter { return &counter{counts: map[string]int{}} }

func (c *counter) add(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	c.counts[key]++
}

// total returns the sum of all counts (i.e. how many times add recorded a
// non-empty key).
func (c *counter) total() int {
	sum := 0
	for _, v := range c.counts {
		sum += v
	}
	return sum
}

// rankedEntry is one (label, count) pair for sorted output.
type rankedEntry struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// top returns the n highest-count entries, ties broken alphabetically for
// deterministic output.
func (c *counter) top(n int) []rankedEntry {
	entries := make([]rankedEntry, 0, len(c.counts))
	for k, v := range c.counts {
		entries = append(entries, rankedEntry{Label: k, Count: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Label < entries[j].Label
	})
	if n > 0 && len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

// percentOf returns part/whole as a rounded percentage, 0 when whole is 0.
func percentOf(part, whole int) int {
	if whole <= 0 {
		return 0
	}
	return int(float64(part)*100.0/float64(whole) + 0.5)
}

// phaseLabel maps a CT.gov phase enum to a short human label.
func phaseLabel(phase string) string {
	switch phase {
	case "EARLY_PHASE1":
		return "Early Phase 1"
	case "PHASE1":
		return "Phase 1"
	case "PHASE2":
		return "Phase 2"
	case "PHASE3":
		return "Phase 3"
	case "PHASE4":
		return "Phase 4"
	case "NA", "":
		return "N/A"
	default:
		return phase
	}
}

// resultEnvelope is the standard JSON shape intelligence commands emit: the
// computed payload plus a meta block describing scan effort and partial
// failures so agents can distinguish "no data" from "scan capped" or
// "source degraded".
type resultEnvelope struct {
	Query   string         `json:"query,omitempty"`
	Summary map[string]any `json:"summary,omitempty"`
	Items   any            `json:"items,omitempty"`
	Note    string         `json:"note,omitempty"`
	Sources []string       `json:"sources,omitempty"`
	Errors  []string       `json:"errors,omitempty"`
}
