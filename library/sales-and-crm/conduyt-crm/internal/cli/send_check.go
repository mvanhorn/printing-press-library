// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"text/tabwriter"
)

// pp:data-source auto
func newNovelSendCheckCmd(flags *rootFlags) *cobra.Command {
	var list, contact, dbPath string
	var tags []string
	var limit int
	cmd := &cobra.Command{Use: "send-check", Short: "A go/no-go verdict for a send list before any SMS or drip", Long: "Use this command for a go/no-go verdict on exactly one smart list, tag, or contact. Local mirror filters are applied before --limit; live list reads paginate up to --limit, while repeatable live tags are merged and de-duplicated before --limit. Do NOT use it to count unverified numbers or their Twilio cost; use 'contacts verify-line-type --estimate' instead. Exits 4 when DNC access is unauthorized and 5 when the DNC API cannot answer.", Example: "  conduyt-crm-pp-cli send-check --list 8f2c1a7e --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "send-check")
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
		useLocal := false
		if _, err := os.Stat(dbPath); err == nil {
			db, e := store.OpenWithContext(ctx, dbPath)
			if e != nil {
				return fmt.Errorf("opening local mirror: %w", e)
			}
			defer db.Close()
			_, syncedAt, _, stateErr := db.GetSyncState("contacts")
			count, countErr := db.Count("contacts")
			if stateErr != nil || countErr != nil {
				return fmt.Errorf("checking contacts mirror state: %w", errors.Join(stateErr, countErr))
			}
			useLocal = !syncedAt.IsZero() && count > 0
			if useLocal && hintIfStale(cmd, db, "contacts", flags.maxAge) {
				useLocal = false
			}
			query, queryArgs, expressible := sendLocalQuery(list, contact, tags, limit)
			if !expressible {
				useLocal = false
			}
			var qr interface {
				Next() bool
				Scan(...any) error
				Err() error
				Close() error
			}
			if useLocal {
				qr, e = db.DB().QueryContext(ctx, query, queryArgs...)
			}
			if e != nil {
				return fmt.Errorf("reading contacts mirror: %w", e)
			}
			for useLocal && qr.Next() {
				var id string
				var raw []byte
				if e = qr.Scan(&id, &raw); e != nil {
					_ = qr.Close()
					return e
				}
				var m map[string]any
				if decodeErr := json.Unmarshal(raw, &m); decodeErr != nil {
					partial = true
					failures = append(failures, fmt.Sprintf("decoding mirrored contact %s: %v", id, decodeErr))
					continue
				}
				rows = append(rows, decodeSendContact(m, id))
			}
			if useLocal {
				if rowsErr := qr.Err(); rowsErr != nil {
					partial = true
					failures = append(failures, fmt.Sprintf("reading contacts mirror: %v", rowsErr))
				}
				e = qr.Close()
			}
			if e != nil {
				return e
			}
			if useLocal && len(rows) > limit {
				partial = true
				countQuery, countArgs, _ := sendLocalCountQuery(list, contact, tags)
				var n int
				if e = db.DB().QueryRowContext(ctx, countQuery, countArgs...).Scan(&n); e != nil {
					return fmt.Errorf("counting contacts mirror audience: %w", e)
				}
				total = &n
				rows = rows[:limit]
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking local mirror: %w", err)
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
			if out.Partial {
				return apiErr(fmt.Errorf("send-check is inconclusive: %s", sendCheckPartialReason(out.Failures, limit, out.Summary.Checked)))
			}
			return nil
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
		if out.Partial {
			return apiErr(fmt.Errorf("send-check is inconclusive: %s", sendCheckPartialReason(out.Failures, limit, out.Summary.Checked)))
		}
		return nil
	}}
	cmd.Flags().StringVar(&list, "list", "", "Smart list ID")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Contact tag (repeatable)")
	cmd.Flags().StringVar(&contact, "contact", "", "Contact ID")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum contacts to check")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path")
	return cmd
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
		return `(json_extract(data, '$.smartListId') = ? OR json_extract(data, '$.smart_list_id') = ? OR EXISTS (SELECT 1 FROM json_each(data, '$.smartListIds') WHERE value = ?))`, []any{list, list, list}, true
	case len(tags) > 0:
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(tags)), ",")
		args := make([]any, 0, len(tags))
		for _, tag := range tags {
			args = append(args, tag)
		}
		return `EXISTS (SELECT 1 FROM json_each(data, '$.tags') WHERE value IN (` + placeholders + `))`, args, true
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
