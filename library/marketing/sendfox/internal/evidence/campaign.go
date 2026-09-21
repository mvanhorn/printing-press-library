package evidence

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"golang.org/x/net/html"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

func campaignLinks(content string) []Row {
	out := []Row{}
	seen := map[string]bool{}
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		for _, a := range token.Attr {
			if a.Key != "href" {
				continue
			}
			if seen[a.Val] {
				continue
			}
			seen[a.Val] = true
			u, e := url.Parse(a.Val)
			valid := e == nil && ((u.Scheme == "https" || u.Scheme == "http") && u.Host != "" || u.Scheme == "mailto")
			out = append(out, Row{"url": a.Val, "structurally_valid": valid, "live_checked": false})
		}
	}
	return out
}
func hasID(v any, id string) bool {
	for _, x := range IDs(v) {
		if x == id {
			return true
		}
	}
	return false
}

// CampaignPreflight composes a draft and computes exclusion-aware local targeting evidence.
func CampaignPreflight(s Snapshot, now time.Time, maxAge time.Duration) Report {
	r := NewReport("campaign-preflight", s)
	r.Require(s, now, maxAge, "contacts", "lists", "memberships", "tags", "contact_tags", "unsubscribed", "domains")
	draft := Row{}
	op, _ := contract.Match("POST", "/campaigns")
	properties := op.BodySchema["properties"].(map[string]any)
	for key, v := range s.Campaign {
		if _, ok := properties[key]; ok && key != "scheduled_at" {
			draft[key] = v
		}
	}
	if Text(s.Campaign["scheduled_at"]) != "" {
		r.Blockers = append(r.Blockers, "scheduled_at was supplied; removed from draft and requires separate send approval")
	}
	if err := contract.Validate(op.BodySchema, draft); err != nil {
		r.Blockers = append(r.Blockers, err.Error())
	}
	subject := strings.ToLower(strings.TrimSpace(Text(draft["subject"])))
	if strings.HasPrefix(subject, "re:") || strings.HasPrefix(subject, "fw:") || strings.HasPrefix(subject, "fwd:") {
		r.Blockers = append(r.Blockers, "subject uses a forbidden reply/forward prefix")
	}
	links := campaignLinks(Text(draft["html"]))
	for _, link := range links {
		if link["structurally_valid"] != true {
			r.Blockers = append(r.Blockers, "malformed or unsupported link: "+Text(link["url"]))
		}
	}
	included, excluded := map[string]bool{}, map[string]bool{}
	for _, m := range s.Resources["memberships"].Items {
		cid, lid := Text(m["contact_id"]), Text(m["list_id"])
		if hasID(draft["lists"], lid) {
			included[cid] = true
		}
		if hasID(draft["excluded_lists"], lid) {
			excluded[cid] = true
		}
	}
	for _, m := range s.Resources["contact_tags"].Items {
		cid, tid := Text(m["contact_id"]), Text(m["tag_id"])
		if hasID(draft["to_contact_tags"], tid) {
			included[cid] = true
		}
		if hasID(draft["excluded_contact_tags"], tid) {
			excluded[cid] = true
		}
	}
	for key, resource := range map[string]string{"lists": "lists", "excluded_lists": "lists", "to_contact_tags": "tags", "excluded_contact_tags": "tags"} {
		idx := Index(s.Resources[resource].Items, "id")
		for _, id := range IDs(draft[key]) {
			if idx[id] == nil {
				r.Blockers = append(r.Blockers, key+": unknown ID "+id)
			}
		}
	}
	eligible, exclusions := []Row{}, []Row{}
	identities := map[string]bool{}
	suppressed := Suppressed(s)
	for _, c := range s.Resources["contacts"].Items {
		cid := Text(c["id"])
		if !included[cid] {
			continue
		}
		email := Email(c["email"])
		if identities[email] {
			r.Blockers = append(r.Blockers, "duplicate targeted identity: "+email)
			continue
		}
		identities[email] = true
		if excluded[cid] || suppressed[Email(c["email"])] || !ValidEmail(Email(c["email"])) {
			exclusions = append(exclusions, c)
		} else {
			eligible = append(eligible, c)
		}
	}
	if len(IDs(draft["lists"]))+len(IDs(draft["to_contact_tags"])) == 0 {
		r.Blockers = append(r.Blockers, "no target list or tag selected")
	}
	for _, name := range []string{"memberships", "contact_tags"} {
		if !s.Resources[name].ScopesComplete {
			r.Unknowns = append(r.Unknowns, name+": child scopes incomplete; recipient count is provisional")
		}
	}
	from := strings.Split(Email(draft["from_email"]), "@")
	domainReady := false
	if len(from) == 2 {
		for _, d := range s.Resources["domains"].Items {
			if strings.EqualFold(Text(d["domain"]), from[1]) && Text(d["validated_at"]) != "" {
				domainReady = true
			}
		}
	}
	if !domainReady {
		r.Blockers = append(r.Blockers, "sender domain has no validated evidence")
	}
	r.Data = Row{"draft": draft, "links": links, "eligible_observed": eligible, "eligible_observed_count": len(eligible), "exclusions": exclusions, "domain_evidence_ready": domainReady, "live_checks": "Unverified: current DNS, plan entitlement, account timezone, final recipients and delivery.", "readiness_scope": "local review only"}
	r.ProposedActions = append(r.ProposedActions, Row{"method": "POST", "path": "/campaigns", "body": draft, "draft": true, "execute": false})
	r.Finish()
	return r
}

