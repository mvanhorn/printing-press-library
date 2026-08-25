// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared iClassPro plumbing for the hand-written commands.
//
// Kept in its own file so `generate --force` preserves it: everything here is
// specific to the portal's envelope conventions and has no generated counterpart.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/store"
)

// icpEnvelope is the shape every Open API response shares. `data` is `false`
// rather than null when the account is unknown, so it is decoded loosely.
type icpEnvelope struct {
	Data         json.RawMessage `json:"data"`
	Message      string          `json:"message"`
	TotalRecords int             `json:"totalRecords"`
	CampTypeName string          `json:"campTypeName"`
}

// icpGate classifies why an endpoint returned no rows. The portal reports both
// gates with HTTP 200, so the message string is the only signal available.
type icpGate string

const (
	icpGateNone     icpGate = "open"
	icpGateSignIn   icpGate = "sign-in-required"
	icpGatePlan     icpGate = "plan-required"
	icpGateNotFound icpGate = "account-not-found"
	icpGateEmpty    icpGate = "empty"
)

func icpClassifyMessage(msg string) icpGate {
	m := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case m == "":
		return icpGateNone
	case strings.Contains(m, "sign in"):
		return icpGateSignIn
	case strings.Contains(m, "subscription") || strings.Contains(m, "plan expired"):
		return icpGatePlan
	case strings.Contains(m, "organization not found"):
		return icpGateNotFound
	case strings.Contains(m, "not found") || strings.Contains(m, "no camps") || strings.Contains(m, "no programs") || strings.Contains(m, "no sessions"):
		return icpGateEmpty
	default:
		return icpGateNone
	}
}

// icpGet performs one Open API GET and splits the envelope into rows plus the
// gate the message implies. A gate is never an error: the caller decides whether
// an empty-but-gated result should stop the command or just be reported.
func icpGet(ctx context.Context, c *client.Client, path string, params map[string]string) ([]map[string]any, icpEnvelope, icpGate, error) {
	raw, err := icpGetWithSessionFallback(ctx, c, path, params, nil)
	if err != nil {
		return nil, icpEnvelope{}, icpGateNone, err
	}
	var env icpEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, icpEnvelope{}, icpGateNone, fmt.Errorf("decoding %s: %w", path, err)
	}
	gate := icpClassifyMessage(env.Message)
	rows := make([]map[string]any, 0)
	if len(env.Data) > 0 {
		// `data` is an array for collections, an object for detail endpoints, and
		// literal false when the account does not exist.
		if err := json.Unmarshal(env.Data, &rows); err != nil {
			var one map[string]any
			if err2 := json.Unmarshal(env.Data, &one); err2 == nil && one != nil {
				rows = append(rows, one)
			}
		}
	}
	return rows, env, gate, nil
}

// icpGetWithSessionFallback uses a stored customer JWT when available, but an
// expired token must not break catalog data the account publishes anonymously.
// Only a typed HTTP 401 retries against the Open API; every other error keeps
// its original meaning. Sign-in-gated tenants still return their normal gate
// message from the anonymous retry and can direct the user to auth login.
func icpGetWithSessionFallback(ctx context.Context, c *client.Client, path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	restore, sessionPath, sessionHeaders, hasSession := icpApplySession(c, path, headers)
	if !hasSession {
		return c.GetWithHeaders(ctx, path, params, headers)
	}

	raw, err := c.GetWithHeaders(ctx, sessionPath, params, sessionHeaders)
	restore()
	if err == nil || !icpIsUnauthorized(err) {
		return raw, err
	}
	return c.GetWithHeaders(ctx, path, params, headers)
}

