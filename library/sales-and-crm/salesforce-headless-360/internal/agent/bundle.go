// Package agent assembles and renders signed agent-context bundles.
// The bundle is the primary portable artifact this CLI produces.
//
// Shape at v1 (see schemas/manifest.v1.json and the PP-core envelope schema):
//
//	{
//	  "$schema": "pp-salesforce-360/bundle/v1",
//	  "envelope": {...},
//	  "manifest": {...},
//	  "signature": "<compact JWS>"
//	}
//
// The envelope carries provenance (org_id, user_id, generated_at, expires_at,
// redactions, sources_used, sources_unavailable, trace_id). The manifest
// carries the Salesforce-specific data. The signature covers the manifest
// sha256 via the JWS claims.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	crmclient "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/datacloud"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	agentslack "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/slack"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/schemas"
)

// Bundle is the top-level artifact emitted by `agent context`.
type Bundle struct {
	Schema    string   `json:"$schema"`
	Envelope  Envelope `json:"envelope"`
	Manifest  Manifest `json:"manifest"`
	Signature string   `json:"signature"`
}

// Envelope carries provenance + signature metadata. Reusable across CLIs —
// reserved for extraction to PP core as `bundle-envelope.v1.json`.
type Envelope struct {
	OrgID              string         `json:"org_id"`
	InstanceURL        string         `json:"instance_url"`
	UserID             string         `json:"user_id"`
	QueryWindow        string         `json:"query_window"`
	GeneratedAt        time.Time      `json:"generated_at"`
	ExpiresAt          time.Time      `json:"expires_at"`
	Redactions         map[string]int `json:"redactions"`
	SourcesUsed        []string       `json:"sources_used"`
	SourcesUnavailable []string       `json:"sources_unavailable"`
	TraceID            string         `json:"trace_id"`
	Aud                string         `json:"aud"` // "agent-context" in v1; reserved as union
	AuditRecorded      bool           `json:"audit_recorded"`
}

// Manifest is the Salesforce-specific data shape. Fields are populated by
// the assembler; absent sources leave their fields nil.
type Manifest struct {
	Account          *Account       `json:"account,omitempty"`
	Contacts         []Contact      `json:"contacts,omitempty"`
	Opportunities    []Opportunity  `json:"opportunities,omitempty"`
	Cases            []Case         `json:"cases,omitempty"`
	Tasks            []Activity     `json:"tasks,omitempty"`
	Events           []Activity     `json:"events,omitempty"`
	Chatter          []FeedItem     `json:"chatter,omitempty"`
	Files            []FileRef      `json:"files,omitempty"`
	DataCloudProfile map[string]any `json:"data_cloud_profile,omitempty"`
	SlackChannels    []SlackChannel `json:"slack_channels,omitempty"`
}

type Account struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Type              string  `json:"type,omitempty"`
	Industry          string  `json:"industry,omitempty"`
	AnnualRevenue     float64 `json:"annual_revenue,omitempty"`
	NumberOfEmployees int     `json:"number_of_employees,omitempty"`
	OwnerID           string  `json:"owner_id,omitempty"`
	Website           string  `json:"website,omitempty"`
	Description       string  `json:"description,omitempty"`
}

type Contact struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Title     string `json:"title,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

type Opportunity struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	StageName   string  `json:"stage_name,omitempty"`
	Amount      float64 `json:"amount,omitempty"`
	CloseDate   string  `json:"close_date,omitempty"`
	Probability float64 `json:"probability,omitempty"`
	AccountID   string  `json:"account_id,omitempty"`
	OwnerID     string  `json:"owner_id,omitempty"`
}

type Case struct {
	ID         string `json:"id"`
	CaseNumber string `json:"case_number,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Status     string `json:"status,omitempty"`
	Priority   string `json:"priority,omitempty"`
	AccountID  string `json:"account_id,omitempty"`
}

type Activity struct {
	ID           string `json:"id"`
	Subject      string `json:"subject,omitempty"`
	ActivityDate string `json:"activity_date,omitempty"`
	WhoID        string `json:"who_id,omitempty"`
	WhatID       string `json:"what_id,omitempty"`
}