// CampaignReview preserves server counters and proposes an unscheduled resend with explicit exclusions.
func CampaignReview(s Snapshot, id, audience string, now time.Time, maxAge time.Duration) Report {
	r := NewReport("campaign-review", s)
	r.Require(s, now, maxAge, "campaigns", "campaign_stats", "engagement", "contacts", "unsubscribed")
	if audience != "non_openers" && audience != "openers" && audience != "clickers" {
		r.Blockers = append(r.Blockers, "audience must be non_openers, openers or clickers")
	}
	if id == "" {
		id = Text(s.Campaign["id"])
	}
	campaign := Index(s.Resources["campaigns"].Items, "id")[id]
	if campaign == nil {
		r.Blockers = append(r.Blockers, "campaign ID missing or not found in snapshot")
	}
	if campaign != nil && Text(campaign["status"]) != "sent" {
		r.Blockers = append(r.Blockers, "resend requires evidence of a sent campaign")
	}
	stats := Index(s.Resources["campaign_stats"].Items, "campaign_id")[id]
	if stats == nil {
		r.Unknowns = append(r.Unknowns, "authoritative campaign stats missing")
	}
	links := Rows(stats["link_stats"])
	if _, ok := stats["link_stats"]; !ok {
		r.Unknowns = append(r.Unknowns, "link_stats not requested or not captured; missing is not zero")
	}
	sort.SliceStable(links, func(i, j int) bool {
		a, _ := number(links[i]["clicks"])
		b, _ := number(links[j]["clicks"])
		return a > b
	})
	eligible, excluded := []Row{}, []Row{}
	suppressed := Suppressed(s)
	contacts := Index(s.Resources["contacts"].Items, "id")
	seen := map[string]bool{}
	for _, e := range s.Resources["engagement"].Items {
		if Text(e["campaign_id"]) != id || Text(e["type"]) != audience {
			continue
		}
		c := contacts[Text(e["contact_id"])]
		email := Email(c["email"])
		if email == "" {
			email = Email(e["email"])
		}
		if seen[email] {
			continue
		}
		seen[email] = true
		if suppressed[email] || !ValidEmail(email) {
			excluded = append(excluded, Row{"email": email, "reason": "suppressed_or_invalid"})
		} else {
			eligible = append(eligible, Row{"email": email, "contact_id": e["contact_id"]})
		}
	}
	if !s.Resources["engagement"].ScopesComplete {
		r.Unknowns = append(r.Unknowns, "engagement scopes incomplete; eligible count not proven")
	}
	benchmarks := campaignBenchmarks(s)
	if benchmarks["complete"] != true {
		r.Unknowns = append(r.Unknowns, "campaign benchmarks: complete sent/open/click counters are unavailable for every supplied campaign")
	}
	r.Data = Row{"campaign": campaign, "authoritative_stats": stats, "benchmarks": benchmarks, "links": links, "eligible_observed": eligible, "eligible_observed_count": len(eligible), "exclusions": excluded, "note": "non_openers means recipients actually sent to who did not open; resend drafts do not inherit source lists. Final server recipients can change. Benchmarks recompute rates from counts and never average supplied percentages."}
	if campaign != nil {
		r.ProposedActions = append(r.ProposedActions, Row{"method": "POST", "path": "/campaigns/" + id + "/resend", "body": Row{"audience": audience}, "draft": true, "execute": false})
	}
	r.Finish()
	return r
}

type campaignBenchmarkPoint struct {
	id         string
	sent       float64
	opens      float64
	clicks     float64
	when       time.Time
	whenParsed bool
}

