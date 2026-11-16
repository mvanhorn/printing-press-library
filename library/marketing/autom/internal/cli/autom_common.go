// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written Autom commands: rate-limit and quota
// classification, the local credit ledger, live searches, the snapshot store
// and small formatting helpers.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/store"
)

// searchConcurrency caps parallel searches in the fan-out commands. Autom
// rate-limits per key, so a handful of parallel calls is the useful maximum.
const searchConcurrency = 4

// noSnapshotsHint is the one message shown when no results page is saved yet.
const noSnapshotsHint = `no saved results pages yet: save some with 'autom-pp-cli rank check <domain> --query "<keyword>"', 'autom-pp-cli batch run' or the --save flag of engines/geo compare`

// ---------- 401 classification ----------

// automRateLimitText reports Autom's "Rate limit exceeded" wording, which it
// sends with HTTP 401 for rate limits and key quotas.
func automRateLimitText(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "rate limit")
}

// automRateLimit401Err classifies a 401 rate-limit or quota body as a rate
// limit (exit 7) with a hint, instead of a bad-key auth failure.
func automRateLimit401Err(err error, msg string) error {
	if strings.Contains(strings.ToLower(msg), "quota") {
		return rateLimitErr(fmt.Errorf("%w\nhint: this API key reached its quota. Raise the key's request limits in the Autom dashboard, use another key, or wait for the quota period to reset. Run 'autom-pp-cli usage' to see the key's quotas", err))
	}
	return rateLimitErr(fmt.Errorf("%w\nhint: Autom rate-limited this key (Autom signals rate limits with HTTP 401). Wait a moment and retry, or lower --concurrency on batch commands", err))
}

// isAuthOrRateLimit reports errors that would fail every other request too.
func isAuthOrRateLimit(err error) bool {
	code := ExitCode(err)
	return code == ExitCode(authErr(nil)) || code == ExitCode(rateLimitErr(nil))
}

// ---------- current command ----------

// currentCommandPath is the executing command path without the binary name
// ("rank check"), set before RunE by the hook in autom_fields.go.
var currentCommandPath string

// ---------- credit ledger ----------

// ledgerEntry is one network call that reached Autom and succeeded.
type ledgerEntry struct {
	At      time.Time `json:"at"`
	Command string    `json:"command"`
	Method  string    `json:"method"`
	Path    string    `json:"path"`
	Engine  string    `json:"engine"`
	Query   string    `json:"query,omitempty"`
	Status  int       `json:"status"`
	Credits int       `json:"credits"`           // billed now
	Pending bool      `json:"pending,omitempty"` // async 202: billed by Autom when the job completes
}

var ledgerMu sync.Mutex

func init() {
	client.ResponseObserver = recordLedger
}

// ledgerPath is the JSONL file holding the credit ledger.
func ledgerPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("AUTOM_LEDGER_PATH")); p != "" {
		return p, nil
	}
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credit-ledger.jsonl"), nil
}

// isFreePath reports the Autom endpoints that never consume credits.
func isFreePath(path string) bool {
	p := strings.SplitN(path, "?", 2)[0]
	switch {
	case p == "/v1/usage", p == "/v1/stats":
		return true
	case strings.HasPrefix(p, "/v1/finder/"), strings.HasPrefix(p, "/v1/jobs/"):
		return true
	}
	return false
}

// engineOfPath returns the engine segment of a search path ("/v1/google/news" -> "google").
func engineOfPath(path string) string {
	parts := strings.Split(strings.Trim(strings.SplitN(path, "?", 2)[0], "/"), "/")
	if len(parts) >= 2 && parts[0] == "v1" {
		if parts[1] == "search" {
			return "multi"
		}
		return parts[1]
	}
	return ""
}

