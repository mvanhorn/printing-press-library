package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	crmsync "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/sync"
)

type LiveClient interface {
	crmsync.AccountSyncClient
}

type LiveAssemblyOptions struct {
	AccountID   string
	Since       time.Time
	Filter      security.Filter
	FileFetcher ContentVersionFetcher
}

// AssembleLiveManifest builds the Salesforce manifest directly from Composite
// Graph output, applies the Unit D security filter to every record, and hashes
// linked ContentVersion bytes when file records are present.
func AssembleLiveManifest(ctx context.Context, c LiveClient, opts LiveAssemblyOptions) (Manifest, *security.Provenance, error) {
	if c == nil {
		return Manifest{}, nil, fmt.Errorf("live Salesforce client is required")
	}
	request, err := crmsync.BuildAccountGraph(opts.AccountID, opts.Since)
	if err != nil {
		return Manifest{}, nil, err
	}
	body, _, _, err := c.PostWithResponseHeaders(crmsync.CompositeGraphPath, request)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("composite graph: %w", err)
	}
	graph, err := crmsync.ParseGraphResponse(body)
	if err != nil {
		return Manifest{}, nil, err
	}
	gate := crmsync.NewGate()
	for _, pageRef := range graph.PageRefs {
		rows, err := crmsync.FetchRemainingPages(c, pageRef, gate)
		if err != nil {
			return Manifest{}, nil, err
		}
		graph.Records[pageRef.SObject] = append(graph.Records[pageRef.SObject], rows...)
	}

	prov := &security.Provenance{Redactions: map[string]int{}}
	filtered := crmsync.FilterGraphRecords(ctx, graph, crmsync.NewSecurityFilter(opts.Filter, prov))
	manifest, err := manifestFromGraph(ctx, filtered, opts.FileFetcher)
	if err != nil {
		return Manifest{}, nil, err
	}
	return manifest, prov, nil
}

func manifestFromGraph(ctx context.Context, records map[string][]json.RawMessage, fetcher ContentVersionFetcher) (Manifest, error) {
	var m Manifest
	for _, raw := range records["Account"] {
		account, err := parseAccount(raw)
		if err != nil {
			return m, err
		}
		m.Account = &account
		break
	}
	for _, raw := range records["Contact"] {
		v, err := parseContact(raw)
		if err != nil {
			return m, err
		}
		m.Contacts = append(m.Contacts, v)
	}
	for _, raw := range records["Opportunity"] {
		v, err := parseOpportunity(raw)
		if err != nil {
			return m, err
		}
		m.Opportunities = append(m.Opportunities, v)
	}
	for _, raw := range records["Case"] {
		v, err := parseCase(raw)
		if err != nil {
			return m, err
		}
		m.Cases = append(m.Cases, v)
	}
	for _, raw := range records["Task"] {
		v, err := parseActivity(raw)
		if err != nil {
			return m, err
		}
		m.Tasks = append(m.Tasks, v)
	}
	for _, raw := range records["Event"] {
		v, err := parseActivity(raw)
		if err != nil {
			return m, err
		}
		m.Events = append(m.Events, v)
	}
	for _, raw := range records["FeedItem"] {
		v, err := parseFeedItem(raw)
		if err != nil {
			return m, err
		}
		m.Chatter = append(m.Chatter, v)
	}
	for _, raw := range records["ContentDocumentLink"] {
		file, ok, err := fileRefFromContentDocumentLink(ctx, raw, fetcher)
		if err != nil {
			return m, err
		}
		if ok {
			m.Files = append(m.Files, file)
		}
	}
	return m, nil
}

func parseAccount(raw json.RawMessage) (Account, error) {
	var sf struct {
		ID                string  `json:"Id"`
		Name              string  `json:"Name"`
		Type              string  `json:"Type"`
		Industry          string  `json:"Industry"`
		AnnualRevenue     float64 `json:"AnnualRevenue"`
		NumberOfEmployees int     `json:"NumberOfEmployees"`
		OwnerID           string  `json:"OwnerId"`
		Website           string  `json:"Website"`
		Description       string  `json:"Description"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return Account{}, err
	}
	return Account{ID: sf.ID, Name: sf.Name, Type: sf.Type, Industry: sf.Industry, AnnualRevenue: sf.AnnualRevenue, NumberOfEmployees: sf.NumberOfEmployees, OwnerID: sf.OwnerID, Website: sf.Website, Description: sf.Description}, nil
}

func parseContact(raw json.RawMessage) (Contact, error) {
	var sf struct {
		ID        string `json:"Id"`
		FirstName string `json:"FirstName"`
		LastName  string `json:"LastName"`
		Email     string `json:"Email"`
		Title     string `json:"Title"`
		AccountID string `json:"AccountId"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return Contact{}, err
	}
	return Contact{ID: sf.ID, FirstName: sf.FirstName, LastName: sf.LastName, Email: sf.Email, Title: sf.Title, AccountID: sf.AccountID}, nil
}

