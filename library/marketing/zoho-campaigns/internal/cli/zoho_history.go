// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared plumbing for the novel history commands (delta, digest,
// growth, engagement, bounce-audit, journey). Lives beside the generated files
// so regeneration preserves it whole.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/internal/store"
)

const historyTimeFormat = time.RFC3339

// openHistoryStore opens the local store and lazily creates the history tables.
func openHistoryStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("zoho-campaigns-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := store.EnsureCampaignHistoryTables(ctx, db.DB()); err != nil {
		_ = db.Close() // best-effort cleanup; the Ensure error is the one worth returning
		return nil, err
	}
	return db, nil
}

// historyDBExists reports whether the local mirror file exists at all.
func historyDBExists(dbPath string) (string, bool) {
	if dbPath == "" {
		dbPath = defaultDBPath("zoho-campaigns-pp-cli")
	}
	_, err := os.Stat(dbPath)
	return dbPath, !os.IsNotExist(err)
}

func parseSinceLoose(s, fallback string) (time.Duration, error) {
	if s == "" {
		s = fallback
	}
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return 0, usageErr(fmt.Errorf("--since %q: %w (use forms like 24h, 7d, 4w)", s, err))
	}
	return d, nil
}

// reportMetrics is the parsed campaign-reports block of /campaignreports.
type reportMetrics struct {
	CampaignKey  string  `json:"campaign_key"`
	CampaignName string  `json:"campaign_name"`
	TakenAt      string  `json:"taken_at,omitempty"`
	EmailsSent   int64   `json:"emails_sent"`
	Delivered    int64   `json:"delivered"`
	Opens        int64   `json:"opens"`
	UniqueClicks int64   `json:"unique_clicks"`
	Bounces      int64   `json:"bounces"`
	HardBounces  int64   `json:"hard_bounces"`
	SoftBounces  int64   `json:"soft_bounces"`
	Unsubscribes int64   `json:"unsubscribes"`
	Spams        int64   `json:"spams"`
	OpenPercent  float64 `json:"open_percent"`
	ClickPercent float64 `json:"click_percent"`
}

