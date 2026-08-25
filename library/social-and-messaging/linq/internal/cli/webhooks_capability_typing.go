// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/internal/client"
	"github.com/spf13/cobra"
)

const webhookUpdateEventSemantics = "PUT /v3/webhook-subscriptions/{subscriptionId} treats subscribed_events as a full replacement for that field, not a delta"

var defaultWebhookDoctorEvents = []string{
	"message.received",
	"chat.typing_indicator.started",
	"chat.typing_indicator.stopped",
}

var typingInboundWebhookEvents = map[string]bool{
	"chat.typing_indicator.started": true,
	"chat.typing_indicator.stopped": true,
}

type webhookSubscriptionView struct {
	ID                    string   `json:"id,omitempty"`
	TargetURL             string   `json:"target_url,omitempty"`
	IsActive              any      `json:"is_active,omitempty"`
	SubscribedEvents      []string `json:"subscribed_events"`
	PhoneNumbersCount     int      `json:"phone_numbers_count,omitempty"`
	MissingExpectedEvents []string `json:"missing_expected_events,omitempty"`
	UnexpectedEvents      []string `json:"unexpected_events,omitempty"`
	Status                string   `json:"status,omitempty"`
	Warning               string   `json:"warning,omitempty"`
}

func newWebhooksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Manage Linq webhook subscriptions and subscribed events",
		Long: `Manage Linq webhook subscriptions without hand-rolling raw API bodies.

add-event and remove-event fetch the current subscription, validate event names
against /v3/webhook-events, compute the new subscribed_events array, and send
the complete replacement array for that field. Use --dry-run to inspect the
before and after event set without mutating the subscription.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebhooksListCmd(flags))
	cmd.AddCommand(newWebhooksShowCmd(flags))
	cmd.AddCommand(newWebhooksAddEventCmd(flags))
	cmd.AddCommand(newWebhooksRemoveEventCmd(flags))
	cmd.AddCommand(newWebhooksSetEventsCmd(flags))
	cmd.AddCommand(newWebhooksDoctorCmd(flags))
	return cmd
}

func newWebhooksListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List webhook subscriptions and subscribed events",
		Example:     "  linq-pp-cli webhooks list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			data, err := c.GetNoCache(cmd.Context(), "/v3/webhook-subscriptions", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			subs, err := extractWebhookSubscriptions(data)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), webhookSubscriptionViews(subs), flags)
		},
	}
}

func newWebhooksShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show <subscription-id>",
		Short:       "Show one webhook subscription and its subscribed events",
		Example:     "  linq-pp-cli webhooks show sub_123 --agent",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sub, err := fetchWebhookSubscription(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), webhookSubscriptionViewFromMap(sub), flags)
		},
	}
}

func newWebhooksAddEventCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add-event <subscription-id> <event>...",
		Short:   "Add events to a webhook subscription with read-modify-write safety",
		Long:    "Add events to a webhook subscription with read-modify-write safety.\n\n" + webhookUpdateEventSemantics + ".",
		Example: "  linq-pp-cli webhooks add-event sub_123 chat.typing_indicator.started chat.typing_indicator.stopped --agent --dry-run",
		Args:    cobra.MinimumNArgs(2),
		RunE:    runWebhooksChangeEvents(flags, "add-event"),
	}
}

func newWebhooksRemoveEventCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove-event <subscription-id> <event>...",
		Short:   "Remove events from a webhook subscription with read-modify-write safety",
		Long:    "Remove events from a webhook subscription with read-modify-write safety.\n\n" + webhookUpdateEventSemantics + ".",
		Example: "  linq-pp-cli webhooks remove-event sub_123 chat.typing_indicator.started --agent --dry-run",
		Args:    cobra.MinimumNArgs(2),
		RunE:    runWebhooksChangeEvents(flags, "remove-event"),
	}
}

func newWebhooksSetEventsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "set-events <subscription-id> <event>...",
		Short:   "Replace a webhook subscription's event set explicitly",
		Long:    "Replace a webhook subscription's event set explicitly.\n\n" + webhookUpdateEventSemantics + ".",
		Example: "  linq-pp-cli webhooks set-events sub_123 message.received chat.typing_indicator.started chat.typing_indicator.stopped --agent --dry-run",
		Args:    cobra.MinimumNArgs(2),
		RunE:    runWebhooksChangeEvents(flags, "set-events"),
	}
}

func runWebhooksChangeEvents(flags *rootFlags, op string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		subscriptionID := args[0]
		requestedEvents := normalizeStringSet(args[1:])
		if len(requestedEvents) == 0 {
			return usageErr(fmt.Errorf("at least one event is required"))
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		c.NoCache = true
		// Dry-run still needs live reads so add/remove/set can validate the
		// event catalog and compute an accurate before/after event set. Only
		// the final PUT remains suppressed by flags.dryRun below.
		c.DryRun = false
		catalog, err := fetchWebhookEventCatalog(cmd.Context(), c)
		if err != nil {
			return err
		}
		if err := validateWebhookEvents(requestedEvents, catalog); err != nil {
			return err
		}
		sub, err := fetchWebhookSubscriptionWithClient(cmd.Context(), c, subscriptionID)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		before := extractStringSliceFromMap(sub, "subscribed_events", "subscribedEvents")
		after := applyWebhookEventChange(before, requestedEvents, op)
		if op == "remove-event" && len(after) == 0 {
			return usageErr(fmt.Errorf("remove-event would leave subscribed_events empty; use set-events only when an empty subscription is intentional"))
		}
		body := map[string]any{"subscribed_events": after}
		path := replacePathParam("/v3/webhook-subscriptions/{subscriptionId}", "subscriptionId", subscriptionID)
		out := map[string]any{
			"action":             op,
			"resource":           "webhook-subscriptions",
			"id":                 subscriptionID,
			"path":               path,
			"before_events":      before,
			"after_events":       after,
			"requested_events":   requestedEvents,
			"would_update":       !sameStringSet(before, after),
			"update_semantics":   webhookUpdateEventSemantics,
			"subscribed_events":  after,
			"typing_event_ready": hasAllEvents(after, []string{"chat.typing_indicator.started", "chat.typing_indicator.stopped"}),
		}
		if flags.dryRun {
			out["dry_run"] = true
			out["success"] = false
			out["body"] = body
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		}
		data, status, err := c.PutWithParams(cmd.Context(), path, map[string]string{}, body)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		out["status"] = status
		out["success"] = status >= 200 && status < 300
		if len(data) > 0 {
			var parsed any
			if json.Unmarshal(data, &parsed) == nil {
				out["data"] = parsed
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
}

func newWebhooksDoctorCmd(flags *rootFlags) *cobra.Command {
	var expected []string
	var targetURL string
	var subscriptionID string
	var strict bool
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Check webhook subscriptions for expected event drift",
		Example:     "  linq-pp-cli webhooks doctor --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(expected) == 0 {
				expected = append([]string(nil), defaultWebhookDoctorEvents...)
			}
			expected = normalizeStringSet(expected)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			c.DryRun = false
			catalog, err := fetchWebhookEventCatalog(cmd.Context(), c)
			if err != nil {
				return err
			}
			if err := validateWebhookEvents(expected, catalog); err != nil {
				return err
			}
			var subs []map[string]any
			if subscriptionID != "" {
				sub, err := fetchWebhookSubscriptionWithClient(cmd.Context(), c, subscriptionID)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				subs = []map[string]any{sub}
			} else {
				data, err := c.GetNoCache(cmd.Context(), "/v3/webhook-subscriptions", map[string]string{})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				subs, err = extractWebhookSubscriptions(data)
				if err != nil {
					return err
				}
			}
			views := make([]webhookSubscriptionView, 0, len(subs))
			drifted := 0
			for _, sub := range subs {
				view := webhookSubscriptionViewFromMap(sub)
				if targetURL != "" && view.TargetURL != targetURL {
					continue
				}
				view.MissingExpectedEvents = stringSetDiff(expected, view.SubscribedEvents)
				view.UnexpectedEvents = stringSetDiff(view.SubscribedEvents, expected)
				if len(view.MissingExpectedEvents) > 0 {
					drifted++
					view.Status = "drift"
					view.Warning = webhookDoctorWarning(view)
				} else {
					view.Status = "ok"
				}
				views = append(views, view)
			}
			out := map[string]any{
				"ok":                drifted == 0,
				"expected_events":   expected,
				"drifted_count":     drifted,
				"checked_count":     len(views),
				"typing_event_pair": []string{"chat.typing_indicator.started", "chat.typing_indicator.stopped"},
				"subscriptions":     views,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
				return err
			}
			if strict && drifted > 0 {
				return fmt.Errorf("webhook drift detected on %d subscription(s)", drifted)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&expected, "expect", nil, "Expected subscribed event; repeatable. Defaults to message.received plus inbound typing start/stop")
	cmd.Flags().StringVar(&targetURL, "target-url", "", "Only check subscriptions with this target URL")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Only check this subscription ID")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero when drift is found")
	return cmd
}

func newCapabilityCheckCmd(flags *rootFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:     "check <address>...",
		Aliases: []string{"route"},
		Short:   "Resolve addresses to imessage, rcs, or sms",
		Long: `Resolve addresses to the richest available Linq channel.

