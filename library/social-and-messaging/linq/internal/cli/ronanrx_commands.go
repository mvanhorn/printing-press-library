package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/store"
	"github.com/spf13/cobra"
)

func newInviteLinkCmd(flags *rootFlags) *cobra.Command {
	var base, from, routing, token string
	cmd := &cobra.Command{
		Use:         "invite-link --base-url URL --from NUMBER --routing TEXT --token TOKEN",
		Short:       "Build a click-to-text inbound front door link without PHI payloads",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" && from == "" && routing == "" && token == "" {
				return cmd.Help()
			}
			if base == "" || from == "" || routing == "" || token == "" {
				return fmt.Errorf("--base-url, --from, --routing, and --token are required")
			}
			if containsPHI(routing) || containsPHI(token) {
				return fmt.Errorf("invite-link refuses PHI-shaped routing text or token")
			}
			u, err := buildInviteURL(base, from, routing, token)
			if err != nil {
				return err
			}
			return printJSONMap(cmd, map[string]any{"invite_link": u.String(), "pointer_not_payload": true, "phi_in_body": false})
		},
	}
	cmd.Flags().StringVar(&base, "base-url", "", "HTTPS click-to-text/deep-link base URL")
	cmd.Flags().StringVar(&from, "from", "", "Linq sender phone number")
	cmd.Flags().StringVar(&routing, "routing", "", "Non-PHI routing text")
	cmd.Flags().StringVar(&token, "token", "", "Opaque authenticated token or link slug")
	return cmd
}

func newWelcomeFlowCmd(flags *rootFlags) *cobra.Command {
	var base, from, routing, token, secureLink, chatID string
	var allowedHosts []string
	cmd := &cobra.Command{
		Use:         "welcome-flow --base-url URL --from NUMBER --token TOKEN --secure-link URL",
		Short:       "Plan the RonanRx inbound-first welcome flow without sending",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" && from == "" && token == "" && secureLink == "" && chatID == "" {
				return cmd.Help()
			}
			if base == "" || from == "" || routing == "" || token == "" || secureLink == "" {
				return fmt.Errorf("--base-url, --from, --routing, --token, and --secure-link are required")
			}
			if containsPHI(routing) || containsPHI(token) || containsPHI(secureLink) {
				return fmt.Errorf("welcome-flow refuses PHI-shaped routing text, token, or link")
			}
			invite, err := buildInviteURL(base, from, routing, token)
			if err != nil {
				return err
			}
			linkAudit := auditLink(secureLink, allowedHosts)
			preflight := evaluateSendGuard(chatID, "INBOUND_OK "+routing, secureLink)
			if chatID == "" {
				preflight = sendGuardResult{Allowed: false, Reasons: []string{"chat ID is not known until the patient enters through the invite link"}, Checks: map[string]string{"chat_id": "pending: wait for inbound chat", "inbound_first": "pending: wait for inbound patient message", "pointer_not_payload": "pass"}}
			}
			steps := []map[string]any{
				{"name": "generate_invite_link", "status": "ready", "owner": "operator", "artifact": invite.String()},
				{"name": "patient_inbound_opt_in", "status": "waiting", "owner": "patient", "rule": "patient must enter through the invite link before any outbound send"},
				{"name": "sync_and_consent_audit", "status": "next", "owner": "agent", "command": "linq-pp-cli sync && linq-pp-cli consent-audit --chat-id <chat>"},
				{"name": "draft_secure_welcome_reply", "status": "ready_after_inbound", "owner": "agent", "command": "linq-pp-cli safe-reply-draft --chat-id <chat> --intent 'welcome intake next step' --link <secure-link>"},
				{"name": "guarded_send", "status": "blocked_until_human_and_inbound", "owner": "human", "command": "linq-pp-cli send --chat-id <chat> --routing 'INBOUND_OK WELCOME_FLOW' --link <secure-link>"},
				{"name": "monitor", "status": "after_send", "owner": "agent", "command": "linq-pp-cli delivery-health && linq-pp-cli needs-human"},
			}
			return printJSONMap(cmd, map[string]any{
				"flow":                "ronanrx_welcome",
				"dry_run":             true,
				"would_send":          false,
				"pointer_not_payload": true,
				"invite_link":         invite.String(),
				"secure_link_audit":   linkAudit,
				"send_preflight":      preflight,
				"steps":               steps,
			})
		},
	}
	cmd.Flags().StringVar(&base, "base-url", "", "HTTPS click-to-text/deep-link base URL")
	cmd.Flags().StringVar(&from, "from", "", "Linq sender phone number")
	cmd.Flags().StringVar(&routing, "routing", "WELCOME_FLOW", "Non-PHI routing text")
	cmd.Flags().StringVar(&token, "token", "", "Opaque authenticated token or link slug")
	cmd.Flags().StringVar(&secureLink, "secure-link", "", "Opaque secure welcome/intake link")
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Optional known inbound chat ID for preflight")
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed secure-link host suffix; repeatable")
	return cmd
}