func atoiLoose(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func atofLoose(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// zohoErrorCode returns the Zoho error code when the body is a status:error
// envelope (Zoho returns these with HTTP 200), or "" for success bodies.
func zohoErrorCode(data json.RawMessage) (code, message string) {
	var env struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", ""
	}
	if env.Status == "error" {
		return env.Code, env.Message
	}
	return "", ""
}

// fetchReportMetrics pulls /campaignreports for one campaign and parses it.
func fetchReportMetrics(ctx context.Context, c *client.Client, campaignKey string) (reportMetrics, error) {
	data, err := c.Get(ctx, "/campaignreports", map[string]string{
		"resfmt":      "JSON",
		"campaignkey": campaignKey,
	})
	if err != nil {
		return reportMetrics{}, fmt.Errorf("fetching report for %s: %w", campaignKey, err)
	}
	if code, msg := zohoErrorCode(data); code != "" {
		return reportMetrics{}, fmt.Errorf("report for %s: zoho error %s: %s", campaignKey, code, msg)
	}
	var body struct {
		Reports []map[string]string `json:"campaign-reports"`
	}
	if err := json.Unmarshal(data, &body); err != nil || len(body.Reports) == 0 {
		return reportMetrics{}, fmt.Errorf("report for %s: unexpected response shape", campaignKey)
	}
	r := body.Reports[0]
	return reportMetrics{
		CampaignKey:  campaignKey,
		CampaignName: r["campaign_name"],
		EmailsSent:   atoiLoose(r["emails_sent_count"]),
		Delivered:    atoiLoose(r["delivered_count"]),
		Opens:        atoiLoose(r["opens_count"]),
		UniqueClicks: atoiLoose(r["unique_clicks_count"]),
		Bounces:      atoiLoose(r["bounces_count"]),
		HardBounces:  atoiLoose(r["hardbounce_count"]),
		SoftBounces:  atoiLoose(r["softbounce_count"]),
		Unsubscribes: atoiLoose(r["unsub_count"]),
		Spams:        atoiLoose(r["spams_count"]),
		OpenPercent:  atofLoose(r["open_percent"]),
		ClickPercent: atofLoose(r["unique_clicked_percent"]),
	}, nil
}

// snapshotReport stores a report snapshot unless it is identical to the most
// recent one for the same campaign (no-change snapshots are noise).
func snapshotReport(ctx context.Context, db *store.Store, m reportMetrics, sentTime string) (bool, error) {
	var last reportMetrics
	err := db.DB().QueryRowContext(ctx, `
		SELECT emails_sent, delivered, opens, unique_clicks, bounces, hard_bounces, soft_bounces, unsubscribes, spams
		FROM campaign_report_snapshots WHERE campaign_key = ?
		ORDER BY taken_at DESC LIMIT 1`, m.CampaignKey).
		Scan(&last.EmailsSent, &last.Delivered, &last.Opens, &last.UniqueClicks, &last.Bounces,
			&last.HardBounces, &last.SoftBounces, &last.Unsubscribes, &last.Spams)
	if err == nil &&
		last.EmailsSent == m.EmailsSent && last.Delivered == m.Delivered &&
		last.Opens == m.Opens && last.UniqueClicks == m.UniqueClicks &&
		last.Bounces == m.Bounces && last.HardBounces == m.HardBounces &&
		last.SoftBounces == m.SoftBounces && last.Unsubscribes == m.Unsubscribes &&
		last.Spams == m.Spams {
		return false, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("reading last snapshot: %w", err)
	}
	_, err = db.DB().ExecContext(ctx, `
		INSERT OR REPLACE INTO campaign_report_snapshots
		(campaign_key, campaign_name, taken_at, emails_sent, delivered, opens, unique_clicks,
		 bounces, hard_bounces, soft_bounces, unsubscribes, spams, open_percent, click_percent, sent_time)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.CampaignKey, m.CampaignName, time.Now().UTC().Format(historyTimeFormat),
		m.EmailsSent, m.Delivered, m.Opens, m.UniqueClicks,
		m.Bounces, m.HardBounces, m.SoftBounces, m.Unsubscribes, m.Spams,
		m.OpenPercent, m.ClickPercent, sentTime)
	if err != nil {
		return false, fmt.Errorf("writing report snapshot: %w", err)
	}
	return true, nil
}

// listCounts is one mailing list's current counters.
type listCounts struct {
	ListKey  string `json:"listkey"`
	ListName string `json:"listname"`
	Contacts int64  `json:"contacts"`
	Unsubs   int64  `json:"unsubs"`
	Bounces  int64  `json:"bounces"`
}

// snapshotListCounts pulls /getmailinglists and snapshots per-list counters,
// skipping lists whose counters are unchanged since the last snapshot.
func snapshotListCounts(ctx context.Context, c *client.Client, db *store.Store) ([]listCounts, error) {
	data, err := c.Get(ctx, "/getmailinglists", map[string]string{
		"resfmt": "JSON", "fromindex": "1", "range": "100", "sort": "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("fetching mailing lists: %w", err)
	}
	if code, msg := zohoErrorCode(data); code != "" {
		return nil, fmt.Errorf("mailing lists: zoho error %s: %s", code, msg)
	}
	var body struct {
		Lists []map[string]any `json:"list_of_details"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("mailing lists: unexpected response shape: %w", err)
	}
	str := func(m map[string]any, k string) string {
		if v, ok := m[k].(string); ok {
			return v
		}
		return ""
	}
	now := time.Now().UTC().Format(historyTimeFormat)
	out := make([]listCounts, 0, len(body.Lists))
	for _, l := range body.Lists {
		lc := listCounts{
			ListKey:  str(l, "listkey"),
			ListName: str(l, "listname"),
			Contacts: atoiLoose(str(l, "noofcontacts")),
			Unsubs:   atoiLoose(str(l, "noofunsubcnt")),
			Bounces:  atoiLoose(str(l, "noofbouncecnt")),
		}
		if lc.ListKey == "" {
			continue
		}
		out = append(out, lc)
		var prev listCounts
		err := db.DB().QueryRowContext(ctx, `
			SELECT contacts, unsubs, bounces FROM list_count_snapshots
			WHERE listkey = ? ORDER BY taken_at DESC LIMIT 1`, lc.ListKey).
			Scan(&prev.Contacts, &prev.Unsubs, &prev.Bounces)
		if err == nil && prev.Contacts == lc.Contacts && prev.Unsubs == lc.Unsubs && prev.Bounces == lc.Bounces {
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("reading last list snapshot: %w", err)
		}
		if _, err := db.DB().ExecContext(ctx, `
			INSERT OR REPLACE INTO list_count_snapshots (listkey, listname, taken_at, contacts, unsubs, bounces)
			VALUES (?,?,?,?,?,?)`,
			lc.ListKey, lc.ListName, now, lc.Contacts, lc.Unsubs, lc.Bounces); err != nil {
			return nil, fmt.Errorf("writing list snapshot: %w", err)
		}
	}
	return out, nil
}

// campaignRef identifies a sent campaign for history work.
type campaignRef struct {
	Key      string `json:"campaign_key"`
	Name     string `json:"campaign_name"`
	SentGMT  string `json:"sent_time_gmt"` // epoch millis as string
	SentTime time.Time
}

func parseEpochMillis(s string) time.Time {
	ms := atoiLoose(s)
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// sortCampaignRefs orders refs most-recent-first; refs with unknown sent
// times sort last so bounded "first N" caps take genuinely recent campaigns.
func sortCampaignRefs(refs []campaignRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		a, b := refs[i].SentTime, refs[j].SentTime
		if a.IsZero() != b.IsZero() {
			return !a.IsZero()
		}
		return a.After(b)
	})
}

