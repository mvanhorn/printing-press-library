// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

const (
	bouncePageSize = 500
	// Postmark rejects bounce searches where count + offset exceeds 10,000.
	bounceDeepPagingCap = 10000
)

// reactivateDecision is what bounces reactivate plans for one bounce.
type reactivateDecision string

const (
	decisionActivate reactivateDecision = "activate"
	decisionSkip     reactivateDecision = "skip"
)

var postmarkBounceTypes = []string{
	postmarkHardBounce, "Transient", "Unsubscribe", "Subscribe", "AutoResponder", "AddressChange", "DnsError",
	"SpamNotification", "OpenRelayTest", "Unknown", "SoftBounce", "VirusNotification", "ChallengeVerification",
	"BadEmailAddress", postmarkSpamComplaint, "ManuallyDeactivated", "Unconfirmed", "Blocked", "SMTPApiError",
	"InboundError", "DMARCPolicy", "TemplateRenderingFailed",
}

type bounceRecord struct {
	ID            int64  `json:"ID"`
	Type          string `json:"Type"`
	TypeCode      int64  `json:"TypeCode"`
	Name          string `json:"Name"`
	Details       string `json:"Details"`
	Email         string `json:"Email"`
	BouncedAt     string `json:"BouncedAt"`
	Inactive      bool   `json:"Inactive"`
	CanActivate   bool   `json:"CanActivate"`
	MessageID     string `json:"MessageID"`
	MessageStream string `json:"MessageStream"`
	Subject       string `json:"Subject"`
	From          string `json:"From"`
	Tag           string `json:"Tag"`
}

type bounceListPage struct {
	TotalCount int            `json:"TotalCount"`
	Bounces    []bounceRecord `json:"Bounces"`
}

type bounceScan struct {
	Bounces []bounceRecord
	Scanned int
	Total   int
	CapHit  bool
}

// resolveBounceType maps --type to the API filter. "any" or "all" disables
// the filter; other values must be a documented bounce type.
func resolveBounceType(v string) (string, error) {
	v = strings.TrimSpace(v)
	switch strings.ToLower(v) {
	case "", "any", "all":
		return "", nil
	}
	for _, t := range postmarkBounceTypes {
		if strings.EqualFold(t, v) {
			return t, nil
		}
	}
	return "", fmt.Errorf("invalid --type %q: use any, or one of %s", v, strings.Join(postmarkBounceTypes, ", "))
}

// sinceToEastern turns a --since duration into the Eastern fromdate value
// the bounce search expects.
func sinceToEastern(since string, now time.Time) (string, error) {
	d, err := parsePositiveDuration(since, "since", "7d, 4w, or 72h")
	if err != nil {
		return "", err
	}
	return postmarkEasternTimestamp(now.Add(-d)), nil
}

// matchesBounceFilters applies the filters Postmark cannot: an exact
// recipient domain and an exact address (emailFilter may match partially).
func matchesBounceFilters(b bounceRecord, domain, email string) bool {
	if domain != "" && addrDomain(b.Email) != strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "@")) {
		return false
	}
	if email != "" && !sameAddr(b.Email, email) {
		return false
	}
	return true
}

// reactivationBlocker explains why a bounce cannot be reactivated, or
// returns "" when it can.
func reactivationBlocker(b bounceRecord) string {
	switch {
	case isSpamComplaint(b.Type):
		return "spam complaints cannot be reactivated; the recipient marked your mail as spam"
	case !b.Inactive:
		return "address is already active"
	case !b.CanActivate:
		return "Postmark reports CanActivate=false for this bounce"
	}
	return ""
}

