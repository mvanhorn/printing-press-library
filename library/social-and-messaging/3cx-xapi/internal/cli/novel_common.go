// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the 3CX XAPI novel commands
// (audit, trace, posture, changed, qrollup, diff, provision). These read the
// local SQLite mirror (resource_type keys are the kebab-case sync names) and
// extract fields defensively because the upstream OData JSON uses PascalCase
// keys whose exact nav-property names vary across entity subtypes.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/store"
	"github.com/spf13/cobra"
)

// kebab-case resource_type keys as stored by sync (see defaultSyncResources).
const (
	rtUsers         = "users"
	rtGroups        = "groups"
	rtRingGroups    = "ring-groups"
	rtQueues        = "queues"
	rtReceptionists = "receptionists"
	rtParkings      = "parkings"
	rtInboundRules  = "inbound-rules"
	rtOutboundRules = "outbound-rules"
	rtDidNumbers    = "did-numbers"
	rtTrunks        = "trunks"
	rtActiveCalls   = "active-calls"
	rtActivityLog   = "activity-log"
	rtEventLogs     = "event-logs"
	rtSystemStatus  = "system-status"
	rtBlocklist     = "blocklist"
	rtBlackList     = "black-list-numbers"
	rtFirewall      = "firewall"
	rtReportAudit   = "report-audit-log"
)

