package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/store"
	"github.com/spf13/cobra"
)

func newInviteLinkCmd(flags *rootFlags) *cobra.Command {
	var base, frontDoorPath, from, routing, token, prefillBody string
	var patientAuthoredPrefill, devMode, checkFrontDoor bool
	cmd := &cobra.Command{
		Use:   "invite-link --base-url URL [--front-door-path PATH] --from NUMBER --routing WELCOME_FLOW --token TOKEN",
		Short: "Build an inbound-first HTTPS front-door link plus sms: URI preview without sending",
		Long: `Build the RonanRx click-to-text front-door contract.

The HTTPS front_door_link is what belongs on the website or CTA. It does not
itself open Messages unless that page performs the required sms: handoff. The
sms_uri_preview shows the exact URI that the browser page should open after the
patient taps Get Started. This command never sends a Linq message.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" && from == "" && routing == "" && token == "" {
				return cmd.Help()
			}
			if base == "" || from == "" || routing == "" || token == "" {
				return fmt.Errorf("--base-url, --from, --routing, and --token are required")
			}
			invite, err := buildInviteSchema(inviteLinkOptions{
				BaseURL:                base,
				FrontDoorPath:          frontDoorPath,
				From:                   from,
				Routing:                routing,
				Token:                  token,
				PrefillBody:            prefillBody,
				PatientAuthoredPrefill: patientAuthoredPrefill,
				DevMode:                devMode,
			})
			if err != nil {
				return err
			}
			if checkFrontDoor {
				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()
				check := inspectFrontDoor(ctx, invite.FrontDoorLink, token, true)
				invite.FrontDoorCheck = &check
				if !check.Allowed {
					invite.Warnings = append(invite.Warnings, "front-door check did not prove the page performs a safe sms: handoff")
				}
			}
			return printJSONValue(cmd, invite)
		},
	}
	cmd.Flags().StringVar(&base, "base-url", "", "HTTPS front-door URL or origin")
	cmd.Flags().StringVar(&frontDoorPath, "front-door-path", "", "Front-door path to apply to --base-url, e.g. /start/text/")
	cmd.Flags().StringVar(&from, "from", "", "Linq sender phone number in E.164 format")
	cmd.Flags().StringVar(&routing, "routing", "", "Allowlisted non-PHI route ID, e.g. WELCOME_FLOW")
	cmd.Flags().StringVar(&token, "token", "", "Opaque authenticated token or link slug")
	cmd.Flags().StringVar(&prefillBody, "prefill-body", "", "Optional patient-authored prefill body; must pass no-PHI checks")
	cmd.Flags().BoolVar(&patientAuthoredPrefill, "patient-authored-prefill", false, "Required when overriding the default prefill body")
	cmd.Flags().BoolVar(&devMode, "dev-mode", false, "Allow http://localhost front-door URLs for local development only")
	cmd.Flags().BoolVar(&checkFrontDoor, "check-front-door", false, "Fetch and statically inspect the front-door page contract")
	return cmd
}

func newWelcomeFlowCmd(flags *rootFlags) *cobra.Command {
	var base, frontDoorPath, from, routing, token, secureLink, chatID, mode string
	var allowedHosts []string
	cmd := &cobra.Command{
		Use:         "welcome-flow --base-url URL [--front-door-path PATH] --from NUMBER --token TOKEN --secure-link URL",
		Short:       "Plan the RonanRx inbound-first welcome flow without sending",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" && from == "" && token == "" && secureLink == "" && chatID == "" {
				return cmd.Help()
			}
			if base == "" || from == "" || routing == "" || token == "" || secureLink == "" {
				return fmt.Errorf("--base-url, --from, --routing, --token, and --secure-link are required")
			}
			invite, err := buildInviteSchema(inviteLinkOptions{
				BaseURL:       base,
				FrontDoorPath: frontDoorPath,
				From:          from,
				Routing:       routing,
				Token:         token,
			})
			if err != nil {
				return err
			}
			linkAudit := auditLink(secureLink, allowedHosts)
			preflight := evaluateSendPreflight(chatID, "INBOUND_OK "+routing, secureLink, sendPreflightOptions{
				Mode:         mode,
				AllowedHosts: allowedHosts,
			})
			if chatID == "" {
				preflight.Allowed = false
				preflight.Reasons = append(preflight.Reasons, "chat ID is not known until the patient sends the prefilled inbound message")
				preflight.BlockingReasons = preflight.Reasons
				preflight.Checks["chat_id"] = "pending: wait for inbound chat discovery"
				preflight.Checks["inbound_evidence"] = "pending: wait for inbound patient message"
			}
			steps := []map[string]any{
				{"step": 1, "name": "publish_front_door_link", "status": "ready", "owner": "operator", "artifact": invite.FrontDoorLink},
				{"step": 2, "name": "patient_taps_and_sends_prefilled_inbound_message", "status": "waiting_on_patient", "owner": "patient", "prefill_body": invite.PrefillBody, "first_sender": "patient"},
				{"step": 3, "name": "inbound_chat_created_or_discovered", "status": "blocked_until_step_2", "owner": "agent", "command": "linq-pp-cli sync && linq-pp-cli search WELCOME_FLOW"},
				{"step": 4, "name": "consent_inbound_audit", "status": "blocked_until_chat_id", "owner": "agent", "command": "linq-pp-cli consent-audit --chat-id <inbound-chat-id>"},
				{"step": 5, "name": "draft_and_preflight_secure_pointer_reply", "status": "draft_only", "owner": "agent", "command": "linq-pp-cli safe-reply-draft --chat-id <inbound-chat-id> --intent welcome_next_step --link <secure-link> && linq-pp-cli send-preflight --mode real --chat-id <inbound-chat-id> --routing 'INBOUND_OK WELCOME_FLOW' --link <secure-link> --allow-host <host>"},
				{"step": 6, "name": "real_send_guard", "status": "blocked_until_human_approval_and_inbound_evidence", "owner": "human", "would_send": false},
			}
			return printJSONValue(cmd, map[string]any{
				"flow":                    "ronanrx_welcome",
				"dry_run":                 true,
				"would_send":              false,
				"outbound_send_performed": false,
				"first_real_action":       "patient_sends_prefilled_inbound_message",
				"pointer_not_payload":     invite.PointerNotPayload && linkAudit["allowed"] == true,
				"invite":                  invite,
				"invite_link":             invite.FrontDoorLink,
				"invite_link_deprecated":  true,
				"secure_link_audit":       linkAudit,
				"send_preflight":          preflight,
				"steps":                   steps,
			})
		},
	}
	cmd.Flags().StringVar(&base, "base-url", "", "HTTPS front-door URL or origin")
	cmd.Flags().StringVar(&frontDoorPath, "front-door-path", "", "Front-door path to apply to --base-url, e.g. /start/text/")
	cmd.Flags().StringVar(&from, "from", "", "Linq sender phone number in E.164 format")
	cmd.Flags().StringVar(&routing, "routing", "WELCOME_FLOW", "Non-PHI routing text")
	cmd.Flags().StringVar(&token, "token", "", "Opaque authenticated token or link slug")
	cmd.Flags().StringVar(&secureLink, "secure-link", "", "Opaque secure welcome/intake link")
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Optional known inbound chat ID for preflight")
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed secure-link host suffix; repeatable")
	cmd.Flags().StringVar(&mode, "mode", "real", "Preflight mode: synthetic, demo, or real")
	return cmd
}

func newGuardedSendCmd(flags *rootFlags) *cobra.Command {
	var chatID, link, routing, inboundMessageID, inboundAt string
	var allowedHosts []string
	var humanApproved bool
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
			checks := evaluateSendPreflight(chatID, routing, link, sendPreflightOptions{
				Mode:             "real",
				AllowedHosts:     allowedHosts,
				InboundMessageID: inboundMessageID,
				InboundAt:        inboundAt,
			})
			if !humanApproved {
				checks.Allowed = false
				checks.Reasons = append(checks.Reasons, "human approval is required for real send")
				checks.BlockingReasons = checks.Reasons
				checks.Checks["human_approval"] = "fail: --human-approved not set"
			} else {
				checks.Checks["human_approval"] = "pass"
			}
			if !checks.Allowed {
				return fmt.Errorf("send refused: %s", strings.Join(checks.BlockingReasons, "; "))
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
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed secure-link host suffix; repeatable")
	cmd.Flags().StringVar(&inboundMessageID, "inbound-message-id", "", "Explicit inbound message evidence ID")
	cmd.Flags().StringVar(&inboundAt, "inbound-at", "", "Explicit inbound message timestamp evidence")
	cmd.Flags().BoolVar(&humanApproved, "human-approved", false, "Required final human approval for real send")
	return cmd
}

func newSendPreflightCmd(flags *rootFlags) *cobra.Command {
	var chatID, link, routing, mode, inboundMessageID, inboundAt string
	var allowedHosts []string
	var limit int
	cmd := &cobra.Command{
		Use:         "send-preflight --chat-id CHAT --link URL --routing TEXT",
		Short:       "Explain whether guarded send would be allowed without sending",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var messages []map[string]any
			if strings.EqualFold(mode, "real") && strings.TrimSpace(chatID) != "" && !isPlaceholderChatID(chatID) {
				loaded, err := loadLocalMessages(limit)
				if err == nil {
					messages = loaded
				}
			}
			checks := evaluateSendPreflight(chatID, routing, link, sendPreflightOptions{
				Mode:             mode,
				AllowedHosts:     allowedHosts,
				InboundMessageID: inboundMessageID,
				InboundAt:        inboundAt,
				LocalMessages:    messages,
			})
			return printJSONValue(cmd, checks)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat-id", "", "Existing inbound chat ID")
	cmd.Flags().StringVar(&link, "link", "", "Opaque authenticated link")
	cmd.Flags().StringVar(&routing, "routing", "", "Non-PHI routing text; must include INBOUND_OK marker")
	cmd.Flags().StringVar(&mode, "mode", "real", "Preflight mode: synthetic, demo, or real")
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed secure-link host suffix; repeatable")
	cmd.Flags().StringVar(&inboundMessageID, "inbound-message-id", "", "Explicit inbound message evidence ID")
	cmd.Flags().StringVar(&inboundAt, "inbound-at", "", "Explicit inbound message timestamp evidence")
	cmd.Flags().IntVar(&limit, "limit", 250, "Local messages to inspect for inbound evidence in real mode")
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
	var networkCheck bool
	cmd := &cobra.Command{
		Use:         "link-audit URL... [--link URL] [--allow-host HOST]",
		Short:       "Check links for pointer-not-payload safety",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			links = append(links, args...)
			if len(links) == 0 {
				messages, err := loadLocalMessages(limit)
				if err != nil {
					return err
				}
				links = extractLinks(messages)
			}
			results := make([]map[string]any, 0, len(links))
			for _, raw := range links {
				result := auditLink(raw, allowedHosts)
				if networkCheck {
					result = auditLinkPreview(cmd.Context(), raw, result)
				}
				results = append(results, result)
			}
			return printJSONMap(cmd, map[string]any{"links_checked": len(results), "results": results, "source": "explicit --link values or local encrypted mirror"})
		},
	}
	cmd.Flags().StringArrayVar(&links, "link", nil, "Link to audit; repeatable")
	cmd.Flags().StringArrayVar(&allowedHosts, "allow-host", nil, "Allowed host suffix; repeatable")
	cmd.Flags().IntVar(&limit, "limit", 250, "Local messages to inspect when --link is omitted")
	cmd.Flags().BoolVar(&networkCheck, "network-check", false, "Fetch page metadata and verify link-preview surfaces are generic")
	return cmd
}

func newFrontDoorCheckCmd(flags *rootFlags) *cobra.Command {
	var rawURL, token string
	var noFetch bool
	cmd := &cobra.Command{
		Use:         "front-door-check --url URL",
		Short:       "Statically inspect a RonanRx front-door page for safe sms: handoff behavior",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if rawURL == "" {
				return fmt.Errorf("--url is required")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			result := inspectFrontDoor(ctx, rawURL, token, !noFetch)
			return printJSONValue(cmd, result)
		},
	}
	cmd.Flags().StringVar(&rawURL, "url", "", "Front-door URL to inspect")
	cmd.Flags().StringVar(&token, "token", "", "Expected opaque token; used only to ensure preview meta does not expose it")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "Do not fetch the page; return the pluggable check shape only")
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
	return printJSONValue(cmd, out)
}

func printJSONValue(cmd *cobra.Command, out any) error {
	b, _ := json.Marshal(out)
	return printOutput(cmd.OutOrStdout(), b, true)
}

func containsPHI(s string) bool { return analyzePHI(s).Contains || store.RedactPHI(s) != s }