func parseOpportunity(raw json.RawMessage) (Opportunity, error) {
	var sf struct {
		ID          string  `json:"Id"`
		Name        string  `json:"Name"`
		StageName   string  `json:"StageName"`
		Amount      float64 `json:"Amount"`
		CloseDate   string  `json:"CloseDate"`
		Probability float64 `json:"Probability"`
		AccountID   string  `json:"AccountId"`
		OwnerID     string  `json:"OwnerId"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return Opportunity{}, err
	}
	return Opportunity{ID: sf.ID, Name: sf.Name, StageName: sf.StageName, Amount: sf.Amount, CloseDate: sf.CloseDate, Probability: sf.Probability, AccountID: sf.AccountID, OwnerID: sf.OwnerID}, nil
}

func parseCase(raw json.RawMessage) (Case, error) {
	var sf struct {
		ID         string `json:"Id"`
		CaseNumber string `json:"CaseNumber"`
		Subject    string `json:"Subject"`
		Status     string `json:"Status"`
		Priority   string `json:"Priority"`
		AccountID  string `json:"AccountId"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return Case{}, err
	}
	return Case{ID: sf.ID, CaseNumber: sf.CaseNumber, Subject: sf.Subject, Status: sf.Status, Priority: sf.Priority, AccountID: sf.AccountID}, nil
}

func parseActivity(raw json.RawMessage) (Activity, error) {
	var sf struct {
		ID           string `json:"Id"`
		Subject      string `json:"Subject"`
		ActivityDate string `json:"ActivityDate"`
		WhoID        string `json:"WhoId"`
		WhatID       string `json:"WhatId"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return Activity{}, err
	}
	return Activity{ID: sf.ID, Subject: sf.Subject, ActivityDate: sf.ActivityDate, WhoID: sf.WhoID, WhatID: sf.WhatID}, nil
}

func parseFeedItem(raw json.RawMessage) (FeedItem, error) {
	var sf struct {
		ID          string `json:"Id"`
		LowerID     string `json:"id"`
		Type        string `json:"Type"`
		CreatedDate string `json:"CreatedDate"`
		LowerDate   string `json:"createdDate"`
		Body        any    `json:"Body"`
		LowerBody   any    `json:"body"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return FeedItem{}, err
	}
	id := firstString(sf.ID, sf.LowerID)
	return FeedItem{ID: id, Type: sf.Type, Body: feedBodyText(firstAny(sf.Body, sf.LowerBody)), CreatedAt: firstString(sf.CreatedDate, sf.LowerDate)}, nil
}

func fileRefFromContentDocumentLink(ctx context.Context, raw json.RawMessage, fetcher ContentVersionFetcher) (FileRef, bool, error) {
	var sf struct {
		ID                       string `json:"Id"`
		Name                     string `json:"Name"`
		Title                    string `json:"Title"`
		ContentVersionID         string `json:"ContentVersionId"`
		LatestPublishedVersionID string `json:"LatestPublishedVersionId"`
		ContentDocumentID        string `json:"ContentDocumentId"`
	}
	if err := json.Unmarshal(raw, &sf); err != nil {
		return FileRef{}, false, err
	}
	versionID := firstString(sf.ContentVersionID, sf.LatestPublishedVersionID, sf.ContentDocumentID)
	if versionID == "" {
		return FileRef{}, false, nil
	}
	name := firstString(sf.Name, sf.Title, sf.ID, versionID)
	if fetcher == nil {
		return FileRef{Name: name, ContentVersionID: versionID}, true, nil
	}
	file, err := AttestContentVersion(ctx, fetcher, name, versionID)
	return file, true, err
}

func feedBodyText(v any) string {
	switch body := v.(type) {
	case string:
		return body
	case map[string]any:
		segs, _ := body["messageSegments"].([]any)
		var b strings.Builder
		for _, seg := range segs {
			obj, _ := seg.(map[string]any)
			text, _ := obj["text"].(string)
			b.WriteString(text)
		}
		return b.String()
	default:
		return ""
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

type responseHeaderGetter interface {
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
}
