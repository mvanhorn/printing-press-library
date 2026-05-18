// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// companyfacts JSON → edgar_xbrl_facts table population.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
)

// companyFactsResp models data.sec.gov/api/xbrl/companyfacts/CIK<cik>.json
type companyFactsResp struct {
	CIK        int    `json:"cik"`
	EntityName string `json:"entityName"`
	Facts      struct {
		USGAAP map[string]struct {
			Label string `json:"label"`
			Units map[string][]struct {
				End   string  `json:"end"`
				Val   float64 `json:"val"`
				FY    int     `json:"fy"`
				FP    string  `json:"fp"`
				Form  string  `json:"form"`
				Filed string  `json:"filed"`
			} `json:"units"`
		} `json:"us-gaap"`
	} `json:"facts"`
}

// syncCompanyFactsForCIK fetches companyfacts JSON and populates edgar_xbrl_facts.
// Best-effort: errors are returned but caller may choose to continue.
func syncCompanyFactsForCIK(ctx context.Context, c *client.Client, db *store.Store, cik string) error {
	if err := requireEdgarUA(c); err != nil {
		return err
	}
	body, _, err := fetchAbsoluteRaw(ctx, c,
		fmt.Sprintf("https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json", cik))
	if err != nil {
		return fmt.Errorf("fetching companyfacts: %w", err)
	}
	var resp companyFactsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing companyfacts: %w", err)
	}
	for concept, conceptData := range resp.Facts.USGAAP {
		for unit, observations := range conceptData.Units {
			for _, obs := range observations {
				_ = db.UpsertEdgarXBRLFact(ctx, store.EdgarXBRLFact{
					CIK:          cik,
					Concept:      concept,
					Unit:         unit,
					PeriodEnd:    obs.End,
					FiscalYear:   obs.FY,
					FiscalPeriod: obs.FP,
					Value:        obs.Val,
					FormType:     obs.Form,
					FiledAt:      obs.Filed,
				})
			}
		}
	}
	return nil
}
