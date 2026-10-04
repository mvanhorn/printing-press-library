// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package tabiwa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const Origin = "https://app.tabi-wester.westjr.co.jp"
const MaxPayload = 8 << 20
const MaxRecords = 1000

var regions = map[string]string{"10": "せとうち", "20": "北陸", "30": "山陰", "40": "九州"}
var idPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{5,15}$`)
var numericPattern = regexp.MustCompile(`^[0-9]{1,5}$`)
var pricePattern = regexp.MustCompile(`^([0-9][0-9,]*)(円|P)(?:\s*→\s*([0-9][0-9,]*)(円|P))?$`)
var cuePattern = regexp.MustCompile(`※|注意|不可|できません|必要|含まれ|含まれて|引換|交換|提示|QR|事前|予約|別途|限定|除外|休館|運休|無料|税|1台|１台|全額|付与対象外`)

type Named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Area struct {
	ID    json.Number `json:"id"`
	Name  string      `json:"name"`
	Areas []Named     `json:"areas"`
}
type RawProduct struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Price        *string `json:"price"`
	Overview     string  `json:"overview"`
	TicketType   string  `json:"ticket_type"`
	Areas        []Named `json:"areas"`
	Categories   []Named `json:"categories"`
	PointOnly    *bool   `json:"is_point_only"`
	UsageTypes   string  `json:"usage_types"`
	ExternalType string  `json:"external_ticket_type"`
}
type Price struct {
	Display        *string `json:"display"`
	Amount         *string `json:"amount"`
	OriginalAmount *string `json:"original_amount"`
	Unit           string  `json:"unit"`
	Kind           string  `json:"kind"`
	Basis          string  `json:"basis"`
	PointsOnly     *bool   `json:"points_only"`
}
type Evidence struct {
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}
type Product struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Region            Named      `json:"region"`
	TicketType        string     `json:"ticket_type"`
	Areas             []Named    `json:"area_tags"`
	Categories        []Named    `json:"categories"`
	Price             Price      `json:"price"`
	Overview          string     `json:"overview"`
	OverviewTruncated bool       `json:"overview_truncated"`
	Evidence          []Evidence `json:"restriction_evidence"`
	EvidenceTruncated bool       `json:"restriction_evidence_truncated"`
	UsageHints        string     `json:"usage_type_hints"`
	Status            string     `json:"published_status"`
	Availability      string     `json:"availability"`
	FullTerms         string     `json:"full_redemption_terms"`
	CoverageStatus    string     `json:"included_routes"`
	ProductURL        string     `json:"product_url"`
	SourceURL         string     `json:"source_url"`
	ObservedAt        string     `json:"observed_at"`
	RequestedDate     string     `json:"requested_date,omitempty"`
}
type Query struct {
	Region, Type, Category, Prefecture, Area, Date string
	IDs                                            []string
}
type Client struct {
	Base    string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New(base string, rate float64) *Client {
	if rate <= 0 || rate > 2 {
		rate = 2
	}
	return &Client{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, limiter: cliutil.NewAdaptiveLimiter(rate)}
}
func Region(id string) (Named, error) {
	n, ok := regions[id]
	if !ok {
		return Named{}, fmt.Errorf("--region must be 10 (せとうち), 20 (北陸), 30 (山陰) or 40 (九州)")
	}
	return Named{id, n}, nil
}
func ValidateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid product ID %q: use an exact ID from catalog search", id)
	}
	return nil
}
func Validate(q Query) error {
	if _, err := Region(q.Region); err != nil {
		return err
	}
	for name, value := range map[string]string{"prefecture": q.Prefecture, "area": q.Area} {
		if value != "" && !numericPattern.MatchString(value) {
			return fmt.Errorf("--%s must be an exact numeric ID from geography list", name)
		}
	}
	if q.Date != "" {
		v, err := time.Parse("2006-01-02", q.Date)
		if err != nil || v.Format("2006-01-02") != q.Date {
			return fmt.Errorf("--on must be a real date in YYYY-MM-DD format")
		}
	}
	if q.Type != "" && !oneOf(q.Type, []string{"freepass", "ticket", "multi-coupon", "discount-coupon"}) {
		return fmt.Errorf("--type must be freepass, ticket, multi-coupon or discount-coupon")
	}
	if q.Category != "" && !oneOf(q.Category, []string{"transportation", "tourism_experience", "gourmet", "other"}) {
		return fmt.Errorf("--category must be transportation, tourism_experience, gourmet or other")
	}
	if len(q.IDs) > 5 {
		return fmt.Errorf("at most five product IDs may be requested")
	}
	for _, id := range q.IDs {
		if err := ValidateID(id); err != nil {
			return err
		}
	}
	return nil
}
func oneOf(s string, values []string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func (q Query) Values() url.Values {
	v := url.Values{"ticket_type": {q.Type}, "category_id": {q.Category}, "area_id": {q.Area}, "prefecture_id": {q.Prefecture}, "usage_date": {q.Date}}
	for _, id := range q.IDs {
		v.Add("ticket_ids", id)
	}
	return v
}
func (c *Client) read(ctx context.Context, region, path string, values url.Values) ([]byte, string, error) {
	if _, err := Region(region); err != nil {
		return nil, "", err
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, "", err
	}
	u := c.Base + path
	if len(values) > 0 {
		u += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", "regionId="+region)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("catalog read failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, "", &cliutil.RateLimitError{URL: c.Base + path, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, "", fmt.Errorf("source_access_queue_or_redirect: public catalog redirected; open the canonical website and do not bypass access controls")
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("public catalog returned HTTP %d; source read failed, availability is unknown", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxPayload+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > MaxPayload {
		return nil, "", fmt.Errorf("catalog response exceeds the 8 MiB source bound")
	}
	if !json.Valid(b) {
		return nil, "", fmt.Errorf("public catalog returned HTML instead of JSON or invalid JSON; source access is unavailable")
	}
	c.limiter.OnSuccess()
	canonical := Origin + path
	if len(values) > 0 {
		canonical += "?" + values.Encode()
	}
	return b, canonical, nil
}
func (c *Client) Search(ctx context.Context, q Query) ([]RawProduct, string, string, error) {
	if err := Validate(q); err != nil {
		return nil, "", "", err
	}
	b, u, err := c.read(ctx, q.Region, "/ticketList/search", q.Values())
	if err != nil {
		return nil, "", "", err
	}
	var env struct {
		Response json.RawMessage `json:"response"`
	}
	if err = json.Unmarshal(b, &env); err != nil {
		return nil, "", "", err
	}
	if len(env.Response) == 0 || env.Response[0] != '[' {
		return nil, "", "", fmt.Errorf("catalog response contract changed or provider reported an error; no availability conclusion is possible")
	}
	var rows []RawProduct
	if err = json.Unmarshal(env.Response, &rows); err != nil {
		return nil, "", "", fmt.Errorf("catalog response contract invalid: %w", err)
	}
	if len(rows) > MaxRecords {
		return nil, "", "", fmt.Errorf("source catalog exceeds the %d record response bound", MaxRecords)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.Price != nil && utf8.RuneCountInString(*r.Price) > 200 {
			return nil, "", "", fmt.Errorf("source price display exceeds 200-character bound")
		}
		if ValidateID(r.ID) != nil || strings.TrimSpace(r.Name) == "" || seen[r.ID] {
			return nil, "", "", fmt.Errorf("catalog response has an invalid or duplicate identity")
		}
		seen[r.ID] = true
	}
	return rows, u, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), nil
}
func (c *Client) Geography(ctx context.Context, region string) ([]Area, string, string, error) {
	b, u, err := c.read(ctx, region, "/ticketList/area", nil)
	if err != nil {
		return nil, "", "", err
	}
	var env struct {
		Response json.RawMessage `json:"response"`
	}
	if err = json.Unmarshal(b, &env); err != nil {
		return nil, "", "", err
	}
	if len(env.Response) == 0 || env.Response[0] != '[' {
		return nil, "", "", fmt.Errorf("geography response contract changed or provider reported an error")
	}
	var rows []Area
	dec := json.NewDecoder(bytes.NewReader(env.Response))
	dec.UseNumber()
	if err = dec.Decode(&rows); err != nil {
		return nil, "", "", err
	}
	if len(rows) > 100 {
		return nil, "", "", fmt.Errorf("source geography exceeds 100 prefectures")
	}
	for _, r := range rows {
		if !numericPattern.MatchString(string(r.ID)) || r.Name == "" || len(r.Areas) > 100 {
			return nil, "", "", fmt.Errorf("source geography has invalid IDs or exceeds area bounds")
		}
	}
	return rows, u, time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z"), nil
}
func clipped(s string, n int) (string, bool) {
	r := []rune(s)
	if len(r) <= n {
		return s, false
	}
	return string(r[:n]), true
}
func quote(r RawProduct) Price {
	p := Price{Display: r.Price, Unit: "unknown", Kind: "unknown", Basis: "unknown", PointsOnly: r.PointOnly}
	if r.Price != nil {
		d := strings.TrimSpace(*r.Price)
		m := pricePattern.FindStringSubmatch(d)
		if len(m) > 0 {
			a := strings.ReplaceAll(m[1], ",", "")
			p.Amount = &a
			p.Kind = "quoted"
			if m[2] == "P" {
				p.Unit = "WESTER_POINT"
			} else {
				p.Unit = "JPY"
			}
			if m[3] != "" {
				if m[2] != m[4] {
					p.Kind = "unknown"
					p.Unit = "unknown"
					p.Amount = nil
				} else {
					old := a
					a = strings.ReplaceAll(m[3], ",", "")
					p.Amount = &a
					p.OriginalAmount = &old
					p.Kind = "discounted_quote"
				}
			}
		} else if strings.Contains(d, "日付により変動") {
			p.Kind = "variable_by_date"
		}
	}
	if strings.Contains(r.Overview, "タクシー代金は1台あたり") || strings.Contains(r.Overview, "タクシー代金は１台あたり") {
		p.Basis = "per_vehicle"
	}
	return p
}
func Normalize(r RawProduct, region, date, source, observed string) Product {
	reg, _ := Region(region)
	overview, truncated := clipped(strings.ReplaceAll(r.Overview, "\r\n", "\n"), 1400)
	evidence := []Evidence{}
	evidenceTruncated := false
	for _, line := range strings.Split(strings.ReplaceAll(r.Overview, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !cuePattern.MatchString(line) {
			continue
		}
		if len(evidence) >= 8 {
			evidenceTruncated = true
			break
		}
		txt, cut := clipped(line, 160)
		evidence = append(evidence, Evidence{"overview_cue", txt, cut})
		evidenceTruncated = evidenceTruncated || cut
	}
	status := "unknown"
	if strings.Contains(r.Overview, "【完売御礼】") || strings.Contains(r.Overview, "本チケットは完売いたしました") {
		status = "overview_declares_sold_out"
	}
	name, _ := clipped(r.Name, 200)
	hints, _ := clipped(r.UsageTypes, 400)
	return Product{ID: r.ID, Name: name, Region: reg, TicketType: r.TicketType, Areas: boundNames(r.Areas), Categories: boundNames(r.Categories), Price: quote(r), Overview: overview, OverviewTruncated: truncated, Evidence: evidence, EvidenceTruncated: evidenceTruncated, UsageHints: hints, Status: status, Availability: "unknown", FullTerms: "unknown", CoverageStatus: "unknown", ProductURL: Origin + "/eticketDetails?ticket_id=" + url.QueryEscape(r.ID), SourceURL: source, ObservedAt: observed, RequestedDate: date}
}
func boundNames(rows []Named) []Named {
	out := make([]Named, 0, min(len(rows), 30))
	for i, r := range rows {
		if i == 30 {
			break
		}
		id, _ := clipped(r.ID, 30)
		name, _ := clipped(r.Name, 120)
		out = append(out, Named{id, name})
	}
	return out
}
func Match(r RawProduct, query string) bool {
	return strings.Contains(strings.ToLower(r.Name+"\n"+r.Overview), strings.ToLower(query))
}
func ValidateQuery(query string) error {
	if !utf8.ValidString(query) || utf8.RuneCountInString(query) > 100 {
		return fmt.Errorf("--query must be valid UTF-8 with at most 100 characters")
	}
	return nil
}
