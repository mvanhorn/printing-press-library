// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

const (
	postmarkServerTokenEnv = "POSTMARK_SERVER_TOKEN"
	postmarkDefaultStream  = "outbound"

	// Postmark uses these names both as bounce types and as suppression
	// reasons.
	postmarkHardBounce    = "HardBounce"
	postmarkSpamComplaint = "SpamComplaint"

	messageStreamTypeInbound    = "Inbound"
	messageStreamTypeBroadcasts = "Broadcasts"

	// postmarkSourceLive and postmarkSourceLocal name where a report's data
	// came from: the Postmark API or the local archive.
	postmarkSourceLive  = "live"
	postmarkSourceLocal = "local"
)

// runMode says whether a command that plans first only printed the plan or
// also carried it out.
type runMode string

const (
	modePlan  runMode = "plan"
	modeApply runMode = "apply"
	modeSend  runMode = "send"
)

// itemStatus is the per-item outcome reported by commands that plan first
// and write only with --yes or --send.
type itemStatus string

const (
	statusPlanned   itemStatus = "planned"
	statusActivated itemStatus = "activated"
	statusFailed    itemStatus = "failed"
	statusSkipped   itemStatus = "skipped"

	resendActivationNotNeeded itemStatus = "not-needed"
	resendActivationShared    itemStatus = "shared"
	resendSendSent            itemStatus = "sent"

	pushStatusDone         itemStatus = "done"
	pushStatusNotAttempted itemStatus = "not-attempted"
)

var shellSafeWord = regexp.MustCompile(`^[A-Za-z0-9@%+=:,./_-]+$`)

// shellQuoteWord quotes s for a POSIX shell so a printed next command can be
// pasted as-is.
func shellQuoteWord(s string) string {
	if s != "" && shellSafeWord.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// postmarkServerArg returns the " --server <name>" part of a next command,
// or "" when name is blank.
func postmarkServerArg(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	return " --server " + shellQuoteWord(name)
}

// bareAddr strips a display name and lowercases: `"Acme" <A@b.co>` -> a@b.co.
func bareAddr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "<"); i >= 0 {
		s = strings.TrimSuffix(strings.TrimSpace(s[i+1:]), ">")
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// addrDomain returns the lowercased domain after the last '@', or "" when
// there is none.
func addrDomain(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(strings.TrimRight(addr[at+1:], ">")))
}

// sameAddr compares two addresses case-insensitively, ignoring surrounding
// whitespace.
func sameAddr(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// isSpamComplaint reports whether a bounce type or suppression reason is a
// spam complaint, which Postmark never lets you reactivate or remove.
func isSpamComplaint(reason string) bool {
	return strings.EqualFold(reason, postmarkSpamComplaint)
}

// hasAccountToken reports whether c can call the account API, which lists
// servers with their tokens and owns domains, senders, and template push.
func hasAccountToken(c *client.Client) bool {
	return c.Config != nil && strings.TrimSpace(c.Config.PostmarkAccountToken) != ""
}

// hasServerToken reports whether cfg carries a server token, which scopes
// requests to one server without an account token lookup.
func hasServerToken(cfg *config.Config) bool {
	return cfg != nil && strings.TrimSpace(cfg.PostmarkServerToken) != ""
}

// newUncachedClient returns a client that bypasses the response cache, for
// commands that must see the server's current state or that write.
func newUncachedClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.NoCache = true
	return c, nil
}

// postmarkGetJSON issues a GET and decodes the body into out.
func postmarkGetJSON(ctx context.Context, c *client.Client, path string, params map[string]string, out any) error {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return classifyAPIErrorOnly(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("parsing %s response: %w", path, err)
	}
	return nil
}

type postmarkStream struct {
	ID                string `json:"ID"`
	Name              string `json:"Name"`
	MessageStreamType string `json:"MessageStreamType"`
	ArchivedAt        any    `json:"ArchivedAt"`
}

// active reports whether the stream is not archived.
func (s postmarkStream) active() bool {
	return s.ArchivedAt == nil
}

// inbound reports whether the stream receives mail; inbound streams never
// send.
func (s postmarkStream) inbound() bool {
	return strings.EqualFold(s.MessageStreamType, messageStreamTypeInbound)
}

