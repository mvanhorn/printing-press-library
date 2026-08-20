package cli

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/tenderned/internal/store"
)

const tnBaseURL = "https://www.tenderned.nl/papi/tenderned-rs-tns/v2"

// tnPublicationXML fetches the eForms XML for one publication. Uses HTTP
// Basic auth from TENDERNED_USERNAME / TENDERNED_PASSWORD. Returns an
// authErr-shaped error when the credentials are missing so doctor-style
// callers can render an actionable message.
func tnPublicationXML(ctx context.Context, pubID int64) ([]byte, error) {
	user := os.Getenv("TENDERNED_USERNAME")
	pass := os.Getenv("TENDERNED_PASSWORD")
	if user == "" || pass == "" {
		return nil, fmt.Errorf("eForms XML endpoint requires Basic auth: set TENDERNED_USERNAME and TENDERNED_PASSWORD (request credentials via functioneelbeheer@tenderned.nl)")
	}
	url := fmt.Sprintf("%s/publicaties/%d/public-xml", tnBaseURL, pubID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "text/xml")
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("HTTP 401: TENDERNED_USERNAME/PASSWORD rejected by TenderNed (request credentials via functioneelbeheer@tenderned.nl)")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("publication %d not found (404)", pubID)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// tnOpenStore opens the canonical SQLite store for offline queries. The
// path is resolvable through the global --db flag in callers; this helper
// returns the default when path is empty.
func tnOpenStore(ctx context.Context, path string) (*store.Store, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultDBPath("tenderned-pp-cli")
	}
	return store.OpenWithContext(ctx, path)
}

// tnFTSMatch escapes a user-provided substring for the FTS5 MATCH operator.
// We treat input as a phrase rather than parsing FTS syntax.
func tnFTSMatch(q string) string {
	q = strings.ReplaceAll(q, "\"", "\"\"")
	return "\"" + q + "\""
}

// tnNotice is a typed view over the JSON blob stored in the local
// `resources` table for resource_type='notices'. Only the fields that
// drive aggregations and filtering are pulled out.
type tnNotice struct {
	PublicatieID           json.Number  `json:"publicatieId"`
	AanbestedingNaam       string       `json:"aanbestedingNaam"`
	OpdrachtgeverNaam      string       `json:"opdrachtgeverNaam"`
	OpdrachtBeschrijving   string       `json:"opdrachtBeschrijving"`
	PublicatieDatum        string       `json:"publicatieDatum"`
	SluitingsDatum         string       `json:"sluitingsDatum"`
	PublicatieCode         string       `json:"publicatieCode"`
	TypeOpdrachtCode       codeOmschr   `json:"typeOpdrachtCode"`
	ProcedureCode          codeOmschr   `json:"procedureCode"`
	NationaalOfEuropees    codeOmschr   `json:"nationaalOfEuropeesCode"`
	AankondigingCode       codeOmschr   `json:"aankondigingCode"`
	CPVCodes               []cpvEntry   `json:"cpvCodes"`
	NUTSCodes              []codeOmschr `json:"nutsCodes"`
	IsGegund               bool         `json:"isGegund"`
	AfgerondeAanbesteding  bool         `json:"afgerondeAanbesteding"`
	IsVroegtijdigBeeindigd bool         `json:"isVroegtijdigBeeindigd"`
}

type codeOmschr struct {
	Code         string `json:"code"`
	Omschrijving string `json:"omschrijving"`
}

type cpvEntry struct {
	Code            string `json:"code"`
	Omschrijving    string `json:"omschrijving"`
	IsHoofdOpdracht bool   `json:"isHoofdOpdracht"`
}