func newGuardedSendCmd(flags *rootFlags) *cobra.Command {
	var chatID, link, routing string
	cmd := &cobra.Command{
		Use:   "send --chat-id CHAT --link URL --routing TEXT",
		Short: "Inbound-first pointer-not-payload send path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), `{"verify_noop":true,"success":false,"reason":"PRINTING_PRESS_VERIFY"}`)
				return nil
			}
			if chatID == "" || link == "" || routing == "" {
				return fmt.Errorf("--chat-id, --link, and --routing are required")
			}
			checks := evaluateSendGuard(chatID, routing, link)
			if !checks.Allowed {
				return fmt.Errorf("send refused: %s", strings.Join(checks.Reasons, "; "))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"parts": []map[string]any{{"type": "text", "value": routing + " " + link}}}
			data, status, err := c.PostWithParams(cmd.Context(), replacePathParam("/v3/chats/{chatId}/messages", "chatId", chatID), map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			env := map[string]any{"action": "guarded_send", "status": status, "success": status >= 200 && status < 300, "pointer_not_payload": true, "checks": checks.Checks}
			if len(data) > 0 {
				var parsed any
				if json.Unmarshal(data, &parsed) == nil {
					env["data"] = parsed
				}
			}
			return printJSONMap(cmd, env)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing inbound chat ID")
	cmd.Flags().StringVar(&link, "link", "", "Opaque authenticated link")
	cmd.Flags().StringVar(&routing, "routing", "", "Non-PHI routing text; must include INBOUND_OK marker")
	return cmd
}

func newSendPreflightCmd(flags *rootFlags) *cobra.Command {
	var chatID, link, routing string
	cmd := &cobra.Command{
		Use:         "send-preflight --chat-id CHAT --link URL --routing TEXT",
		Short:       "Explain whether guarded send would be allowed without sending",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := evaluateSendGuard(chatID, routing, link)
			return printJSONMap(cmd, map[string]any{"allowed": checks.Allowed, "reasons": checks.Reasons, "checks": checks.Checks, "pointer_not_payload": true, "would_send": false})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing inbound chat ID")
	cmd.Flags().StringVar(&link, "link", "", "Opaque authenticated link")
	cmd.Flags().StringVar(&routing, "routing", "", "Non-PHI routing text; must include INBOUND_OK marker")
	return cmd
}

func newSafeReplyDraftCmd(flags *rootFlags) *cobra.Command {
	var chatID, intent, link string
	cmd := &cobra.Command{
		Use:         "safe-reply-draft --chat-id CHAT --intent TEXT [--link URL]",
		Short:       "Create a redacted human-review reply draft without sending",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" && intent == "" && link == "" {
				return cmd.Help()
			}
			if strings.TrimSpace(chatID) == "" || strings.TrimSpace(intent) == "" {
				return fmt.Errorf("--chat-id and --intent are required")
			}
			if containsPHI(intent) || containsPHI(link) {
				return fmt.Errorf("safe-reply-draft refuses PHI-shaped intent or link")
			}
			parts := []string{"Thanks — here is the next step for your RonanRx request."}
			if intent != "" {
				parts = append(parts, "Context: "+store.RedactPHI(intent)+".")
			}
			if link != "" {
				parts = append(parts, "Please use this secure link: "+link)
			}
			draft := strings.Join(parts, " ")
			return printJSONMap(cmd, map[string]any{"chat_id": chatID, "draft": draft, "send_ready": false, "requires_human_review": true, "pointer_not_payload": true})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Chat ID for the draft")
	cmd.Flags().StringVar(&intent, "intent", "", "Non-PHI reply intent")
	cmd.Flags().StringVar(&link, "link", "", "Optional opaque secure link")
	return cmd
}

func newConsentAuditCmd(flags *rootFlags) *cobra.Command {
	var chatID, routing, link string
	var limit int
	cmd := &cobra.Command{
		Use:         "consent-audit --chat-id CHAT",
		Short:       "Explain inbound-first/opt-out evidence for a chat",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" && routing == "INBOUND_OK" && link == "https://example.invalid/opaque-token" {
				return cmd.Help()
			}
			if chatID == "" {
				return fmt.Errorf("--chat-id is required")
			}
			messages, err := loadLocalMessages(limit)
			if err != nil {
				return err
			}
			evidence := consentEvidence(chatID, messages)
			checks := evaluateSendGuard(chatID, routing, link)
			return printJSONMap(cmd, map[string]any{"chat_id": chatID, "local_messages_checked": len(messages), "evidence": evidence, "preflight": checks, "note": "Local evidence only; run sync before relying on this audit."})
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Chat ID to audit")
	cmd.Flags().StringVar(&routing, "routing", "INBOUND_OK", "Optional routing text to preflight")
	cmd.Flags().StringVar(&link, "link", "https://example.invalid/opaque-token", "Optional opaque link to preflight")
	cmd.Flags().IntVar(&limit, "limit", 250, "Local messages to inspect")
	return cmd
}

func newNeedsHumanCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "needs-human",
		Short:       "Find local conversations that should be reviewed by a human",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			messages, err := loadLocalMessages(limit)
			if err != nil {
				return err
			}
			items := humanReviewCandidates(messages)
			return printJSONMap(cmd, map[string]any{"candidates": items, "count": len(items), "local_messages_checked": len(messages), "source": "local encrypted mirror"})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 250, "Local messages to inspect")
	return cmd
}