// fetchPostmarkStreams returns the server's message streams in API order.
// Request errors come back unclassified so the caller decides how to report
// them.
func fetchPostmarkStreams(ctx context.Context, c *client.Client, params map[string]string) ([]postmarkStream, error) {
	raw, err := c.Get(ctx, "/message-streams", params)
	if err != nil {
		return nil, err
	}
	var doc struct {
		MessageStreams []postmarkStream `json:"MessageStreams"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing message streams: %w", err)
	}
	return doc.MessageStreams, nil
}

// listPostmarkStreams returns the server's message streams sorted by ID.
// outboundOnly drops the inbound stream, which never sends.
func listPostmarkStreams(ctx context.Context, c *client.Client, outboundOnly bool) ([]postmarkStream, error) {
	all, err := fetchPostmarkStreams(ctx, c, nil)
	if err != nil {
		return nil, fmt.Errorf("listing message streams: %w", classifyAPIErrorOnly(err))
	}
	out := make([]postmarkStream, 0, len(all))
	for _, s := range all {
		if outboundOnly && s.inbound() {
			continue
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type suppressionEntry struct {
	EmailAddress      string `json:"EmailAddress"`
	SuppressionReason string `json:"SuppressionReason"`
	Origin            string `json:"Origin"`
	CreatedAt         string `json:"CreatedAt"`
}

// dumpStreamSuppressions returns a stream's suppressions matching email.
// The filter can match partially, so callers compare addresses exactly.
func dumpStreamSuppressions(ctx context.Context, c *client.Client, streamID, email string) ([]suppressionEntry, error) {
	raw, err := c.Get(ctx, "/message-streams/"+url.PathEscape(streamID)+"/suppressions/dump", map[string]string{"EmailAddress": email})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Suppressions []suppressionEntry `json:"Suppressions"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing suppressions for %s: %w", streamID, err)
	}
	return resp.Suppressions, nil
}

// postmarkAccountPageSize is the largest page the account API's server,
// domain, and sender lists return.
const postmarkAccountPageSize = 500

func postmarkPageParams(offset int) map[string]string {
	return map[string]string{"count": strconv.Itoa(postmarkAccountPageSize), "offset": strconv.Itoa(offset)}
}

// postmarkPaging shapes one offset-paged list walk.
type postmarkPaging struct {
	pageSize int
	// maxPages ends the walk after this many pages; 0 means no limit.
	maxPages int
	// untilEmpty keeps paging past a short page, ending only on an empty
	// page or once TotalCount rows were seen.
	untilEmpty bool
}

// walkPostmarkPagesWith reads an offset-paged list one page at a time. fetch
// loads the page at offset and returns its rows and the reported TotalCount;
// visit sees each row and returns true to stop early. The walk ends on an
// empty page, on a short page unless untilEmpty is set, or once TotalCount
// rows were seen. exhausted reports that maxPages ran out first.
func walkPostmarkPagesWith[T any](p postmarkPaging, fetch func(offset int) ([]T, int, error), visit func(T) bool) (exhausted bool, err error) {
	seen := 0
	for page := 0; p.maxPages <= 0 || page < p.maxPages; page++ {
		rows, total, err := fetch(page * p.pageSize)
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if visit(r) {
				return false, nil
			}
		}
		seen += len(rows)
		if len(rows) == 0 || seen >= total || (!p.untilEmpty && len(rows) < p.pageSize) {
			return false, nil
		}
	}
	return true, nil
}

// walkPostmarkPages reads an account-API list one page at a time, ending on
// a short page or once TotalCount rows were seen; see walkPostmarkPagesWith.
func walkPostmarkPages[T any](fetch func(offset int) ([]T, int, error), visit func(T) bool) error {
	_, err := walkPostmarkPagesWith(postmarkPaging{pageSize: postmarkAccountPageSize}, fetch, visit)
	return err
}

// listPostmarkPages collects every row of an account-API list.
func listPostmarkPages[T any](fetch func(offset int) ([]T, int, error)) ([]T, error) {
	var all []T
	err := walkPostmarkPages(fetch, func(r T) bool {
		all = append(all, r)
		return false
	})
	return all, err
}

type postmarkDomainRef struct {
	ID   int64  `json:"ID"`
	Name string `json:"Name"`
}

// postmarkDomainPages returns the page fetcher for the account's sending
// domains, for use with walkPostmarkPages or listPostmarkPages.
func postmarkDomainPages(ctx context.Context, c *client.Client) func(offset int) ([]postmarkDomainRef, int, error) {
	return func(offset int) ([]postmarkDomainRef, int, error) {
		var page struct {
			TotalCount int                 `json:"TotalCount"`
			Domains    []postmarkDomainRef `json:"Domains"`
		}
		if err := postmarkGetJSON(ctx, c, "/domains", postmarkPageParams(offset), &page); err != nil {
			return nil, 0, fmt.Errorf("listing domains: %w", err)
		}
		return page.Domains, page.TotalCount, nil
	}
}

// getPostmarkDomain reads one domain with its DNS details.
func getPostmarkDomain(ctx context.Context, c *client.Client, id int64) (bootstrapDomain, error) {
	var d bootstrapDomain
	err := postmarkGetJSON(ctx, c, "/domains/"+strconv.FormatInt(id, 10), nil, &d)
	return d, err
}

