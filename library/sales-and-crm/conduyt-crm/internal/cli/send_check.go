// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"text/tabwriter"
)

// PATCH(novel1219-send-check): the public library entry's hardened send-check ported back over the reprint's simple
// version: smart-list members through GET /contacts?smartListId (the /smart-lists/{id}/contacts route is add/remove
// only), full pagination of lists and repeatable tags with a progress guard and a metadata-total cross-check, DNC
// verdicts from the contact-specific GET /contacts/dnc-status (chunks of 500, truncated/missing answers make the
// result inconclusive) instead of a one-page /dnc list matched on the last 7 digits, phone identity through
// normalizePhoneIdentity, and a synced, fresh local mirror read before --limit. The reprint's gate contract stays:
// a no_go verdict exits 2.
// pp:data-source auto
func newNovelSendCheckCmd(flags *rootFlags) *cobra.Command {
	var list, contact, dbPath string
	var tags []string
	var limit int
	cmd := &cobra.Command{Use: "send-check", Short: "A go/no-go verdict for a send list before any SMS or drip", Long: "Use this command for a go/no-go verdict on exactly one smart list, tag, or contact: per contact, phone present and mappable, Twilio-verified line type, Twilio validity, and the contact's SMS DNC status from GET /contacts/dnc-status. Local mirror filters are applied before --limit; live list reads paginate up to --limit, while repeatable live tags are merged and de-duplicated before --limit. An empty scope is a clean 0-checked answer. Exits 0 on go (or an empty scope), 2 on no_go or a usage error, 3 when --contact does not exist, 4 when DNC access is unauthorized (needs the 'communications' view scope), and 5 when the result is inconclusive (a capped, short or failed audience read, or a DNC answer that is missing, truncated or omits a contact). Do NOT use it to count unverified numbers or their Twilio cost; use 'contacts verify-line-type --estimate' instead.", Example: strings.Trim(`
  conduyt-crm-pp-cli send-check --tag new-leads
  conduyt-crm-pp-cli send-check --contact 9e778307-e575-420b-b02c-dc9cc564febd --json
  conduyt-crm-pp-cli send-check --list 7c1d2e3f-0000-0000-0000-000000000000 --agent --select summary`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2,3,4,5"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "send-check") // PATCH(gocli1219-live-gate-defects): --dry-run --json answers a dry_run object
		}
		set := 0
		if list != "" {
			set++
		}
		if contact != "" {
			set++
		}
		if len(tags) > 0 {
			set++
		}
		if set == 0 && !cmd.Flags().Changed("limit") && !cmd.Flags().Changed("db") {
			return cmd.Help()
		}
		if set != 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("exactly one of --list, --tag, or --contact is required"))
		}
		if limit <= 0 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--limit must be greater than zero"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		rows := make([]sendContact, 0)
		partial := false
		failures := make([]string, 0)
		var total *int
		if dbPath == "" {
			dbPath = defaultDBPath("conduyt-crm-pp-cli")
		}
		// PATCH(novel1219-send-check) r2: the audience source follows --data-source, and a scope the mirror cannot
		// resolve is never an empty audience. Smart-list membership is not in synced contacts, so --list is always
		// live; a tag or contact is read locally only from a synced, fresh mirror that returns rows (tags match the
		// tag objects the contacts route serializes, by name or id). Under --data-source local an unresolvable or
		// unusable mirror is an inconclusive answer (exit 5), never an empty one.
		useLocal := false
		if flags.dataSource != "live" {
			local, e := sendCheckLocalAudience(cmd, flags, dbPath, list, contact, tags, limit)
			if e != nil {
				return e
			}
			if local.usable {
				useLocal = true
				rows, partial, failures, total = local.rows, local.partial, local.failures, local.total
			} else if flags.dataSource == "local" {
				return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("the local mirror cannot resolve this audience (%s); re-run with --data-source live or auto", local.reason))
			}
		}
		if useLocal {
			flags.agentSource = "local"
		} else {
			c, e := flags.newClient()
			if e != nil {
				return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("stale or unavailable contacts mirror requires a live audience fetch: %w", e))
			}
			path, params := "/contacts", map[string]string{}
			if contact != "" {
				path = "/contacts/" + contact
				params = nil
			} else if list != "" {
				// /smart-lists/{id}/contacts is add/remove only (POST/DELETE); members are listed
				// through the contacts route's smartListId selector (dynamic lists apply their
				// filters, static lists resolve by membership).
				params["smartListId"] = list
			}
			if contact != "" {
				data, getErr := c.Get(ctx, path, params)
				if getErr != nil {
					return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("fetching contacts: %w", getErr))
				}
				rows, e = decodeSendContacts(data, true) // GET /contacts/{id} legitimately returns one object.
				if e != nil {
					return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("decoding contacts: %w", e))
				}
			} else if list != "" {
				perPage := min(200, limit)
				progress := newPaginationProgressGuard()
				for page := 1; len(rows) <= limit; page++ {
					params["page"], params["per_page"] = fmt.Sprint(page), fmt.Sprint(perPage)
					data, getErr := c.Get(ctx, path, params)
					if getErr != nil {
						return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("fetching contacts: %w", getErr))
					}
					pageRows, decodeErr := decodeSendContacts(data, false)
					if decodeErr != nil {
						return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("decoding contacts: %w", decodeErr))
					}
					if n, ok := sendResponseTotal(data); ok {
						total = &n
					}
					if guardErr := progress.observe(sendContactIDs(pageRows), "", len(pageRows) == perPage); guardErr != nil {
						partial = true
						failures = append(failures, guardErr.Error())
						break
					}
					rows = append(rows, pageRows...)
					if len(pageRows) < perPage {
						break
					}
				}
				if len(rows) > limit {
					partial = true
					rows = rows[:limit]
				}
				if total != nil && *total > len(rows) {
					partial = true
					failures = append(failures, fmt.Sprintf("contacts ended after %d rows but response metadata reports total %d", len(rows), *total))
				}
			} else {
				const perPage = 200
				seen := make(map[string]struct{})
				for _, tag := range tags {
					progress := newPaginationProgressGuard()
					params := map[string]string{"tag": tag, "per_page": fmt.Sprint(perPage)}
					for page := 1; ; page++ {
						params["page"] = fmt.Sprint(page)
						data, getErr := c.Get(ctx, path, params)
						if getErr != nil {
							return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("fetching contacts for tag %q: %w", tag, getErr))
						}
						pageRows, decodeErr := decodeSendContacts(data, false)
						if decodeErr != nil {
							return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("decoding contacts for tag %q: %w", tag, decodeErr))
						}
						if guardErr := progress.observe(sendContactIDs(pageRows), "", len(pageRows) == perPage); guardErr != nil {
							partial = true
							failures = append(failures, fmt.Sprintf("tag %q: %v", tag, guardErr))
							break
						}
						for _, row := range pageRows {
							if _, exists := seen[row.ID]; exists {
								continue
							}
							seen[row.ID] = struct{}{}
							rows = append(rows, row)
						}
						if len(pageRows) < perPage {
							break
						}
					}
				}
				if len(rows) > limit {
					partial = true
					n := len(rows)
					total = &n
					rows = rows[:limit]
				}
			}
			flags.agentSource = "live"
		}
		out := sendCheckView{Verdict: "go", Partial: partial, Checked: len(rows), Total: total, Verdicts: make([]sendVerdict, 0), Summary: sendSummary{Checked: len(rows)}, Failures: failures}
		if len(rows) == 0 {
			if out.Partial {
				out.Verdict = "inconclusive"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
						return err
					}
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "INCONCLUSIVE: contact audience is incomplete.")
				}
				return apiErr(fmt.Errorf("send-check is incomplete: %s", strings.Join(out.Failures, "; ")))
			}
			out.Verdict = "empty"
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No contacts to check.")
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		dnc := make(map[string]dncStatus, len(rows))
		for start := 0; start < len(rows); start += 500 {
			end := min(start+500, len(rows))
			ids := make([]string, 0, end-start)
			for _, row := range rows[start:end] {
				ids = append(ids, row.ID)
			}
			dncData, getErr := c.Get(ctx, "/contacts/dnc-status", map[string]string{"ids": strings.Join(ids, ",")})
			if getErr != nil {
				return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("fetching DNC status: %w", getErr))
			}
			batch, decodeErr := decodeDNCResponse(dncData)
			if decodeErr != nil {
				return writeSendCheckAudienceFailure(cmd, flags, fmt.Errorf("decoding DNC status: %w", decodeErr))
			}
			for id, status := range batch.Statuses {
				dnc[id] = status
			}
			if batch.Truncated {
				out.Partial = true
				out.Failures = append(out.Failures, fmt.Sprintf("DNC response for contacts %d-%d was truncated", start+1, end))
			}
			if batch.Missing > 0 {
				out.Partial = true
				out.Failures = append(out.Failures, fmt.Sprintf("DNC response omitted %d requested contacts", batch.Missing))
			}
		}
		for _, r := range rows {
			status, verified := dnc[r.ID]
			v := sendVerdict{ContactID: r.ID, Phone: r.Phone, LineType: r.LineType, Verdict: "ok", VoiceBlocked: status.VoiceBlocked}
			_, phoneOK := normalizePhoneIdentity(r.Phone)
			switch {
			case !verified:
				v.Verdict = "unverified"
				out.Partial = true
				out.Failures = append(out.Failures, fmt.Sprintf("contact %s is absent from DNC statuses", r.ID))
			case strings.TrimSpace(r.Phone) == "":
				v.Verdict = "no_phone"
			case !phoneOK:
				v.Verdict = "invalid"
				out.Partial = true
				out.Failures = append(out.Failures, fmt.Sprintf("contact %s has an unmappable phone identity", r.ID))
			case status.SMSBlocked:
				v.Verdict = "dnc"
			case r.Valid != nil && !*r.Valid:
				v.Verdict = "invalid"
			case r.LineType == "" || strings.EqualFold(r.LineType, "unknown"):
				v.Verdict = "unverified"
			}
			out.Summary.add(v.Verdict)
			out.Verdicts = append(out.Verdicts, v)
		}
		if out.Summary.Checked != out.Summary.OK {
			out.Verdict = "no_go"
		}
		if out.Partial {
			out.Verdict = "inconclusive"
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
				return err
			}
			return sendCheckExit(out, limit)
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
		if out.Partial {
			fmt.Fprintln(tw, "WARNING: result is partial; verdict is inconclusive: "+sendCheckPartialReason(out.Failures, limit, out.Summary.Checked))
		}
		fmt.Fprintln(tw, "CONTACT\tPHONE\tLINE TYPE\tVERDICT")
		for _, v := range out.Verdicts {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", v.ContactID, v.Phone, v.LineType, v.Verdict)
		}
		fmt.Fprintf(tw, "TOTAL\t%d ok\t%d blocked\t%d checked\n", out.Summary.OK, out.Summary.Checked-out.Summary.OK, out.Summary.Checked)
		if err := tw.Flush(); err != nil {
			return err
		}
		return sendCheckExit(out, limit)
	}}
	cmd.Flags().StringVar(&list, "list", "", "Smart list ID")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Contact tag (repeatable)")
	cmd.Flags().StringVar(&contact, "contact", "", "Contact ID")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum contacts to check")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path")
	return cmd
}

