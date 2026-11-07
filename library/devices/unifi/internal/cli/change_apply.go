// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source live.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

const (
	changeKindCreate = "create"
	changeKindUpdate = "update"
	changeKindDelete = "delete"
	changeKindAction = "action"
	changeKindUndo   = "undo"
)

// changeNow is replaceable so the trial's midnight rule can be tested.
var changeNow = time.Now

type changeSpec struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type changeTrial struct {
	Date           string `json:"date"`
	StartTime      string `json:"start_time"`
	StopTime       string `json:"stop_time"`
	Timezone       string `json:"timezone"`
	ExpiryVerified bool   `json:"expiry_verified"`
	Note           string `json:"note"`
}

type changeResult struct {
	JournalID string       `json:"journal_id,omitempty"`
	State     string       `json:"state"`
	Kind      string       `json:"kind"`
	Method    string       `json:"method"`
	Path      string       `json:"path"`
	ConsoleID string       `json:"console_id,omitempty"`
	Transport string       `json:"transport,omitempty"`
	EntityID  string       `json:"entity_id,omitempty"`
	Verified  bool         `json:"verified"`
	Mismatch  []string     `json:"mismatched_fields,omitempty"`
	Detail    string       `json:"detail,omitempty"`
	Undoable  bool         `json:"undoable"`
	UndoNote  string       `json:"undo_note,omitempty"`
	Reverted  bool         `json:"reverted,omitempty"`
	Trial     *changeTrial `json:"trial,omitempty"`
	Sent      bool         `json:"sent"`
	Preview   bool         `json:"preview,omitempty"`
	DryRun    bool         `json:"dry_run,omitempty"`
	URL       string       `json:"url,omitempty"`
	Body      any          `json:"body,omitempty"`
}

var officialChangePath = regexp.MustCompile(`^/(network|protect)/integration/v1/[A-Za-z0-9._~:/-]+$`)

var firewallPolicyCollection = regexp.MustCompile(`^/network/integration/v1/sites/[^/]+/firewall/policies$`)

// changeKindFor decides how a request is verified and undone. /actions is called out
// because a restart or power-cycle has no inverse and no state to compare.
func changeKindFor(method, path string) string {
	switch method {
	case http.MethodDelete:
		return changeKindDelete
	case http.MethodPut, http.MethodPatch:
		return changeKindUpdate
	}
	if strings.HasSuffix(path, "/actions") {
		return changeKindAction
	}
	return changeKindCreate
}