The command checks iMessage first, then RCS, and reports SMS when neither check
is available. Output is structural and masks the address by default.`,
		Example: `  linq-pp-cli capability check +15551234567 --agent
  linq-pp-cli capability check --file ./addresses.txt --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			addresses, err := capabilityAddresses(args, file)
			if err != nil {
				return err
			}
			if len(addresses) == 0 {
				return usageErr(fmt.Errorf("at least one address or --file entry is required"))
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":       true,
					"would_check":   len(addresses),
					"address_refs":  maskedAddresses(addresses),
					"request_shape": map[string]any{"handle_check": map[string]any{"address": "<address>"}},
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := make([]map[string]any, 0, len(addresses))
			for i, address := range addresses {
				result, err := resolveCapabilityRoute(cmd.Context(), c, address)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				result["index"] = i
				results = append(results, result)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"results": results,
				"count":   len(results),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Read one address per line for batch checks")
	return cmd
}

func newTypingWatchCmd(flags *rootFlags) *cobra.Command {
	var file string
	var follow bool
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch a captured webhook/debug stream for inbound typing events",
		Long: `Watch a captured webhook/debug stream for inbound typing events.

Linq sends inbound typing indicators only by webhook push. This command never
receives webhooks from Linq and does not poll the Linq API. It reads NDJSON or
line-delimited JSON from stdin or --file and emits only structural typing event
records: event type, chat_id, and observed_at.`,
		Example: `  linq-pp-cli typing watch --file ./linq-webhooks.ndjson --agent
  tail -f ./debug/linq-webhooks.ndjson | linq-pp-cli typing watch --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if follow && file == "" {
				return usageErr(fmt.Errorf("--follow requires --file; for pipes, use tail -f FILE | linq-pp-cli typing watch"))
			}
			reader, closeFn, err := typingWatchReader(file, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if closeFn != nil {
				defer closeFn()
			}
			return streamTypingWatchEvents(cmd.Context(), reader, cmd.OutOrStdout(), follow)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Captured NDJSON/JSONL webhook stream to inspect; defaults to stdin")
	cmd.Flags().BoolVar(&follow, "follow", false, "Continue watching --file for new lines")
	return cmd
}

func fetchWebhookSubscription(ctx context.Context, flags *rootFlags, id string) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.NoCache = true
	c.DryRun = false
	return fetchWebhookSubscriptionWithClient(ctx, c, id)
}

func fetchWebhookSubscriptionWithClient(ctx context.Context, c *client.Client, id string) (map[string]any, error) {
	path := replacePathParam("/v3/webhook-subscriptions/{subscriptionId}", "subscriptionId", id)
	data, err := c.GetNoCache(ctx, path, map[string]string{})
	if err != nil {
		return nil, err
	}
	var sub map[string]any
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("parsing webhook subscription: %w", err)
	}
	return sub, nil
}

func fetchWebhookEventCatalog(ctx context.Context, c *client.Client) (map[string]bool, error) {
	data, err := c.GetNoCache(ctx, "/v3/webhook-events", map[string]string{})
	if err != nil {
		return nil, err
	}
	events := extractWebhookEventNames(data)
	if len(events) == 0 {
		return nil, fmt.Errorf("webhook event catalog response did not include events")
	}
	catalog := make(map[string]bool, len(events))
	for _, event := range events {
		catalog[event] = true
	}
	return catalog, nil
}

func validateWebhookEvents(events []string, catalog map[string]bool) error {
	var unknown []string
	for _, event := range events {
		if !catalog[event] {
			unknown = append(unknown, event)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	available := make([]string, 0, len(catalog))
	for event := range catalog {
		available = append(available, event)
	}
	sort.Strings(available)
	return usageErr(fmt.Errorf("unknown webhook event(s): %s; valid events: %s", strings.Join(unknown, ", "), strings.Join(available, ", ")))
}

func extractWebhookSubscriptions(data json.RawMessage) ([]map[string]any, error) {
	var direct []map[string]any
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parsing webhook subscriptions: %w", err)
	}
	for _, key := range []string{"data", "items", "results", "webhook_subscriptions", "subscriptions"} {
		if raw, ok := obj[key]; ok {
			var nested []map[string]any
			if err := json.Unmarshal(raw, &nested); err == nil {
				return nested, nil
			}
		}
	}
	return nil, fmt.Errorf("webhook subscriptions response did not include a subscription list")
}

func extractWebhookEventNames(data json.RawMessage) []string {
	var direct []string
	if err := json.Unmarshal(data, &direct); err == nil {
		return normalizeStringSet(direct)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"events", "data", "items", "results"} {
		if raw, ok := obj[key]; ok {
			var stringsOut []string
			if err := json.Unmarshal(raw, &stringsOut); err == nil {
				return normalizeStringSet(stringsOut)
			}
			var rows []map[string]any
			if err := json.Unmarshal(raw, &rows); err == nil {
				out := make([]string, 0, len(rows))
				for _, row := range rows {
					if event := firstString(row, "event", "type", "name", "id", "value"); event != "" {
						out = append(out, event)
					}
				}
				return normalizeStringSet(out)
			}
		}
	}
	return nil
}

func webhookSubscriptionViews(subs []map[string]any) []webhookSubscriptionView {
	views := make([]webhookSubscriptionView, 0, len(subs))
	for _, sub := range subs {
		views = append(views, webhookSubscriptionViewFromMap(sub))
	}
	return views
}

func webhookSubscriptionViewFromMap(sub map[string]any) webhookSubscriptionView {
	return webhookSubscriptionView{
		ID:                firstString(sub, "id", "subscription_id", "subscriptionId"),
		TargetURL:         firstString(sub, "target_url", "targetURL", "targetUrl"),
		IsActive:          firstPresent(sub, "is_active", "isActive", "active"),
		SubscribedEvents:  extractStringSliceFromMap(sub, "subscribed_events", "subscribedEvents", "events"),
		PhoneNumbersCount: len(extractStringSliceFromMap(sub, "phone_numbers", "phoneNumbers")),
	}
}

func firstPresent(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			return v
		}
	}
	return nil
}

func extractStringSliceFromMap(obj map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch typed := v.(type) {
		case []string:
			return normalizeStringSet(typed)
		case []any:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				s := strings.TrimSpace(fmt.Sprint(item))
				if s != "" && s != "<nil>" {
					out = append(out, s)
				}
			}
			return normalizeStringSet(out)
		}
	}
	return nil
}

func normalizeStringSet(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func applyWebhookEventChange(before, requested []string, op string) []string {
	set := map[string]bool{}
	switch op {
	case "set-events":
		return normalizeStringSet(requested)
	default:
		for _, event := range before {
			set[event] = true
		}
	}
	for _, event := range requested {
		switch op {
		case "add-event":
			set[event] = true
		case "remove-event":
			delete(set, event)
		}
	}
	out := make([]string, 0, len(set))
	for event := range set {
		out = append(out, event)
	}
	return normalizeStringSet(out)
}

func sameStringSet(a, b []string) bool {
	a = normalizeStringSet(a)
	b = normalizeStringSet(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hasAllEvents(events, required []string) bool {
	set := map[string]bool{}
	for _, event := range events {
		set[event] = true
	}
	for _, event := range required {
		if !set[event] {
			return false
		}
	}
	return true
}

func stringSetDiff(have, compare []string) []string {
	set := map[string]bool{}
	for _, value := range compare {
		set[value] = true
	}
	var out []string
	for _, value := range normalizeStringSet(have) {
		if !set[value] {
			out = append(out, value)
		}
	}
	return out
}

func webhookDoctorWarning(view webhookSubscriptionView) string {
	if hasAllEvents(view.SubscribedEvents, []string{"message.received"}) && !hasAllEvents(view.SubscribedEvents, []string{"chat.typing_indicator.started", "chat.typing_indicator.stopped"}) {
		return "subscribed to message.received but not chat.typing_indicator.*; typing presence will not be delivered"
	}
	return "subscription is missing expected webhook events"
}

func capabilityAddresses(args []string, file string) ([]string, error) {
	addresses := append([]string(nil), args...)
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			addresses = append(addresses, line)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return normalizeStringSet(addresses), nil
}

func resolveCapabilityRoute(ctx context.Context, c *client.Client, address string) (map[string]any, error) {
	imessage, err := runHandleCapabilityCheck(ctx, c, "/v3/capability/check_imessage", address)
	if err != nil {
		return nil, err
	}
	channel := "sms"
	rcsChecked := false
	var rcsAvailable any
	if imessage {
		channel = "imessage"
	} else {
		rcsChecked = true
		available, err := runHandleCapabilityCheck(ctx, c, "/v3/capability/check_rcs", address)
		if err != nil {
			return nil, err
		}
		rcsAvailable = available
		if available {
			channel = "rcs"
		}
	}
	return map[string]any{
		"address_ref": maskAddress(address),
		"channel":     channel,
		"checks": map[string]any{
			"imessage_available": imessage,
			"rcs_checked":        rcsChecked,
			"rcs_available":      rcsAvailable,
		},
		"features": capabilityFeaturesForChannel(channel),
	}, nil
}

func runHandleCapabilityCheck(ctx context.Context, c *client.Client, path, address string) (bool, error) {
	body := map[string]any{"handle_check": map[string]any{"address": address}}
	data, _, err := c.PostQueryWithParams(ctx, path, map[string]string{}, body)
	if err != nil {
		return false, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return false, fmt.Errorf("parsing capability response: %w", err)
	}
	available, _ := parsed["available"].(bool)
	return available, nil
}

func capabilityFeaturesForChannel(channel string) map[string]any {
	return map[string]any{
		"typing_indicators":       channel == "imessage",
		"typing_inbound_webhooks": channel == "imessage",
		"effects":                 channel == "imessage",
		"read_receipts":           channel == "imessage" || channel == "rcs",
		"typing_event_names":      []string{"chat.typing_indicator.started", "chat.typing_indicator.stopped"},
		"protocol_warning":        "typing indicators and effects are iMessage-only; SMS has no typing or read receipt support",
		"subscription_required":   channel == "imessage",
	}
}

func maskedAddresses(addresses []string) []string {
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, maskAddress(address))
	}
	return out
}

func maskAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.Contains(address, "@") {
		local, domain, _ := strings.Cut(address, "@")
		if len(local) <= 1 {
			return "*@" + domain
		}
		return local[:1] + "***@" + domain
	}
	digits := make([]rune, 0, len(address))
	for _, r := range address {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) >= 4 {
		return "***" + string(digits[len(digits)-4:])
	}
	return "***"
}

func typingWatchReader(file string, stdin io.Reader) (io.Reader, func(), error) {
	if file == "" {
		return stdin, nil, nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

func streamTypingWatchEvents(ctx context.Context, reader io.Reader, writer io.Writer, follow bool) error {
	buf := bufio.NewReader(reader)
	enc := json.NewEncoder(writer)
	lineNo := 0
	for {
		line, err := buf.ReadBytes('\n')
		if len(line) > 0 {
			lineNo++
			if event, ok := parseTypingWatchLine(line, lineNo); ok {
				if err := enc.Encode(event); err != nil {
					return err
				}
			}
		}
		if err == nil {
			continue
		}
		if err != io.EOF {
			return err
		}
		if !follow {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func parseTypingWatchLine(line []byte, lineNo int) (map[string]any, bool) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal(line, &obj); err != nil {
		return nil, false
	}
	eventName := firstString(obj, "event", "type", "name", "webhook_event", "webhook_event_type")
	if eventName == "" {
		if data, ok := obj["data"].(map[string]any); ok {
			eventName = firstString(data, "event", "type", "name")
		}
	}
	if !typingInboundWebhookEvents[eventName] {
		return nil, false
	}
	chatID := firstString(obj, "chat_id", "chatId")
	if data, ok := obj["data"].(map[string]any); ok && chatID == "" {
		chatID = firstString(data, "chat_id", "chatId")
	}
	return map[string]any{
		"event":       eventName,
		"chat_id":     chatID,
		"observed_at": time.Now().UTC().Format(time.RFC3339),
		"source_line": lineNo,
	}, true
}
