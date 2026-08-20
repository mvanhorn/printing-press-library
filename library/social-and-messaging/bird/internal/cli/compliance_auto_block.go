// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/internal/store"
	"github.com/spf13/cobra"
)

type optOutCandidate struct {
	IdentifierKey   string `json:"identifierKey"`
	IdentifierValue string `json:"identifierValue"`
	Keyword         string `json:"keyword"`
	MessageID       string `json:"messageId,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
}

type autoBlockReport struct {
	Since      string            `json:"since"`
	Found      int               `json:"found"`
	Applied    int               `json:"applied"`
	DryRun     bool              `json:"dryRun"`
	Candidates []optOutCandidate `json:"candidates"`
}

var stopKeywords = []string{"stop", "stopall", "unsubscribe", "cancel", "end", "quit", "optout", "opt-out"}

// PATCH: word-boundary-aware keyword matcher. The previous implementation used
// a mix of strings.EqualFold / Contains(" KW ") / HasPrefix(KW), where the
// HasPrefix arm matched any body that *starts* with a keyword regardless of
// the next character. A body like "STOPPER SERVICE" therefore matched the
// "stop" keyword and, under --apply, would submit that recipient to the bulk-
// block endpoint. The pre-compiled regex below anchors each keyword between
// regex word boundaries (`\b`), so only standalone occurrences trigger an
// opt-out candidate. Surfaced by Greptile P1 in PR #417 review.
var stopKeywordRE = func() *regexp.Regexp {
	parts := make([]string, len(stopKeywords))
	for i, kw := range stopKeywords {
		parts[i] = regexp.QuoteMeta(kw)
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(parts, "|") + `)\b`)
}()

func newComplianceAutoBlockCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		since  string
		apply  bool
	)
	cmd := &cobra.Command{
		Use:   "auto-block",
		Short: "Scan local inbound messages for STOP-keyword fires; emit a CSV ready for bulk-add (or apply directly).",
		Long: `Reads the local conversations_messages table for inbound messages whose
body matches a STOP-style opt-out keyword (STOP, UNSUBSCRIBE, CANCEL, END,
QUIT, OPTOUT, OPT-OUT) within the --since window. Resolves each match to a
participant identifier and emits a list of (identifierKey, identifierValue)
pairs ready to add as block rules.

Print-by-default; pass --apply to call the workspace allow/block bulk-add
endpoint. Verify mode (PRINTING_PRESS_VERIFY=1) always short-circuits to
dry behavior.`,
		Example: `  bird-pp-cli compliance auto-block --since 7d --json
  bird-pp-cli compliance auto-block --since 30d --apply --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDayDuration(since)
			if err != nil {
				return fmt.Errorf("parsing --since %q: %w", since, err)
			}
			cutoff := time.Now().Add(-d)
			if dbPath == "" {
				dbPath = defaultDBPath("bird-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			candidates, err := scanOptOuts(db, cutoff)
			if err != nil {
				return err
			}
			report := autoBlockReport{
				Since:      since,
				Found:      len(candidates),
				DryRun:     !apply || cliutil.IsVerifyEnv(),
				Candidates: candidates,
			}
			if !report.DryRun && len(candidates) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				rules := make([]map[string]string, 0, len(candidates))
				for _, ck := range candidates {
					rules = append(rules, map[string]string{
						"kind":            "block",
						"identifierKey":   ck.IdentifierKey,
						"identifierValue": ck.IdentifierValue,
					})
				}
				body := map[string]any{"rules": rules}
				_, _, err = c.Post("/conversation-settings/allow-block-rules/bulk", body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				report.Applied = len(rules)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "7d", "Look back this far for opt-out messages (e.g. 7d, 30d)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually call the bulk-add endpoint (default is dry-run)")
	return cmd
}

func scanOptOuts(db *store.Store, cutoff time.Time) ([]optOutCandidate, error) {
	rows, err := db.DB().Query(`SELECT id, conversations_id, data FROM conversations_messages`)
	if err != nil {
		return nil, fmt.Errorf("scan conversations_messages: %w", err)
	}
	defer rows.Close()
	out := make([]optOutCandidate, 0, 16)
	for rows.Next() {
		var id, convID string
		var raw []byte
		if err := rows.Scan(&id, &convID, &raw); err != nil {
			return nil, err
		}
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		if dir, _ := m["direction"].(string); dir != "incoming" {
			continue
		}
		body := strings.TrimSpace(extractBodyText(m))
		// PATCH: keyword match must respect word boundaries (see stopKeywordRE
		// definition above). The previous strings.HasPrefix arm matched any
		// message starting with a keyword character sequence, regardless of
		// whether the keyword was a full word -- so "STOPPER SERVICE" matched
		// "stop" and, with --apply, would block the sender. The regex
		// canonicalises the keyword to upper case before assignment so the
		// downstream consumer continues to see "STOP" / "UNSUBSCRIBE" etc.
		hit := stopKeywordRE.FindString(body)
		if hit == "" {
			continue
		}
		matched := strings.ToUpper(hit)
		// PATCH: messages with no createdAt or an unparseable timestamp must be
		// excluded by the --since window, not silently included. The previous
		// guard only skipped when createdAt parsed AND was older than cutoff,
		// so messages without createdAt (or with a malformed value) bypassed
		// the window and were always considered opt-out candidates. With
		// --apply, that could bulk-block phone numbers from arbitrarily old
		// STOP messages. Surfaced by Greptile P1 in PR #417 review.
		ts, _ := m["createdAt"].(string)
		if ts == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, ts); err != nil || t.Before(cutoff) {
			continue
		}
		key, value := lookupParticipantIdentifier(db, convID)
		if value == "" {
			continue
		}
		out = append(out, optOutCandidate{
			IdentifierKey:   key,
			IdentifierValue: value,
			Keyword:         matched,
			MessageID:       id,
			Timestamp:       ts,
		})
	}
	// PATCH: surface mid-iteration scan errors. rows.Next() returns false
	// both on exhaustion and on internal error; without this check
	// scanOptOuts would silently return a partial candidate list and
	// --apply would bulk-block a truncated set of phone numbers without
	// any indication that the scan was incomplete. Surfaced by Greptile
	// P1 in PR #417 ninth review pass.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan conversations_messages: %w", err)
	}
	return out, nil
}

func lookupParticipantIdentifier(db *store.Store, convID string) (string, string) {
	rows, err := db.DB().Query(`SELECT data FROM conversations_participants WHERE conversations_id = ?`, convID)
	if err != nil {
		return "", ""
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var p map[string]any
		if json.Unmarshal(raw, &p) != nil {
			continue
		}
		if t, _ := p["type"].(string); t == "contact" {
			k, _ := p["identifierKey"].(string)
			v, _ := p["identifierValue"].(string)
			if v != "" {
				return k, v
			}
		}
	}
	// PATCH: surface mid-iteration scan errors here too. The helper has no
	// error return, so the best we can do is make the no-result path
	// explicit and document that a DB failure mid-scan is indistinguishable
	// from "no contact found" -- which is the safer of the two failure
	// modes under --apply (skip rather than mis-block). Closing this
	// missed-by-the-sweep gap surfaced by Greptile P1 in PR #417 tenth
	// review pass.
	if rows.Err() != nil {
		return "", ""
	}
	return "", ""
}
