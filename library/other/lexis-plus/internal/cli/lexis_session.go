// Hand-written: Lexis+ citation lookup and Word delivery. Replays the user's own
// logged-in session (cookies stored by `auth login`). One authority at a time;
// no crawling.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/lexis-plus/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/lexis-plus/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/lexis-plus/internal/config"
)

// citeNotFoundError means Lexis+ answered but the citation does not point to one document.
// It is distinct from a request failure (expired session, timeout, server error), which must
// never be reported as an unrecognized citation.
type citeNotFoundError struct{ msg string }

func (e *citeNotFoundError) Error() string { return e.msg }

func isCiteNotFound(err error) bool {
	var nf *citeNotFoundError
	return As(err, &nf)
}

const (
	lexisBase       = "https://plus.lexis.com"
	lexisDomain     = "plus.lexis.com"
	lexisProductID  = "1530671"
	lexisCAFilter   = "jur:1:62,26"
	lexisUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
	lexisSessionMsg = "Lexis+ session missing or expired: run `lexis-plus-pp-cli auth login --chrome` (after `press-auth login plus.lexis.com`)"
)

type lexisSession struct {
	cookie  string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter // nil unless --rate-limit sets an explicit ceiling
}

// errLocalOnly is returned when a command would have to contact Lexis+ under --data-source local.
var errLocalOnly = fmt.Errorf("--data-source local: this needs a request to Lexis+, which local mode does not make; use a copy already on disk or drop --data-source local")