// readPathFor is where the object can be read back. An action reads its parent entity.
func readPathFor(kind, path string) string {
	if kind != changeKindAction {
		return path
	}
	trimmed := strings.TrimSuffix(path, "/actions")
	if i := strings.Index(trimmed, "/interfaces/ports/"); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

func lastSegment(path string) string {
	return path[strings.LastIndexByte(path, '/')+1:]
}

var changeSitePath = regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/`)

// mirrorTypeForPath maps a collection path to its mirror type, so a verified write can
// refresh the row without waiting for the next sync.
func mirrorTypeForPath(path string) (resourceType, site string) {
	match := changeSitePath.FindStringSubmatch(path)
	if match == nil {
		return "", ""
	}
	rest := strings.TrimPrefix(path, match[0])
	for _, candidate := range []struct{ prefix, rt string }{
		{"firewall/policies", rtFirewall}, {"firewall/zones", rtZones}, {"wifi/broadcasts", rtWifi},
		{"dns/policies", rtDNS}, {"acl-rules", rtACL}, {"traffic-matching-lists", rtTML}, {"networks", rtNetworks},
	} {
		if rest == candidate.prefix || strings.HasPrefix(rest, candidate.prefix+"/") {
			return candidate.rt, match[1]
		}
	}
	return "", match[1]
}

func parseChangeFile(data []byte, trial bool, site string) (changeSpec, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return changeSpec{}, fmt.Errorf("change file is not a JSON object: %w", err)
	}
	if _, enveloped := probe["path"]; enveloped {
		var spec changeSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			return changeSpec{}, fmt.Errorf("change file: %w", err)
		}
		spec.Method = strings.ToUpper(spec.Method)
		return spec, nil
	}
	if !trial {
		return changeSpec{}, errors.New(`change file must be {"method","path","body"}; a bare firewall policy is accepted only with --trial`)
	}
	if site == "" {
		return changeSpec{}, errors.New("--trial with a bare policy file needs --site")
	}
	return changeSpec{Method: http.MethodPost, Path: "/network/integration/v1/sites/" + site + "/firewall/policies", Body: data}, nil
}

func validateChangeSpec(spec changeSpec) error {
	switch spec.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return fmt.Errorf("method %q must be POST, PUT, PATCH or DELETE", spec.Method)
	}
	if !officialChangePath.MatchString(spec.Path) || client.PathHasTraversal(spec.Path) || strings.Contains(spec.Path, "//") || strings.HasSuffix(spec.Path, "/") {
		return fmt.Errorf("path %q must be an official Network or Protect integration path; use 'raw' for anything else", spec.Path)
	}
	if spec.Method != http.MethodDelete && len(spec.Body) > 0 && !json.Valid(spec.Body) {
		return errors.New("body is not valid JSON")
	}
	return nil
}

// applyTrialSchedule rewrites a new firewall policy to stop by itself. The schedule
// takes one date and HH:MM with no timezone, so it needs the console's local time, and
// a window that crosses midnight appears inexpressible. It refuses rather than shorten
// the window silently. Whether a gateway really stops enforcing at expiry is unverified.
func applyTrialSchedule(spec *changeSpec, minutes int, zone string, now time.Time) (*changeTrial, error) {
	if spec.Method != http.MethodPost || !firewallPolicyCollection.MatchString(spec.Path) {
		return nil, errors.New("--trial is only for creating a new firewall policy; it cannot be applied to edits of an existing policy")
	}
	if minutes < 1 {
		return nil, errors.New("--trial-minutes must be at least 1")
	}
	var body map[string]any
	if err := json.Unmarshal(spec.Body, &body); err != nil || body == nil {
		return nil, errors.New("--trial needs a JSON firewall policy body")
	}
	if id, ok := body["id"]; ok && id != nil && id != "" {
		return nil, errors.New("--trial is only for new policies, and this body names an existing id")
	}
	location := time.Local
	if zone != "" {
		loaded, err := time.LoadLocation(zone)
		if err != nil {
			return nil, fmt.Errorf("--trial-timezone %q: %w", zone, err)
		}
		location = loaded
	}
	start := now.In(location)
	stop := start.Add(time.Duration(minutes) * time.Minute)
	if stop.Format("2006-01-02") != start.Format("2006-01-02") {
		return nil, fmt.Errorf("a %d minute trial from %s %s crosses midnight, which the schedule's single date cannot express; shorten it or wait", minutes, start.Format("15:04"), location)
	}
	body["schedule"] = map[string]any{
		"mode":       "ONE_TIME_ONLY",
		"date":       start.Format("2006-01-02"),
		"timeFilter": map[string]any{"startTime": start.Format("15:04"), "stopTime": stop.Format("15:04")},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	spec.Body = encoded
	return &changeTrial{Date: start.Format("2006-01-02"), StartTime: start.Format("15:04"), StopTime: stop.Format("15:04"), Timezone: location.String(), ExpiryVerified: false,
		Note: "Times are in the timezone above; set --trial-timezone if the console's differs. That the gateway stops enforcing at expiry has not been verified live."}, nil
}

// subsetMismatches lists requested fields the server does not show. Only requested
// fields count, since servers add defaults. Secret-named fields are skipped because
// many APIs never echo them, and their absence proves nothing.
func subsetMismatches(prefix string, want, got any) []string {
	if prefix == "" && want == nil {
		return nil // nothing was requested, so nothing can disagree
	}
	wantMap, wantIsMap := want.(map[string]any)
	if !wantIsMap {
		if reflect.DeepEqual(want, got) {
			return nil
		}
		return []string{prefix}
	}
	gotMap, _ := got.(map[string]any)
	out := make([]string, 0)
	for key, value := range wantMap {
		if client.IsSecretField(key) {
			continue
		}
		child := key
		if prefix != "" {
			child = prefix + "." + key
		}
		out = append(out, subsetMismatches(child, value, gotMap[key])...)
	}
	sort.Strings(out)
	return out
}

func decodeAny(data []byte) any {
	var v any
	if len(data) == 0 || json.Unmarshal(data, &v) != nil {
		return nil
	}
	return v
}

// scrubForJournal also reports whether anything was removed: undo must know it cannot
// restore those fields exactly.
func scrubForJournal(data []byte) (json.RawMessage, bool) {
	if len(data) == 0 {
		return nil, false
	}
	decoded := decodeAny(data)
	if decoded == nil {
		return client.ScrubJSON(data), true
	}
	scrubbed := client.ScrubSecrets(decoded)
	encoded, err := json.Marshal(scrubbed)
	if err != nil {
		return nil, true
	}
	return encoded, !reflect.DeepEqual(decoded, scrubbed)
}

func newJournalID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("chg_%d", time.Now().UnixNano())
	}
	return "chg_" + hex.EncodeToString(buf)
}

func statusOf(status int, err error) int {
	var apiError *client.APIError
	if errors.As(err, &apiError) {
		return apiError.StatusCode
	}
	return status
}

// isAmbiguousFailure: the write may have landed. A clean 4xx (other than 408) means
// the server said no, and nothing changed.
func isAmbiguousFailure(status int) bool {
	return status == 0 || status == http.StatusRequestTimeout || status >= 500
}

func refusedWriteError(err error, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusMethodNotAllowed:
		return fmt.Errorf("the console refused this write (HTTP %d). Site Manager documents cloud keys as read-only today, so writes may need --gateway with a local key; run 'unifi-pp-cli reach --probe-writes'. %w", status, err)
	}
	return fmt.Errorf("the console refused this write (HTTP %d), and nothing changed: %w", status, err)
}

type changeRunner struct {
	c    *client.Client
	db   *store.Store
	warn io.Writer
	// caller is the context the operator controls. Follow-up reads outlive it on
	// purpose, but a new write must not: cancelling is not consent to another change.
	caller context.Context
}

func nowStamp() string { return changeNow().UTC().Format(time.RFC3339Nano) }

// settle writes the outcome. The journal row must commit before any mirror write, so
// the journal can never claim less than the mirror shows.
func (r *changeRunner) settle(ctx context.Context, e *store.JournalEntry, state, detail string, after []byte) error {
	e.State, e.Detail, e.UpdatedAt = state, detail, nowStamp()
	if after != nil {
		e.AfterImage, _ = scrubForJournal(after)
	}
	// A dead caller context must not leave the entry pending forever.
	ctx, cancel := followUpContext(ctx)
	defer cancel()
	return r.db.UpdateJournalEntry(ctx, *e)
}

func (r *changeRunner) refreshMirror(e store.JournalEntry, after []byte) {
	resourceType, site := mirrorTypeForPath(e.Path)
	if resourceType == "" || site == "" || after == nil {
		return
	}
	var obj map[string]any
	if json.Unmarshal(after, &obj) != nil || obj == nil {
		return
	}
	obj["parent_id"] = site
	item, err := json.Marshal(obj)
	if err != nil {
		return
	}
	if _, _, err := r.db.UpsertBatch(resourceType, []json.RawMessage{item}); err != nil {
		fmt.Fprintf(r.warn, "warning: mirror not refreshed: %v; the next sync will catch up\n", err)
	}
}

// removeFromMirror reads the same table as the delete hook it replaces, so turning that
// hook off cannot leave behind a type only the hook knew. ReadPath because it names the
// object for every kind; the Path of a journaled create is the collection.
func (r *changeRunner) removeFromMirror(e store.JournalEntry) {
	for _, candidate := range mirroredDeletePatterns {
		match := candidate.pattern.FindStringSubmatch(e.ReadPath)
		if match == nil {
			continue
		}
		if _, err := r.db.DeleteResource(candidate.resourceType, match[2], match[1]); err != nil {
			fmt.Fprintf(r.warn, "warning: mirror row not removed: %v\n", err)
		}
		return
	}
}

// changeFollowUpBudget bounds work after a write was sent. It is separate from the
// caller's deadline because a write that used up that deadline is exactly the one whose
// outcome most needs reading back and recording.
const changeFollowUpBudget = 30 * time.Second

func followUpContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), changeFollowUpBudget)
}

// newChangeRunner turns off the client's delete hook. It fires inside the request, before
// the journal has settled, and journaled writes promise the reverse order; the runner
// updates the mirror itself once the outcome is durable.
func newChangeRunner(c *client.Client, db *store.Store, warn io.Writer) *changeRunner {
	c.SetMutationSuccessHook(nil)
	return &changeRunner{c: c, db: db, warn: warn}
}

// unresolvedExit is non-zero by contract: the caller must not assume anything landed
// or reverted, and the entry tells them what to check.
func unresolvedExit(id, detail string) error {
	return apiErr(fmt.Errorf("change %s is unresolved: %s. It will not be replayed. Check the object, then run sync", id, detail))
}

// applyChange performs: before-image, durable intent, mutation, read-back, outcome.
func (r *changeRunner) applyChange(ctx context.Context, spec changeSpec, autoRevert bool, trial *changeTrial) (changeResult, error) {
	kind := changeKindFor(spec.Method, spec.Path)
	readPath := readPathFor(kind, spec.Path)
	console, route, err := r.c.ResolveRoute(ctx, spec.Path)
	if err != nil {
		return changeResult{}, err
	}
	result := changeResult{Kind: kind, Method: spec.Method, Path: spec.Path, ConsoleID: console, Transport: string(route), Trial: trial, Sent: true}
	var before json.RawMessage
	if kind != changeKindCreate {
		data, status, err := r.c.Do(ctx, http.MethodGet, readPath, nil, nil)
		if err != nil {
			// No before-image means no undo, so the write does not go ahead.
			return result, fmt.Errorf("reading the before-image at %s failed (HTTP %d), so nothing was changed: %w", readPath, statusOf(status, err), err)
		}
		before = data
	}
	scrubbedBefore, beforeHadSecrets := scrubForJournal(before)
	scrubbedBody, _ := scrubForJournal(spec.Body)
	entry := store.JournalEntry{
		ID: newJournalID(), ConsoleID: console, CreatedAt: nowStamp(), UpdatedAt: nowStamp(), State: journalPending, Kind: kind,
		Method: spec.Method, Path: spec.Path, ReadPath: readPath, RequestBody: scrubbedBody, BeforeImage: scrubbedBefore,
		SecretsScrubbed: beforeHadSecrets, Transport: string(route),
	}
	entry.ResourceType, _ = mirrorTypeForPath(spec.Path)
	if kind == changeKindUpdate || kind == changeKindDelete {
		entry.EntityID = lastSegment(spec.Path)
	}
	if err := r.db.InsertJournalEntry(ctx, entry); err != nil {
		return result, fmt.Errorf("journal write failed, so nothing was changed: %w", err)
	}
	result.JournalID = entry.ID
	result.EntityID = entry.EntityID

	var body any
	if len(spec.Body) > 0 {
		body = decodeAny(spec.Body)
	}
	response, status, sendErr := r.c.Do(ctx, spec.Method, spec.Path, nil, body)
	status = statusOf(status, sendErr)
	r.caller = ctx
	ctx, cancel := followUpContext(ctx)
	defer cancel()

	if client.NotSent(sendErr) {
		result.State = journalFailed
		if err := r.settle(ctx, &entry, journalFailed, "not sent: "+sendErr.Error(), nil); err != nil {
			return result, err
		}
		return result, classifyAPIErrorOnly(fmt.Errorf("nothing was sent: %w", sendErr))
	}
	if sendErr != nil && !isAmbiguousFailure(status) {
		result.State = journalFailed
		if err := r.settle(ctx, &entry, journalFailed, fmt.Sprintf("refused with HTTP %d", status), nil); err != nil {
			return result, err
		}
		return result, classifyAPIErrorOnly(refusedWriteError(sendErr, status))
	}

	if sendErr == nil && kind == changeKindCreate {
		if id := asString(asMap(decodeAny(response))["id"]); id != "" {
			entry.EntityID = id
			entry.ReadPath = spec.Path + "/" + id
			result.EntityID = id
		}
	}

	// Ambiguous: read back and never replay. A create with no id cannot be located.
	if sendErr != nil {
		return r.resolveAmbiguous(ctx, &entry, &result, body, before, status)
	}
	return r.verifyApplied(ctx, &entry, &result, spec, body, before, autoRevert)
}

func (r *changeRunner) resolveAmbiguous(ctx context.Context, entry *store.JournalEntry, result *changeResult, body any, before []byte, status int) (changeResult, error) {
	prefix := fmt.Sprintf("the write ended ambiguously (HTTP %d)", status)
	fail := func(detail string) (changeResult, error) {
		result.State, result.Detail = journalUnresolved, detail
		if err := r.settle(ctx, entry, journalUnresolved, detail, nil); err != nil {
			return *result, err
		}
		return *result, unresolvedExit(entry.ID, detail)
	}
	switch entry.Kind {
	case changeKindCreate:
		return fail(prefix + " and a new object has no id to read back, so it may or may not exist")
	case changeKindAction:
		return fail(prefix + " and an action leaves nothing to compare")
	case changeKindDelete:
		_, readStatus, readErr := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil)
		if statusOf(readStatus, readErr) == http.StatusNotFound {
			result.State, result.Verified, result.Detail = journalVerified, true, prefix+"; read-back shows it is gone"
			if err := r.settle(ctx, entry, journalVerified, result.Detail, nil); err != nil {
				return *result, err
			}
			r.removeFromMirror(*entry)
			return *result, nil
		}
		return fail(prefix + "; read-back does not show it gone")
	}
	// If the object already held these values, reading them back says nothing about
	// the write. Journaling that as verified would also let reach claim writes work
	// on a console that may have refused this one.
	if len(subsetMismatches("", body, decodeAny(before))) == 0 {
		return fail(prefix + "; the object already held the requested values, so read-back cannot show whether the write landed")
	}
	data, readStatus, readErr := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil)
	if readErr != nil {
		return fail(fmt.Sprintf("%s and read-back also failed (HTTP %d)", prefix, statusOf(readStatus, readErr)))
	}
	if mismatched := subsetMismatches("", body, decodeAny(data)); len(mismatched) > 0 {
		result.Mismatch = mismatched
		return fail(prefix + "; read-back does not show the requested values, though it could still land")
	}
	result.State, result.Verified, result.Detail = journalVerified, true, prefix+"; read-back shows the requested values"
	if err := r.settle(ctx, entry, journalVerified, result.Detail, data); err != nil {
		return *result, err
	}
	r.refreshMirror(*entry, data)
	return *result, nil
}

func (r *changeRunner) verifyApplied(ctx context.Context, entry *store.JournalEntry, result *changeResult, spec changeSpec, body any, before json.RawMessage, autoRevert bool) (changeResult, error) {
	switch entry.Kind {
	case changeKindAction:
		result.Undoable = false
		result.UndoNote = "restarts and power-cycles cannot be undone"
		result.State, result.Verified, result.Detail = journalVerified, true, "accepted by the API; an action's effect is not directly observable"
		return *result, r.settle(ctx, entry, journalVerified, result.Detail, nil)
	case changeKindDelete:
		_, status, err := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil)
		if statusOf(status, err) == http.StatusNotFound {
			result.State, result.Verified, result.Undoable = journalVerified, true, !entry.SecretsScrubbed
			result.Detail = "read-back shows it is gone"
			result.UndoNote = "undo recreates the object with a new id, which can break references to it"
			if err := r.settle(ctx, entry, journalVerified, result.Detail, nil); err != nil {
				return *result, err
			}
			r.removeFromMirror(*entry)
			return *result, nil
		}
		detail := "delete was accepted but read-back still finds it, or read-back failed"
		result.State, result.Detail = journalUnresolved, detail
		if err := r.settle(ctx, entry, journalUnresolved, detail, nil); err != nil {
			return *result, err
		}
		return *result, unresolvedExit(entry.ID, detail)
	}
	if entry.Kind == changeKindCreate && entry.EntityID == "" {
		detail := "create was accepted but the response has no id to read back"
		result.State, result.Detail = journalUnresolved, detail
		if err := r.settle(ctx, entry, journalUnresolved, detail, nil); err != nil {
			return *result, err
		}
		return *result, unresolvedExit(entry.ID, detail)
	}
	data, status, err := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil)
	if err != nil {
		// The write landed and now the console does not answer. The change may have
		// cut the path, and with no path nothing can revert it from here.
		detail := fmt.Sprintf("the write was accepted but read-back failed (HTTP %d); the change may have cut the path, so it cannot be verified or reverted from here", statusOf(status, err))
		result.State, result.Detail = journalUnresolved, detail
		if settleErr := r.settle(ctx, entry, journalUnresolved, detail, nil); settleErr != nil {
			return *result, settleErr
		}
		return *result, unresolvedExit(entry.ID, detail)
	}
	mismatched := subsetMismatches("", body, decodeAny(data))
	if len(mismatched) == 0 {
		result.State, result.Verified, result.Detail = journalVerified, true, "read-back shows the requested values"
		result.Undoable = entry.Kind == changeKindCreate || !entry.SecretsScrubbed
		if !result.Undoable {
			result.UndoNote = "the before-image held secrets, which are not journaled, so undo would refuse"
		}
		if err := r.settle(ctx, entry, journalVerified, result.Detail, data); err != nil {
			return *result, err
		}
		r.refreshMirror(*entry, data)
		return *result, nil
	}
	result.Mismatch = mismatched
	detail := "read-back differs from the request: " + strings.Join(mismatched, ", ")
	result.State, result.Detail = journalMismatch, detail
	if err := r.settle(ctx, entry, journalMismatch, detail, data); err != nil {
		return *result, err
	}
	if !autoRevert {
		return *result, apiErr(fmt.Errorf("change %s applied but %s. Run 'unifi-pp-cli change undo %s --confirm' to revert", entry.ID, detail, entry.ID))
	}
	return r.autoRevert(ctx, entry, result, before, detail)
}

// autoRevert only runs if the rollback host still answers. If not, a revert cannot be
// sent, and the honest result is unresolved plus a non-zero exit.
func (r *changeRunner) autoRevert(ctx context.Context, entry *store.JournalEntry, result *changeResult, before json.RawMessage, why string) (changeResult, error) {
	failRevert := func(detail string) (changeResult, error) {
		result.State, result.Detail = journalUnresolved, detail
		if err := r.settle(ctx, entry, journalUnresolved, detail, nil); err != nil {
			return *result, err
		}
		return *result, unresolvedExit(entry.ID, detail)
	}
	if r.caller != nil && r.caller.Err() != nil {
		return failRevert(why + "; auto-revert not sent because the command was cancelled or timed out")
	}
	if entry.Kind == changeKindCreate {
		if _, _, err := r.c.Do(ctx, http.MethodDelete, entry.ReadPath, nil, nil); err != nil {
			// Status only: this text is stored, and an API error can quote request values.
			return failRevert(fmt.Sprintf("%s; auto-revert delete failed (HTTP %d)", why, statusOf(0, err)))
		}
	} else {
		if entry.SecretsScrubbed {
			return failRevert(why + "; auto-revert skipped because the before-image held secrets that are not journaled")
		}
		if restorableBody(before) == nil {
			return failRevert(why + "; auto-revert skipped because the before-image is not an object")
		}
		if _, status, err := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil); err != nil {
			return failRevert(fmt.Sprintf("%s; auto-revert skipped because the console no longer answers (HTTP %d)", why, statusOf(status, err)))
		}
		if r.caller != nil && r.caller.Err() != nil {
			return failRevert(why + "; auto-revert not sent because the command was cancelled or timed out")
		}
		if _, _, err := r.c.Do(ctx, http.MethodPut, entry.ReadPath, nil, restorableBody(before)); err != nil {
			return failRevert(fmt.Sprintf("%s; auto-revert write failed (HTTP %d)", why, statusOf(0, err)))
		}
		data, _, err := r.c.Do(ctx, http.MethodGet, entry.ReadPath, nil, nil)
		if err != nil || len(subsetMismatches("", restorableBody(before), decodeAny(data))) > 0 {
			return failRevert(why + "; auto-revert was sent but read-back does not confirm it")
		}
	}
	result.State, result.Reverted, result.Detail = journalReverted, true, why+"; reverted automatically"
	if err := r.settle(ctx, entry, journalReverted, result.Detail, nil); err != nil {
		return *result, err
	}
	return *result, apiErr(fmt.Errorf("change %s did not verify and was reverted: %s", entry.ID, why))
}

// restorableBody strips server-owned fields from a before-image so it can be sent back.
func restorableBody(before []byte) any {
	obj, _ := decodeAny(before).(map[string]any)
	if obj == nil {
		return nil
	}
	delete(obj, "id")
	delete(obj, "metadata")
	delete(obj, "parent_id")
	delete(obj, store.ConsoleIDField)
	return obj
}

func emitChange(cmd *cobra.Command, flags *rootFlags, result changeResult, runErr error) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
			return err
		}
		return runErr
	}
	out := cmd.OutOrStdout()
	if result.Preview || result.DryRun {
		fmt.Fprintf(out, "would send %s %s\nnothing was sent\n", result.Method, firstNonEmpty(result.URL, result.Path))
		if result.UndoNote != "" {
			fmt.Fprintf(out, "%s\n", result.UndoNote)
		}
		return runErr
	}
	fmt.Fprintf(out, "%s: %s %s\n", result.State, result.Method, result.Path)
	if result.JournalID != "" {
		fmt.Fprintf(out, "journal: %s\n", result.JournalID)
	}
	if result.Detail != "" {
		fmt.Fprintf(out, "%s\n", result.Detail)
	}
	return runErr
}

func newNovelChangeApplyCmd(flags *rootFlags) *cobra.Command {
	var flagTrial, confirm, autoRevert bool
	var dbFlag, site, trialZone, changeJSON string
	var trialMinutes int
	cmd := &cobra.Command{
		Use:   "apply [file]",
		Short: "Send one change to an official Network or Protect path with a journaled before-image, a read-back check and undo. Nothing is sent without --confirm.",
		Long: `Use this command to make a change with a recorded before-image, read-back verification
and undo. Do NOT use it to preview consequences; use 'impact' instead.

The file is {"method","path","body"} for an official Network or Protect path, or '-' for
stdin, or pass the same JSON with --change-json. MCP callers must use --change-json: the
server does not open file paths on a caller's behalf. Without --confirm it prints the plan and sends nothing. With --confirm it reads the
before-image, commits a journal entry, sends the write once, and reads it back. An
ambiguous failure is read back and never replayed. Runs in the foreground only.

--trial creates a new firewall policy with a ONE_TIME_ONLY schedule, so the gateway should
stop enforcing it by itself. That expiry has not been verified live, trials cannot cross
midnight, and times must be console-local (--trial-timezone).

Docs say cloud keys are read-only today; a refused write fails clearly and changes nothing.
Exits non-zero when the outcome is unresolved or does not verify.`,
		Example: strings.Trim(`
  unifi-pp-cli change apply --change-json '{"method":"PATCH","path":"/network/integration/v1/sites/SITE/firewall/policies/POLICY","body":{"enabled":false}}' --agent
  unifi-pp-cli change apply firewall-policy.json --trial --site 11111111-2222-3333-4444-555555555555 --agent
  unifi-pp-cli change apply disable-guest-wifi.json --confirm --json
  unifi-pp-cli change apply - --confirm --auto-revert < change.json`, "\n"),
		// Dogfood runs the first Example, so it leads with the inline form. It has no --confirm,
		// so it stops at the preview and sends nothing, and SITE and POLICY are placeholders.
		Annotations: map[string]string{"pp:data-source": "live", "pp:preview-happy-path": "true", "mcp:read-only-unless-flags": "confirm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) == 0 && changeJSON == "" {
				return writeDryRun(cmd.OutOrStdout(), flags, "change apply")
			}
			if (len(args) == 1) == (changeJSON != "") || len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(errors.New("expected one of: a change file, - for stdin, or --change-json"))
			}
			var raw []byte
			var err error
			switch {
			case changeJSON != "":
				raw = []byte(changeJSON)
			// Arguments from an MCP caller are not trusted to name files. This reads as the
			// server's account, and --trial echoes a bare JSON object back in its preview,
			// which made any JSON file on the server readable.
			case cliutil.IsMCPChildEnv() && args[0] != "-":
				return usageErr(errors.New("MCP callers cannot name a file; pass the change as --change-json"))
			case args[0] == "-":
				raw, err = io.ReadAll(cmd.InOrStdin())
			default:
				raw, err = os.ReadFile(args[0])
			}
			if err != nil {
				return usageErr(fmt.Errorf("reading change file: %w", err))
			}
			spec, err := parseChangeFile(raw, flagTrial, site)
			if err != nil {
				return usageErr(err)
			}
			if err := validateChangeSpec(spec); err != nil {
				return usageErr(err)
			}
			var trial *changeTrial
			if flagTrial {
				if trial, err = applyTrialSchedule(&spec, trialMinutes, trialZone, changeNow()); err != nil {
					return usageErr(err)
				}
			}
			kind := changeKindFor(spec.Method, spec.Path)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if flags.dryRun || !confirm {
				url, route, err := c.PreviewTarget(ctx, spec.Path)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				preview := changeResult{State: "not-sent", Kind: kind, Method: spec.Method, Path: spec.Path, URL: url, Transport: string(route), Trial: trial, Sent: false, Undoable: kind != changeKindAction, DryRun: flags.dryRun, Preview: !flags.dryRun}
				if len(spec.Body) > 0 {
					scrubbed, _ := scrubForJournal(spec.Body)
					preview.Body = decodeAny(scrubbed)
				}
				if kind == changeKindAction {
					preview.UndoNote = "restarts and power-cycles cannot be undone"
				}
				if !flags.dryRun {
					fmt.Fprintln(cmd.ErrOrStderr(), "preview only, nothing sent: pass --confirm to apply. Check consequences first with 'unifi-pp-cli impact'.")
				}
				return emitChange(cmd, flags, preview, nil)
			}
			// Refusing here avoids a journal row for a write that was never possible.
			if client.ReadOnlyModeActive() {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("%w: UNIFI_READ_ONLY is set, refusing change apply", client.ErrReadOnlyMode), flags)
			}
			db, err := openMirror(ctx, mirrorDBPath(dbFlag))
			if err != nil {
				return err
			}
			defer db.Close()
			runner := newChangeRunner(c, db, cmd.ErrOrStderr())
			result, runErr := runner.applyChange(ctx, spec, autoRevert, trial)
			if runErr != nil && result.JournalID == "" {
				return classifyAPIError(cmd.OutOrStdout(), runErr, flags)
			}
			return emitChange(cmd, flags, result, runErr)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually apply the change; without it a preview is printed")
	cmd.Flags().BoolVar(&autoRevert, "auto-revert", false, "If read-back does not match, revert when the console still answers; otherwise journal unresolved and exit non-zero")
	cmd.Flags().BoolVar(&flagTrial, "trial", false, "Create a new firewall policy with a ONE_TIME_ONLY schedule so it expires by itself; refused for edits")
	cmd.Flags().IntVar(&trialMinutes, "trial-minutes", 30, "Trial length in minutes; cannot cross midnight")
	cmd.Flags().StringVar(&trialZone, "trial-timezone", "", "Console's IANA timezone for the trial schedule (default: this machine's)")
	cmd.Flags().StringVar(&site, "site", "", "Network site id, for --trial with a bare policy file")
	cmd.Flags().StringVar(&changeJSON, "change-json", "", `The change as inline JSON, {"method","path","body"}, instead of a file`)
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror and journal path (default: the standard mirror)")
	return cmd
}

// undoRefusal explains why a journal entry cannot be undone, "" when it can. Kept pure
// because these rules are the safety contract of undo.
func undoRefusal(e store.JournalEntry) string {
	switch e.State {
	case journalVerified, journalMismatch:
	case journalUndone, journalReverted:
		return "already undone or reverted"
	case journalFailed:
		return "the original write was refused, so there is nothing to undo"
	case journalPending, journalUnresolved:
		return "the outcome of the original write is unknown, so there is no known state to undo from; read the object and run sync"
	default:
		return "unknown journal state " + e.State
	}
	switch e.Kind {
	case changeKindAction:
		return "restarts and power-cycles cannot be undone"
	case changeKindUndo:
		return "this entry is itself an undo; apply a new change instead"
	}
	if (e.Kind == changeKindUpdate || e.Kind == changeKindDelete) && e.SecretsScrubbed {
		return "the before-image held secret fields, which are never journaled, so it cannot be restored exactly"
	}
	if (e.Kind == changeKindUpdate || e.Kind == changeKindDelete) && len(e.BeforeImage) == 0 {
		return "the journal has no before-image"
	}
	// restorableBody yields nil for anything but an object, and a bodyless PUT or POST
	// is not a restore.
	if (e.Kind == changeKindUpdate || e.Kind == changeKindDelete) && restorableBody(e.BeforeImage) == nil {
		return "the journal's before-image is not an object, so it cannot be restored"
	}
	return ""
}

func stripBookkeeping(v any) any {
	obj, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(obj))
	for k, value := range obj {
		if k == store.ConsoleIDField || k == "parent_id" {
			continue
		}
		out[k] = value
	}
	return out
}

// currentMatchesAfter compares on scrubbed forms, because the after-image was scrubbed.
func currentMatchesAfter(current []byte, after []byte) bool {
	scrubbed, _ := scrubForJournal(current)
	return reflect.DeepEqual(stripBookkeeping(decodeAny(scrubbed)), stripBookkeeping(decodeAny(after)))
}

// planUndo returns the inverse request.
func planUndo(e store.JournalEntry) (method, path string, body any) {
	switch e.Kind {
	case changeKindCreate:
		return http.MethodDelete, e.ReadPath, nil
	case changeKindDelete:
		return http.MethodPost, strings.TrimSuffix(e.Path, "/"+lastSegment(e.Path)), restorableBody(e.BeforeImage)
	default:
		return http.MethodPut, e.ReadPath, restorableBody(e.BeforeImage)
	}
}

func (r *changeRunner) undoChange(ctx context.Context, e store.JournalEntry) (changeResult, error) {
	method, path, body := planUndo(e)
	result := changeResult{Kind: changeKindUndo, Method: method, Path: path, ConsoleID: e.ConsoleID, Transport: e.Transport, EntityID: e.EntityID, Sent: true}
	// Journal paths are relative to a console, and site ids repeat across consoles. A
	// deleted object reads as 404 on any other console too, which passes the check below,
	// so without this the undo recreates it on whichever console is now selected.
	console, route, err := r.c.ResolveRoute(ctx, path)
	if err != nil {
		return result, err
	}
	if console != e.ConsoleID {
		return result, usageErr(fmt.Errorf("undo refused: this change was made on console %q, and the current target is %q. Select the original console and try again", e.ConsoleID, console))
	}
	// The same console id over another route is not proof of the same object: on a
	// gateway the id is only a label, so the check above cannot catch a different box.
	if string(route) != e.Transport {
		return result, usageErr(fmt.Errorf("undo refused: this change was made over %s, and the current route is %s. Select the original route and try again", e.Transport, route))
	}
	result.Transport = string(route)
	current, status, readErr := r.c.Do(ctx, http.MethodGet, e.ReadPath, nil, nil)
	status = statusOf(status, readErr)
	if e.Kind == changeKindDelete {
		if readErr == nil || status != http.StatusNotFound {
			return result, usageErr(fmt.Errorf("undo refused: %s should be gone, but it exists or could not be read (HTTP %d); state has moved since the journal", e.ReadPath, status))
		}
	} else {
		if readErr != nil {
			return result, fmt.Errorf("undo refused: could not read current state (HTTP %d), so nothing was changed: %w", status, readErr)
		}
		if !currentMatchesAfter(current, e.AfterImage) {
			return result, usageErr(fmt.Errorf("undo refused: current state differs from the journal's after-image, so something else changed it. Check 'unifi-pp-cli drift' and apply a new change instead"))
		}
	}
	undoEntry := store.JournalEntry{ID: newJournalID(), ConsoleID: e.ConsoleID, CreatedAt: nowStamp(), UpdatedAt: nowStamp(), State: journalPending, Kind: changeKindUndo, Method: method, Path: path, ReadPath: e.ReadPath, ResourceType: e.ResourceType, EntityID: e.EntityID, Undoes: e.ID, Transport: string(route)}
	if body != nil {
		if encoded, err := json.Marshal(body); err == nil {
			undoEntry.RequestBody, _ = scrubForJournal(encoded)
		}
	}
	scrubbedCurrent, _ := scrubForJournal(current)
	undoEntry.BeforeImage = scrubbedCurrent
	if err := r.db.InsertJournalEntry(ctx, undoEntry); err != nil {
		return result, fmt.Errorf("journal write failed, so nothing was changed: %w", err)
	}
	result.JournalID = undoEntry.ID
	response, sendStatus, sendErr := r.c.Do(ctx, method, path, nil, body)
	sendStatus = statusOf(sendStatus, sendErr)
	ctx, cancel := followUpContext(ctx)
	defer cancel()
	unresolved := func(detail string) (changeResult, error) {
		result.State, result.Detail = journalUnresolved, detail
		if err := r.settle(ctx, &undoEntry, journalUnresolved, detail, nil); err != nil {
			return result, err
		}
		return result, unresolvedExit(undoEntry.ID, detail)
	}
	if sendErr != nil && !isAmbiguousFailure(sendStatus) {
		result.State = journalFailed
		if err := r.settle(ctx, &undoEntry, journalFailed, fmt.Sprintf("refused with HTTP %d", sendStatus), nil); err != nil {
			return result, err
		}
		return result, classifyAPIErrorOnly(refusedWriteError(sendErr, sendStatus))
	}
	if client.NotSent(sendErr) {
		result.State = journalFailed
		if err := r.settle(ctx, &undoEntry, journalFailed, "not sent: "+sendErr.Error(), nil); err != nil {
			return result, err
		}
		return result, classifyAPIErrorOnly(fmt.Errorf("nothing was sent: %w", sendErr))
	}
	if sendErr != nil {
		return unresolved(fmt.Sprintf("the undo ended ambiguously (HTTP %d)", sendStatus))
	}
	var after []byte
	switch e.Kind {
	case changeKindCreate:
		_, readStatus, err := r.c.Do(ctx, http.MethodGet, e.ReadPath, nil, nil)
		if statusOf(readStatus, err) != http.StatusNotFound {
			return unresolved("undo delete was accepted but read-back does not show it gone")
		}
		result.Detail = "the created object is gone"
	case changeKindDelete:
		newID := asString(asMap(decodeAny(response))["id"])
		result.Detail = "recreated"
		if newID != "" {
			result.EntityID = newID
			result.Detail = "recreated with new id " + newID + ", which can break references to the old id " + e.EntityID
			undoEntry.ReadPath = path + "/" + newID
			undoEntry.EntityID = newID
			after = response
		}
		result.UndoNote = "recreated objects get new ids"
	default:
		data, readStatus, err := r.c.Do(ctx, http.MethodGet, e.ReadPath, nil, nil)
		if err != nil {
			return unresolved(fmt.Sprintf("undo was accepted but read-back failed (HTTP %d)", statusOf(readStatus, err)))
		}
		if mismatched := subsetMismatches("", body, decodeAny(data)); len(mismatched) > 0 {
			result.Mismatch = mismatched
			return unresolved("undo was accepted but read-back differs: " + strings.Join(mismatched, ", "))
		}
		after = data
		result.Detail = "read-back shows the before-image restored"
	}
	result.State, result.Verified = journalVerified, true
	if err := r.settle(ctx, &undoEntry, journalVerified, result.Detail, after); err != nil {
		return result, err
	}
	e.UndoneBy, e.State, e.UpdatedAt = undoEntry.ID, journalUndone, nowStamp()
	if err := r.db.UpdateJournalEntry(ctx, e); err != nil {
		return result, fmt.Errorf("undo succeeded but marking the journal failed: %w", err)
	}
	if e.Kind == changeKindCreate {
		r.removeFromMirror(e)
	} else {
		r.refreshMirror(undoEntry, after)
	}
	return result, nil
}

func newNovelChangeUndoCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	var dbFlag string
	cmd := &cobra.Command{
		Use:   "undo [journal-id]",
		Short: "Revert a journaled change when current state still matches its after-image.",
		Long: `Reverses a change made with 'change apply'. Without --confirm it prints the plan.

Refuses when current state differs from the journal's after-image, since that means
something else changed the object. Restarts and power-cycles cannot be undone. A deleted
object is recreated with a new id, which can break references to it. A before-image that
held secrets cannot be restored, because secrets are never journaled.`,
		Example: strings.Trim(`
  unifi-pp-cli change undo chg_0123456789abcdef --json
  unifi-pp-cli change undo chg_0123456789abcdef --confirm --agent`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only-unless-flags": "confirm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && len(args) == 0 {
				return writeDryRun(cmd.OutOrStdout(), flags, "change undo")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(errors.New("expected a journal id; see 'unifi-pp-cli change list'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				return notFoundErr(fmt.Errorf("journal entry %s not found: no local journal at %s", args[0], dbPath))
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entry, err := db.GetJournalEntry(ctx, args[0])
			if errors.Is(err, store.ErrJournalEntryNotFound) {
				return notFoundErr(fmt.Errorf("journal entry %s not found; see 'unifi-pp-cli change list'", args[0]))
			}
			if err != nil {
				return err
			}
			if refusal := undoRefusal(entry); refusal != "" {
				return usageErr(fmt.Errorf("undo refused for %s: %s", entry.ID, refusal))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			method, path, body := planUndo(entry)
			if flags.dryRun || !confirm {
				preview := changeResult{JournalID: entry.ID, State: "not-sent", Kind: changeKindUndo, Method: method, Path: path, ConsoleID: entry.ConsoleID, EntityID: entry.EntityID, Sent: false, Undoable: true, DryRun: flags.dryRun, Preview: !flags.dryRun, Body: body}
				if entry.Kind == changeKindDelete {
					preview.UndoNote = "recreates the object with a new id, which can break references to it"
				}
				if !flags.dryRun {
					fmt.Fprintln(cmd.ErrOrStderr(), "preview only, nothing sent: pass --confirm to undo")
				}
				return emitChange(cmd, flags, preview, nil)
			}
			if client.ReadOnlyModeActive() {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("%w: UNIFI_READ_ONLY is set, refusing change undo", client.ErrReadOnlyMode), flags)
			}
			runner := newChangeRunner(c, db, cmd.ErrOrStderr())
			result, runErr := runner.undoChange(ctx, entry)
			if runErr != nil && result.State == "" {
				return runErr
			}
			return emitChange(cmd, flags, result, runErr)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send the undo; without it a preview is printed")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror and journal path (default: the standard mirror)")
	return cmd
}

func newNovelChangeListCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceRaw string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List journaled changes, so an undo can find its id.",
		Example: strings.Trim(`
  unifi-pp-cli change list --since 7d --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "change list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cutoff, err := parseSinceFlag(sinceRaw, "--since", changeNow())
			if err != nil {
				return err
			}
			entries := make([]store.JournalEntry, 0)
			dbPath := mirrorDBPath(dbFlag)
			if !mirrorMissing(dbPath) {
				db, err := openMirror(ctx, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				if entries, err = db.ListJournalEntries(ctx, store.ActiveConsole(), cutoff); err != nil {
					return err
				}
			}
			if limit > 0 && len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No journaled changes.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tCREATED\tSTATE\tKIND\tMETHOD\tPATH")
			for _, e := range entries {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.ID, e.CreatedAt, e.State, e.Kind, e.Method, e.Path)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror and journal path (default: the standard mirror)")
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Only entries newer than this, e.g. 7d")
	cmd.Flags().IntVar(&limit, "limit", 100, "Newest entries to return; 0 for all")
	return cmd
}

func init() {
	// A hook rather than an edit to change.go: that group file is generator-owned and
	// may refresh, and a hook survives regeneration.
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"change"})
		if err != nil || parent == nil || parent.Name() != "change" {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelChangeUndoCmd(flags))
		addNovelCommandIfAbsent(parent, newNovelChangeListCmd(flags))
	})
}
