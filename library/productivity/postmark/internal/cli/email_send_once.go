// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

const (
	sendOnceMetadataKey = "pp_idempotency_key"
	sendOnceMaxKeyLen   = 80

	sendOnceSourceLedger   = "ledger"
	sendOnceSourcePostmark = "postmark"
	sendOnceSourceSent     = "sent"
	sendOnceSourcePlan     = "plan"
)

type sendOnceDedupe struct {
	Ledger   string `json:"ledger"`
	Postmark string `json:"postmark"`
}

type sendOnceResult struct {
	Key         string         `json:"key"`
	KeySource   string         `json:"key_source"`
	Duplicate   bool           `json:"duplicate"`
	Sent        bool           `json:"sent"`
	WouldSend   bool           `json:"would_send"`
	Source      string         `json:"source"`
	MessageID   string         `json:"message_id,omitempty"`
	SubmittedAt string         `json:"submitted_at,omitempty"`
	To          string         `json:"to"`
	Server      string         `json:"server,omitempty"`
	Stream      string         `json:"stream"`
	Sandbox     bool           `json:"sandbox"`
	Window      string         `json:"window"`
	WindowStart string         `json:"window_start"`
	Dedupe      sendOnceDedupe `json:"dedupe"`
	// DeliveryUnknown is set when an earlier run reserved this key but never
	// confirmed delivery; the command refuses to send rather than risk a copy.
	DeliveryUnknown bool           `json:"delivery_unknown,omitempty"`
	Endpoint        string         `json:"endpoint,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
	Next            string         `json:"next,omitempty"`
}

type sendOnceError struct {
	Error     string `json:"error"`
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
	Recipient string `json:"recipient"`
	Next      string `json:"next"`
}

// sendOnceSplitAddrs splits a comma-separated address list.
func sendOnceSplitAddrs(s string) []string {
	out := make([]string, 0)
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sendOnceCanonicalJSON re-encodes JSON with sorted keys so key derivation
// ignores formatting and key order.
func sendOnceCanonicalJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var v any
	if err := decodeSingleJSON(raw, &v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(b)
}

// sendOnceDerivedKey hashes the fields that make two sends "the same
// message": recipients, sender, subject or template, template model, stream.
func sendOnceDerivedKey(to, cc, bcc, from, subject, template, model, stream, body string) string {
	content := "subject:" + strings.TrimSpace(subject)
	if strings.TrimSpace(template) != "" {
		content = "template:" + strings.TrimSpace(template)
	}
	if strings.TrimSpace(stream) == "" {
		stream = postmarkDefaultStream
	}
	parts := []string{
		"to:" + sendOnceCanonicalAddrs(to),
		"cc:" + sendOnceCanonicalAddrs(cc),
		"bcc:" + sendOnceCanonicalAddrs(bcc),
		"from:" + bareAddr(from),
		content,
		"model:" + sendOnceCanonicalJSON(model),
		"stream:" + strings.ToLower(strings.TrimSpace(stream)),
		"body:" + body,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return "sha256-" + hex.EncodeToString(sum[:])[:32]
}

// decodeSingleJSON decodes exactly one JSON value from raw, keeping numbers
// exact (UseNumber) and rejecting anything after it, so trailing text or a
// second object is an error instead of being silently dropped.
func decodeSingleJSON(raw string, v any) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("unexpected content after the JSON value")
	}
	return nil
}

// sendOnceCanonicalAddrs lowercases bare addresses and sorts them so order and
// display names do not change the derived key.
func sendOnceCanonicalAddrs(list string) string {
	addrs := sendOnceSplitAddrs(list)
	for i := range addrs {
		addrs[i] = bareAddr(addrs[i])
	}
	sort.Strings(addrs)
	return strings.Join(addrs, ",")
}

// sendOnceParseMetadata parses repeatable k=v flags.
func sendOnceParseMetadata(pairs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, fmt.Errorf("--metadata %q must be key=value", p)
		}
		if strings.EqualFold(k, sendOnceMetadataKey) {
			return nil, fmt.Errorf("--metadata key %q is reserved for the idempotency key; use --key", sendOnceMetadataKey)
		}
		out[k] = v
	}
	return out, nil
}

type sendOnceInput struct {
	from, to, cc, bcc, subject, text, html, template, model, stream, tag string
	metadata                                                             map[string]string
}

// sendOncePayload builds the /email or /email/withTemplate body with the
// idempotency key stamped into Metadata.
func sendOncePayload(in sendOnceInput, key string) (string, map[string]any, error) {
	payload := map[string]any{
		"From":          in.from,
		"To":            in.to,
		"MessageStream": in.stream,
	}
	if in.cc != "" {
		payload["Cc"] = in.cc
	}
	if in.bcc != "" {
		payload["Bcc"] = in.bcc
	}
	if in.tag != "" {
		payload["Tag"] = in.tag
	}
	meta := map[string]string{}
	for k, v := range in.metadata {
		meta[k] = v
	}
	meta[sendOnceMetadataKey] = key
	payload["Metadata"] = meta
	if in.template != "" {
		if id, err := strconv.ParseInt(in.template, 10, 64); err == nil {
			payload["TemplateId"] = id
		} else {
			payload["TemplateAlias"] = in.template
		}
		model := map[string]any{}
		if strings.TrimSpace(in.model) != "" {
			// UseNumber keeps large integers (IDs, amounts) exact instead of
			// rounding them through float64.
			if err := decodeSingleJSON(in.model, &model); err != nil {
				return "", nil, fmt.Errorf("--model must be one JSON object: %w", err)
			}
		}
		payload["TemplateModel"] = model
		return "/email/withTemplate", payload, nil
	}
	payload["Subject"] = in.subject
	if in.text != "" {
		payload["TextBody"] = in.text
	}
	if in.html != "" {
		payload["HtmlBody"] = in.html
	}
	return "/email", payload, nil
}

func newNovelEmailSendOnceCmd(flags *rootFlags) *cobra.Command {
	var in sendOnceInput
	var flagKey, flagWindow, dbPath string
	var metadataPairs []string
	var send bool

	cmd := &cobra.Command{
		Use:   "send-once",
		Short: "Send an email at most once per idempotency key so a retried agent never delivers a second copy; previews the payload and dedupe check unless --send is set",
		Long: strings.Trim(`
Use this command for one-off agent or scripted sends that may be retried (one-time codes, receipts, follow-ups), where a second copy would reach the recipient. Do NOT use this command when a repeat send is intended, such as a deliberate resend after fixing an attachment; use 'email send' instead. Do NOT use it for multi-message sends; use 'email send-batch' instead.

The idempotency key is --key, or a hash of the recipients, sender, subject or
template, model, and stream when --key is omitted. Before sending, the command
checks the local send ledger and then searches Postmark for an outbound message
to the first recipient carrying Metadata.pp_idempotency_key within --window.
A match returns the prior MessageID with duplicate:true and sends nothing.

Without --send it prints the payload and the dedupe decision. With --send it
posts to /email (or /email/withTemplate for --template), stamps the key into
Metadata, and records the send in the ledger. --sandbox validates through
Postmark's test token without delivering; sandbox sends dedupe only against
other sandbox sends.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli email send-once --key otp-4821 --from app@example.com --to jane@example.com --subject "Your code" --text "Code: 4821" --agent
  postmark-pp-cli email send-once --key receipt-1042 --server "Main App" --from app@example.com --to jane@example.com --template password-reset --model '{"name":"Jane"}' --send
  postmark-pp-cli email send-once --sandbox --key otp-test --from app@example.com --to jane@example.com --subject "Your code" --text "Code: 4821" --send --json`, "\n"),
		// MCP tools can plan a send but never deliver: --send is withheld from
		// the MCP schema, which is what makes the read-only hint truthful there.
		// Delivery needs the CLI with an explicit --send.
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "mcp:write-flags": "send"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "email send-once")
			}
			if send && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "send email")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			in.to = strings.TrimSpace(in.to)
			in.from = strings.TrimSpace(in.from)
			in.template = strings.TrimSpace(in.template)
			switch {
			case in.to == "":
				_ = cmd.Usage()
				return usageErr(errors.New("--to is required"))
			case in.from == "":
				_ = cmd.Usage()
				return usageErr(errors.New("--from is required (or save one with 'servers use <name> --from <address>')"))
			case in.template == "" && strings.TrimSpace(in.subject) == "":
				_ = cmd.Usage()
				return usageErr(errors.New("--subject is required unless --template is given"))
			case in.template == "" && in.text == "" && in.html == "":
				_ = cmd.Usage()
				return usageErr(errors.New("--text or --html is required unless --template is given"))
			case in.template == "" && in.model != "":
				return usageErr(errors.New("--model only applies with --template"))
			}
			window, err := parsePositiveDuration(flagWindow, "window", "15m or 1h")
			if err != nil {
				return err
			}
			meta, err := sendOnceParseMetadata(metadataPairs)
			if err != nil {
				return usageErr(err)
			}
			in.metadata = meta
			if in.stream == "" {
				in.stream = sendOnceSavedStream(flags)
			}
			if in.stream == "" {
				in.stream = postmarkDefaultStream
			}
			res := sendOnceResult{KeySource: "flag", Key: strings.TrimSpace(flagKey), Stream: in.stream, Window: flagWindow, To: in.to, Sandbox: postmarkSelection.sandbox}
			if res.Key == "" {
				res.Key = sendOnceDerivedKey(in.to, in.cc, in.bcc, in.from, in.subject, in.template, in.model, in.stream, in.text+"\x00"+in.html)
				res.KeySource = "derived"
			}
			if len(res.Key) > sendOnceMaxKeyLen {
				return usageErr(fmt.Errorf("--key is %d characters; Postmark metadata values allow at most %d", len(res.Key), sendOnceMaxKeyLen))
			}
			endpoint, payload, err := sendOncePayload(in, res.Key)
			if err != nil {
				return usageErr(err)
			}
			recipients := sendOnceSplitAddrs(in.to)
			if len(recipients) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to must contain at least one address"))
			}
			firstTo := bareAddr(recipients[0])
			now := time.Now()
			windowStart := now.Add(-window)
			res.WindowStart = windowStart.UTC().Format(time.RFC3339)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res.Server = sendOnceServerName(c, flags)
			scope, err := sendOnceLedgerScope(ctx, c, res.Sandbox)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("postmark-pp-cli")
			}

			// 1. Local ledger.
			res.Dedupe.Ledger = "miss"
			var db *store.Store
			if _, statErr := os.Stat(dbPath); statErr == nil || send {
				db, err = store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("opening send ledger: %w", err)
				}
				defer db.Close()
				prior, err := db.PostmarkLedgerLookup(ctx, res.Key, scope, res.Sandbox, windowStart)
				if err != nil {
					return err
				}
				if prior != nil {
					sendOnceLedgerDuplicate(&res, prior, firstTo)
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
			}

			// 2. Postmark metadata search. The sandbox token cannot search.
			if res.Sandbox {
				res.Dedupe.Postmark = "skipped: sandbox token cannot search messages"
			} else {
				prior, err := sendOnceRemoteLookup(ctx, c, firstTo, res.Key, in.stream, windowStart)
				if err != nil {
					return fmt.Errorf("checking Postmark for a prior send (nothing was sent): %w", err)
				}
				res.Dedupe.Postmark = "miss"
				if prior != nil {
					res.Dedupe.Postmark = "hit"
					res.Duplicate, res.Source, res.MessageID, res.SubmittedAt = true, sendOnceSourcePostmark, prior.MessageID, prior.ReceivedAt
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
			}

			res.Endpoint, res.Payload = endpoint, payload
			if !send {
				res.WouldSend, res.Source = true, sendOnceSourcePlan
				res.Next = "rerun with --send to deliver"
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			// 3. Reserve the key, send, then confirm. The reservation is taken
			// in one SQLite write transaction, so a concurrent run with the same
			// key sees it and stops instead of sending a second copy.
			// The ledger stamps the reservation once it holds the write lock, so
			// neither a slow Postmark search nor lock contention can leave a
			// reservation already outside a short window.
			reservation, prior, err := db.PostmarkLedgerReserve(ctx, store.PostmarkLedgerEntry{
				Key: res.Key, Recipient: firstTo, Server: scope, Stream: in.stream, Sandbox: res.Sandbox,
			}, window)
			if err != nil {
				return fmt.Errorf("reserving the send ledger key (nothing was sent): %w", err)
			}
			if prior != nil {
				res.Endpoint, res.Payload = "", nil
				sendOnceLedgerDuplicate(&res, prior, firstTo)
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			// --send is the explicit delivery opt-in the transport requires.
			data, err := withSendAllowed(func() (json.RawMessage, error) {
				raw, _, err := c.Post(ctx, endpoint, payload)
				return raw, err
			})
			// Ledger bookkeeping after the POST must not inherit a send
			// deadline that may already have passed.
			ledgerCtx, ledgerCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer ledgerCancel()
			if err != nil {
				if sendOnceDefinitelyRefused(err) {
					if rerr := db.PostmarkLedgerRelease(ledgerCtx, reservation); rerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not release the send ledger reservation; retries with this key are blocked until the window passes: %v\n", rerr)
					}
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: the send outcome is unknown, so the key stays reserved for the window; check Postmark activity before retrying with a new --key.")
				}
				if f, ok := postmarkAPIFailure(err); ok && f.ErrorCode == postmarkErrInactiveRecipient {
					serr := sendOnceError{
						Error:     "inactive_recipient",
						ErrorCode: f.ErrorCode,
						Message:   f.Message,
						Recipient: firstTo,
						Next:      "postmark-pp-cli diagnose " + shellQuoteWord(firstTo) + postmarkServerArg(res.Server) + " --json",
					}
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						_ = printJSONFiltered(cmd.OutOrStdout(), serr, flags)
					}
					return apiErr(fmt.Errorf("Postmark refused the send: %s (ErrorCode 406). Run: %s", f.Message, serr.Next))
				}
				return classifyAPIErrorOnly(err)
			}
			var resp struct {
				To          string `json:"To"`
				SubmittedAt string `json:"SubmittedAt"`
				MessageID   string `json:"MessageID"`
				ErrorCode   int    `json:"ErrorCode"`
				Message     string `json:"Message"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing send response: %w", err)
			}
			if resp.ErrorCode != 0 || resp.MessageID == "" {
				return apiErr(fmt.Errorf("Postmark answered without confirming delivery (ErrorCode %d: %s); the key stays reserved for the window, so check Postmark activity before retrying with a new --key", resp.ErrorCode, resp.Message))
			}
			if err := db.PostmarkLedgerComplete(ledgerCtx, reservation, resp.MessageID); err != nil {
				return fmt.Errorf("sent MessageID %s but could not confirm it in the send ledger at %s; the key stays reserved for the window: %w", resp.MessageID, dbPath, err)
			}
			res.Sent, res.Source, res.MessageID, res.SubmittedAt = true, sendOnceSourceSent, resp.MessageID, resp.SubmittedAt
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagKey, "key", "", "Idempotency key (max 80 chars); omitted = hash of To/Cc/Bcc, sender, subject or template, model, stream, and body")
	cmd.Flags().StringVar(&in.from, "from", "", "Sender address (a confirmed sender signature)")
	cmd.Flags().StringVar(&in.to, "to", "", "Recipient address(es), comma-separated")
	cmd.Flags().StringVar(&in.cc, "cc", "", "Cc address(es), comma-separated")
	cmd.Flags().StringVar(&in.bcc, "bcc", "", "Bcc address(es), comma-separated")
	cmd.Flags().StringVar(&in.subject, "subject", "", "Subject (not used with --template)")
	cmd.Flags().StringVar(&in.text, "text", "", "Plain-text body")
	cmd.Flags().StringVar(&in.html, "html", "", "HTML body")
	cmd.Flags().StringVar(&in.template, "template", "", "Template alias or numeric ID; sends through /email/withTemplate")
	cmd.Flags().StringVar(&in.model, "model", "", "Template model as a JSON object (with --template)")
	cmd.Flags().StringVar(&in.stream, "stream", "", "Message stream ID (default: the server's saved default, else outbound)")
	cmd.Flags().StringVar(&in.tag, "tag", "", "Tag for the message")
	cmd.Flags().StringArrayVar(&metadataPairs, "metadata", nil, "Extra metadata key=value (repeatable)")
	cmd.Flags().StringVar(&flagWindow, "window", "15m", "How far back a prior send with the same key counts as a duplicate (e.g. 15m, 2h, 1d)")
	cmd.Flags().BoolVar(&send, "send", false, "Actually send (default prints the payload and dedupe decision)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Send ledger database path (default: the CLI's data.db)")
	return cmd
}

