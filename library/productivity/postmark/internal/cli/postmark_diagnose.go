// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

const (
	verdictDelivered     = "delivered"
	verdictSent          = "sent"
	verdictBounced       = "bounced"
	verdictSuppressed    = "suppressed"
	verdictSpamComplaint = "spam_complaint"
	verdictQueued        = "queued"
	verdictNotFound      = "not_found"

	// Message event types and statuses from the message details API.
	messageEventBounced   = "Bounced"
	messageEventDelivered = "Delivered"
	messageEventTransient = "Transient"
	messageStatusQueued   = "Queued"

	diagnoseRecentMessages = 5
	diagnoseLocalHistory   = 20
)

type diagnoseEvent struct {
	Type   string `json:"type"`
	At     string `json:"at"`
	Detail string `json:"detail"`
}

type diagnoseMessage struct {
	MessageID  string          `json:"message_id"`
	Subject    string          `json:"subject"`
	Status     string          `json:"status"`
	Stream     string          `json:"stream"`
	Tag        string          `json:"tag"`
	ReceivedAt string          `json:"received_at"`
	Events     []diagnoseEvent `json:"events"`
	Source     string          `json:"source"`
	at         time.Time
}

type diagnoseBounce struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	MessageID   string `json:"message_id"`
	Stream      string `json:"stream"`
	BouncedAt   string `json:"bounced_at"`
	Inactive    bool   `json:"inactive"`
	CanActivate bool   `json:"can_activate"`
	Details     string `json:"details"`
	at          time.Time
}

type diagnoseSuppression struct {
	Stream    string `json:"stream"`
	Reason    string `json:"reason"`
	Origin    string `json:"origin"`
	CreatedAt string `json:"created_at"`
}

type diagnoseServer struct {
	Server       string                `json:"server"`
	ServerID     int64                 `json:"server_id"`
	Verdict      string                `json:"verdict"`
	Reason       string                `json:"reason"`
	MessageID    string                `json:"message_id,omitempty"`
	Next         string                `json:"next,omitempty"`
	Bounce       *diagnoseBounce       `json:"bounce,omitempty"`
	Suppression  *diagnoseSuppression  `json:"suppression,omitempty"`
	Messages     []diagnoseMessage     `json:"messages"`
	Bounces      []diagnoseBounce      `json:"bounces"`
	Suppressions []diagnoseSuppression `json:"suppressions"`
}

type diagnoseResult struct {
	Email         string                 `json:"email"`
	Since         string                 `json:"since"`
	Source        string                 `json:"source"`
	Verdict       string                 `json:"verdict"`
	Reason        string                 `json:"reason"`
	Server        string                 `json:"server,omitempty"`
	MessageID     string                 `json:"message_id,omitempty"`
	Next          string                 `json:"next,omitempty"`
	Bounce        *diagnoseBounce        `json:"bounce,omitempty"`
	Suppression   *diagnoseSuppression   `json:"suppression,omitempty"`
	Servers       []diagnoseServer       `json:"servers"`
	LocalHistory  []diagnoseMessage      `json:"local_history"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures"`
	Note          string                 `json:"note,omitempty"`
}

// findEvent returns the message's first event of eventType.
func findEvent(m diagnoseMessage, eventType string) (diagnoseEvent, bool) {
	for _, e := range m.Events {
		if strings.EqualFold(e.Type, eventType) {
			return e, true
		}
	}
	return diagnoseEvent{}, false
}