type FeedItem struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	Body      string `json:"body,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// FileRef is the content-addressed reference per D5 file-byte attestation.
// bytes_ref has been replaced with {sha256, sf_content_version_id, size_bytes}
// so the manifest SHA transitively covers file bytes.
type FileRef struct {
	Name             string `json:"name"`
	SHA256           string `json:"sha256"`
	ContentVersionID string `json:"sf_content_version_id"`
	SizeBytes        int    `json:"size_bytes"`
}

type SlackChannel struct {
	ChannelID      string         `json:"channel_id"`
	WorkspaceID    string         `json:"workspace_id,omitempty"`
	Name           string         `json:"name,omitempty"`
	LatestMessages []SlackMessage `json:"latest_messages,omitempty"`
	Warning        string         `json:"warning,omitempty"`
}

type SlackMessage struct {
	TS             string         `json:"ts"`
	User           string         `json:"user,omitempty"`
	Text           string         `json:"text,omitempty"`
	ReactionCounts map[string]int `json:"reaction_counts,omitempty"`
}

// AssembleOptions configures bundle assembly.
type AssembleOptions struct {
	OrgAlias            string
	OrgID               string
	InstanceURL         string
	UserID              string
	AccountHint         string // id, name, or domain
	QueryWindow         string // e.g., "P90D"
	TTL                 time.Duration
	IncludePII          bool
	Redactions          map[string]int
	SourcesUsed         []string
	SourcesMissing      []string
	TraceID             string
	EnrichmentClient    EnrichmentClient
	EnrichmentFilter    security.Filter
	DMOMap              datacloud.DMOMap
	DataCloudHTTPClient *http.Client
	SlackBotToken       string
	SlackHTTPClient     *http.Client
	SlackAPIBaseURL     string
	AuditClient         trust.BundleAuditClient
	AuditDBPath         string
	HIPAAMode           bool
	AuditLogger         func(format string, args ...any)
}

// Signer is the dependency required to sign the bundle.
type Signer interface {
	Sign(payload []byte) ([]byte, error)
	KID() string
}

type EnrichmentClient interface {
	datacloud.CoreClient
	agentslack.SOQLClient
}

// Assemble builds a Manifest from provided data and signs the Bundle. The
// data-layer integration is intentionally a parameter, not an import: real
// syncs populate it via the store; tests can construct it directly; dry-run
// passes skip signing and the raw manifest returns.
func Assemble(m Manifest, opts AssembleOptions, signer Signer) (*Bundle, error) {
	if opts.TTL == 0 {
		opts.TTL = 24 * time.Hour
	}
	now := time.Now().UTC()
	if err := enrichManifest(context.Background(), &m, &opts, now); err != nil {
		return nil, err
	}
	env := Envelope{
		OrgID:              opts.OrgID,
		InstanceURL:        opts.InstanceURL,
		UserID:             opts.UserID,
		QueryWindow:        opts.QueryWindow,
		GeneratedAt:        now,
		ExpiresAt:          now.Add(opts.TTL),
		Redactions:         opts.Redactions,
		SourcesUsed:        opts.SourcesUsed,
		SourcesUnavailable: opts.SourcesMissing,
		TraceID:            opts.TraceID,
		Aud:                "agent-context",
	}
	if env.Redactions == nil {
		env.Redactions = map[string]int{}
	}
	if env.SourcesUsed == nil {
		env.SourcesUsed = []string{}
	}
	if env.SourcesUnavailable == nil {
		env.SourcesUnavailable = []string{}
	}

	manifestJSON, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	sum := sha256.Sum256(manifestJSON)
	manifestSHA := hex.EncodeToString(sum[:])

	claims := map[string]any{
		"iss":                   opts.OrgID,
		"sub":                   opts.UserID,
		"iat":                   now.Unix(),
		"exp":                   env.ExpiresAt.Unix(),
		"jti":                   opts.TraceID,
		"aud":                   "agent-context",
		"manifest_sha256":       manifestSHA,
		"query_window":          opts.QueryWindow,
		"compliance_redactions": env.Redactions,
		"trace_id":              opts.TraceID,
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshal claims: %w", err)
	}

	var jws string
	if signer != nil {
		jws, err = trust.SignJWS(signerAdapter{signer}, claimsJSON)
		if err != nil {
			return nil, fmt.Errorf("sign bundle: %w", err)
		}
	}

	bundle := &Bundle{
		Schema:    "pp-salesforce-360/bundle/v1",
		Envelope:  env,
		Manifest:  m,
		Signature: jws,
	}
	if signer != nil {
		if err := schemas.ValidateBundleValue(bundle); err != nil {
			return nil, err
		}
		accountID := opts.AccountHint
		if m.Account != nil && m.Account.ID != "" {
			accountID = m.Account.ID
		}
		if opts.AuditClient != nil || opts.AuditDBPath != "" || opts.HIPAAMode {
			if err := trust.RecordBundleAudit(context.Background(), trust.BundleAuditRequest{
				KID:             signer.KID(),
				GeneratedBy:     opts.UserID,
				GeneratedAt:     env.GeneratedAt,
				AccountID:       accountID,
				BundleJTI:       opts.TraceID,
				SourcesUsed:     env.SourcesUsed,
				RedactionCounts: env.Redactions,
				TraceID:         env.TraceID,
				HIPAAMode:       opts.HIPAAMode,
			}, trust.BundleAuditOptions{
				Client:  opts.AuditClient,
				DBPath:  opts.AuditDBPath,
				Sync:    opts.HIPAAMode,
				LogWarn: opts.AuditLogger,
			}); err != nil {
				return nil, err
			}
			bundle.Envelope.AuditRecorded = true
		}
	}
	return bundle, nil
}

func enrichManifest(ctx context.Context, m *Manifest, opts *AssembleOptions, now time.Time) error {
	if m == nil || opts == nil {
		return nil
	}
	configureDefaultEnrichment(opts)
	if opts.EnrichmentClient == nil {
		return nil
	}
	accountID := opts.AccountHint
	if accountID == "" && m.Account != nil {
		accountID = m.Account.ID
	}
	if accountID == "" {
		return nil
	}
	opts.SourcesUsed = normalizeSources(opts.SourcesUsed)
	opts.SourcesMissing = removeSources(opts.SourcesMissing, "data_cloud", "slack_linkage")

	redactions := opts.Redactions
	if redactions == nil {
		redactions = map[string]int{}
		opts.Redactions = redactions
	}

	profile, err := datacloud.Fetch(ctx, opts.EnrichmentClient, accountID, datacloud.FetchOptions{
		Filter:     opts.EnrichmentFilter,
		DMOMap:     opts.DMOMap,
		HTTPClient: opts.DataCloudHTTPClient,
	})
	if err != nil {
		opts.SourcesMissing = appendSource(opts.SourcesMissing, "data_cloud")
	} else if profile.Availability.Available && profile.Profile != nil {
		m.DataCloudProfile = profile.Profile
		opts.SourcesUsed = appendSource(opts.SourcesUsed, "data_cloud")
		mergeRedactions(redactions, profile.Provenance)
	} else {
		opts.SourcesMissing = appendSource(opts.SourcesMissing, "data_cloud")
	}

	linkage, err := agentslack.LinkageFetch(ctx, opts.EnrichmentClient, accountID, opts.EnrichmentFilter)
	if err != nil {
		opts.SourcesMissing = appendSource(opts.SourcesMissing, "slack_linkage")
		return nil
	}
	mergeRedactions(redactions, linkage.Provenance)
	if !linkage.Availability.Available || len(linkage.Rows) == 0 {
		opts.SourcesMissing = appendSource(opts.SourcesMissing, "slack_linkage")
		return nil
	}
	opts.SourcesUsed = appendSource(opts.SourcesUsed, "slack_linkage")
	m.SlackChannels = slackChannelsFromLinkage(linkage.Rows)

	token := opts.SlackBotToken
	if token == "" {
		token = os.Getenv("SLACK_BOT_TOKEN")
	}
	if token == "" {
		return nil
	}
	start := now.Add(-90 * 24 * time.Hour)
	histories, availability, err := agentslack.HistoryFetch(ctx, linkage.Rows, agentslack.HistoryOptions{
		BotToken:   token,
		Start:      start,
		End:        now,
		HTTPClient: opts.SlackHTTPClient,
		BaseURL:    opts.SlackAPIBaseURL,
	})
	if err != nil {
		return nil
	}
	if availability.Warning != "" {
		opts.SourcesMissing = appendSource(opts.SourcesMissing, "slack_history: "+availability.Warning)
	}
	mergeSlackHistories(m.SlackChannels, histories)
	return nil
}

func configureDefaultEnrichment(opts *AssembleOptions) {
	if opts.EnrichmentClient != nil || !hasRESTSource(opts.SourcesUsed) {
		return
	}
	accessToken := os.Getenv("SALESFORCE_ACCESS_TOKEN")
	instanceURL := opts.InstanceURL
	if accessToken == "" || instanceURL == "" {
		return
	}
	cfg := &config.Config{
		BaseURL:               strings.TrimRight(instanceURL, "/"),
		AccessToken:           accessToken,
		SalesforceInstanceUrl: instanceURL,
		AuthSource:            "env:SALESFORCE_ACCESS_TOKEN",
	}
	c := crmclient.New(cfg, 10*time.Second, 0)
	c.NoCache = true
	opts.EnrichmentClient = c
	if opts.EnrichmentFilter == nil {
		opts.EnrichmentFilter = security.NewDefaultFilter(security.Options{Client: c, OrgAlias: opts.OrgAlias})
	}
	if opts.DataCloudHTTPClient == nil {
		opts.DataCloudHTTPClient = c.HTTPClient
	}
	if opts.SlackHTTPClient == nil {
		opts.SlackHTTPClient = c.HTTPClient
	}
}

func hasRESTSource(sources []string) bool {
	for _, source := range sources {
		if source == "rest" || source == "composite_graph" {
			return true
		}
	}
	return false
}

func slackChannelsFromLinkage(rows []agentslack.Linkage) []SlackChannel {
	out := make([]SlackChannel, 0, len(rows))
	for _, row := range rows {
		out = append(out, SlackChannel{ChannelID: row.ChannelID, WorkspaceID: row.WorkspaceID})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WorkspaceID == out[j].WorkspaceID {
			return out[i].ChannelID < out[j].ChannelID
		}
		return out[i].WorkspaceID < out[j].WorkspaceID
	})
	return out
}

func mergeSlackHistories(channels []SlackChannel, histories []agentslack.ChannelHistory) {
	byChannel := map[string]agentslack.ChannelHistory{}
	for _, history := range histories {
		byChannel[history.ChannelID] = history
	}
	for i := range channels {
		history, ok := byChannel[channels[i].ChannelID]
		if !ok {
			continue
		}
		channels[i].Warning = history.Warning
		for _, msg := range history.Messages {
			channels[i].LatestMessages = append(channels[i].LatestMessages, SlackMessage{
				TS:             msg.TS,
				User:           msg.User,
				Text:           msg.Text,
				ReactionCounts: msg.ReactionCounts,
			})
		}
	}
}

func normalizeSources(sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		if source == "composite_graph" {
			source = "rest"
		}
		out = appendSource(out, source)
	}
	return orderSources(out)
}

func orderSources(sources []string) []string {
	knownOrder := []string{"local", "rest", "data_cloud", "slack_linkage"}
	seen := map[string]bool{}
	unique := make([]string, 0, len(sources))
	for _, source := range sources {
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		unique = append(unique, source)
	}
	var out []string
	for _, known := range knownOrder {
		if seen[known] {
			out = append(out, known)
			delete(seen, known)
		}
	}
	var rest []string
	for source := range seen {
		rest = append(rest, source)
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func appendSource(sources []string, source string) []string {
	if source == "" {
		return sources
	}
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	sources = append(sources, source)
	return orderSources(sources)
}

func removeSources(sources []string, remove ...string) []string {
	removeSet := map[string]bool{}
	for _, source := range remove {
		removeSet[source] = true
	}
	out := sources[:0]
	for _, source := range sources {
		if !removeSet[source] {
			out = append(out, source)
		}
	}
	return orderSources(out)
}

func mergeRedactions(redactions map[string]int, provenance *security.Provenance) {
	if redactions == nil || provenance == nil {
		return
	}
	for reason, count := range provenance.Redactions {
		redactions[reason] += count
	}
}

// DryRunSummary returns a human-readable preview of what the bundle WOULD
// contain without signing or persisting. Used by `agent context --dry-run`.
func DryRunSummary(m Manifest, env Envelope) string {
	manifestJSON, _ := json.MarshalIndent(m, "", "  ")
	sum := sha256.Sum256(manifestJSON)
	return fmt.Sprintf(
		"DRY RUN — bundle not signed, not persisted.\n"+
			"manifest_sha256: %s\n"+
			"sources_used: %v\n"+
			"sources_unavailable: %v\n"+
			"redactions: %v\n"+
			"records: accounts=%s contacts=%d opportunities=%d cases=%d tasks=%d events=%d chatter=%d files=%d slack_channels=%d\n",
		hex.EncodeToString(sum[:]),
		env.SourcesUsed,
		env.SourcesUnavailable,
		env.Redactions,
		boolCount(m.Account != nil), len(m.Contacts), len(m.Opportunities), len(m.Cases),
		len(m.Tasks), len(m.Events), len(m.Chatter), len(m.Files), len(m.SlackChannels),
	)
}

func boolCount(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// signerAdapter adapts the public `agent.Signer` interface to the `trust.Signer`
// interface. They are separated so the agent package can be consumed without
// pulling in the trust package for callers who only need the types.
type signerAdapter struct{ s Signer }

func (a signerAdapter) Sign(payload []byte) ([]byte, error) { return a.s.Sign(payload) }
func (a signerAdapter) KID() string                         { return a.s.KID() }
func (a signerAdapter) PublicKeyPEM() string                { return "" }