// sendOnceSavedStream returns the saved default stream for the selected server.
func sendOnceSavedStream(flags *rootFlags) string {
	name := postmarkSendDefaultsServer(flags)
	if name == "" {
		return ""
	}
	d, err := loadPostmarkDefaults()
	if err != nil {
		return ""
	}
	def, _ := d.forServer(name)
	return def.MessageStream
}

// sendOnceServerName scopes ledger rows to the selected server. The hook has
// already resolved an explicit --server into the token cache.
func sendOnceServerName(c *client.Client, flags *rootFlags) string {
	name := postmarkSendDefaultsServer(flags)
	if name == "" {
		return ""
	}
	if !postmarkSelection.sandbox && hasAccountToken(c) {
		if ref, err := resolvePostmarkServer(c, name); err == nil {
			return ref.Name
		}
	}
	return name
}

// sendOnceLedgerScope keys the send ledger by the server's stable ID, so every
// token for one server shares the same reservations. Sandbox sends never
// deliver and cannot look up a server, so they share one separate scope.
func sendOnceLedgerScope(ctx context.Context, c *client.Client, sandbox bool) (string, error) {
	if sandbox {
		return "sandbox", nil
	}
	target, err := currentPostmarkServer(ctx, c)
	if err != nil {
		return "", fmt.Errorf("identifying the server for the send ledger (nothing was sent): %w", classifyAPIErrorOnly(err))
	}
	if target.ID == 0 {
		return "", errors.New("identifying the server for the send ledger (nothing was sent): Postmark returned no server ID")
	}
	return fmt.Sprintf("server-id:%d", target.ID), nil
}