// recordLedger appends a successful network call to the ledger. Failures are
// ignored: the ledger is an aid, never a reason to fail a command.
func recordLedger(method, path string, params map[string]string, status int) {
	if cliutil.IsVerifyEnv() || localDataDisabled() {
		return
	}
	credits, pending := 0, false
	if !isFreePath(path) {
		switch {
		case status == 202:
			pending = true
		case status >= 200 && status < 300:
			credits = 1
		}
	}
	e := ledgerEntry{
		At:      time.Now().UTC(),
		Command: currentCommandPath,
		Method:  method,
		Path:    strings.SplitN(path, "?", 2)[0],
		Engine:  engineOfPath(path),
		Query:   params["query"],
		Status:  status,
		Credits: credits,
		Pending: pending,
	}
	p, err := ledgerPath()
	if err != nil {
		return
	}
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Chmod(0o600) // an older or copied ledger keeps owner-only access too
	_, _ = f.Write(append(line, '\n'))
}

// readLedger returns ledger entries at or after since (zero = all). Lines that
// do not parse are skipped, whatever their length.
func readLedger(since time.Time) ([]ledgerEntry, error) {
	p, err := ledgerPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return make([]ledgerEntry, 0), nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make([]ledgerEntry, 0)
	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			var e ledgerEntry
			if json.Unmarshal(line, &e) == nil && (since.IsZero() || !e.At.Before(since)) {
				out = append(out, e)
			}
		}
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}

// ---------- live searches ----------

// searchOpts are the locale options shared by search commands.
type searchOpts struct {
	Country  string
	Lang     string
	Location string
	Page     int
}

// fetchResults runs one organic search on an engine. Results are fetched
// without the response cache: a saved snapshot must reflect a real call.
func fetchResults(ctx context.Context, c *client.Client, e serp.Engine, query string, o searchOpts) ([]serp.Result, error) {
	params := e.Params(query, o.Country, o.Lang, o.Page)
	if o.Location != "" && (e.Name == "google" || e.Name == "bing") {
		params["location"] = o.Location
	}
	raw, err := c.GetNoCache(ctx, e.Path, fullResponse(params))
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	return serp.ParseOrganic(raw)
}

// fetchNews runs one Google News search.
func fetchNews(ctx context.Context, c *client.Client, query, country, lang string) ([]serp.Result, error) {
	params := map[string]string{"query": query}
	if country != "" {
		params["gl"] = strings.ToLower(country)
	}
	if lang != "" {
		params["hl"] = strings.ToLower(lang)
	}
	raw, err := c.GetNoCache(ctx, "/v1/google/news", fullResponse(params))
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	return serp.ParseOrganic(raw)
}

// fetchFailure is one engine, country or keyword that could not be fetched.
type fetchFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

// fanoutFailures converts fan-out errors into JSON-friendly failures plus
// the errors themselves (already classified by fetchResults), in source order.
func fanoutFailures(f []cliutil.FanoutError) ([]fetchFailure, []error) {
	fails := make([]fetchFailure, 0, len(f))
	errs := make([]error, 0, len(f))
	for _, x := range f {
		fails = append(fails, fetchFailure{Source: x.Source, Error: firstLine(x.Err.Error())})
		errs = append(errs, x.Err)
	}
	return fails, errs
}

// allFailedErr reports a fan-out where every source failed. When every
// failure has the same exit code (for example a missing key on every
// engine), that code is kept so callers see an auth or rate-limit exit
// instead of a generic API error.
func allFailedErr(label string, failures []fetchFailure, errs []error) error {
	msg := fmt.Errorf("every %s failed: %s", label, failureSummary(failures))
	if len(errs) == 0 {
		return apiErr(msg)
	}
	code := ExitCode(errs[0])
	for _, e := range errs[1:] {
		if ExitCode(e) != code {
			return apiErr(msg)
		}
	}
	if code == 0 {
		return apiErr(msg)
	}
	return &cliError{code: code, err: msg}
}

