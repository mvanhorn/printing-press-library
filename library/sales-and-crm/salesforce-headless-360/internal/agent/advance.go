package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
)

// Opportunity describe support is not guaranteed in the local mock server yet.
// When the describe call is unavailable or does not include StageName values,
// validation falls back to the standard Salesforce sales-process stages below.
var defaultOpportunityStagePicklist = []string{
	"Prospecting",
	"Qualification",
	"Needs Analysis",
	"Value Proposition",
	"Id. Decision Makers",
	"Perception Analysis",
	"Proposal/Price Quote",
	"Negotiation/Review",
	"Closed Won",
	"Closed Lost",
}

type AdvanceOptions struct {
	OpportunityID string
	StageName     string
	CloseDate     string
	Client        WriteClient
}

func NewAdvanceWriteOptions(ctx context.Context, opts AdvanceOptions) (WriteOptions, error) {
	opts.OpportunityID = strings.TrimSpace(opts.OpportunityID)
	opts.StageName = strings.TrimSpace(opts.StageName)
	opts.CloseDate = strings.TrimSpace(opts.CloseDate)

	if opts.OpportunityID == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --opp is required")
	}
	if opts.StageName == "" {
		return WriteOptions{}, fmt.Errorf("MISSING_REQUIRED_FLAG: --stage is required")
	}
	if opts.CloseDate != "" {
		if _, err := time.Parse("2006-01-02", opts.CloseDate); err != nil {
			return WriteOptions{}, fmt.Errorf("INVALID_DATE: --close-date must be YYYY-MM-DD")
		}
	}
	if err := validateOpportunityStage(ctx, opts.Client, opts.StageName); err != nil {
		return WriteOptions{}, err
	}

	fields := map[string]any{"StageName": opts.StageName}
	if opts.CloseDate != "" {
		fields["CloseDate"] = opts.CloseDate
	}
	return NewUpdateWriteOptions(opts.OpportunityID, fields), nil
}

func validateOpportunityStage(ctx context.Context, c WriteClient, stage string) error {
	valid := describeOpportunityStages(ctx, c)
	if len(valid) == 0 {
		valid = defaultOpportunityStagePicklist
	}
	for _, value := range valid {
		if stage == value {
			return nil
		}
	}
	return fmt.Errorf("INVALID_PICKLIST_VALUE: %q is not valid for Opportunity.StageName; valid values: %s", stage, strings.Join(valid, ", "))
}

func describeOpportunityStages(ctx context.Context, c WriteClient) []string {
	if c == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return nil
	}
	raw, _, err := c.GetWithResponseHeaders("/services/data/"+security.APIVersion+"/sobjects/Opportunity/describe", nil)
	if err != nil || len(raw) == 0 {
		return nil
	}
	raw = unwrapWriteEnvelope(raw)
	var payload struct {
		Fields []struct {
			Name           string `json:"name"`
			PicklistValues []struct {
				Value  string `json:"value"`
				Active bool   `json:"active"`
			} `json:"picklistValues"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	for _, field := range payload.Fields {
		if field.Name != "StageName" {
			continue
		}
		values := make([]string, 0, len(field.PicklistValues))
		for _, pick := range field.PicklistValues {
			if pick.Value != "" && pick.Active {
				values = append(values, pick.Value)
			}
		}
		return values
	}
	return nil
}