// sendOnceLedgerDuplicate fills res for a key already sent or reserved.
func sendOnceLedgerDuplicate(res *sendOnceResult, prior *store.PostmarkLedgerEntry, recipient string) {
	res.Duplicate, res.Source = true, sendOnceSourceLedger
	res.Dedupe.Postmark = "skipped: ledger hit"
	res.SubmittedAt = prior.SentAt.UTC().Format(time.RFC3339)
	if prior.Pending() {
		res.Dedupe.Ledger = "pending"
		res.DeliveryUnknown = true
		res.Next = "an earlier run reserved this key and never confirmed delivery; check with 'postmark-pp-cli messages list --recipient " + shellQuoteWord(recipient) + " --count 20 --offset 0" + postmarkServerArg(res.Server) + "' before sending again with a new --key"
		return
	}
	res.Dedupe.Ledger = "hit"
	res.MessageID = prior.MessageID
}

// sendOnceDefinitelyRefused reports whether Postmark rejected the request, so
// the message was not accepted and the reservation can be released. Timeouts,
// connection errors, and 5xx responses leave delivery unknown.
func sendOnceDefinitelyRefused(err error) bool {
	if isRateLimited(err) {
		return true
	}
	f, ok := postmarkAPIFailure(err)
	return ok && f.Status >= 400 && f.Status < 500
}