// diagnoseDecide sets the verdict for one server's evidence. Messages and
// bounces must be newest first. Order: spam complaint (never resend), the
// latest message bounced, the address is suppressed, the latest message was
// delivered, it is still queued, nothing found.
func diagnoseDecide(email string, d *diagnoseServer) {
	server := postmarkServerArg(d.Server)
	for i, s := range d.Suppressions {
		if isSpamComplaint(s.Reason) {
			d.Verdict, d.Suppression = verdictSpamComplaint, &d.Suppressions[i]
			d.Reason = fmt.Sprintf("%s marked a message as spam (stream %s, %s). Postmark will not deliver to this address on that stream and the suppression cannot be removed; do not resend.", email, s.Stream, s.CreatedAt)
			return
		}
	}
	for i, b := range d.Bounces {
		if isSpamComplaint(b.Type) {
			d.Verdict, d.Bounce = verdictSpamComplaint, &d.Bounces[i]
			d.Reason = fmt.Sprintf("%s filed a spam complaint at %s; do not resend.", email, b.BouncedAt)
			return
		}
	}
	var latest *diagnoseMessage
	if len(d.Messages) > 0 {
		latest = &d.Messages[0]
		d.MessageID = latest.MessageID
	}
	var bounce *diagnoseBounce
	if latest != nil {
		for i, b := range d.Bounces {
			if b.MessageID != "" && b.MessageID == latest.MessageID {
				bounce = &d.Bounces[i]
				break
			}
		}
	} else if len(d.Bounces) > 0 {
		bounce = &d.Bounces[0]
		d.MessageID = bounce.MessageID
	}
	bouncedEvent := false
	if latest != nil {
		_, bouncedEvent = findEvent(*latest, messageEventBounced)
	}
	if bounce != nil || bouncedEvent {
		d.Verdict, d.Bounce = verdictBounced, bounce
		if bounce == nil {
			d.Reason = fmt.Sprintf("the latest message (%s) bounced; the bounce record is not searchable yet", latest.MessageID)
			d.Next = "postmark-pp-cli bounces list" + server + " --email-filter " + shellQuoteWord(email) + " --json"
			return
		}
		d.Reason = fmt.Sprintf("%s bounce at %s (%s): %s", bounce.Type, bounce.BouncedAt, bounce.Name, bounce.Details)
		switch {
		case bounce.Inactive && bounce.CanActivate:
			d.Reason += ". Postmark deactivated the address; reactivate it once the mailbox is fixed."
			d.Next = fmt.Sprintf("postmark-pp-cli bounces activate %d%s", bounce.ID, server)
		case bounce.Inactive:
			d.Reason += ". Postmark deactivated the address and does not allow reactivating it."
		default:
			d.Reason += ". The address is still active, so the next send will be attempted."
			d.Next = fmt.Sprintf("postmark-pp-cli bounces get %d%s --json", bounce.ID, server)
		}
		return
	}
	if len(d.Suppressions) > 0 {
		pick := 0
		if latest != nil {
			for i, s := range d.Suppressions {
				if strings.EqualFold(s.Stream, latest.Stream) {
					pick = i
					break
				}
			}
		}
		s := &d.Suppressions[pick]
		d.Verdict, d.Suppression = verdictSuppressed, s
		d.Reason = fmt.Sprintf("%s is suppressed on stream %s (%s, origin %s, since %s); sends to it on that stream are rejected.", email, s.Stream, s.Reason, s.Origin, s.CreatedAt)
		if strings.EqualFold(s.Reason, postmarkHardBounce) {
			for _, b := range d.Bounces {
				if b.Inactive && b.CanActivate {
					d.Next = fmt.Sprintf("postmark-pp-cli bounces activate %d%s", b.ID, server)
					return
				}
			}
		}
		// Spam complaints returned above, so this always yields a command.
		d.Next, _ = suppressionNextCommand(s.Stream, email, s.Reason, d.Server)
		return
	}
	if latest == nil {
		d.Verdict = verdictNotFound
		d.Reason = fmt.Sprintf("no outbound messages, bounces, or suppressions for %s in this window", email)
		return
	}
	if ev, ok := findEvent(*latest, messageEventDelivered); ok {
		d.Verdict = verdictDelivered
		d.Reason = fmt.Sprintf("the latest message %q was delivered at %s", latest.Subject, ev.At)
		if ev.Detail != "" {
			d.Reason += " (" + ev.Detail + ")"
		}
		return
	}
	if latest.Source == postmarkSourceLocal && !strings.EqualFold(latest.Status, messageStatusQueued) {
		// Status Sent means Postmark accepted and handed off the message; with
		// no delivery event in the archive, delivery itself is unconfirmed.
		d.Verdict = verdictSent
		d.Reason = fmt.Sprintf("the latest archived message %q was accepted by Postmark (status %s) with no bounce recorded; the local archive holds no delivery events, so delivery is unconfirmed", latest.Subject, latest.Status)
		diagnoseLiveDetailsNext(d, *latest, server)
		return
	}
	d.Verdict = verdictQueued
	transient := 0
	for _, e := range latest.Events {
		if strings.EqualFold(e.Type, messageEventTransient) {
			transient++
		}
	}
	if transient > 0 {
		d.Reason = fmt.Sprintf("the latest message is still being retried after %d temporary delivery failures", transient)
	} else {
		d.Reason = fmt.Sprintf("the latest message has status %s and no delivery event yet", latest.Status)
	}
	diagnoseLiveDetailsNext(d, *latest, server)
}