func newLexisSession(flags *rootFlags) (*lexisSession, error) {
	if flags.dataSource == "local" {
		return nil, usageErr(errLocalOnly)
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	cookie := strings.TrimSpace(cfg.CookieCredential())
	if cookie == "" {
		if c, perr := tryPressAuth(lexisDomain); perr == nil {
			cookie = strings.TrimSpace(c)
		}
	}
	if cookie == "" {
		return nil, authErr(fmt.Errorf("%s", lexisSessionMsg))
	}
	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	// Reuse the generated client's transport so --ca-cert, --tls-server-name, and proxy
	// settings apply to Lexis+ requests too. The session sends its own Cookie header, so
	// the shared cookie jar is dropped to avoid sending the session twice.
	flags.applyTLSFlags(cfg)
	shared := client.New(cfg, timeout, flags.rateLimit)
	if tlsErr := shared.TLSConfigError(); tlsErr != nil {
		return nil, configErr(tlsErr)
	}
	hc := *shared.HTTPClient
	hc.Jar = nil
	hc.Timeout = timeout
	s := &lexisSession{cookie: cookie, http: &hc}
	// The commands already pace Lexis+ requests 1-1.5 seconds apart. An explicit --rate-limit
	// adds a hard ceiling on top of that; --rate-limit 0 or the auto default leave pacing as is.
	if flags.rateLimit > 0 {
		s.limiter = cliutil.NewAdaptiveLimiter(flags.rateLimit)
	}
	return s, nil
}

func (s *lexisSession) do(ctx context.Context, method, path string, q url.Values, body []byte, ctype string) ([]byte, *http.Response, error) {
	u := lexisBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Cookie", s.cookie)
	req.Header.Set("User-Agent", lexisUserAgent)
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if s.limiter != nil {
		if err := s.limiter.Wait(ctx); err != nil {
			return nil, nil, err
		}
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, resp, err
	}
	if resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Host != lexisDomain {
		return nil, resp, authErr(fmt.Errorf("%s", lexisSessionMsg))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, resp, authErr(fmt.Errorf("Lexis+ returned HTTP %d. %s", resp.StatusCode, lexisSessionMsg))
	}
	if resp.StatusCode >= 400 {
		return nil, resp, fmt.Errorf("Lexis+ %s %s returned HTTP %d: %s", method, path, resp.StatusCode, snippet(data))
	}
	return data, resp, nil
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// authority is a resolved Lexis document.
type authority struct {
	Cite       string `json:"cite"`
	Title      string `json:"title,omitempty"`
	Kind       string `json:"kind"` // case | statute
	Collection string `json:"collection"`
	DocID      string `json:"doc_id"`
	DocPath    string `json:"doc_path"`
	Status     string `json:"status,omitempty"`
}

var searchIDRe = regexp.MustCompile(`"searchId":"([0-9a-fA-F-]{36}-\d+)"`)

type queueResponse struct {
	IsSdr bool `json:"isSdrResult"`
	Sdr   struct {
		Refs []struct {
			Available bool `json:"available"`
			Single    bool `json:"singleDoc"`
			Links     []struct {
				Href string `json:"href"`
				Req  struct {
					Text string `json:"linkText"`
				} `json:"searchLinkRequest"`
			} `json:"searchResultLinks"`
		} `json:"queryReferences"`
	} `json:"sdrResult"`
	IsMultiple bool `json:"isMultipleSdrResult"`
	Items      []struct {
		Title string `json:"title"`
		Link  struct {
			Href string `json:"href"`
		} `json:"link"`
	} `json:"items"`
	Total int `json:"totalResults"`
}

// resolveCite runs the Lexis+ search box for one citation and returns the document it points to.
func (s *lexisSession) resolveCite(ctx context.Context, cite string) (*authority, error) {
	q := url.Values{}
	q.Set("pdmfid", lexisProductID)
	q.Set("pdsearchterms", cite)
	q.Set("pdtypeofsearch", "searchboxclick")
	q.Set("pdsearchtype", "SearchBox")
	q.Set("pdstartin", "")
	q.Set("pdpsf", lexisCAFilter)
	q.Set("pdqttype", "and")
	q.Set("pdquerytemplateid", "")
	q.Set("pdsf", "")
	q.Set("ecomp", "1phtk")
	q.Set("earg", "pdpsf")
	html, _, err := s.do(ctx, "GET", "/search/", q, nil, "")
	if err != nil {
		return nil, err
	}
	m := searchIDRe.FindSubmatch(html)
	if m == nil {
		return nil, fmt.Errorf("could not find the search id for %q (Lexis+ page layout may have changed)", cite)
	}
	var qr queueResponse
	for attempt := 0; attempt < 12; attempt++ {
		body, _, err := s.do(ctx, "GET", "/f/Search/queue/response/"+string(m[1]), nil, nil, "")
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(body)) > 2 && json.Unmarshal(body, &qr) == nil && (qr.IsSdr || qr.IsMultiple || qr.Total > 0 || attempt > 6) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
	if !qr.IsSdr && qr.IsMultiple && len(qr.Items) > 0 && len(qr.Items) <= 5 {
		// A citation that matches a few documents (typically the current and a future/prior
		// version of a code section). Prefer the one whose title does not mark it as a
		// non-current version; otherwise take Lexis's first result.
		pick := -1
		for i, it := range qr.Items {
			if os.Getenv("LEXIS_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "multi[%d]: %s | %s\n", i, it.Title, it.Link.Href)
			}
			if !strings.HasPrefix(it.Link.Href, "/shared/document/") {
				continue
			}
			if pick < 0 {
				pick = i
			}
			if versionIsCurrent(it.Title, time.Now()) {
				pick = i
				break
			}
		}
		if pick >= 0 {
			a := parseDocHref(cite, qr.Items[pick].Link.Href, "")
			a.Status = fmt.Sprintf("current version picked (Lexis has %d versions of this section)", len(qr.Items))
			return a, nil
		}
	}
	if !qr.IsSdr {
		return nil, &citeNotFoundError{fmt.Sprintf("%q did not resolve to a single document (Lexis+ returned a general search result; check the citation form)", cite)}
	}
	for _, ref := range qr.Sdr.Refs {
		for _, l := range ref.Links {
			if strings.HasPrefix(l.Href, "/shared/document/") {
				return parseDocHref(cite, l.Href, l.Req.Text), nil
			}
		}
	}
	return nil, &citeNotFoundError{fmt.Sprintf("%q was not recognized as a document citation", cite)}
}

func parseDocHref(cite, href, linkText string) *authority {
	parts := strings.Split(strings.TrimPrefix(href, "/shared/document/"), "/")
	a := &authority{Cite: cite, DocPath: href}
	if linkText != "" {
		a.Cite = linkText
	}
	if len(parts) >= 2 {
		a.Collection = parts[0]
		a.DocID = strings.TrimPrefix(parts[len(parts)-1], "urn:contentItem:")
	}
	if a.Collection == "statutes-legislation" {
		a.Kind = "statute"
	} else {
		a.Kind = "case"
	}
	return a
}

var docTitleRes = []*regexp.Regexp{
	regexp.MustCompile(`"(?:documentTitle|docTitle|DocumentTitle|title)"\s*:\s*"([^"]{6,200})"`),
	regexp.MustCompile(`<h1[^>]*>\s*([^<]{6,200}?)\s*</h1>`),
}

// docTitle reads the document title from the document page (Lexis requires one on the delivery job).
func (s *lexisSession) docTitle(ctx context.Context, a *authority) string {
	q := url.Values{}
	q.Set("context", lexisProductID)
	body, _, err := s.do(ctx, "GET", "/api/document/collection/"+a.Collection+"/id/"+a.DocID, q, nil, "")
	if err != nil {
		return ""
	}
	for _, re := range docTitleRes {
		for _, m := range re.FindAllSubmatch(body, 6) {
			t := strings.TrimSpace(string(m[1]))
			if os.Getenv("LEXIS_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "title candidate: %s\n", t)
			}
			if t != "" && !strings.EqualFold(t, "Document page") && !strings.Contains(strings.ToLower(t), "lexis") && !strings.Contains(t, "{") {
				return t
			}
		}
	}
	return ""
}

// suggestTitle asks the search-box type-ahead for the document name.
func (s *lexisSession) suggestTitle(ctx context.Context, cite string) string {
	form := url.Values{}
	form.Set("searchTerm", cite)
	form.Set("pctPrefilter", "")
	form.Set("pdpsf", lexisCAFilter)
	body, _, err := s.do(ctx, "POST", "/r/searchbox/1phtk/wordwheel", nil, []byte(form.Encode()), "application/x-www-form-urlencoded; charset=UTF-8")
	if os.Getenv("LEXIS_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "wordwheel err=%v len=%d head=%s\n", err, len(body), snippet(body))
	}
	if err != nil {
		return ""
	}
	var w struct {
		Collections struct {
			Results []struct {
				Props struct {
					Text string `json:"text"`
				} `json:"props"`
			} `json:"results"`
		} `json:"collections"`
	}
	if json.Unmarshal(body, &w) != nil || len(w.Collections.Results) == 0 {
		return ""
	}
	return w.Collections.Results[0].Props.Text
}

type deliverOpts struct {
	Format      string // docx | pdf | rtf
	Annotations bool
	Headnotes   bool
	Summary     bool
	OutDir      string
	Name        string
	Matter      string
	NoLog       bool // do not record this pull in the retrieval log (used for reading, not keeping)
}

func retrieveOptions(a *authority, o deliverOpts) map[string]any {
	b := func(v bool) string {
		if v {
			return "true"
		}
		return "false"
	}
	// Every key the Lexis+ Download dialog sends; unset ones are the literal string "undefined".
	ro := map[string]any{}
	for _, k := range []string{"adminCodesIncludeAnnotations", "caCasesQuickCiteSignal", "caLegislationQuickCiteSignal", "caseSummary",
		"includeAllProceedingEntries", "includeAnnotations", "includeAnnotationsAndComments", "includeMiscInfo", "includeParticipants",
		"includeSmartIndex", "includeSummary", "showCoreTerms", "showHeadnotes", "showShepardSignal"} {
		ro[k] = "undefined"
	}
	for k, v := range map[string]any{
		"format": o.Format, "fontType": "arial", "fontSize": "10", "lineSpacing": "single", "numberOfColumns": "single",
		"includeCoverPage": "false", "includeLexisNexisLogo": "false", "includeAttachments": "false", "zipFile": "false", "sendAs": "true",
		"embeddedReferences": "true", "displayPaginationInBoldEnabled": "true", "embeddedLegalCitationInItalicTypeEnabled": "true",
		"searchTermsInBoldTypeEnabled": "true", "searchTermsInItalicTypeEnabled": "true", "searchTermsUnderlinedEnabled": "true",
		"displayFirstLastNameEnabled": "false", "pageNumberSelected": "false", "customFooterText": "", "deliveryAnnotation": "false",
		"isIncludeLpaDraftingNotes": "false", "isIncludeLpaInlineSummary": "false", "lpaDraftingNoteOptions": "None",
		"caseSummaryOnly": "false", "displayPageNumberAt": nil,
	} {
		ro[k] = v
	}
	if a.Kind == "statute" {
		ro["includeAnnotations"] = b(o.Annotations)
		ro["caseDocumentOnly"] = "false"
	} else {
		ro["caseDocumentOnly"] = "true"
		ro["caseSummary"] = b(o.Summary)
		ro["showHeadnotes"] = b(o.Headnotes)
		ro["showShepardSignal"] = "true"
		ro["showCoreTerms"] = "false"
	}
	return ro
}

func docDescriptor(a *authority) map[string]any {
	d := map[string]any{
		"cartSourceKey": "Coreapp", "category": "", "clientId": "-None-", "documentNumber": "undefined", "isMultipart": false,
		"itemId": "urn:contentItem:" + a.DocID, "docFullPath": a.DocPath, "lpaDocViewType": "", "masterFeatureId": lexisProductID,
		"title": a.Title, "keyValueInfo": map[string]string{"citation": a.Cite, "currentdoc": "true"},
	}
	if a.Kind == "statute" {
		d["keyValueInfo"] = map[string]string{"citation": "", "currentdoc": "true"}
		d["activeContentName"], d["activeContentPGUID"], d["contentType"], d["productContentTypeId"] = "Codes", "urn:hlct:15", "urn:hlct:15", "urn:pct:83"
	} else {
		d["activeContentName"], d["activeContentPGUID"], d["contentType"], d["productContentTypeId"] = "Cases", "urn:hlct:5", "urn:hlct:5", "urn:pct:30"
	}
	return d
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9 ._,&()'-]+`)

func safeFileName(s string) string {
	s = strings.TrimSpace(unsafeName.ReplaceAllString(s, "_"))
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// deliver queues a Lexis+ delivery job, waits for it, and writes the file. Returns the saved path.
func (s *lexisSession) deliver(ctx context.Context, a *authority, o deliverOpts) (string, error) {
	if o.Format == "" {
		o.Format = "docx"
	}
	name := o.Name
	if name == "" {
		name = safeFileName(a.Cite)
		if a.Title != "" && a.Title != a.Cite {
			name = safeFileName(a.Title + "_" + a.Cite)
		}
	}
	jobBody := map[string]any{
		"deliveryItemType": "DocumentsOnly", "destinationType": 1, "fileName": name,
		"isCourtlinkWorkfolder": false, "isShepardsPeer": false, "kwicWindowValue": "25",
		"masterFeatureId": lexisProductID, "multipleDocuments": []any{docDescriptor(a)},
		"printerId": nil, "retrieveoptions": retrieveOptions(a, o), "searchId": nil,
		"selectedRange": "1-1", "updatedRetrieveOptions": map[string]any{},
	}
	raw, _ := json.Marshal(jobBody)
	resp, _, err := s.do(ctx, "POST", "/f/DeliveryToolbar/CreateMultiDocJobAsync", nil, raw, "application/json")
	if err != nil {
		return "", err
	}
	var job struct {
		JobID string `json:"jobId"`
	}
	if json.Unmarshal(resp, &job) != nil || job.JobID == "" {
		return "", fmt.Errorf("Lexis+ did not return a delivery job id: %s", snippet(resp))
	}
	for i := 0; ; i++ {
		if i > 60 {
			return "", fmt.Errorf("delivery job %s did not finish in time", job.JobID)
		}
		st, _, err := s.do(ctx, "GET", "/f/delivery/"+job.JobID+"/status", nil, nil, "")
		if err != nil {
			return "", err
		}
		var sv struct {
			Status int    `json:"status"`
			Text   string `json:"statusText"`
		}
		_ = json.Unmarshal(st, &sv)
		if sv.Status == 6 || strings.EqualFold(sv.Text, "success") {
			break
		}
		if t := strings.ToLower(sv.Text); strings.Contains(t, "fail") || strings.Contains(t, "error") {
			detail := ""
			if jb, _, e := s.do(ctx, "GET", "/f/delivery/"+job.JobID, nil, nil, ""); e == nil {
				var d struct {
					Msg    json.RawMessage `json:"courierJobStatusMsg"`
					Reason json.RawMessage `json:"reasonCode"`
					Items  []struct {
						Text json.RawMessage `json:"itemStatusText"`
					} `json:"courierItem"`
				}
				_ = json.Unmarshal(jb, &d)
				detail = fmt.Sprintf(" (msg=%s reason=%s", d.Msg, d.Reason)
				if len(d.Items) > 0 {
					detail += fmt.Sprintf(" item=%s", d.Items[0].Text)
				}
				detail += ")"
			}
			return "", fmt.Errorf("Lexis+ delivery job %s failed: %s%s", job.JobID, sv.Text, detail)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(1200 * time.Millisecond):
		}
	}
	jb, _, err := s.do(ctx, "GET", "/f/delivery/"+job.JobID, nil, nil, "")
	if err != nil {
		return "", err
	}
	var jd struct {
		DownloadURI string `json:"downloadUri"`
		Items       []struct {
			Core struct {
				Title json.RawMessage `json:"title"`
			} `json:"coreDataItem"`
		} `json:"courierItem"`
	}
	if json.Unmarshal(jb, &jd) != nil || jd.DownloadURI == "" {
		return "", fmt.Errorf("delivery job %s finished but returned no download location", job.JobID)
	}
	if a.Title == "" && len(jd.Items) > 0 {
		var t string
		if json.Unmarshal(jd.Items[0].Core.Title, &t) == nil {
			a.Title = strings.TrimSpace(t)
		}
		if o.Name == "" && a.Title != "" {
			name = safeFileName(a.Title + "_" + a.Cite)
		}
	}
	ssa := jd.DownloadURI[strings.LastIndex(jd.DownloadURI, "/")+1:]
	ext := "." + o.Format
	q := url.Values{}
	q.Set("ssaId", ssa)
	q.Set("mediaType", "application/octet-stream")
	q.Set("fileName", name+ext)
	data, _, err := s.do(ctx, "GET", "/f/SharedStorage/sharedStorageDelivery", q, nil, "")
	if err != nil {
		return "", err
	}
	if o.Format == "docx" && !bytes.HasPrefix(data, []byte("PK")) {
		return "", fmt.Errorf("downloaded data is not a Word file (%d bytes); the Lexis+ response may have changed", len(data))
	}
	dir := o.OutDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	// Name case files after the case ("Sherman v. Kinetic Concepts, Inc. - 67 Cal. App. 4th 1152.docx").
	if a.Kind == "case" && o.Format == "docx" && o.Name == "" {
		if n := caseNameFromDocx(path); n != "" {
			renamed := filepath.Join(dir, safeFileName(n+" - "+a.Cite)+ext)
			if err := os.Rename(path, renamed); err == nil {
				path = renamed
				a.Title = n
			}
		}
	}
	sum := sha256.Sum256(data)
	if !o.NoLog {
		if err := appendLog(logEntry{Cite: a.Cite, Title: a.Title, Kind: a.Kind, DocID: a.DocID, Path: mustAbs(path), Format: o.Format,
			Annotations: o.Annotations && a.Kind == "statute", Matter: o.Matter, At: time.Now().UTC().Format(time.RFC3339), SHA256: hex.EncodeToString(sum[:])}); err != nil {
			// The download itself succeeded; say plainly that matter commands will not see it.
			fmt.Fprintf(os.Stderr, "warning: saved %s, but could not record it in the retrieval log (%v); `matter list`, `brief gaps`, and reuse will not know about this file\n", path, err)
		}
	}
	return path, nil
}

func mustAbs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// ---- local retrieval log (the user's own pulls; no Lexis content beyond the files they saved) ----

type logEntry struct {
	Cite        string `json:"cite"`
	Title       string `json:"title,omitempty"`
	Kind        string `json:"kind"`
	DocID       string `json:"doc_id"`
	Path        string `json:"path"`
	Format      string `json:"format"`
	Annotations bool   `json:"annotations"`
	Matter      string `json:"matter,omitempty"`
	At          string `json:"at"`
	SHA256      string `json:"sha256"`
}

// lexisDataDir is where the retrieval log and saved red-flag reports live. It resolves through
// the shared path ladder, so --home, LEXIS_PLUS_HOME, and LEXIS_PLUS_DATA_DIR keep separate
// homes' matter history and reports apart.
func lexisDataDir() string {
	if d, err := cliutil.DataDir(); err == nil {
		return d
	}
	return "."
}

// legacyLexisDir is where releases before the shared path ladder kept these records. It is
// read (and migrated) only when no home override is active, so one home never sees another's.
func legacyLexisDir() string {
	if cliutil.HomeOverrideActive() || os.Getenv("LEXIS_PLUS_HOME") != "" || os.Getenv("LEXIS_PLUS_DATA_DIR") != "" {
		return ""
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "lexis-plus-pp-cli")
}

func logPath() string {
	if p := os.Getenv("LEXIS_PLUS_LOG"); p != "" {
		return p
	}
	return filepath.Join(lexisDataDir(), "retrievals.jsonl")
}

// migrateLegacy copies a record from the pre-ladder location to its new path once.
func migrateLegacy(newPath, rel string) {
	old := legacyLexisDir()
	if old == "" || os.Getenv("LEXIS_PLUS_LOG") != "" {
		return
	}
	oldPath := filepath.Join(old, rel)
	if oldPath == newPath {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	b, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(newPath), 0o700) == nil {
		_ = os.WriteFile(newPath, b, 0o600)
	}
}

func appendLog(e logEntry) error {
	p := logPath()
	migrateLegacy(p, "retrievals.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, err = f.Write(append(b, '\n'))
	return err
}

func readLog() []logEntry {
	migrateLegacy(logPath(), "retrievals.jsonl")
	b, err := os.ReadFile(logPath())
	if err != nil {
		return nil
	}
	var out []logEntry
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var e logEntry
		if json.Unmarshal([]byte(ln), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// citeKey normalizes a citation so "27 Cal.3d 167" and "27 Cal. 3d 167" compare equal.
func citeKey(c string) string { return nonAlnum.ReplaceAllString(strings.ToLower(c), "") }

var effectiveRe = regexp.MustCompile(`(?i)\[\s*effective\s+(until\s+)?([A-Za-z]+\.?\s+\d{1,2},\s*\d{4})`)

// versionIsCurrent reads Deering's "[Effective until <date>]" / "[Effective <date>]" title
// tags. A title with no tag is treated as current.
func versionIsCurrent(title string, now time.Time) bool {
	m := effectiveRe.FindStringSubmatch(title)
	if m == nil {
		return true
	}
	d, err := time.Parse("January 2, 2006", strings.Join(strings.Fields(strings.ReplaceAll(m[2], ".", "")), " "))
	if err != nil {
		return false
	}
	if m[1] != "" { // "Effective until <date>": current while now is before that date
		return now.Before(d)
	}
	return !now.Before(d) // "Effective <date>": current once that date has arrived
}

var caseNameRe = regexp.MustCompile(`([A-Z][A-Za-z0-9.,&'\x{2019} -]{2,90}? v\. [A-Za-z0-9.,&'\x{2019} -]{2,90}?)(?: Supreme Court of| Court of Appeal| Appellate Division| \(|,? \d{1,3} Cal)`)
var amPmRe = regexp.MustCompile(`^[AP]M Z `)

// caseNameFromDocx reads the case caption ("X v. Y") from the top of a downloaded opinion.
func caseNameFromDocx(path string) string {
	t, err := readDocText(path)
	if err != nil {
		return ""
	}
	t = strings.Join(strings.Fields(t), " ")
	if len(t) > 4000 {
		t = t[:4000]
	}
	m := caseNameRe.FindStringSubmatch(t)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(amPmRe.ReplaceAllString(m[1], ""))
}