type sendOncePrior struct {
	MessageID  string
	ReceivedAt string
}

// sendOnceRemoteLookup searches outbound messages to recipient stamped with
// the key since windowStart. The metadata value is re-checked locally so an
// ignored filter can never produce a false duplicate.
func sendOnceRemoteLookup(ctx context.Context, c *client.Client, recipient, key, stream string, windowStart time.Time) (*sendOncePrior, error) {
	params := map[string]string{
		"recipient":                       recipient,
		"metadata_" + sendOnceMetadataKey: key,
		"fromdate":                        postmarkEasternTimestamp(windowStart),
		"count":                           "5",
		"offset":                          "0",
		"messagestream":                   stream,
	}
	raw, err := c.GetWithHeadersNoCache(ctx, "/messages/outbound", params, nil)
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	var doc struct {
		Messages []struct {
			MessageID  string            `json:"MessageID"`
			ReceivedAt string            `json:"ReceivedAt"`
			Recipients []string          `json:"Recipients"`
			Metadata   map[string]string `json:"Metadata"`
		} `json:"Messages"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing outbound search: %w", err)
	}
	for _, m := range doc.Messages {
		if m.Metadata[sendOnceMetadataKey] != key {
			continue
		}
		return &sendOncePrior{MessageID: m.MessageID, ReceivedAt: m.ReceivedAt}, nil
	}
	return nil, nil
}