func newLinkAuditCmd(flags *rootFlags) *cobra.Command {
	var links, allowedHosts []string
	var limit int
	cmd := &cobra.Command{
		Use:         "link-audit [--link URL] [--allow-host HOST]",
		Short:       "Check links for pointer-not-payload safety",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(links) == 0 {
				messages, err := loadLocalMessages(limit)
				if err != nil {
					return err
				}
				links = extractLinks(messages)
			}
			results := make([]map[string]any, 0, len(links))
			for _, raw := range links {
				results = append(results, auditLink(raw, allowedHosts))
			}
			return printJSONMap(cmd, map[string]any{"links_checked": len(results), "results": results, "source": "explicit --link values or local encrypted mirror"})
		},
	}
	cmd.Flags().StringArrayVar(&links, "link", nil, "Link to audit; repeatable")
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed host suffix; repeatable")
	cmd.Flags().IntVar(&limit, "limit", 250, "Local messages to inspect when --link is omitted")
	return cmd
}

func newPurgeCmd(flags *rootFlags) *cobra.Command {
	var olderThan time.Duration
	cmd := &cobra.Command{Use: "purge", Short: "Purge encrypted local PHI mirror rows older than the retention window", RunE: func(cmd *cobra.Command, args []string) error {
		db, err := store.Open(defaultDBPath("linq-pp-cli"))
		if err != nil {
			return err
		}
		defer db.Close()
		n, err := db.PurgeOlderThan(time.Now().Add(-olderThan))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "purged %d rows\n", n)
		return err
	}}
	cmd.Flags().DurationVar(&olderThan, "older-than", 30*24*time.Hour, "retention window")
	return cmd
}