// scanPostmarkBounces pages GET /bounces with the given filters, stopping at
// maxPages, the 10,000 count+offset cap, or TotalCount.
func scanPostmarkBounces(ctx context.Context, c *client.Client, filters map[string]string, maxPages int) (bounceScan, error) {
	scan := bounceScan{Bounces: make([]bounceRecord, 0)}
	pages := min(maxPages, bounceDeepPagingCap/bouncePageSize)
	if pages < 1 {
		return scan, nil
	}
	paging := postmarkPaging{pageSize: bouncePageSize, maxPages: pages, untilEmpty: true}
	exhausted, err := walkPostmarkPagesWith(paging, func(offset int) ([]bounceRecord, int, error) {
		params := map[string]string{"count": strconv.Itoa(bouncePageSize), "offset": strconv.Itoa(offset)}
		for k, v := range filters {
			if v != "" {
				params[k] = v
			}
		}
		raw, err := c.Get(ctx, "/bounces", params)
		if err != nil {
			return nil, 0, err
		}
		var p bounceListPage
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, 0, fmt.Errorf("parsing bounces: %w", err)
		}
		scan.Total = p.TotalCount
		return p.Bounces, p.TotalCount, nil
	}, func(b bounceRecord) bool {
		scan.Scanned++
		scan.Bounces = append(scan.Bounces, b)
		return false
	})
	if err != nil {
		return scan, err
	}
	scan.CapHit = exhausted && scan.Scanned < scan.Total
	return scan, nil
}

// ---------------------------------------------------------------------------
// bounces reactivate
// ---------------------------------------------------------------------------

type bounceReactivateItem struct {
	ID            int64              `json:"id"`
	Email         string             `json:"email"`
	Domain        string             `json:"domain"`
	Type          string             `json:"type"`
	BouncedAt     string             `json:"bounced_at"`
	MessageStream string             `json:"message_stream"`
	CanActivate   bool               `json:"can_activate"`
	Decision      reactivateDecision `json:"decision"`
	Status        itemStatus         `json:"status"`
	Reason        string             `json:"reason,omitempty"`
	Error         string             `json:"error,omitempty"`
}

type bounceReactivateFilters struct {
	Type     string `json:"type"`
	Domain   string `json:"domain,omitempty"`
	Email    string `json:"email,omitempty"`
	Since    string `json:"since"`
	FromDate string `json:"fromdate"`
	Limit    int    `json:"limit"`
	Stream   string `json:"stream"`
}

type bounceReactivateView struct {
	Mode           runMode                 `json:"mode"`
	Filters        bounceReactivateFilters `json:"filters"`
	ScannedBounces int                     `json:"scanned_bounces"`
	MaxScanPages   int                     `json:"max_scan_pages"`
	Planned        int                     `json:"planned"`
	Activated      int                     `json:"activated"`
	Failed         int                     `json:"failed"`
	Writes         int                     `json:"writes"`
	OverLimit      int                     `json:"over_limit"`
	Items          []bounceReactivateItem  `json:"items"`
	Skipped        []bounceReactivateItem  `json:"skipped"`
	Note           string                  `json:"note,omitempty"`
}

func newBounceReactivateItem(b bounceRecord) bounceReactivateItem {
	return bounceReactivateItem{
		ID:            b.ID,
		Email:         b.Email,
		Domain:        addrDomain(b.Email),
		Type:          b.Type,
		BouncedAt:     b.BouncedAt,
		MessageStream: b.MessageStream,
		CanActivate:   b.CanActivate,
	}
}

// planBounceReactivation filters scanned bounces and splits them into
// activations (up to limit) and skips with a reason.
func planBounceReactivation(bounces []bounceRecord, domain, email string, limit int) (items, skipped []bounceReactivateItem, overLimit int) {
	items = make([]bounceReactivateItem, 0)
	skipped = make([]bounceReactivateItem, 0)
	for _, b := range bounces {
		if !matchesBounceFilters(b, domain, email) {
			continue
		}
		item := newBounceReactivateItem(b)
		if reason := reactivationBlocker(b); reason != "" {
			item.Decision, item.Status, item.Reason = decisionSkip, statusSkipped, reason
			skipped = append(skipped, item)
			continue
		}
		if limit > 0 && len(items) >= limit {
			overLimit++
			continue
		}
		item.Decision, item.Status = decisionActivate, statusPlanned
		items = append(items, item)
	}
	return items, skipped, overLimit
}

func activatePostmarkBounce(ctx context.Context, c *client.Client, id int64) error {
	_, _, err := c.Put(ctx, "/bounces/"+strconv.FormatInt(id, 10)+"/activate", map[string]any{})
	return err
}