// tnLoadNotices loads all notices from the local store. Post-load filtering
// is done in Go rather than SQL so callers never interpolate user input
// into a query string.
//
// PATCH: removed whereClause/args params; the original signature allowed
// raw SQL concatenation. No caller used it (all passed ""); filtering
// happens post-load in every command.
//
// Emits a one-line stderr hint when the local cache is empty so callers
// don't silently receive zero-result aggregations and conclude the slice
// is genuinely empty.
func tnLoadNotices(ctx context.Context, s *store.Store) ([]tnNotice, error) {
	q := "SELECT data FROM resources WHERE resource_type IN ('notices','publications')"
	rows, err := s.DB().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tnNotice
	for rows.Next() {
		var blob sql.NullString
		if err := rows.Scan(&blob); err != nil {
			continue
		}
		if !blob.Valid {
			continue
		}
		var n tnNotice
		if json.Unmarshal([]byte(blob.String), &n) == nil {
			out = append(out, n)
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, rerr
	}
	if len(out) == 0 {
		tnWarnEmptyCache(ctx, s)
	}
	return out, nil
}

// tnWarnEmptyCache prints a one-shot stderr hint when the local notices
// cache is empty. The check counts rows in the resources table; only
// triggers when the cache truly has no notices (vs. a filter excluding
// everything). Safe to call from multiple commands per process — the
// once guard suppresses duplicates within a single invocation.
var tnEmptyCacheWarned bool

func tnWarnEmptyCache(ctx context.Context, s *store.Store) {
	if tnEmptyCacheWarned {
		return
	}
	var n int
	row := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM resources WHERE resource_type IN ('notices','publications')")
	if err := row.Scan(&n); err != nil {
		return
	}
	if n == 0 {
		fmt.Fprintln(os.Stderr, "warning: local cache is empty — run 'tenderned-pp-cli sync' to hydrate notices, or the aggregation will return all zeros")
		tnEmptyCacheWarned = true
	}
}

// tnParseDate accepts the API's mixed date shapes (YYYY-MM-DD and
// ISO8601 with time/tz) and returns a time.Time at UTC; zero on failure.
func tnParseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// PATCH: time.RFC3339Nano covers fractional-second timestamps that
	// also carry a "Z" / offset suffix ("2026-05-17T10:30:00.123456Z").
	// The earlier list had ".999999" (no tz) and time.RFC3339 (no fraction)
	// but not the combination, so such values fell through all layouts and
	// the notice was silently dropped in every time-filtered command.
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// tnYear returns the 4-digit year for a date string, or 0 if unparseable.
func tnYear(s string) int {
	t := tnParseDate(s)
	if t.IsZero() {
		return 0
	}
	return t.Year()
}

// tnMatchesNational returns true when the notice carries the national
// (sub-threshold) scope marker. TenderNed uses either NA or "Nationaal"
// in the omschrijving across historical exports.
func tnMatchesNational(n tnNotice) bool {
	c := strings.ToUpper(n.NationaalOfEuropees.Code)
	if c == "NA" || c == "NAT" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(n.NationaalOfEuropees.Omschrijving), "nation")
}

// tnHasCPV returns true when any of the notice's CPV codes match any of
// the supplied codes (prefix match, dash-stripped).
func tnHasCPV(n tnNotice, codes []string) bool {
	if len(codes) == 0 {
		return true
	}
	for _, want := range codes {
		want = strings.TrimSpace(strings.ReplaceAll(want, "-", ""))
		if want == "" {
			continue
		}
		for _, c := range n.CPVCodes {
			got := strings.ReplaceAll(c.Code, "-", "")
			if strings.HasPrefix(got, want) {
				return true
			}
		}
	}
	return false
}