func newInsightCmd(name string) *cobra.Command {
	return &cobra.Command{Use: name, Short: "Compute " + name + " from the local encrypted mirror", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		out := map[string]any{"insight": name, "source": "local encrypted store", "status": "requires synced Linq chats/messages mirror"}
		return printJSONMap(cmd, out)
	}}
}

type sendGuardResult struct {
	Allowed bool              `json:"allowed"`
	Reasons []string          `json:"reasons"`
	Checks  map[string]string `json:"checks"`
}

func buildInviteURL(base, from, routing, token string) (*url.URL, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("--base-url must be an https URL")
	}
	q := u.Query()
	q.Set("from", from)
	q.Set("text", routing+" "+token)
	u.RawQuery = q.Encode()
	return u, nil
}

func evaluateSendGuard(chatID, routing, link string) sendGuardResult {
	checks := map[string]string{}
	var reasons []string
	add := func(name string, ok bool, reason string) {
		if ok {
			checks[name] = "pass"
			return
		}
		checks[name] = "fail: " + reason
		reasons = append(reasons, reason)
	}
	add("chat_id", strings.TrimSpace(chatID) != "", "chat ID is required")
	add("routing", strings.TrimSpace(routing) != "", "routing text is required")
	add("link", strings.TrimSpace(link) != "", "opaque link is required")
	add("inbound_first", strings.Contains(strings.ToUpper(routing), "INBOUND_OK"), "inbound-first guard requires prior inbound marker INBOUND_OK; no cold-send override exists")
	add("opt_out", !strings.Contains(strings.ToUpper(routing), "OPTED_OUT"), "recipient appears opted out")
	add("phi_payload", !containsPHI(routing) && !containsPHI(link), "message body or link contains PHI-shaped content")
	if link != "" {
		u, err := url.Parse(link)
		add("https_link", err == nil && u.Scheme == "https" && u.Host != "", "link must be an https URL")
	}
	return sendGuardResult{Allowed: len(reasons) == 0, Reasons: reasons, Checks: checks}
}

func loadLocalMessages(limit int) ([]map[string]any, error) {
	db, err := store.Open(defaultDBPath("linq-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	raw, err := db.List("messages", limit)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(raw))
	for _, msg := range raw {
		var obj map[string]any
		if json.Unmarshal(msg, &obj) == nil {
			items = append(items, obj)
		}
	}
	return items, nil
}

func consentEvidence(chatID string, messages []map[string]any) map[string]any {
	var inbound, outbound, optedOut int
	var lastInbound, lastOutbound string
	for _, msg := range messages {
		if !messageMatchesChat(msg, chatID) {
			continue
		}
		text := strings.ToLower(messageText(msg))
		direction := strings.ToLower(firstString(msg, "direction", "type", "kind"))
		if strings.Contains(direction, "inbound") || strings.Contains(direction, "incoming") {
			inbound++
			lastInbound = firstString(msg, "created_at", "createdAt", "timestamp", "updated_at")
		}
		if strings.Contains(direction, "outbound") || strings.Contains(direction, "sent") {
			outbound++
			lastOutbound = firstString(msg, "created_at", "createdAt", "timestamp", "updated_at")
		}
		if strings.Contains(text, "stop") || strings.Contains(text, "unsubscribe") || strings.Contains(text, "opt out") || strings.Contains(text, "opt-out") {
			optedOut++
		}
	}
	return map[string]any{"prior_inbound_messages": inbound, "prior_outbound_messages": outbound, "opt_out_signals": optedOut, "last_inbound_at": lastInbound, "last_outbound_at": lastOutbound, "inbound_first_satisfied_by_local_evidence": inbound > 0, "safe_to_send_from_local_evidence": inbound > 0 && optedOut == 0}
}

func humanReviewCandidates(messages []map[string]any) []map[string]any {
	riskTerms := regexp.MustCompile(`(?i)\b(angry|upset|lawsuit|urgent|emergency|pain|allergic|allergy|side effect|doctor|hospital|stop|unsubscribe|opt[- ]?out)\b`)
	byChat := map[string]map[string]any{}
	for _, msg := range messages {
		chatID := firstString(msg, "chat_id", "chatId", "chat", "conversation_id")
		if chatID == "" {
			chatID = "unknown"
		}
		text := store.RedactPHI(messageText(msg))
		if !riskTerms.MatchString(text) {
			continue
		}
		sev := "medium"
		if regexp.MustCompile(`(?i)\b(emergency|hospital|lawsuit|allergic|allergy|side effect|stop|unsubscribe|opt[- ]?out)\b`).MatchString(text) {
			sev = "high"
		}
		byChat[chatID] = map[string]any{"chat_id": chatID, "severity": sev, "reason": "message matched human-review risk language", "redacted_snippet": truncateRonanRx(text, 180)}
	}
	keys := make([]string, 0, len(byChat))
	for k := range byChat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, byChat[k])
	}
	return out
}