func campaignBenchmarks(s Snapshot) Row {
	statsResource := s.Resources["campaign_stats"]
	campaigns := Index(s.Resources["campaigns"].Items, "id")
	points := []campaignBenchmarkPoint{}
	warnings := []string{}
	invalid := 0
	for _, row := range statsResource.Items {
		id := Text(row["campaign_id"])
		sent, sentOK := number(row["sent_count"])
		opens, openOK := number(row["unique_open_count"])
		clicks, clickOK := number(row["unique_click_count"])
		if !sentOK || !openOK || !clickOK || sent <= 0 || opens < 0 || clicks < 0 || opens > sent || clicks > sent {
			invalid++
			continue
		}
		when, parsed := rowTime(row, "sent_at", "created_at")
		if !parsed {
			when, parsed = rowTime(campaigns[id], "sent_at", "created_at")
		}
		points = append(points, campaignBenchmarkPoint{id: id, sent: sent, opens: opens, clicks: clicks, when: when, whenParsed: parsed})
	}
	complete := resourceComplete(statsResource) && statsResource.ScopesComplete && invalid == 0 && len(points) > 0
	if invalid > 0 {
		warnings = append(warnings, fmt.Sprintf("campaign_stats: %d row(s) lack valid sent/open/click counts and were excluded; missing counters are unknown, not zero", invalid))
	}
	if len(points) == 0 {
		warnings = append(warnings, "No campaign has a positive sent_count with both unique open and click counters.")
	}
	window := statsResource.ObservedAt
	if window == "" {
		window = "supplied campaign_stats snapshot"
	}
	return Row{
		"method":             "recipient-weighted totals plus medians of rates recomputed from per-campaign counters",
		"window":             window,
		"campaigns_supplied": len(statsResource.Items),
		"campaigns_included": len(points),
		"complete":           complete,
		"warnings":           warnings,
		"open_rate":          benchmarkRate(points, window, "unique opens", complete, warnings, func(p campaignBenchmarkPoint) float64 { return p.opens }),
		"click_rate":         benchmarkRate(points, window, "unique clicks", complete, warnings, func(p campaignBenchmarkPoint) float64 { return p.clicks }),
	}
}

func benchmarkRate(points []campaignBenchmarkPoint, window, label string, complete bool, warnings []string, numerator func(campaignBenchmarkPoint) float64) Row {
	var totalNumerator, totalRecipients float64
	rates := make([]float64, 0, len(points))
	for _, point := range points {
		totalNumerator += numerator(point)
		totalRecipients += point.sent
		rates = append(rates, numerator(point)/point.sent*100)
	}
	var weightedValue any
	if complete && totalRecipients > 0 {
		weightedValue = totalNumerator / totalRecipients * 100
	}
	weighted := metric("sum("+label+") / sum(sent_count) * 100", window, weightedValue, totalRecipients, "recipient_weighted", complete && totalRecipients > 0, warnings)
	weighted["numerator"] = nullableNumber(totalNumerator, complete)
	weighted["denominator"] = nullableNumber(totalRecipients, complete)

	var medianValue any
	if complete && len(rates) > 0 {
		medianValue = median(rates)
	}
	medianMetric := metric("median(per-campaign "+label+" / sent_count * 100)", window, medianValue, nil, "campaign_median", complete && len(rates) > 0, warnings)

	trendWarnings := append([]string{}, warnings...)
	trendComplete := complete && len(points) >= 2
	for _, point := range points {
		if !point.whenParsed {
			trendComplete = false
		}
	}
	var trendValue any
	var priorRate, recentRate any
	trendWindow := "unknown"
	if trendComplete {
		ordered := append([]campaignBenchmarkPoint{}, points...)
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].when.Before(ordered[j].when) })
		mid := len(ordered) / 2
		priorNum, priorDen := benchmarkTotals(ordered[:mid], numerator)
		recentNum, recentDen := benchmarkTotals(ordered[mid:], numerator)
		if priorDen == 0 || recentDen == 0 {
			trendComplete = false
		} else {
			priorRate = priorNum / priorDen * 100
			recentRate = recentNum / recentDen * 100
			trendValue = recentRate.(float64) - priorRate.(float64)
			trendWindow = ordered[0].when.Format(time.RFC3339) + "/" + ordered[len(ordered)-1].when.Format(time.RFC3339)
		}
	}
	if !trendComplete {
		trendWarnings = append(trendWarnings, "Trend requires at least two campaigns with sortable sent_at or created_at timestamps and valid counters.")
	}
	trend := metric("recent-half recipient-weighted rate - prior-half recipient-weighted rate (percentage points)", trendWindow, trendValue, nil, "chronological_recipient_weighted", trendComplete, trendWarnings)
	trend["prior_rate"] = priorRate
	trend["recent_rate"] = recentRate

	return Row{"recipient_weighted": weighted, "campaign_median": medianMetric, "trend": trend}
}

func benchmarkTotals(points []campaignBenchmarkPoint, numerator func(campaignBenchmarkPoint) float64) (float64, float64) {
	var totalNumerator, totalRecipients float64
	for _, point := range points {
		totalNumerator += numerator(point)
		totalRecipients += point.sent
	}
	return totalNumerator, totalRecipients
}

func median(values []float64) float64 {
	ordered := append([]float64{}, values...)
	sort.Float64s(ordered)
	mid := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[mid]
	}
	return (ordered[mid-1] + ordered[mid]) / 2
}

func nullableNumber(value float64, complete bool) any {
	if !complete || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return value
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