// tnSplitCSV splits a comma-separated list and trims surrounding space.
func tnSplitCSV(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// tnPubIDToInt converts the JSON-mixed publicatieId field (string or int)
// to int64.
func tnPubIDToInt(n json.Number) int64 {
	v, _ := n.Int64()
	return v
}

// tnExtractTEDPublicationNumber pulls the OJ-S publication number out of
// an eForms XML payload. eForms cbac:NoticeNumber appears under several
// element paths depending on the form version; we walk the bytes
// pragmatically with a regex rather than full XML parsing.
func tnExtractTEDPublicationNumber(xml []byte) string {
	// Canonical eForms shape: <efbc:NoticePublicationID>NNNNNN-YYYY</efbc:NoticePublicationID>
	re := regexp.MustCompile(`(?s)<(?:efbc|cbc|ext):NoticePublicationID[^>]*>([0-9]{1,7}-[0-9]{4})</`)
	if m := re.FindSubmatch(xml); len(m) == 2 {
		return string(m[1])
	}
	// Fallback: bare cbc:ID directly under the publication header.
	re2 := regexp.MustCompile(`<cbc:ID[^>]*schemeName="ojs-notice-id"[^>]*>([0-9]{1,7}-[0-9]{4})</cbc:ID>`)
	if m := re2.FindSubmatch(xml); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

// tnExtractPredecessorRefs returns notice publication IDs referenced as
// predecessors inside the eForms XML (used by thread reconcile).
func tnExtractPredecessorRefs(xml []byte) []string {
	re := regexp.MustCompile(`(?s)<efac:NoticePurpose>.*?<efbc:NoticePublicationID[^>]*>([0-9]{1,7}-[0-9]{4})</efbc:NoticePublicationID>.*?</efac:NoticePurpose>`)
	matches := re.FindAllSubmatch(xml, -1)
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		ref := string(m[1])
		if !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out
}

// tnPublicationTypeBucket maps the eForms publicatieCode (EF##) to one
// of the lifecycle buckets used by thread reconcile.
func tnPublicationTypeBucket(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	switch {
	case c == "EF01" || c == "EF03" || c == "EF04" || c == "EF05" || c == "EF06" || c == "EF07":
		return "PIN" // Prior Information Notice
	// PATCH: PMC must be checked BEFORE the EF1-prefix CN case below; otherwise
	// strings.HasPrefix(c, "EF1") matches "EF10" first and routes it into CN.
	case c == "EF10":
		return "PMC" // Prior Market Consultation
	case strings.HasPrefix(c, "EF1") || c == "EF16" || c == "EF17" || c == "EF18" || c == "EF20" || c == "EF21" || c == "EF22":
		return "CN" // Contract Notice
	case c == "EF25" || c == "EF26" || c == "EF27" || c == "EF28" || c == "EF29" || c == "EF30" || c == "EF31" || c == "EF32":
		return "CAN" // Contract Award Notice
	case c == "EF36" || c == "EF37" || c == "EF38" || c == "EF39":
		return "MOD" // Modification post-award
	default:
		return "OTHER"
	}
}

// tnTEDCacheLookup returns the cached TED publication number for a
// publicatieId, or "" on miss. The store table is created lazily.
func tnTEDCacheLookup(ctx context.Context, s *store.Store, pubID int64) string {
	if s == nil {
		return ""
	}
	if _, err := s.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tn_ted_cache (
		publicatie_id INTEGER PRIMARY KEY,
		ted_publication_number TEXT NOT NULL,
		cached_at TEXT NOT NULL
	)`); err != nil {
		return ""
	}
	var ted sql.NullString
	err := s.DB().QueryRowContext(ctx,
		`SELECT ted_publication_number FROM tn_ted_cache WHERE publicatie_id = ?`,
		pubID).Scan(&ted)
	if err != nil {
		return ""
	}
	return ted.String
}

// tnTEDCacheStore upserts the TED number for a publicatieId.
func tnTEDCacheStore(ctx context.Context, s *store.Store, pubID int64, ted string) {
	if s == nil || ted == "" {
		return
	}
	if _, err := s.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tn_ted_cache (
		publicatie_id INTEGER PRIMARY KEY,
		ted_publication_number TEXT NOT NULL,
		cached_at TEXT NOT NULL
	)`); err != nil {
		return
	}
	_, _ = s.DB().ExecContext(ctx,
		`INSERT INTO tn_ted_cache(publicatie_id, ted_publication_number, cached_at)
		 VALUES(?, ?, ?) ON CONFLICT(publicatie_id) DO UPDATE SET
		   ted_publication_number=excluded.ted_publication_number,
		   cached_at=excluded.cached_at`,
		pubID, ted, time.Now().UTC().Format(time.RFC3339))
}