var linkRE = regexp.MustCompile(`https?://[^\s"'<>]+`)

func extractLinks(messages []map[string]any) []string {
	seen := map[string]bool{}
	var links []string
	for _, msg := range messages {
		for _, link := range linkRE.FindAllString(messageText(msg), -1) {
			link = strings.TrimRight(link, ").,;]")
			if !seen[link] {
				seen[link] = true
				links = append(links, link)
			}
		}
	}
	sort.Strings(links)
	return links
}

func auditLink(raw string, allowedHosts []string) map[string]any {
	checks := map[string]string{}
	var reasons []string
	add := func(name string, ok bool, reason string) {
		if ok {
			checks[name] = "pass"
		} else {
			checks[name] = "fail: " + reason
			reasons = append(reasons, reason)
		}
	}
	u, err := url.Parse(raw)
	add("parse", err == nil && u.Host != "", "not a valid absolute URL")
	add("https", err == nil && u.Scheme == "https", "URL must use https")
	add("no_phi", !containsPHI(raw), "URL contains PHI-shaped data")
	if len(allowedHosts) > 0 && err == nil {
		ok := false
		for _, h := range allowedHosts {
			h = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), ".")
			host := strings.ToLower(u.Hostname())
			if host == h || strings.HasSuffix(host, "."+h) {
				ok = true
				break
			}
		}
		add("allowlisted_host", ok, "host is not allowlisted")
	} else if len(allowedHosts) == 0 {
		checks["allowlisted_host"] = "skip: pass --allow-host to enforce RonanRx host policy"
	}
	return map[string]any{"link": raw, "allowed": len(reasons) == 0, "reasons": reasons, "checks": checks}
}

func messageMatchesChat(msg map[string]any, chatID string) bool {
	if chatID == "" {
		return false
	}
	for _, key := range []string{"chat_id", "chatId", "chat", "conversation_id"} {
		if fmt.Sprint(msg[key]) == chatID {
			return true
		}
	}
	return strings.Contains(fmt.Sprint(msg), chatID)
}

func messageText(msg map[string]any) string {
	for _, key := range []string{"text", "body", "content", "message", "value"} {
		if v := firstString(msg, key); v != "" {
			return v
		}
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func truncateRonanRx(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func printJSONMap(cmd *cobra.Command, out map[string]any) error {
	b, _ := json.Marshal(out)
	return printOutput(cmd.OutOrStdout(), b, true)
}

func containsPHI(s string) bool { return store.RedactPHI(s) != s }