// postmarkDefaultRetention is how long Postmark keeps message details unless
// the server's retention was changed.
const postmarkDefaultRetention = 45 * 24 * time.Hour

// diagnoseLiveDetailsNext suggests the live message lookup only while
// Postmark still retains m by default; for an older archived message it says
// why no live lookup is offered instead of suggesting one that would fail.
func diagnoseLiveDetailsNext(d *diagnoseServer, m diagnoseMessage, server string) {
	if m.Source == postmarkSourceLocal && !m.at.IsZero() && time.Since(m.at) > postmarkDefaultRetention {
		d.Reason += "; it is older than Postmark's default 45-day retention, so its delivery events can no longer be fetched"
		return
	}
	d.Next = "postmark-pp-cli messages get " + m.MessageID + server + " --json"
}

func diagnoseVerdictRank(v string) int {
	switch v {
	case verdictSpamComplaint:
		return 0
	case verdictBounced:
		return 1
	case verdictSuppressed:
		return 2
	case verdictQueued:
		return 3
	case verdictSent:
		return 4
	case verdictDelivered:
		return 5
	default:
		return 6
	}
}

// diagnosePrimary picks the server whose latest message is newest; with no
// messages anywhere it picks the most severe verdict.
func diagnosePrimary(servers []diagnoseServer) int {
	best := -1
	var bestAt time.Time
	for i, s := range servers {
		if len(s.Messages) == 0 {
			continue
		}
		if best < 0 || s.Messages[0].at.After(bestAt) {
			best, bestAt = i, s.Messages[0].at
		}
	}
	if best >= 0 {
		return best
	}
	best = 0
	for i, s := range servers {
		if diagnoseVerdictRank(s.Verdict) < diagnoseVerdictRank(servers[best].Verdict) {
			best = i
		}
	}
	return best
}

func newDiagnoseCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string
	var allServers bool
	cmd := &cobra.Command{
		Use:   "diagnose [email]",
		Short: "Explain whether one recipient got their email, and what to do next",
		Long: strings.Trim(`
Use this command for one recipient's delivery history and what to do next. Do NOT use it for delivery outcomes grouped by mailbox provider; use 'recipient-domains' instead. Do NOT use it to only check suppression status; use 'suppressions check' instead.

Checks recent outbound messages (with the newest message's delivery events),
bounces, and suppressions on every outbound stream of the server, plus the
local archive for messages older than Postmark's retention. The verdict is one
of delivered, sent (accepted by Postmark, delivery unconfirmed), bounced,
suppressed, spam_complaint, queued, or not_found, with a next command where
one helps. --data-source local reads only the archive, which mixes every
synced server and may hold suppressions lifted since the last sync. --all-servers repeats the check on every
server the account token lists.

Argument: the recipient address, as the first positional (diagnose <email>).`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli diagnose jane@example.com --agent
  postmark-pp-cli diagnose jane@example.com --server "Main App" --agent
  postmark-pp-cli diagnose jane@example.com --all-servers --since 45d --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "email=jane@example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diagnose")
			}
			if len(args) == 0 || !strings.Contains(args[0], "@") {
				_ = cmd.Usage()
				return usageErr(errors.New("a recipient email address is required: diagnose <email>"))
			}
			email := strings.ToLower(strings.TrimSpace(args[0]))
			sinceDur, err := parsePositiveDuration(flagSince, "since", "30d")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			since := time.Now().Add(-sinceDur)
			if dbPath == "" {
				dbPath = defaultDBPath("postmark-pp-cli")
			}
			res := diagnoseResult{
				Email:         email,
				Since:         postmarkEasternTimestamp(since),
				Source:        postmarkSourceLive,
				Servers:       make([]diagnoseServer, 0),
				LocalHistory:  make([]diagnoseMessage, 0),
				FetchFailures: make([]postmarkFetchFailure, 0),
			}
			local, localErr := diagnoseLoadLocal(ctx, cmd, dbPath, email, flags.dataSource != "live")
			if localErr != nil {
				return localErr
			}
			if flags.dataSource == "local" {
				res.Source = postmarkSourceLocal
				if local == nil {
					printNoLocalMirror(cmd.ErrOrStderr(), dbPath, "")
					local = &diagnoseServer{Messages: []diagnoseMessage{}, Bounces: []diagnoseBounce{}, Suppressions: []diagnoseSuppression{}}
				}
				diagnoseDecide(email, local)
				local.Server = "local archive"
				if local.Suppression != nil {
					// The archive keeps suppressions Postmark may have lifted since.
					local.Reason += " (from the local archive as of the last sync; it may have been lifted since)"
					local.Next = "postmark-pp-cli suppressions check " + shellQuoteWord(email) + " --json"
				}
				res.Servers = append(res.Servers, *local)
				diagnoseFinish(&res)
				if _, explicit := postmarkSelectedServer(); explicit {
					res.Note = strings.TrimSpace(res.Note + " the local archive mixes every synced server and archived messages carry no server ID, so --server does not narrow this answer")
				}
				return diagnoseOutput(cmd, flags, res)
			}
			targets, err := resolvePostmarkTargets(ctx, flags, targetScope{allServers: allServers, dogfoodCap: true})
			if err != nil {
				return err
			}
			servers, failures := fanoutTargets(ctx, targets, func(ctx context.Context, t postmarkTarget) (diagnoseServer, error) {
				return diagnoseLive(ctx, t, email, since)
			})
			res.FetchFailures = failures
			warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(targets), "server")
			if len(targets) == 0 {
				return apiErr(errors.New("no servers to check"))
			}
			if err := postmarkAllFailed(failures, len(targets)); err != nil {
				return err
			}
			for i := range servers {
				diagnoseDecide(email, &servers[i])
			}
			res.Servers = servers
			if local != nil {
				seen := map[string]bool{}
				for _, s := range servers {
					for _, m := range s.Messages {
						seen[m.MessageID] = true
					}
				}
				for _, m := range local.Messages {
					if !seen[m.MessageID] && len(res.LocalHistory) < diagnoseLocalHistory {
						res.LocalHistory = append(res.LocalHistory, m)
					}
				}
			}
			diagnoseFinish(&res)
			return diagnoseOutput(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "How far back to search live messages and bounces (Postmark keeps 45 days by default)")
	cmd.Flags().BoolVar(&allServers, "all-servers", false, "Check every server the account token lists")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local archive path for older history (default: the CLI's data.db)")
	return cmd
}