// sentCampaignsSince returns sent campaigns whose sent time falls inside the
// window, most-recent-first. Local synced rows are preferred; when none exist
// and live is allowed it falls back to one /recentcampaigns call.
func sentCampaignsSince(ctx context.Context, c *client.Client, db *store.Store, cutoff time.Time, allowLive bool) ([]campaignRef, error) {
	refs := localSentCampaigns(ctx, db, cutoff)
	if len(refs) > 0 || !allowLive || c == nil {
		sortCampaignRefs(refs)
		return refs, nil
	}
	data, err := c.Get(ctx, "/recentcampaigns", map[string]string{
		"resfmt": "JSON", "status": "sent", "fromindex": "1", "range": "100", "sort": "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("fetching recent campaigns: %w", err)
	}
	if code, msg := zohoErrorCode(data); code != "" {
		if code == "6101" { // no campaigns in this view — a valid empty state
			return nil, nil
		}
		return nil, fmt.Errorf("recent campaigns: zoho error %s: %s", code, msg)
	}
	var body struct {
		Campaigns []map[string]any `json:"recent_campaigns"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("recent campaigns: unexpected response shape: %w", err)
	}
	for _, raw := range body.Campaigns {
		ref := campaignRefFromMap(raw)
		if ref.Key == "" {
			continue
		}
		if !ref.SentTime.IsZero() && ref.SentTime.Before(cutoff) {
			continue
		}
		refs = append(refs, ref)
	}
	sortCampaignRefs(refs)
	return refs, nil
}

func campaignRefFromMap(raw map[string]any) campaignRef {
	str := func(k string) string {
		if v, ok := raw[k].(string); ok {
			return v
		}
		return ""
	}
	ref := campaignRef{Key: str("campaign_key"), Name: str("campaign_name"), SentGMT: str("sent_time_gmt")}
	ref.SentTime = parseEpochMillis(ref.SentGMT)
	return ref
}

// localSentCampaigns reads synced campaign rows from the generic resources
// table. Drain-first: single result set, no nested queries.
func localSentCampaigns(ctx context.Context, db *store.Store, cutoff time.Time) []campaignRef {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT data FROM resources WHERE resource_type IN ('campaigns', 'campaign')`)
	if err != nil {
		return nil
	}
	blobs := make([]json.RawMessage, 0)
	for rows.Next() {
		var data json.RawMessage
		if err := rows.Scan(&data); err != nil {
			continue
		}
		blobs = append(blobs, data)
	}
	_ = rows.Err()
	_ = rows.Close()
	refs := make([]campaignRef, 0, len(blobs))
	for _, b := range blobs {
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			continue
		}
		if status, _ := raw["campaign_status"].(string); !strings.EqualFold(status, "sent") {
			continue
		}
		ref := campaignRefFromMap(raw)
		if ref.Key == "" || (cutoff != (time.Time{}) && !ref.SentTime.IsZero() && ref.SentTime.Before(cutoff)) {
			continue
		}
		refs = append(refs, ref)
	}
	return refs
}

// inWindowCampaignKeys returns campaign keys known (from refs, report
// snapshots, or synced campaign rows) to be sent within the window. An empty
// map means window membership is unknown — callers should aggregate
// unfiltered and say so rather than silently returning nothing.
func inWindowCampaignKeys(ctx context.Context, db *store.Store, refs []campaignRef, cutoff time.Time) map[string]bool {
	keys := map[string]bool{}
	for _, r := range refs {
		if r.SentTime.IsZero() || !r.SentTime.Before(cutoff) {
			keys[r.Key] = true
		}
	}
	rows, err := db.DB().QueryContext(ctx, `
		SELECT campaign_key, MAX(sent_time) FROM campaign_report_snapshots GROUP BY campaign_key`)
	if err == nil {
		type pair struct{ key, sent string }
		pairs := make([]pair, 0)
		for rows.Next() {
			var k string
			var st sql.NullString
			if err := rows.Scan(&k, &st); err != nil {
				continue
			}
			pairs = append(pairs, pair{k, st.String})
		}
		_ = rows.Err()
		_ = rows.Close()
		for _, p := range pairs {
			st := parseEpochMillis(p.sent)
			if st.IsZero() || !st.Before(cutoff) {
				keys[p.key] = true
			}
		}
	}
	for _, r := range localSentCampaigns(ctx, db, cutoff) {
		keys[r.Key] = true
	}
	return keys
}