func icpIsUnauthorized(err error) bool {
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

// icpAccountFromPath extracts the portal slug from an Open API path.
func icpAccountFromPath(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(trimmed, '/'); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

// icpApplySession redirects one read to the JWT API when a customer session is
// stored for the account in the path.
//
// The Open API ignores the customer session entirely — it answers "Please sign
// in to see classes." with or without a token. The signed-in portal reads the
// same catalog from https://app.iclasspro.com/api/jwt/v1 with a Bearer header
// and no account segment in the path (the account rides in the token).
//
// The client concatenates BaseURL+path, so the base is swapped for the duration
// of the call and restored by the returned func. Commands run one request at a
// time; there is no concurrent reader to observe the swap.
func icpApplySession(c *client.Client, path string, headers map[string]string) (restore func(), newPath string, newHeaders map[string]string, gated bool) {
	account := icpAccountFromPath(path)
	token := icpTokenFor(account)
	if token == "" {
		return nil, path, headers, false
	}
	merged := make(map[string]string, len(headers)+3)
	for k, v := range headers {
		merged[k] = v
	}
	merged["Authorization"] = "Bearer " + token
	merged["Origin"] = "https://portal.iclasspro.com"
	merged["Referer"] = "https://portal.iclasspro.com/" + account + "/"

	previous := c.BaseURL
	c.BaseURL = icpJWTBase
	return func() { c.BaseURL = previous }, "/" + icpPathWithoutAccount(path), merged, true
}

// icpPathWithoutAccount drops the leading account segment. The JWT API scopes
// every read to the account carried by the token, so the slug that the Open API
// requires in the path would 404 there.
func icpPathWithoutAccount(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(trimmed, '/'); i >= 0 {
		return trimmed[i+1:]
	}
	return ""
}

// icpLocation is the subset of a location record the novel commands need.
type icpLocation struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func icpLocations(ctx context.Context, c *client.Client, account string) ([]icpLocation, icpGate, error) {
	rows, _, gate, err := icpGet(ctx, c, "/"+account+"/locations", nil)
	if err != nil {
		return nil, gate, err
	}
	out := make([]icpLocation, 0, len(rows))
	for _, r := range rows {
		l := icpLocation{Name: fmt.Sprint(r["name"])}
		if v, ok := r["id"].(float64); ok {
			l.ID = int(v)
		}
		if v, ok := r["active"].(bool); ok {
			l.Active = v
		} else {
			l.Active = true
		}
		if l.ID > 0 && l.Active {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, gate, nil
}

// icpCampTypeIDs reads the booking menu and returns the camp type ids.
//
// This is deliberately the booking menu and not `camp-programs`: the ids that
// endpoint returns are programIds, and passing one as `typeId` returns an empty
// camp list with no error. That trap is the single most common way to build a
// broken iClassPro integration.
func icpCampTypeIDs(ctx context.Context, c *client.Client, account string, locationID int) ([]int, icpGate, error) {
	rows, _, gate, err := icpGet(ctx, c, fmt.Sprintf("/%s/bookings/%d", account, locationID), nil)
	if err != nil {
		return nil, gate, err
	}
	ids := make([]int, 0)
	seen := map[int]bool{}
	for _, r := range rows {
		if fmt.Sprint(r["target"]) != "camps" {
			continue
		}
		tp, ok := r["targetParams"].(map[string]any)
		if !ok {
			continue
		}
		if v, ok := tp["typeId"].(float64); ok && int(v) > 0 && !seen[int(v)] {
			seen[int(v)] = true
			ids = append(ids, int(v))
		}
	}
	sort.Ints(ids)
	return ids, gate, nil
}

// icpCollectOptions bounds how much work a catalog walk performs.
type icpCollectOptions struct {
	IncludeClasses bool
	IncludeCamps   bool
	MaxPages       int
	PageSize       int
}

// icpCollection is the result of walking one account's catalog.
type icpCollection struct {
	Entities  []icp.Entity
	Gate      icpGate
	Gated     bool
	Locations []icpLocation
	Pages     int
	Truncated bool
	Warnings  []string
}

// icpCollect walks every active location of an account and returns normalized
// class and camp entities.
func icpCollect(ctx context.Context, c *client.Client, account string, opts icpCollectOptions) (icpCollection, error) {
	if opts.PageSize <= 0 {
		opts.PageSize = 100
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 20
	}
	out := icpCollection{Entities: make([]icp.Entity, 0), Warnings: make([]string, 0)}

	locs, gate, err := icpLocations(ctx, c, account)
	if err != nil {
		return out, err
	}
	out.Gate = gate
	out.Gated = gate == icpGateSignIn
	out.Locations = locs
	if gate == icpGateNotFound {
		return out, nil
	}
	if len(locs) == 0 {
		out.Warnings = append(out.Warnings, "no active locations returned for this account")
		return out, nil
	}

	for _, loc := range locs {
		if opts.IncludeClasses {
			for page := 1; page <= opts.MaxPages; page++ {
				rows, env, g, err := icpGet(ctx, c, "/"+account+"/classes", map[string]string{
					"locationId": strconv.Itoa(loc.ID),
					"limit":      strconv.Itoa(opts.PageSize),
					"page":       strconv.Itoa(page),
				})
				if err != nil {
					return out, fmt.Errorf("classes for location %d: %w", loc.ID, err)
				}
				out.Pages++
				if g == icpGateSignIn {
					out.Gate = icpGateSignIn
					out.Gated = true
					break
				}
				for _, r := range rows {
					out.Entities = append(out.Entities, icp.NormalizeClass(r, account, loc.ID))
				}
				if len(rows) < opts.PageSize || (env.TotalRecords > 0 && page*opts.PageSize >= env.TotalRecords) {
					break
				}
				if page == opts.MaxPages {
					out.Truncated = true
					out.Warnings = append(out.Warnings, fmt.Sprintf(
						"class scan for location %d stopped at the %d-page cap; raise --max-pages to go deeper", loc.ID, opts.MaxPages))
				}
			}
		}
		if opts.IncludeCamps {
			typeIDs, bookingGate, err := icpCampTypeIDs(ctx, c, account, loc.ID)
			if err != nil {
				return out, fmt.Errorf("booking menu for location %d: %w", loc.ID, err)
			}
			if bookingGate == icpGateSignIn {
				out.Gate = icpGateSignIn
				out.Gated = true
				continue
			}
			for _, tid := range typeIDs {
				for page := 1; page <= opts.MaxPages; page++ {
					rows, env, g, err := icpGet(ctx, c, "/"+account+"/camps", map[string]string{
						"locationId": strconv.Itoa(loc.ID),
						"typeId":     strconv.Itoa(tid),
						"limit":      strconv.Itoa(opts.PageSize),
						"page":       strconv.Itoa(page),
						"sortBy":     "time",
					})
					if err != nil {
						return out, fmt.Errorf("camps type %d for location %d: %w", tid, loc.ID, err)
					}
					out.Pages++
					if g == icpGateSignIn {
						out.Gate = icpGateSignIn
						out.Gated = true
						break
					}
					for _, r := range rows {
						e := icp.NormalizeCamp(r, account, loc.ID)
						if e.TypeID == 0 {
							e.TypeID = tid
						}
						out.Entities = append(out.Entities, e)
					}
					if len(rows) < opts.PageSize || (env.TotalRecords > 0 && page*opts.PageSize >= env.TotalRecords) {
						break
					}
					if page == opts.MaxPages {
						out.Truncated = true
						out.Warnings = append(out.Warnings, fmt.Sprintf(
							"camp scan for location %d and type %d stopped at the %d-page cap; raise --max-pages to go deeper",
							loc.ID, tid, opts.MaxPages))
					}
				}
			}
		}
	}
	return out, nil
}

// ---------- local store plumbing ----------

func icpDBPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return defaultDBPath("iclasspro-pp-cli")
}

// icpOpenStoreForRead opens the local mirror read-only and reports whether it
// exists yet. A missing mirror is an empty-cache state, not an error: callers
// emit a valid empty result plus a hint naming the sync command.
func icpOpenStoreForRead(ctx context.Context, path string) (*store.Store, bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, false, nil
	}
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, false, fmt.Errorf("opening local mirror at %s: %w", path, err)
	}
	return s, true, nil
}

// icpNoMirror writes the standard "you have not synced yet" response. Machine
// formats get a valid empty payload so an agent is never handed a parse error;
// humans get an actionable hint on stderr.
func icpNoMirror(out interface{ Write([]byte) (int, error) }, errOut interface{ Write([]byte) (int, error) }, flags *rootFlags, dbPath, account string, empty any) error {
	fmt.Fprintf(errOut, "no local mirror at %s\nrun: iclasspro-pp-cli sync %s\n", dbPath, account)
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, empty, flags)
	}
	return nil
}

func icpEnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o750)
}

// icpEntitiesFromSnapshot decodes stored snapshot payloads back into entities.
func icpEntitiesFromSnapshot(raw []json.RawMessage) []icp.Entity {
	out := make([]icp.Entity, 0, len(raw))
	for _, r := range raw {
		var e icp.Entity
		if err := json.Unmarshal(r, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// icpRequireAccount validates the positional account argument shared by every
// hand-written command.
func icpRequireAccount(args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("account is required (the portal slug, e.g. scottsdalegymnastics)")
	}
	return strings.TrimSpace(args[0]), nil
}

// icpGateNote renders a human explanation for a gate, or empty when open.
func icpGateNote(account string, g icpGate) string {
	switch g {
	case icpGateSignIn:
		return fmt.Sprintf("account %q hides its catalog behind customer sign-in; run 'iclasspro-pp-cli auth login %s' to attach a read-only session", account, account)
	case icpGatePlan:
		return fmt.Sprintf("account %q does not have this feature in its iClassPro subscription", account)
	case icpGateNotFound:
		return fmt.Sprintf("account %q was not found; check the portal slug in portal.iclasspro.com/<slug>", account)
	default:
		return ""
	}
}

// icpNow is the single clock the hand-written commands read, so tests and
// dry-runs have one place to pin time if that is ever needed.
func icpNow() time.Time { return time.Now().UTC() }

func icpStat(path string) (os.FileInfo, error) { return os.Stat(path) }

func icpMarshal(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// icpLatestEntities returns the entities recorded by the newest non-empty sync
// for an account. An account that has never synced yields an empty slice and no
// error: that is an empty cache, not a failure.
func icpLatestEntities(ctx context.Context, s *store.Store, account string) ([]icp.Entity, error) {
	runs, err := s.ICPRuns(ctx, account, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return make([]icp.Entity, 0), nil
	}
	raw, err := s.ICPSnapshot(ctx, runs[0].ID)
	if err != nil {
		return nil, err
	}
	return icpEntitiesFromSnapshot(raw), nil
}

// icpParseKinds turns a --kinds value into a set. Returns nil for an invalid
// value so the caller can raise a usage error.
func icpParseKinds(v string) map[string]bool {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", "both", "all":
		return map[string]bool{icp.KindClass: true, icp.KindCamp: true}
	case "class", "classes":
		return map[string]bool{icp.KindClass: true}
	case "camp", "camps":
		return map[string]bool{icp.KindCamp: true}
	default:
		return nil
	}
}

func icpFilterKinds(ents []icp.Entity, want map[string]bool) []icp.Entity {
	if want == nil {
		return ents
	}
	out := make([]icp.Entity, 0, len(ents))
	for _, e := range ents {
		if want[e.Kind] {
			out = append(out, e)
		}
	}
	return out
}

// icpStaleHint writes a stderr hint when the newest recorded sync for an account
// is older than the root --max-age budget. It writes only to stderr, so JSON and
// CSV consumers see a stable stdout; --max-age 0 disables the hint entirely.
//
// This is the local-mirror counterpart to the framework's stale-read hints: the
// iClassPro catalog changes daily, and serving a week-old mirror without saying
// so is how an agent ends up confidently reporting a cancelled class.
func icpStaleHint(ctx context.Context, errOut io.Writer, s *store.Store, flags *rootFlags, account string) {
	if flags == nil || flags.maxAge <= 0 || s == nil {
		return
	}
	runs, err := s.ICPRuns(ctx, account, 1)
	if err != nil || len(runs) == 0 {
		return
	}
	age := icpNow().Sub(runs[0].StartedAt)
	if age <= flags.maxAge {
		return
	}
	fmt.Fprintf(errOut,
		"warning: local mirror for %s was last synced %s ago (--max-age %s); run 'iclasspro-pp-cli sync %s' to refresh\n",
		account, age.Round(time.Minute), flags.maxAge, account)
}

// icpNoLocalData ends a local-only command that has no records for the
// requested account.
//
// This deliberately exits non-zero (code 3, not-found) rather than returning an
// empty success. "I have no data about this account" and "I checked this
// account and there is nothing to report" are different answers, and conflating
// them is exactly the failure this CLI exists to prevent: `lint` reporting
// "clean, no findings" for an account that was never synced is false
// reassurance a user could act on. The structured payload is still written
// first, so an agent gets parseable output alongside the non-zero exit.
func icpNoLocalData(cmd *cobra.Command, flags *rootFlags, payload any, account string) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), payload, flags); err != nil {
			return err
		}
	}
	return notFoundErr(fmt.Errorf(
		"no local data for account %q; run 'iclasspro-pp-cli sync %s' first", account, account))
}