// diagnoseFinish copies the primary server's verdict to the top level.
func diagnoseFinish(res *diagnoseResult) {
	if len(res.Servers) == 0 {
		res.Verdict, res.Reason = verdictNotFound, "no servers checked"
		return
	}
	p := res.Servers[diagnosePrimary(res.Servers)]
	res.Verdict, res.Reason, res.Server, res.MessageID, res.Next = p.Verdict, p.Reason, p.Server, p.MessageID, p.Next
	res.Bounce, res.Suppression = p.Bounce, p.Suppression
	if res.Verdict == verdictNotFound && len(res.LocalHistory) > 0 {
		res.Note = fmt.Sprintf("the local archive holds %d older messages to this address; see local_history", len(res.LocalHistory))
	}
	if res.Verdict == verdictNotFound && len(res.Servers) == 1 && res.Source == postmarkSourceLive {
		res.Next = "postmark-pp-cli diagnose " + shellQuoteWord(res.Email) + " --all-servers --since 45d --json"
	}
}

// diagnoseLive gathers one server's evidence across its outbound streams.
func diagnoseLive(ctx context.Context, t postmarkTarget, email string, since time.Time) (diagnoseServer, error) {
	d := diagnoseServer{Server: t.Name, ServerID: t.ID, Messages: []diagnoseMessage{}, Bounces: []diagnoseBounce{}, Suppressions: []diagnoseSuppression{}}
	streams, err := listPostmarkStreams(ctx, t.client, true)
	if err != nil {
		return d, err
	}
	fromdate := postmarkEasternTimestamp(since)
	for _, s := range streams {
		var msgs struct {
			Messages []struct {
				MessageID  string   `json:"MessageID"`
				Subject    string   `json:"Subject"`
				Status     string   `json:"Status"`
				Tag        string   `json:"Tag"`
				Stream     string   `json:"MessageStream"`
				ReceivedAt string   `json:"ReceivedAt"`
				Recipients []string `json:"Recipients"`
			} `json:"Messages"`
		}
		if err := postmarkGetJSON(ctx, t.client, "/messages/outbound", map[string]string{"recipient": email, "messagestream": s.ID, "fromdate": fromdate, "count": "10", "offset": "0"}, &msgs); err != nil {
			return d, fmt.Errorf("searching messages on stream %s: %w", s.ID, err)
		}
		for _, m := range msgs.Messages {
			if !diagnoseAddrIn(email, m.Recipients) {
				continue
			}
			at, _ := postmarkParseTime(m.ReceivedAt)
			stream := m.Stream
			if stream == "" {
				stream = s.ID
			}
			d.Messages = append(d.Messages, diagnoseMessage{MessageID: m.MessageID, Subject: m.Subject, Status: m.Status, Stream: stream, Tag: m.Tag, ReceivedAt: m.ReceivedAt, Events: []diagnoseEvent{}, Source: postmarkSourceLive, at: at})
		}
		var bounces bounceListPage
		if err := postmarkGetJSON(ctx, t.client, "/bounces", map[string]string{"emailFilter": email, "messagestream": s.ID, "fromdate": fromdate, "count": "20", "offset": "0"}, &bounces); err != nil {
			return d, fmt.Errorf("searching bounces on stream %s: %w", s.ID, err)
		}
		for _, b := range bounces.Bounces {
			if sameAddr(b.Email, email) {
				d.Bounces = append(d.Bounces, diagnoseBounceFrom(b, s.ID))
			}
		}
		sups, err := dumpStreamSuppressions(ctx, t.client, s.ID, email)
		if err != nil {
			return d, fmt.Errorf("reading suppressions on stream %s: %w", s.ID, classifyAPIErrorOnly(err))
		}
		for _, sp := range sups {
			if sameAddr(sp.EmailAddress, email) {
				d.Suppressions = append(d.Suppressions, diagnoseSuppression{Stream: s.ID, Reason: sp.SuppressionReason, Origin: sp.Origin, CreatedAt: sp.CreatedAt})
			}
		}
	}
	sortDiagnoseEvidence(&d)
	if len(d.Messages) > diagnoseRecentMessages {
		d.Messages = d.Messages[:diagnoseRecentMessages]
	}
	if len(d.Messages) > 0 {
		var details struct {
			Status        string `json:"Status"`
			MessageEvents []struct {
				Recipient  string         `json:"Recipient"`
				Type       string         `json:"Type"`
				ReceivedAt string         `json:"ReceivedAt"`
				Details    map[string]any `json:"Details"`
			} `json:"MessageEvents"`
		}
		if err := postmarkGetJSON(ctx, t.client, "/messages/outbound/"+url.PathEscape(d.Messages[0].MessageID)+"/details", nil, &details); err != nil {
			return d, fmt.Errorf("reading message details: %w", err)
		}
		if details.Status != "" {
			d.Messages[0].Status = details.Status
		}
		for _, e := range details.MessageEvents {
			if e.Recipient != "" && !sameAddr(e.Recipient, email) {
				continue
			}
			detail := ""
			for _, k := range []string{"DeliveryMessage", "Summary", "BounceID"} {
				if v, ok := e.Details[k]; ok && fmt.Sprint(v) != "" {
					detail = fmt.Sprint(v)
					break
				}
			}
			d.Messages[0].Events = append(d.Messages[0].Events, diagnoseEvent{Type: e.Type, At: e.ReceivedAt, Detail: detail})
		}
	}
	return d, nil
}