// sendCheckExit is the gate contract: inconclusive exits 5, no_go exits 2 (the reprint's "safe to gate a pipeline on"
// promise), go exits 0. The verdict JSON is printed before either error.
func sendCheckExit(out sendCheckView, limit int) error {
	if out.Partial {
		return apiErr(fmt.Errorf("send-check is inconclusive: %s", sendCheckPartialReason(out.Failures, limit, out.Summary.Checked)))
	}
	if out.Verdict == "no_go" {
		return &cliError{code: 2, err: fmt.Errorf("send-check verdict no_go: %d of %d contacts are not ok", out.Summary.Checked-out.Summary.OK, out.Summary.Checked)}
	}
	return nil
}

func sendContactIDs(rows []sendContact) []string {
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	return ids
}

func sendCheckPartialReason(failures []string, limit, checked int) string {
	if len(failures) == 0 {
		return fmt.Sprintf("--limit %d capped the contacts checked (checked %d); re-run with a higher --limit to cover the whole audience", limit, checked)
	}
	return strings.Join(failures, "; ")
}

type dncStatus struct {
	DNC          bool `json:"dnc"`
	Litigator    bool `json:"litigator"`
	VoiceBlocked bool `json:"voiceBlocked"`
	SMSBlocked   bool `json:"smsBlocked"`
}

