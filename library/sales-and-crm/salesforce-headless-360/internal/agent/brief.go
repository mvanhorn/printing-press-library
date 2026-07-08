package agent

import (
	"fmt"
	"strings"
	"time"
)

// BriefInput is the minimal data needed to render an opp-centric brief.
type BriefInput struct {
	Opportunity Opportunity
	Account     *Account
	Contacts    []Contact
	Activities  []Activity
	Feed        []FeedItem
}

// Brief is the structured JSON sidecar of `agent brief --opp`.
type Brief struct {
	OpportunityID    string     `json:"opportunity_id"`
	OppName          string     `json:"opportunity_name"`
	StageName        string     `json:"stage_name"`
	Amount           float64    `json:"amount,omitempty"`
	CloseDate        string     `json:"close_date,omitempty"`
	AccountName      string     `json:"account_name,omitempty"`
	Contacts         []Contact  `json:"contacts,omitempty"`
	RecentActivities []Activity `json:"recent_activities,omitempty"`
	RecentFeed       []FeedItem `json:"recent_feed,omitempty"`
	GeneratedAt      time.Time  `json:"generated_at"`
}

// RenderBrief produces both markdown and JSON from the input. The markdown
// is deterministic and field-gated - no LLM; no free-form prose that could
// re-expose redacted values.
func RenderBrief(in BriefInput) (markdown string, json Brief) {
	json = Brief{
		OpportunityID:    in.Opportunity.ID,
		OppName:          in.Opportunity.Name,
		StageName:        in.Opportunity.StageName,
		Amount:           in.Opportunity.Amount,
		CloseDate:        in.Opportunity.CloseDate,
		Contacts:         in.Contacts,
		RecentActivities: topN(in.Activities, 5),
		RecentFeed:       topN(in.Feed, 3),
		GeneratedAt:      time.Now().UTC(),
	}
	if in.Account != nil {
		json.AccountName = in.Account.Name
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", nonEmpty(in.Opportunity.Name, in.Opportunity.ID))
	fmt.Fprintf(&b, "- Stage: %s\n", nonEmpty(in.Opportunity.StageName, "unknown"))
	if in.Opportunity.Amount > 0 {
		fmt.Fprintf(&b, "- Amount: %.2f\n", in.Opportunity.Amount)
	}
	if in.Opportunity.CloseDate != "" {
		fmt.Fprintf(&b, "- Close date: %s\n", in.Opportunity.CloseDate)
	}
	if in.Account != nil {
		fmt.Fprintf(&b, "- Account: %s (%s)\n", in.Account.Name, in.Account.ID)
	}
	if len(in.Contacts) > 0 {
		fmt.Fprintf(&b, "\n## Contacts\n\n")
		for _, c := range in.Contacts {
			fmt.Fprintf(&b, "- %s %s (%s) - %s\n", c.FirstName, c.LastName, c.Title, c.Email)
		}
	}
	if len(json.RecentActivities) > 0 {
		fmt.Fprintf(&b, "\n## Recent activity (last 5)\n\n")
		for _, a := range json.RecentActivities {
			fmt.Fprintf(&b, "- %s: %s\n", nonEmpty(a.ActivityDate, "(no date)"), nonEmpty(a.Subject, a.ID))
		}
	}
	if len(json.RecentFeed) > 0 {
		fmt.Fprintf(&b, "\n## Recent chatter (last 3)\n\n")
		for _, f := range json.RecentFeed {
			fmt.Fprintf(&b, "- %s: %s\n", nonEmpty(f.CreatedAt, "(no date)"), truncate(f.Body, 160))
		}
	}
	markdown = b.String()
	return
}

func topN[T any](v []T, n int) []T {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