// printThenFail writes the partial view (with its fetch_failures) for machine
// callers, then returns err, so every fan-out command reports total failure
// the same way. Human output prints nothing here: the error already names each
// failure (batch run differs on purpose and also prints its per-keyword table).
func printThenFail(cmd *cobra.Command, flags *rootFlags, view any, err error) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if perr := printJSONFiltered(cmd.OutOrStdout(), view, flags); perr != nil {
			return perr
		}
	}
	return err
}

// ---------- local store ----------

// addDBFlag declares the shared --db flag.
func addDBFlag(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "db", "", "Local database path (default: the CLI data directory)")
}

// engineFilter resolves an optional --engine filter for local reads, so the
// same aliases work as on live commands ("" = any engine).
func engineFilter(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	e, err := serp.LookupEngine(s)
	if err != nil {
		return "", usageErr(err)
	}
	return e.Name, nil
}

// openSerpStore opens the local store, creating it when absent.
func openSerpStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("autom-pp-cli")
	}
	if localDataDisabled() {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("%s=1: results are not saved locally", noLocalDataEnvVar)
		}
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	return db, nil
}

// openSerpStoreIfExists opens the local store for read-only commands. It
// returns nil (and prints the hint) when no store exists yet.
func openSerpStoreIfExists(ctx context.Context, cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("autom-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: "+noSnapshotsHint)
		return nil, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if n, err := db.CountSerpSnapshots(ctx); err == nil && n == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: "+noSnapshotsHint)
	}
	return db, nil
}

// saveSnapshot stores a results page and returns its id (0 when saving failed,
// with a warning on stderr: the live answer is still returned).
func saveSnapshot(ctx context.Context, cmd *cobra.Command, db *store.Store, query, engine string, o searchOpts, results []serp.Result) int64 {
	if db == nil || localDataDisabled() {
		return 0
	}
	id, err := db.SaveSerpSnapshot(ctx, store.SerpSnapshot{
		Query: query, Engine: engine, Country: o.Country, Lang: o.Lang, Location: o.Location,
		Command: currentCommandPath, TakenAt: time.Now(),
	}, results)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save the results locally: %v\n", err)
		return 0
	}
	return id
}

// domainPattern accepts host names such as autom.dev or docs.example.co.uk.
var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)

// domainArg normalizes a domain argument (URLs and "www." are accepted) and
// rejects anything that is not a host name. Empty is allowed when optional.
func domainArg(s string, optional bool) (string, error) {
	if strings.TrimSpace(s) == "" && optional {
		return "", nil
	}
	d := serp.NormalizeDomain(s)
	if !domainPattern.MatchString(d) {
		return "", usageErr(fmt.Errorf("%q is not a domain; give a host name such as autom.dev", s))
	}
	return d, nil
}

// ---------- small helpers ----------

// splitCSV splits a comma-separated flag value, trimming blanks.
func splitCSV(s string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sinceCutoff parses a --since duration (7d, 24h, 2w) into a cutoff time.
func sinceCutoff(since string) (time.Time, error) {
	if strings.TrimSpace(since) == "" {
		return time.Time{}, nil
	}
	d, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return time.Time{}, usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
	}
	return time.Now().Add(-d), nil
}

func firstLine(s string) string {
	return strings.SplitN(s, "\n", 2)[0]
}

func failureSummary(f []fetchFailure) string {
	parts := make([]string, 0, len(f))
	for _, x := range f {
		parts = append(parts, x.Source+": "+x.Error)
	}
	return strings.Join(parts, "; ")
}

func joinOrDash(list []string) string {
	if len(list) == 0 {
		return "-"
	}
	return strings.Join(list, ", ")
}

// truncateText shortens s to n runes with an ellipsis (n < 2 returns s unchanged).
func truncateText(s string, n int) string {
	r := []rune(s)
	if n < 2 || len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// posLabel renders a position, "-" for absent (0).
func posLabel(p int) string {
	if p <= 0 {
		return "-"
	}
	return fmt.Sprintf("#%d", p)
}