type dncResponse struct {
	Statuses  map[string]dncStatus `json:"statuses"`
	Requested int                  `json:"requested"`
	Checked   int                  `json:"checked"`
	Missing   int                  `json:"missing"`
	Truncated bool                 `json:"truncated"`
}

func decodeDNCResponse(raw json.RawMessage) (dncResponse, error) {
	if err := rejectResponseErrorEnvelope(raw); err != nil {
		return dncResponse{}, err
	}
	var envelope struct {
		Data dncResponse `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return dncResponse{}, err
	}
	if envelope.Data.Statuses == nil {
		return dncResponse{}, fmt.Errorf("DNC response missing data.statuses")
	}
	return envelope.Data, nil
}

func validateContactIdentity(row map[string]any, kind string, index int) error {
	_, phoneOK := normalizePhoneIdentity(strAny(row, "phone", "normalizedPhone"))
	if strings.TrimSpace(strAny(row, "contactId", "contact_id", "id")) != "" || phoneOK {
		return nil
	}
	return fmt.Errorf("%s row %d is missing a stable contact id or normalizable phone", kind, index+1)
}

func sendLocalQuery(list, contact string, tags []string, limit int) (string, []any, bool) {
	where, args, ok := sendLocalFilter(list, contact, tags)
	if !ok {
		return "", nil, false
	}
	args = append(args, limit+1)
	return `SELECT id, data FROM resources WHERE resource_type='contacts' AND ` + where + ` ORDER BY id LIMIT ?`, args, true
}

func sendLocalCountQuery(list, contact string, tags []string) (string, []any, bool) {
	where, args, ok := sendLocalFilter(list, contact, tags)
	if !ok {
		return "", nil, false
	}
	return `SELECT COUNT(*) FROM resources WHERE resource_type='contacts' AND ` + where, args, true
}

func sendLocalFilter(list, contact string, tags []string) (string, []any, bool) {
	switch {
	case contact != "":
		return `id = ?`, []any{contact}, true
	case list != "":
		// Smart-list membership (static or dynamic) is resolved by the API only.
		return "", nil, false
	case len(tags) > 0:
		// The contacts route serializes tags as objects ({id, name, ...}); a plain string tag is accepted too.
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		args := make([]any, 0, 3*len(tags))
		for range 3 {
			for _, tag := range tags {
				args = append(args, tag)
			}
		}
		return `EXISTS (SELECT 1 FROM json_each(data, '$.tags') WHERE (json_each.type = 'text' AND json_each.value IN (` + placeholders + `)) OR (json_each.type = 'object' AND (json_extract(json_each.value, '$.name') IN (` + placeholders + `) OR json_extract(json_each.value, '$.id') IN (` + placeholders + `))))`, args, true
	default:
		return "", nil, false
	}
}

func writeSendCheckAudienceFailure(cmd *cobra.Command, flags *rootFlags, cause error) error {
	out := sendCheckView{Verdict: "inconclusive", Error: cause.Error(), Partial: true, Verdicts: []sendVerdict{}, Failures: []string{cause.Error()}}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: send-check is inconclusive: %v\n", cause)
	}
	return classifyAPIErrorOnly(cause)
}

type sendContact struct {
	ID, Phone, LineType string
	Valid               *bool
}
type sendVerdict struct {
	ContactID    string `json:"contact_id"`
	Phone        string `json:"phone,omitempty"`
	LineType     string `json:"line_type,omitempty"`
	Verdict      string `json:"verdict"`
	VoiceBlocked bool   `json:"voice_blocked"`
}
type sendSummary struct {
	Checked    int `json:"checked"`
	OK         int `json:"ok"`
	NoPhone    int `json:"no_phone"`
	Unverified int `json:"unverified"`
	Invalid    int `json:"invalid"`
	DNC        int `json:"dnc"`
}
type sendCheckView struct {
	Verdict  string        `json:"verdict"`
	Error    string        `json:"error,omitempty"`
	Partial  bool          `json:"partial"`
	Checked  int           `json:"checked"`
	Total    *int          `json:"total,omitempty"`
	Verdicts []sendVerdict `json:"verdicts"`
	Summary  sendSummary   `json:"summary"`
	Failures []string      `json:"failures,omitempty"`
}

func validJSON(raw json.RawMessage) bool { var v any; return json.Unmarshal(raw, &v) == nil }

func (s *sendSummary) add(v string) {
	switch v {
	case "ok":
		s.OK++
	case "no_phone":
		s.NoPhone++
	case "unverified":
		s.Unverified++
	case "invalid":
		s.Invalid++
	case "dnc":
		s.DNC++
	}
}
func decodeSendContact(m map[string]any, fallback string) sendContact {
	id := strAny(m, "id")
	if id == "" {
		id = fallback
	}
	lt := contactLineType(m)
	var valid *bool
	for _, k := range []string{"phoneValid", "valid"} {
		if b, ok := m[k].(bool); ok {
			x := b
			valid = &x
		}
	}
	if cf, ok := m["customFields"].(map[string]any); ok {
		if valid == nil {
			if b, ok := cf["sms_phone_valid"].(bool); ok {
				x := b
				valid = &x
			}
		}
	}
	return sendContact{id, strAny(m, "phone"), lt, valid}
}
func decodeSendContacts(raw json.RawMessage, allowSingleObject bool) ([]sendContact, error) {
	items, err := objectItems(raw, allowSingleObject)
	if err != nil {
		return nil, err
	}
	out := make([]sendContact, 0, len(items))
	for i, m := range items {
		contact := decodeSendContact(m, "")
		if strings.TrimSpace(contact.ID) == "" {
			return nil, fmt.Errorf("contact row %d is missing a stable id", i+1)
		}
		out = append(out, contact)
	}
	return out, nil
}

func sendResponseTotal(raw json.RawMessage) (int, bool) {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil {
		return 0, false
	}
	for _, container := range []map[string]any{envelope, mapAny(envelope["meta"]), mapAny(envelope["pagination"])} {
		if container == nil {
			continue
		}
		if value, ok := container["total"]; ok {
			switch n := value.(type) {
			case float64:
				return int(n), true
			case json.Number:
				value, err := n.Int64()
				return int(value), err == nil
			}
		}
	}
	return 0, false
}

func mapAny(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

type sendLocalResult struct {
	usable   bool
	reason   string
	rows     []sendContact
	partial  bool
	failures []string
	total    *int
}

// sendCheckLocalAudience reads the audience from the local mirror when it can answer it: a synced, fresh contacts
// mirror, a scope it can express (never a smart list), and at least one matching row. Anything else is not usable,
// with the reason, so auto falls back live and local reports it as inconclusive.
func sendCheckLocalAudience(cmd *cobra.Command, flags *rootFlags, dbPath, list, contact string, tags []string, limit int) (sendLocalResult, error) {
	ctx := cmd.Context()
	out := sendLocalResult{failures: make([]string, 0)}
	query, queryArgs, expressible := sendLocalQuery(list, contact, tags, limit)
	if !expressible {
		out.reason = "smart-list membership is not stored in synced contacts"
		return out, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			out.reason = "no local mirror at " + dbPath
		} else {
			out.reason = "the local mirror cannot be read: " + err.Error()
		}
		return out, nil
	}
	// PATCH(novel1219-send-check) r4: every local read stage (open, sync state, count, query, scan, iteration,
	// close, the capped count) funnels into one "mirror unusable" result: auto then reads live, local exits 5.
	unusable := func(stage string, err error) (sendLocalResult, error) {
		return sendLocalResult{failures: make([]string, 0), reason: "the local mirror is unusable (" + stage + ": " + err.Error() + ")"}, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return unusable("opening", err)
	}
	defer db.Close()
	reader := newMirrorReader(db.DB())
	// PATCH(novel1219-send-check) r5: the sync state (timestamp, total, attempt completeness) is read once through
	// the mirror reader, and freshness is decided from it; no check re-reads sync_state outside the unusable path.
	state, err := readContactsSyncState(ctx, reader)
	if err != nil {
		return unusable("reading sync state", err)
	}
	syncedAt, syncTotal, attemptComplete := state.SyncedAt, state.Total, state.AttemptComplete
	count, err := reader.Count(ctx, `SELECT COUNT(*) FROM resources WHERE resource_type = 'contacts'`)
	if err != nil {
		return unusable("counting contacts", err)
	}
	if syncedAt.IsZero() || count == 0 {
		out.reason = "contacts were never synced"
		return out, nil
	}
	// PATCH(novel1219-send-check) r3: a mirror counts only when its last sync attempt finished and its row count
	// reconciles with the synced total; an interrupted, aborted or in-progress attempt is not a mirror.
	if !attemptComplete {
		out.reason = "the last contacts sync attempt did not finish"
		return out, nil
	}
	if count != syncTotal {
		out.reason = fmt.Sprintf("the contacts mirror holds %d rows but its last sync reported %d", count, syncTotal)
		return out, nil
	}
	if mirrorOlderThan(cmd, syncedAt, flags.maxAge) {
		out.reason = "the contacts mirror is older than --max-age"
		return out, nil
	}
	rows, err := readMirrorRows(ctx, reader, query, queryArgs...)
	if err != nil {
		return unusable("reading the audience", err)
	}
	for _, row := range rows {
		var m map[string]any
		if decodeErr := json.Unmarshal(row.Data, &m); decodeErr != nil {
			return unusable("decoding mirrored contact "+row.ID, decodeErr)
		}
		out.rows = append(out.rows, decodeSendContact(m, row.ID))
	}
	if len(out.rows) == 0 {
		// Zero local matches cannot prove an empty audience (a tag added after the sync, an unsynced contact).
		out.reason = "the mirror holds no contact in this scope"
		return out, nil
	}
	if len(out.rows) > limit {
		// A deliberate --limit cap: the local answer stands, partial, with the full audience size.
		countQuery, countArgs, _ := sendLocalCountQuery(list, contact, tags)
		n, err := reader.Count(ctx, countQuery, countArgs...)
		if err != nil {
			return unusable("counting the capped audience", err)
		}
		out.partial = true
		out.total = &n
		out.rows = out.rows[:limit]
	}
	out.usable = true
	return out, nil
}