// queryArchive runs query against the local archive and passes each row,
// with every column scanned as a string, to scan. what names the data in the
// query error. The result set is drained and closed before it returns so
// the next query can reuse the connection.
func queryArchive(ctx context.Context, db *store.Store, what, query string, columns int, scan func(cols []string) error, args ...any) error {
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("querying %s: %w", what, err)
	}
	defer rows.Close()
	for rows.Next() {
		cols := make([]string, columns)
		ptrs := make([]any, columns)
		for i := range cols {
			ptrs[i] = &cols[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("scanning local archive: %w", err)
		}
		if err := scan(cols); err != nil {
			return err
		}
	}
	return rows.Err()
}

// printNoLocalMirror says the archive at dbPath is absent and which sync
// populates it; resources narrows the suggested sync when set.
func printNoLocalMirror(w io.Writer, dbPath, resources string) {
	syncCmd := "postmark-pp-cli sync"
	if resources != "" {
		syncCmd += " --resources " + resources
	}
	fmt.Fprintf(w, "no local mirror at %s\nrun: %s --db %s\n", dbPath, syncCmd, dbPath)
}

// localMirrorMissing reports whether dbPath does not exist, printing the
// sync command that creates it when so.
func localMirrorMissing(w io.Writer, dbPath, resources string) bool {
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return false
	}
	printNoLocalMirror(w, dbPath, resources)
	return true
}

// setPostmarkServerToken points cfg at one server. The client sends a saved
// auth_header, or a static X-Postmark-Server-Token header, in place of
// PostmarkServerToken, so both are cleared on this in-memory config;
// otherwise --server, --sandbox, and per-server fan-outs would silently act
// on the saved server instead.
func setPostmarkServerToken(cfg *config.Config, token string) {
	cfg.PostmarkServerToken = token
	cfg.AuthHeaderVal = ""
	for k := range cfg.Headers {
		if strings.EqualFold(k, postmarkServerTokenHeader) {
			delete(cfg.Headers, k)
		}
	}
}

// postmarkEffectiveServerToken is the server token the client will send,
// following its precedence: a static header, then auth_header, then
// PostmarkServerToken.
func postmarkEffectiveServerToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	for k, v := range cfg.Headers {
		if strings.EqualFold(k, postmarkServerTokenHeader) && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(cfg.AuthHeader())
}

// postmarkCredentialHeaderConflict reports a config that sets one Postmark
// token header more than once under different casing with different values.
// The client applies every entry, so which token is sent would depend on map
// order; such a config is refused instead of guessed at.
func postmarkCredentialHeaderConflict(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	for _, header := range []string{postmarkServerTokenHeader, postmarkAccountTokenHeader} {
		seen := ""
		for k, v := range cfg.Headers {
			if !strings.EqualFold(k, header) {
				continue
			}
			v = strings.TrimSpace(v)
			if seen != "" && v != seen {
				return configErr(fmt.Errorf("config headers set %s more than once with different values; keep one", header))
			}
			seen = v
		}
	}
	return nil
}

// postmarkEffectiveAccountToken is the account token the client will send: a
// static X-Postmark-Account-Token header overrides PostmarkAccountToken.
func postmarkEffectiveAccountToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	for k, v := range cfg.Headers {
		if strings.EqualFold(k, postmarkAccountTokenHeader) && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(cfg.PostmarkAccountToken)
}

// postmarkServerScope identifies the server a client acts on by a short hash
// of the server token it will send, so local state can be kept per server
// without storing the token or calling the API. Empty when no server token is
// set.
func postmarkServerScope(c *client.Client) string {
	if c == nil {
		return ""
	}
	token := postmarkEffectiveServerToken(c.Config)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return "token-sha256:" + hex.EncodeToString(sum[:8])
}

// postmarkSecretQueryKey matches query parameter names that usually carry a
// credential in a webhook URL.
var postmarkSecretQueryKey = regexp.MustCompile(`(?i)(token|secret|key|sig|signature|password|passwd|pass|auth|credential)`)

// redactURLSecrets masks the password in a URL's userinfo and the values of
// credential-looking query parameters before the URL is printed. Postmark
// supports basic-auth webhook URLs, so these can hold real credentials.
func redactURLSecrets(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "<unparseable URL>"
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "REDACTED")
		}
	}
	if u.RawQuery != "" {
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			// A query that does not parse cleanly may still hold a credential,
			// so none of it is printed.
			u.RawQuery = "REDACTED"
			return u.String()
		}
		changed := false
		for k := range q {
			if postmarkSecretQueryKey.MatchString(k) {
				q[k] = []string{"REDACTED"}
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
	}
	return u.String()
}
