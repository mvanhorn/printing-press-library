package agent

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

type AudienceMember struct {
	SlackUserID      string
	Email            string
	SalesforceUserID string
	External         bool
	Fields           map[string][]string
}

type AudienceIntersection struct {
	Fields            map[string]map[string]bool `json:"fields"`
	UnmappedEmails    []string                   `json:"unmapped_emails,omitempty"`
	ExternalSlackUser []string                   `json:"external_slack_user_ids,omitempty"`
	Waived            bool                       `json:"waived"`
}

// IntersectAudienceFLS returns only fields every mapped Salesforce user can see.
func IntersectAudienceFLS(members []AudienceMember, allowWaiver bool) (AudienceIntersection, error) {
	out := AudienceIntersection{Fields: map[string]map[string]bool{}, Waived: allowWaiver}
	var mapped []AudienceMember
	for _, member := range members {
		if member.External {
			out.ExternalSlackUser = append(out.ExternalSlackUser, member.SlackUserID)
			continue
		}
		if member.SalesforceUserID == "" {
			out.UnmappedEmails = append(out.UnmappedEmails, member.Email)
			continue
		}
		mapped = append(mapped, member)
	}
	sort.Strings(out.UnmappedEmails)
	sort.Strings(out.ExternalSlackUser)
	if (len(out.UnmappedEmails) > 0 || len(out.ExternalSlackUser) > 0) && !allowWaiver {
		return out, fmt.Errorf("audience contains unmapped or external members")
	}
	if len(mapped) == 0 {
		if allowWaiver {
			return out, nil
		}
		return out, fmt.Errorf("no mapped Salesforce users in Slack audience")
	}
	for sobject, fields := range mapped[0].Fields {
		if out.Fields[sobject] == nil {
			out.Fields[sobject] = map[string]bool{}
		}
		for _, field := range fields {
			out.Fields[sobject][field] = true
		}
	}
	for _, member := range mapped[1:] {
		memberFields := normalizeFieldSet(member.Fields)
		for sobject, fields := range out.Fields {
			for field := range fields {
				if !memberFields[sobject][field] {
					delete(fields, field)
				}
			}
		}
	}
	return out, nil
}

func normalizeFieldSet(in map[string][]string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for sobject, fields := range in {
		out[sobject] = map[string]bool{}
		for _, field := range fields {
			out[sobject][field] = true
		}
	}
	return out
}

type InjectSummary struct {
	Account       map[string]any
	Opportunity   map[string]any
	ContactsCount int
	CasesCount    int
	FilesCount    int
	Waived        bool
}

func RenderInjectMarkdown(bundle Bundle, intersection AudienceIntersection) (string, error) {
	summary := InjectSummary{
		Account:       filterAccount(bundle.Manifest.Account, intersection.Fields["Account"]),
		Opportunity:   filterOpportunity(firstOpportunity(bundle.Manifest.Opportunities), intersection.Fields["Opportunity"]),
		ContactsCount: len(bundle.Manifest.Contacts),
		CasesCount:    len(bundle.Manifest.Cases),
		FilesCount:    len(bundle.Manifest.Files),
		Waived:        intersection.Waived,
	}
	tpl := template.Must(template.New("inject").Parse(`*Salesforce Account Context*
{{- if .Account.name }}

*Account*
- Name: {{ .Account.name }}{{ if .Account.industry }}
- Industry: {{ .Account.industry }}{{ end }}{{ if .Account.website }}
- Website: {{ .Account.website }}{{ end }}
{{- end }}
{{- if .Opportunity.name }}

*Top Opportunity*
- Name: {{ .Opportunity.name }}{{ if .Opportunity.stage_name }}
- Stage: {{ .Opportunity.stage_name }}{{ end }}{{ if .Opportunity.amount }}
- Amount: {{ .Opportunity.amount }}{{ end }}{{ if .Opportunity.close_date }}
- Close date: {{ .Opportunity.close_date }}{{ end }}
{{- end }}

*Included Records*
- Contacts: {{ .ContactsCount }}
- Cases: {{ .CasesCount }}
- Files: {{ .FilesCount }}
{{- if .Waived }}
- Audience waiver: recorded
{{- end }}
`))
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, summary); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()) + "\n", nil
}

func filterAccount(account *Account, fields map[string]bool) map[string]any {
	out := map[string]any{}
	if account == nil {
		return out
	}
	addIfAllowed(out, fields, "Name", "name", account.Name)
	addIfAllowed(out, fields, "Industry", "industry", account.Industry)
	addIfAllowed(out, fields, "Website", "website", account.Website)
	addIfAllowed(out, fields, "Type", "type", account.Type)
	return out
}

func filterOpportunity(opp *Opportunity, fields map[string]bool) map[string]any {
	out := map[string]any{}
	if opp == nil {
		return out
	}
	addIfAllowed(out, fields, "Name", "name", opp.Name)
	addIfAllowed(out, fields, "StageName", "stage_name", opp.StageName)
	addIfAllowed(out, fields, "Amount", "amount", opp.Amount)
	addIfAllowed(out, fields, "CloseDate", "close_date", opp.CloseDate)
	return out
}

func addIfAllowed(out map[string]any, fields map[string]bool, sfName, key string, value any) {
	if len(fields) == 0 || !fields[sfName] {
		return
	}
	switch v := value.(type) {
	case string:
		if v != "" {
			out[key] = v
		}
	case float64:
		if v != 0 {
			out[key] = v
		}
	default:
		out[key] = value
	}
}

func firstOpportunity(opps []Opportunity) *Opportunity {
	if len(opps) == 0 {
		return nil
	}
	return &opps[0]
}