func newBouncesReactivateCmd(flags *rootFlags) *cobra.Command {
	var typeFlag, domain, email, since, stream string
	var limit, maxScanPages int
	cmd := &cobra.Command{
		Use:   "reactivate",
		Short: "Reactivate inactive bounced addresses in bulk, filtered by type, recipient domain, and date; plans by default and applies with --yes",
		Long: strings.Trim(`
List inactive bounces (default type HardBounce) from the last --since window,
narrow them to one recipient domain or address, and plan a reactivation for
every bounce Postmark allows (CanActivate=true). Spam complaints and bounces
that cannot be activated are listed under "skipped" with the reason.

Prints the plan by default and changes nothing. Pass --yes to run
PUT /bounces/{id}/activate for each planned bounce; results are reported per
bounce. --csv prints one row per bounce. --max-scan-pages bounds how many
500-bounce pages are examined; --limit bounds how many bounces are activated.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli bounces reactivate --domain example.com --since 30d --server Staging
  postmark-pp-cli bounces reactivate --type HardBounce --since 90d --server Staging --json
  postmark-pp-cli bounces reactivate --email jane@example.com --server "Main App" --yes`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "bounces reactivate")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			bounceType, err := resolveBounceType(typeFlag)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			fromDate, err := sinceToEastern(since, time.Now())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if limit < 1 || maxScanPages < 1 {
				_ = cmd.Usage()
				return usageErr(errors.New("--limit and --max-scan-pages must be at least 1"))
			}
			apply := flags.yes
			if apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "reactivate bounces")
			}
			if cliutil.IsDogfoodEnv() {
				maxScanPages = 1
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			scan, err := scanPostmarkBounces(ctx, c, map[string]string{
				"inactive":      "true",
				"type":          bounceType,
				"emailFilter":   strings.TrimSpace(email),
				"fromdate":      fromDate,
				"messagestream": strings.TrimSpace(stream),
			}, maxScanPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			items, skipped, overLimit := planBounceReactivation(scan.Bounces, domain, email, limit)
			typeLabel := bounceType
			if typeLabel == "" {
				typeLabel = "any"
			}
			view := bounceReactivateView{
				Mode: modePlan,
				Filters: bounceReactivateFilters{Type: typeLabel, Domain: domain, Email: email, Since: since,
					FromDate: fromDate, Limit: limit, Stream: postmarkBounceStreamLabel(stream)},
				ScannedBounces: scan.Scanned,
				MaxScanPages:   maxScanPages,
				Planned:        len(items),
				OverLimit:      overLimit,
				Items:          items,
				Skipped:        skipped,
			}
			if overLimit > 0 {
				view.Note = fmt.Sprintf("%d more activatable bounce(s) beyond --limit %d", overLimit, limit)
			}
			if scan.CapHit {
				view.Note = strings.TrimSpace(view.Note + fmt.Sprintf(" scanned %d of %d inactive bounces; raise --max-scan-pages or narrow --since to see the rest", scan.Scanned, scan.Total))
			}
			if apply {
				view.Mode = modeApply
				for i := range view.Items {
					it := &view.Items[i]
					if err := activatePostmarkBounce(ctx, c, it.ID); err != nil {
						it.Status, it.Error = statusFailed, postmarkErrorText(err)
						view.Failed++
						if isRateLimited(err) {
							for j := i + 1; j < len(view.Items); j++ {
								view.Items[j].Status = statusSkipped
								view.Items[j].Reason = "not attempted after a rate limit"
							}
							if perr := printBounceReactivateView(cmd, flags, view); perr != nil {
								return perr
							}
							return rateLimitErr(err)
						}
						continue
					}
					it.Status = statusActivated
					view.Activated++
					view.Writes++
				}
			} else if len(items) > 0 {
				view.Note = strings.TrimSpace(view.Note + " plan only: re-run with --yes to reactivate")
			}
			if err := printBounceReactivateView(cmd, flags, view); err != nil {
				return err
			}
			if view.Failed > 0 {
				return apiErr(fmt.Errorf("%d of %d bounce activations failed", view.Failed, len(view.Items)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFlag, "type", postmarkHardBounce, "Bounce type to reactivate, or 'any' for every type")
	cmd.Flags().StringVar(&domain, "domain", "", "Only bounces whose recipient address is at this domain (exact match)")
	cmd.Flags().StringVar(&email, "email", "", "Only bounces for this recipient address")
	cmd.Flags().StringVar(&since, "since", "30d", "Look back this far for bounces (e.g. 7d, 4w, 72h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of bounces to reactivate")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum 500-bounce pages to scan before planning")
	cmd.Flags().StringVar(&stream, "stream", "", "Message stream to search; Postmark searches only the outbound stream when omitted")
	return cmd
}

func printBounceReactivateView(cmd *cobra.Command, flags *rootFlags, view bounceReactivateView) error {
	w := cmd.OutOrStdout()
	if flags.csv {
		rows := make([]bounceReactivateItem, 0, len(view.Items)+len(view.Skipped))
		rows = append(rows, view.Items...)
		rows = append(rows, view.Skipped...)
		return printJSONFiltered(w, rows, flags)
	}
	if !wantsHumanTable(w, flags) {
		return printJSONFilteredKeep(w, view, flags, "reason", "error")
	}
	fmt.Fprintf(w, "Bounce reactivation %s: %d planned, %d skipped, %d activated, %d failed (scanned %d)\n",
		view.Mode, view.Planned, len(view.Skipped), view.Activated, view.Failed, view.ScannedBounces)
	rows := append(append([]bounceReactivateItem{}, view.Items...), view.Skipped...)
	if len(rows) > 0 {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "STATUS\tID\tEMAIL\tTYPE\tBOUNCED AT\tDETAIL")
		for _, r := range rows {
			detail := r.Reason
			if r.Error != "" {
				detail = r.Error
			}
			fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", r.Status, r.ID, r.Email, r.Type, r.BouncedAt, detail)
		}
		_ = tw.Flush()
	} else {
		fmt.Fprintln(w, "No inactive bounces match these filters.")
	}
	if view.Note != "" {
		fmt.Fprintln(w, view.Note)
	}
	return nil
}

// ---------------------------------------------------------------------------
// bounces resend-blocked
// ---------------------------------------------------------------------------

type outboundMessageDetails struct {
	MessageID     string            `json:"MessageID"`
	From          string            `json:"From"`
	Subject       string            `json:"Subject"`
	HtmlBody      string            `json:"HtmlBody"`
	TextBody      string            `json:"TextBody"`
	Tag           string            `json:"Tag"`
	MessageStream string            `json:"MessageStream"`
	Metadata      map[string]any    `json:"Metadata"`
	TrackOpens    *bool             `json:"TrackOpens"`
	TrackLinks    string            `json:"TrackLinks"`
	Attachments   []json.RawMessage `json:"Attachments"`
}

// resendPayload is the POST /email body: the original content addressed to
// the bounced recipient only.
type resendPayload struct {
	From          string         `json:"From"`
	To            string         `json:"To"`
	Subject       string         `json:"Subject"`
	HtmlBody      string         `json:"HtmlBody,omitempty"`
	TextBody      string         `json:"TextBody,omitempty"`
	Tag           string         `json:"Tag,omitempty"`
	MessageStream string         `json:"MessageStream,omitempty"`
	Metadata      map[string]any `json:"Metadata,omitempty"`
	TrackOpens    *bool          `json:"TrackOpens,omitempty"`
	TrackLinks    string         `json:"TrackLinks,omitempty"`
}

type resendBlockedItem struct {
	BounceID         int64      `json:"bounce_id"`
	Email            string     `json:"email"`
	BouncedAt        string     `json:"bounced_at"`
	MessageID        string     `json:"message_id"`
	Subject          string     `json:"subject,omitempty"`
	From             string     `json:"from,omitempty"`
	MessageStream    string     `json:"message_stream,omitempty"`
	Tag              string     `json:"tag,omitempty"`
	Inactive         bool       `json:"inactive"`
	Resendable       bool       `json:"resendable"`
	Reason           string     `json:"reason,omitempty"`
	Steps            []string   `json:"steps"`
	ActivationStatus itemStatus `json:"activation_status,omitempty"`
	SendStatus       itemStatus `json:"send_status,omitempty"`
	NewMessageID     string     `json:"new_message_id,omitempty"`
	Error            string     `json:"error,omitempty"`
	payload          *resendPayload
}

type resendBlockedView struct {
	Mode           runMode                `json:"mode"`
	Since          string                 `json:"since"`
	FromDate       string                 `json:"fromdate"`
	Email          string                 `json:"email,omitempty"`
	Stream         string                 `json:"stream"`
	ScannedBounces int                    `json:"scanned_bounces"`
	TotalBounces   int                    `json:"total_bounces"`
	Truncated      bool                   `json:"truncated"`
	Resendable     int                    `json:"resendable"`
	NotResendable  int                    `json:"not_resendable"`
	Sent           int                    `json:"sent"`
	Failed         int                    `json:"failed"`
	Items          []resendBlockedItem    `json:"items"`
	FetchFailures  []postmarkFetchFailure `json:"fetch_failures,omitempty"`
	Note           string                 `json:"note,omitempty"`
}

func resendPayloadFor(d outboundMessageDetails, recipient string) *resendPayload {
	return &resendPayload{
		From:          d.From,
		To:            recipient,
		Subject:       d.Subject,
		HtmlBody:      d.HtmlBody,
		TextBody:      d.TextBody,
		Tag:           d.Tag,
		MessageStream: d.MessageStream,
		Metadata:      d.Metadata,
		TrackOpens:    d.TrackOpens,
		TrackLinks:    d.TrackLinks,
	}
}

// resendBlocker explains why a retrieved original cannot be resent as-is.
func resendBlocker(d outboundMessageDetails) string {
	switch {
	case len(d.Attachments) > 0:
		return fmt.Sprintf("original had %d attachment(s); the message details API does not return attachment content", len(d.Attachments))
	case d.HtmlBody == "" && d.TextBody == "":
		return "original body was not retained by Postmark"
	case strings.TrimSpace(d.From) == "":
		return "original From address was not returned"
	}
	return ""
}

// planResendSteps fills the activation and send steps. activatedBy tracks
// which bounce reactivates each address so one address is reactivated once.
func planResendSteps(item *resendBlockedItem, activatedBy map[string]int64) {
	key := strings.ToLower(item.Email)
	switch {
	case !item.Inactive:
		item.ActivationStatus = resendActivationNotNeeded
	case activatedBy[key] != 0:
		item.ActivationStatus = resendActivationShared
		item.Steps = append(item.Steps, fmt.Sprintf("address reactivated by bounce %d", activatedBy[key]))
	default:
		item.ActivationStatus = statusPlanned
		activatedBy[key] = item.BounceID
		item.Steps = append(item.Steps, fmt.Sprintf("PUT /bounces/%d/activate", item.BounceID))
	}
	stream := item.MessageStream
	if stream == "" {
		stream = postmarkDefaultStream
	}
	item.Steps = append(item.Steps, fmt.Sprintf("POST /email: resend %q from %s to %s on stream %s", item.Subject, item.From, item.Email, stream))
	item.SendStatus = statusPlanned
}

func getOutboundMessageDetails(ctx context.Context, c *client.Client, id string) (outboundMessageDetails, error) {
	raw, err := c.Get(ctx, "/messages/outbound/"+url.PathEscape(id)+"/details", nil)
	if err != nil {
		return outboundMessageDetails{}, err
	}
	var d outboundMessageDetails
	if err := json.Unmarshal(raw, &d); err != nil {
		return outboundMessageDetails{}, fmt.Errorf("parsing message %s: %w", id, err)
	}
	return d, nil
}

type postmarkSendResponse struct {
	ErrorCode int    `json:"ErrorCode"`
	Message   string `json:"Message"`
	MessageID string `json:"MessageID"`
}

func newBouncesResendBlockedCmd(flags *rootFlags) *cobra.Command {
	var since, email, stream string
	var send bool
	var limit, maxScanPages int
	cmd := &cobra.Command{
		Use:   "resend-blocked",
		Short: "Resend messages whose recipient hard-bounced: reactivate, then resend the original to that address only; plans by default and sends with --send",
		Long: strings.Trim(`
Find hard bounces in the --since window whose original message Postmark still
retains, and plan two steps for each: reactivate the bounced address, then
resend the original From, Subject, HtmlBody, TextBody, Tag, MessageStream, and
Metadata to the bounced recipient only (never to other recipients of the
original). Messages past the retention window, or that had attachments the API
cannot return, are listed as not resendable with the reason.

Prints the plan by default and sends nothing. --send reactivates and resends;
sends are never retried automatically.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli bounces resend-blocked --since 7d --server "Main App"
  postmark-pp-cli bounces resend-blocked --email jane@example.com --since 14d --server "Main App" --json
  postmark-pp-cli bounces resend-blocked --email jane@example.com --server "Main App" --send`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "bounces resend-blocked")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			fromDate, err := sinceToEastern(since, time.Now())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if limit < 1 || maxScanPages < 1 {
				_ = cmd.Usage()
				return usageErr(errors.New("--limit and --max-scan-pages must be at least 1"))
			}
			if send && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "reactivate and resend bounced messages")
			}
			if cliutil.IsDogfoodEnv() {
				maxScanPages = 1
				if limit > 5 {
					limit = 5
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newUncachedClient(flags)
			if err != nil {
				return err
			}
			scan, err := scanPostmarkBounces(ctx, c, map[string]string{
				"type":          postmarkHardBounce,
				"inactive":      "true",
				"emailFilter":   strings.TrimSpace(email),
				"fromdate":      fromDate,
				"messagestream": strings.TrimSpace(stream),
			}, maxScanPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			view := resendBlockedView{
				Mode: modePlan, Since: since, FromDate: fromDate, Email: email, Stream: postmarkBounceStreamLabel(stream),
				ScannedBounces: scan.Scanned, TotalBounces: scan.Total, Truncated: scan.CapHit, Items: make([]resendBlockedItem, 0),
			}
			if scan.CapHit {
				view.Note = fmt.Sprintf("incomplete plan: scanned %d of %d hard bounces; raise --max-scan-pages or narrow --since to see the rest", scan.Scanned, scan.Total)
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+view.Note)
			}
			activatedBy := map[string]int64{}
			considered, lookups := 0, 0
			for _, b := range scan.Bounces {
				if !matchesBounceFilters(b, "", email) {
					continue
				}
				if considered >= limit {
					view.Note = strings.TrimSpace(view.Note + fmt.Sprintf(" stopped at --limit %d hard bounces", limit))
					break
				}
				considered++
				item := resendBlockedItem{
					BounceID: b.ID, Email: b.Email, BouncedAt: b.BouncedAt, MessageID: b.MessageID,
					Subject: b.Subject, From: b.From, MessageStream: b.MessageStream, Tag: b.Tag,
					Inactive: b.Inactive, Steps: make([]string, 0),
				}
				switch {
				case b.MessageID == "":
					item.Reason = "bounce has no MessageID to look up"
				case b.Inactive && !b.CanActivate:
					item.Reason = "address cannot be reactivated (CanActivate=false)"
				default:
					lookups++
					d, err := getOutboundMessageDetails(ctx, c, b.MessageID)
					if err != nil {
						if isRateLimited(err) {
							return rateLimitErr(err)
						}
						if f, ok := postmarkAPIFailure(err); ok && (f.ErrorCode == postmarkErrMessageNotFound || f.Status == 404) {
							item.Reason = "original message is past Postmark's retention window and can no longer be retrieved"
							break
						}
						item.Reason = "could not fetch the original message: " + postmarkErrorText(err)
						view.FetchFailures = append(view.FetchFailures, itemFailure(b.MessageID, err))
						break
					}
					if reason := resendBlocker(d); reason != "" {
						item.Reason = reason
						break
					}
					item.Subject, item.From, item.Tag = d.Subject, d.From, d.Tag
					if d.MessageStream != "" {
						item.MessageStream = d.MessageStream
					}
					item.payload = resendPayloadFor(d, b.Email)
					item.Resendable = true
					planResendSteps(&item, activatedBy)
				}
				view.Items = append(view.Items, item)
			}
			for _, it := range view.Items {
				if it.Resendable {
					view.Resendable++
				} else {
					view.NotResendable++
				}
			}
			warnPartialFailures(cmd.ErrOrStderr(), len(view.FetchFailures), considered, "original messages could not be fetched", "")
			if send {
				view.Mode = modeSend
				executeResendPlan(ctx, c, &view)
			} else if view.Resendable > 0 {
				view.Note = strings.TrimSpace(view.Note + " plan only: re-run with --send to reactivate and resend")
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "reason", "error", "steps", "new_message_id"); err != nil {
					return err
				}
			} else {
				printResendBlockedHuman(cmd.OutOrStdout(), view)
			}
			if view.Failed > 0 {
				return apiErr(fmt.Errorf("%d of %d resends failed", view.Failed, view.Resendable))
			}
			if n := len(view.FetchFailures); n > 0 && n == lookups {
				return apiErr(fmt.Errorf("every original message lookup failed (%d of %d), so nothing could be resent: %s", n, lookups, view.FetchFailures[0].Error))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Look back this far for hard bounces (e.g. 7d, 2w, 48h)")
	cmd.Flags().StringVar(&stream, "stream", "", "Message stream to search; Postmark searches only the outbound stream when omitted")
	cmd.Flags().StringVar(&email, "email", "", "Only hard bounces for this recipient address")
	cmd.Flags().BoolVar(&send, "send", false, "Reactivate each address and resend the original message (default: print the plan)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of hard bounces to plan")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum 500-bounce pages to scan")
	return cmd
}

// executeResendPlan reactivates then resends each resendable item. An item
// whose address failed to reactivate is not sent; sends are never retried.
func executeResendPlan(ctx context.Context, c *client.Client, view *resendBlockedView) {
	failedActivation := map[string]bool{}
	for i := range view.Items {
		it := &view.Items[i]
		if !it.Resendable {
			continue
		}
		key := strings.ToLower(it.Email)
		switch it.ActivationStatus {
		case statusPlanned:
			if err := activatePostmarkBounce(ctx, c, it.BounceID); err != nil {
				it.ActivationStatus, it.SendStatus, it.Error = statusFailed, statusSkipped, "reactivation failed: "+postmarkErrorText(err)
				failedActivation[key] = true
				view.Failed++
				continue
			}
			it.ActivationStatus = statusActivated
		case resendActivationShared:
			if failedActivation[key] {
				it.SendStatus, it.Error = statusSkipped, "address reactivation failed on an earlier bounce"
				view.Failed++
				continue
			}
		}
		raw, err := withSendAllowed(func() (json.RawMessage, error) {
			raw, _, err := c.Post(ctx, "/email", it.payload)
			return raw, err
		})
		if err != nil {
			it.SendStatus, it.Error = statusFailed, postmarkErrorText(err)
			view.Failed++
			continue
		}
		var resp postmarkSendResponse
		if jerr := json.Unmarshal(raw, &resp); jerr == nil && resp.ErrorCode != 0 {
			it.SendStatus, it.Error = statusFailed, fmt.Sprintf("ErrorCode %d: %s", resp.ErrorCode, resp.Message)
			view.Failed++
			continue
		}
		it.SendStatus, it.NewMessageID = resendSendSent, resp.MessageID
		view.Sent++
	}
}

func printResendBlockedHuman(w io.Writer, v resendBlockedView) {
	fmt.Fprintf(w, "Resend blocked messages (%s, since %s): %d resendable, %d not resendable, %d sent, %d failed (scanned %d hard bounces)\n",
		v.Mode, v.Since, v.Resendable, v.NotResendable, v.Sent, v.Failed, v.ScannedBounces)
	if len(v.Items) == 0 {
		fmt.Fprintln(w, "No hard bounces in this window.")
	}
	for _, it := range v.Items {
		fmt.Fprintf(w, "\nbounce %d  %s  %s  message %s\n", it.BounceID, it.Email, it.BouncedAt, it.MessageID)
		if !it.Resendable {
			fmt.Fprintf(w, "  not resendable: %s\n", it.Reason)
			continue
		}
		for _, s := range it.Steps {
			fmt.Fprintf(w, "  - %s\n", s)
		}
		if v.Mode == modeSend {
			fmt.Fprintf(w, "  activation: %s, send: %s %s\n", it.ActivationStatus, it.SendStatus, it.NewMessageID)
		}
		if it.Error != "" {
			fmt.Fprintf(w, "  error: %s\n", it.Error)
		}
	}
	if v.Note != "" {
		fmt.Fprintln(w, v.Note)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"bounces"})
		if err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newBouncesReactivateCmd(flags))
			addNovelCommandIfAbsent(parent, newBouncesResendBlockedCmd(flags))
		}
	})
}

// postmarkBounceStreamLabel names the stream a bounce search covered; Postmark
// searches only the default transactional stream when none is given.
func postmarkBounceStreamLabel(stream string) string {
	if s := strings.TrimSpace(stream); s != "" {
		return s
	}
	return postmarkDefaultStream
}