// openLocalMirror resolves the DB path, applies the standard missing-mirror
// guard (empty [] for machine output + a stderr hint), and opens the store
// read-only. ok=false means the caller should return nil immediately (the
// guard already wrote output). The returned db is non-nil only when ok=true.
func openLocalMirror(cmd *cobra.Command, flags *rootFlags, dbPath string) (db *store.Store, ok bool, err error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("3cx-xapi-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: 3cx-xapi-pp-cli sync --resources users,groups,ring-groups,queues,trunks,inbound-rules,did-numbers --db %s\n",
			dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	// OpenWithContext (not OpenReadOnly) so migrations run: a DB file that
	// exists but was never synced still gets the empty schema, letting reads
	// return [] gracefully instead of failing with "no such table: resources".
	db, err = store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	return db, true, nil
}

// listObjects returns every synced object of a resource_type as decoded maps.
// A decode failure on one row is skipped, not fatal — a single malformed row
// must not blank an entire audit.
func listObjects(db *store.Store, resourceType string) ([]map[string]json.RawMessage, error) {
	raws, err := db.List(resourceType, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]json.RawMessage, 0, len(raws))
	for _, raw := range raws {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

// jsonString extracts a string field from a decoded object, tolerating either a
// JSON string or a JSON number (3CX sometimes serialises extension numbers as
// either). Returns "" when the key is absent or null.
func jsonString(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

// firstString returns the first non-empty field among keys (handles
// nav-property name variants across OData entity subtypes).
func firstString(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if v := jsonString(obj, k); v != "" {
			return v
		}
	}
	return ""
}

// memberNumbers extracts the .Number values from an array-of-objects field
// (e.g. RingGroup.Members, Queue.Agents). Tries each candidate key in order
// and returns the members of the first key that decodes to a non-empty array.
func memberNumbers(obj map[string]json.RawMessage, keys ...string) []string {
	for _, k := range keys {
		raw, ok := obj[k]
		if !ok {
			continue
		}
		var arr []map[string]json.RawMessage
		if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
			continue
		}
		out := make([]string, 0, len(arr))
		for _, m := range arr {
			if num := jsonString(m, "Number"); num != "" {
				out = append(out, num)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// destinationNumber extracts the .Number from a nested Destination object field
// (e.g. InboundRule.OfficeHoursDestination). Returns "" when absent/empty.
func destinationNumber(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var dest map[string]json.RawMessage
	if json.Unmarshal(raw, &dest) != nil {
		return ""
	}
	return jsonString(dest, "Number")
}

// isAllDigits reports whether s is non-empty and every rune is a digit.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// looksInternalExtension reports whether s is shaped like an internal 3CX
// extension/DN number (2-6 digits, no separators). External transfer targets
// (E.164, long PSTN numbers) and named/empty destinations are excluded so the
// audit does not flag legitimate off-PBX routes as dangling.
func looksInternalExtension(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || len(s) > 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// dnNumberSet builds the set of every valid DN number on the PBX (users, ring
// groups, queues, receptionists/IVRs, parkings). These are the legitimate
// internal routing targets; anything referencing a number outside this set is
// a dangling reference.
func dnNumberSet(db *store.Store) (map[string]bool, error) {
	set := map[string]bool{}
	for _, rt := range []string{rtUsers, rtRingGroups, rtQueues, rtReceptionists, rtParkings} {
		objs, err := listObjects(db, rt)
		if err != nil {
			return nil, err
		}
		for _, o := range objs {
			if num := jsonString(o, "Number"); num != "" {
				set[num] = true
			}
		}
	}
	return set, nil
}

// auditFinding is one dangling-reference detection. Pure data so the detector
// is unit-testable without a database.
type auditFinding struct {
	Kind      string `json:"kind"`       // ring-group-member | queue-agent | inbound-rule-destination
	Object    string `json:"object"`     // owning object's number or name
	ObjectKey string `json:"object_key"` // human label of the owning object
	Ref       string `json:"ref"`        // the missing/dangling number referenced
	Detail    string `json:"detail"`
}

// findDanglingRefs is the pure audit core: given the valid DN set and the
// decoded ring groups, queues, and inbound rules, return every reference to a
// number not in the valid set. Members/agents are always internal so any
// missing number is flagged; rule destinations are only flagged when the
// number is internal-shaped (avoids flagging external transfer targets).
func findDanglingRefs(valid map[string]bool, ringGroups, queues, inboundRules []map[string]json.RawMessage) []auditFinding {
	var findings []auditFinding
	for _, rg := range ringGroups {
		owner := firstString(rg, "Number", "Name")
		name := firstString(rg, "Name", "Number")
		for _, ref := range memberNumbers(rg, "Members", "Agents") {
			if !valid[ref] {
				findings = append(findings, auditFinding{
					Kind: "ring-group-member", Object: owner, ObjectKey: name, Ref: ref,
					Detail: fmt.Sprintf("ring group %q has member %s that is not a live extension", name, ref),
				})
			}
		}
	}
	for _, q := range queues {
		owner := firstString(q, "Number", "Name")
		name := firstString(q, "Name", "Number")
		for _, ref := range memberNumbers(q, "Agents", "Members") {
			if !valid[ref] {
				findings = append(findings, auditFinding{
					Kind: "queue-agent", Object: owner, ObjectKey: name, Ref: ref,
					Detail: fmt.Sprintf("queue %q has agent %s that is not a live extension", name, ref),
				})
			}
		}
	}
	for _, r := range inboundRules {
		name := firstString(r, "RuleName", "Id")
		for _, destKey := range []string{"OfficeHoursDestination", "OutOfOfficeHoursDestination", "HolidaysDestination"} {
			ref := destinationNumber(r, destKey)
			if ref == "" || valid[ref] || !looksInternalExtension(ref) {
				continue
			}
			findings = append(findings, auditFinding{
				Kind: "inbound-rule-destination", Object: name, ObjectKey: name, Ref: ref,
				Detail: fmt.Sprintf("inbound rule %q %s points at %s which is not a live extension/queue/ring group", name, destKey, ref),
			})
		}
	}
	return findings
}

// extractTimestamp pulls the most likely event timestamp from a decoded object,
// trying common 3CX field names and both RFC3339 and OData /Date(...)/ formats.
// Returns the zero time and false when no parseable timestamp is present.
func extractTimestamp(obj map[string]json.RawMessage) (time.Time, bool) {
	for _, k := range []string{"Time", "TimeGenerated", "GenerationTime", "StartTime", "EndTime", "Date", "Timestamp", "CreatedAt", "ServerTime"} {
		v := jsonString(obj, k)
		if v == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, true
		}
		if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// expandSyncHint is the canonical sync invocation that pulls the OData
// navigation properties (RingGroup.Members, Queue.Agents) the graph commands
// need. Without $expand the upstream omits these, so memberships look empty.
const expandSyncHint = "3cx-xapi-pp-cli sync --resources ring-groups,queues " +
	"--resource-param 'ring-groups:$expand=Members' --resource-param 'queues:$expand=Agents'"

// hintMembersNotExpanded warns (to stderr only) when ring groups / queues are
// present in the mirror but none carry member/agent arrays — the tell-tale sign
// the resource was synced without $expand. Returns true when the hint fired.
func hintMembersNotExpanded(cmd *cobra.Command, ringGroups, queues []map[string]json.RawMessage) bool {
	objs := append(append([]map[string]json.RawMessage{}, ringGroups...), queues...)
	if len(objs) == 0 {
		return false
	}
	for _, o := range objs {
		if len(memberNumbers(o, "Members", "Agents")) > 0 {
			return false // at least one has members -> expanded
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"hint: ring groups/queues have no member data — re-sync with $expand to populate memberships:\n  %s\n",
		expandSyncHint)
	return true
}

// machineOut reports whether output should be machine-shaped JSON (json/agent
// flags set, or stdout is not a terminal).
func machineOut(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())
}

// ctxForNovel returns a timeout-bounded context for novel commands that make
// live calls. Callers must defer the cancel.
func ctxForNovel(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	return boundCtx(cmd.Context(), flags)
}