// sqlInPlaceholders renders "?,?,?" for len(keys) and the matching arg slice.
func sqlInPlaceholders(keys map[string]bool) (string, []any) {
	ph := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for k := range keys {
		ph = append(ph, "?")
		args = append(args, k)
	}
	return strings.Join(ph, ","), args
}

// campaignNameSentFromLocal resolves a campaign's name and sent time from the
// report-snapshot table first, then falls back to synced campaign rows in the
// generic resources table. Returns empty strings when neither source knows it.
func campaignNameSentFromLocal(ctx context.Context, db *store.Store, key string) (name, sentEpochMS string) {
	var n, st sql.NullString
	err := db.DB().QueryRowContext(ctx, `
		SELECT campaign_name, sent_time FROM campaign_report_snapshots
		WHERE campaign_key = ? ORDER BY taken_at DESC LIMIT 1`, key).Scan(&n, &st)
	if err == nil && n.String != "" {
		return n.String, st.String
	}
	err = db.DB().QueryRowContext(ctx, `
		SELECT json_extract(data, '$.campaign_name'), json_extract(data, '$.sent_time_gmt')
		FROM resources
		WHERE resource_type IN ('campaigns', 'campaign')
		  AND json_extract(data, '$.campaign_key') = ? LIMIT 1`, key).Scan(&n, &st)
	if err == nil {
		return n.String, st.String
	}
	return "", ""
}

// ensureRecipientActions fetches /getcampaignrecipientsdata for each
// (campaign, action) pair not yet cached, bounded by maxCampaigns.
// Returns how many campaigns were scanned live.
func ensureRecipientActions(ctx context.Context, c *client.Client, db *store.Store, refs []campaignRef, actions []string, maxCampaigns int) (int, error) {
	if cliutil.IsDogfoodEnv() && maxCampaigns > 1 {
		maxCampaigns = 1
	}
	scanned := 0
	now := time.Now().UTC().Format(historyTimeFormat)
	for _, ref := range refs {
		if scanned >= maxCampaigns {
			break
		}
		fetchedAny := false
		for _, action := range actions {
			var exists int
			err := db.DB().QueryRowContext(ctx, `
				SELECT 1 FROM recipient_action_syncs WHERE campaign_key = ? AND action = ?`,
				ref.Key, action).Scan(&exists)
			if err == nil {
				continue // already cached
			}
			if err != sql.ErrNoRows {
				return scanned, fmt.Errorf("reading action sync state: %w", err)
			}
			// The endpoint's default page is only 20 rows; fromindex/range are
			// undocumented but live-verified. Paginate with a hard row cap so a
			// huge send cannot burn the 500-calls/5-min budget.
			const pageSize = 500
			const maxRowsPerAction = 4000
			fetched := 0
			for fromIndex := 1; fetched < maxRowsPerAction; fromIndex += pageSize {
				data, err := c.Get(ctx, "/getcampaignrecipientsdata", map[string]string{
					"resfmt":      "JSON",
					"campaignkey": ref.Key,
					"action":      action,
					"fromindex":   strconv.Itoa(fromIndex),
					"range":       strconv.Itoa(pageSize),
				})
				if err != nil {
					return scanned, fmt.Errorf("recipients %s/%s: %w", ref.Key, action, err)
				}
				fetchedAny = true
				if code, _ := zohoErrorCode(data); code != "" {
					break // empty page / no more data — Zoho signals it as an error envelope
				}
				var body struct {
					Details []map[string]any `json:"list_of_details"`
				}
				if err := json.Unmarshal(data, &body); err != nil || len(body.Details) == 0 {
					break
				}
				for _, d := range body.Details {
					str := func(k string) string {
						if v, ok := d[k].(string); ok {
							return v
						}
						return ""
					}
					email := strings.ToLower(strings.TrimSpace(str("contactemailaddress")))
					if email == "" {
						continue
					}
					if _, err := db.DB().ExecContext(ctx, `
						INSERT OR REPLACE INTO recipient_actions
						(campaign_key, email, action, first_name, last_name, company, fetched_at)
						VALUES (?,?,?,?,?,?,?)`,
						ref.Key, email, action, str("contactfn"), str("contactln"), str("companyname"), now); err != nil {
						return scanned, fmt.Errorf("writing recipient action: %w", err)
					}
				}
				fetched += len(body.Details)
				if len(body.Details) < pageSize {
					break
				}
			}
			// Mark synced even for empty/error-envelope results so we don't
			// refetch the same empty set every run.
			if _, err := db.DB().ExecContext(ctx, `
				INSERT OR REPLACE INTO recipient_action_syncs (campaign_key, action, fetched_at)
				VALUES (?,?,?)`, ref.Key, action, now); err != nil {
				return scanned, fmt.Errorf("writing action sync state: %w", err)
			}
		}
		if fetchedAny {
			scanned++
		}
	}
	return scanned, nil
}