// diagnoseBounceFrom converts a bounce record; stream is the fallback when
// the record carries no MessageStream.
func diagnoseBounceFrom(b bounceRecord, stream string) diagnoseBounce {
	if b.MessageStream != "" {
		stream = b.MessageStream
	}
	at, _ := postmarkParseTime(b.BouncedAt)
	return diagnoseBounce{ID: b.ID, Type: b.Type, Name: b.Name, MessageID: b.MessageID, Stream: stream, BouncedAt: b.BouncedAt, Inactive: b.Inactive, CanActivate: b.CanActivate, Details: b.Details, at: at}
}

func diagnoseAddrIn(email string, recipients []string) bool {
	for _, r := range recipients {
		if sameAddr(bareAddr(r), email) {
			return true
		}
	}
	return false
}

func sortDiagnoseEvidence(d *diagnoseServer) {
	sort.SliceStable(d.Messages, func(i, j int) bool { return d.Messages[i].at.After(d.Messages[j].at) })
	sort.SliceStable(d.Bounces, func(i, j int) bool { return d.Bounces[i].at.After(d.Bounces[j].at) })
}

// diagnoseLoadLocal reads the recipient's archived messages, bounces, and
// suppressions. It returns nil when the archive is absent or not wanted.
func diagnoseLoadLocal(ctx context.Context, cmd *cobra.Command, dbPath, email string, wanted bool) (*diagnoseServer, error) {
	if !wanted {
		return nil, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local archive: %w", err)
	}
	defer db.Close()
	hintIfUnsynced(cmd, db, "messages")
	d := &diagnoseServer{Messages: []diagnoseMessage{}, Bounces: []diagnoseBounce{}, Suppressions: []diagnoseSuppression{}}

	err = queryArchive(ctx, db, "archived messages", `SELECT data FROM resources
		WHERE resource_type = 'messages'
		  AND EXISTS (SELECT 1 FROM json_each(resources.data, '$.Recipients') r WHERE lower(r.value) = ?)`, 1, func(cols []string) error {
		var m struct {
			MessageID     string `json:"MessageID"`
			Subject       string `json:"Subject"`
			Status        string `json:"Status"`
			Tag           string `json:"Tag"`
			MessageStream string `json:"MessageStream"`
			ReceivedAt    string `json:"ReceivedAt"`
		}
		if json.Unmarshal([]byte(cols[0]), &m) != nil {
			return nil
		}
		at, _ := postmarkParseTime(m.ReceivedAt)
		d.Messages = append(d.Messages, diagnoseMessage{MessageID: m.MessageID, Subject: m.Subject, Status: m.Status, Tag: m.Tag, Stream: m.MessageStream, ReceivedAt: m.ReceivedAt, Events: []diagnoseEvent{}, Source: postmarkSourceLocal, at: at})
		return nil
	}, email)
	if err != nil {
		return nil, err
	}

	err = queryArchive(ctx, db, "archived bounces", `SELECT data FROM resources WHERE resource_type = 'bounces' AND lower(COALESCE(json_extract(data,'$.Email'),'')) = ?`, 1, func(cols []string) error {
		var b bounceRecord
		if json.Unmarshal([]byte(cols[0]), &b) == nil {
			d.Bounces = append(d.Bounces, diagnoseBounceFrom(b, ""))
		}
		return nil
	}, email)
	if err != nil {
		return nil, err
	}

	err = queryArchive(ctx, db, "archived suppressions", `SELECT data FROM resources WHERE resource_type = 'suppressions' AND lower(COALESCE(json_extract(data,'$.EmailAddress'),'')) = ?`, 1, func(cols []string) error {
		var s struct {
			SuppressionReason string `json:"SuppressionReason"`
			Origin            string `json:"Origin"`
			CreatedAt         string `json:"CreatedAt"`
			StreamsID         string `json:"streams_id"`
		}
		if json.Unmarshal([]byte(cols[0]), &s) == nil {
			d.Suppressions = append(d.Suppressions, diagnoseSuppression{Stream: s.StreamsID, Reason: s.SuppressionReason, Origin: s.Origin, CreatedAt: s.CreatedAt})
		}
		return nil
	}, email)
	if err != nil {
		return nil, err
	}
	sortDiagnoseEvidence(d)
	return d, nil
}

func diagnoseOutput(cmd *cobra.Command, flags *rootFlags, res diagnoseResult) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), res, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s: %s", res.Email, strings.ToUpper(res.Verdict))
	if res.Server != "" {
		fmt.Fprintf(out, " on %s", res.Server)
	}
	fmt.Fprintf(out, "\n  %s\n", res.Reason)
	if res.MessageID != "" {
		fmt.Fprintf(out, "  message: %s\n", res.MessageID)
	}
	if res.Next != "" {
		fmt.Fprintf(out, "  next: %s\n", res.Next)
	}
	if len(res.Servers) > 1 {
		fmt.Fprintln(out, "\nPer server:")
		for _, s := range res.Servers {
			fmt.Fprintf(out, "  %-20s %s\n", s.Server, s.Verdict)
		}
	}
	if res.Note != "" {
		fmt.Fprintf(out, "\n%s\n", res.Note)
	}
	if len(res.FetchFailures) > 0 {
		fmt.Fprintf(out, "\npartial results: %d server checks failed\n", len(res.FetchFailures))
	}
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newDiagnoseCmd(flags))
	})
}
