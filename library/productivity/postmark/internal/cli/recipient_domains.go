// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

const (
	recipientDomainFlagHighBounce  = "high-bounce-rate"
	recipientDomainFlagLowOpen     = "low-open-rate"
	recipientDomainUnknown         = "(unknown)"
	recipientDomainBounceRateRatio = 2.0
	recipientDomainOpenRateRatio   = 0.5
	// recipientDomainHardBounceFloorPct flags a domain on its own
	// hard-bounce rate even when the rest of the account bounces just as much.
	recipientDomainHardBounceFloorPct = 5.0
)

type recipientDomainRow struct {
	Domain               string         `json:"domain"`
	Sent                 int            `json:"sent"`
	Bounced              int            `json:"bounced"`
	BouncesByType        map[string]int `json:"bounces_by_type"`
	HardBounces          int            `json:"hard_bounces"`
	BounceRatePct        float64        `json:"bounce_rate_pct"`
	HardBounceRatePct    float64        `json:"hard_bounce_rate_pct"`
	Complaints           int            `json:"complaints"`
	Suppressed           int            `json:"suppressed"`
	TrackedSent          int            `json:"tracked_sent"`
	UniqueOpens          int            `json:"unique_opens"`
	OpenRatePct          *float64       `json:"open_rate_pct"`
	AccountBounceRatePct float64        `json:"account_bounce_rate_pct"`
	AccountOpenRatePct   *float64       `json:"account_open_rate_pct"`
	RestBounceRatePct    *float64       `json:"rest_of_account_bounce_rate_pct"`
	RestOpenRatePct      *float64       `json:"rest_of_account_open_rate_pct"`
	Flagged              bool           `json:"flagged"`
	Flags                []string       `json:"flags"`
}

type recipientDomainMessage struct {
	MessageID  string
	Recipients []string
	ReceivedAt time.Time
	TrackOpens bool
}

type recipientDomainBounce struct {
	Email     string
	Type      string
	BouncedAt time.Time
}

type recipientDomainOpen struct {
	Recipient  string
	MessageID  string
	ReceivedAt time.Time
}

type recipientDomainSuppression struct {
	EmailAddress string
	Reason       string
}

// recipientDomainOf returns the lowercased mailbox domain of addr, which may
// carry a display name, grouping addresses without one under
// recipientDomainUnknown.
func recipientDomainOf(addr string) string {
	if d := addrDomain(bareAddr(addr)); d != "" {
		return d
	}
	return recipientDomainUnknown
}

// recipientDomainsCompute groups window activity by recipient domain and
// flags domains that depart from the rest of the account.
func recipientDomainsCompute(msgs []recipientDomainMessage, bounces []recipientDomainBounce, opens []recipientDomainOpen, sups []recipientDomainSuppression, since time.Time, minSent int) []recipientDomainRow {
	rows := map[string]*recipientDomainRow{}
	get := func(domain string) *recipientDomainRow {
		r, ok := rows[domain]
		if !ok {
			r = &recipientDomainRow{Domain: domain, BouncesByType: map[string]int{}, Flags: make([]string, 0)}
			rows[domain] = r
		}
		return r
	}
	// trackedSends are the (message, recipient) pairs in the window with open
	// tracking on; only their opens count, so the open rate's numerator and
	// denominator cover the same messages.
	trackedSends := map[string]bool{}
	for _, m := range msgs {
		if m.ReceivedAt.Before(since) {
			continue
		}
		seen := map[string]bool{}
		for _, rcpt := range m.Recipients {
			key := strings.ToLower(strings.TrimSpace(rcpt))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			r := get(recipientDomainOf(rcpt))
			r.Sent++
			if m.TrackOpens {
				r.TrackedSent++
				trackedSends[m.MessageID+"\x00"+strings.ToLower(bareAddr(rcpt))] = true
			}
		}
	}
	for _, b := range bounces {
		if b.BouncedAt.Before(since) {
			continue
		}
		r := get(recipientDomainOf(b.Email))
		if isSpamComplaint(b.Type) {
			r.Complaints++
			continue
		}
		r.Bounced++
		r.BouncesByType[b.Type]++
		if strings.EqualFold(b.Type, postmarkHardBounce) {
			r.HardBounces++
		}
	}
	openSeen := map[string]bool{}
	for _, o := range opens {
		key := o.MessageID + "\x00" + strings.ToLower(bareAddr(o.Recipient))
		if !trackedSends[key] || openSeen[key] {
			continue
		}
		openSeen[key] = true
		get(recipientDomainOf(o.Recipient)).UniqueOpens++
	}
	suppressed := map[string]map[string]bool{}
	for _, s := range sups {
		domain := recipientDomainOf(s.EmailAddress)
		if _, active := rows[domain]; !active {
			continue
		}
		if suppressed[domain] == nil {
			suppressed[domain] = map[string]bool{}
		}
		suppressed[domain][strings.ToLower(strings.TrimSpace(s.EmailAddress))] = true
	}

	var totalSent, totalBounced, totalTracked, totalOpens int
	for domain, r := range rows {
		r.Suppressed = len(suppressed[domain])
		totalSent += r.Sent
		totalBounced += r.Bounced
		totalTracked += r.TrackedSent
		totalOpens += r.UniqueOpens
	}
	accountBounce := ratePct(totalBounced, totalSent)
	var accountOpen *float64
	if totalTracked > 0 {
		v := ratePct(totalOpens, totalTracked)
		accountOpen = &v
	}
	out := make([]recipientDomainRow, 0, len(rows))
	for _, r := range rows {
		r.BounceRatePct = ratePct(r.Bounced, r.Sent)
		r.HardBounceRatePct = ratePct(r.HardBounces, r.Sent)
		if r.TrackedSent > 0 {
			v := ratePct(r.UniqueOpens, r.TrackedSent)
			r.OpenRatePct = &v
		}
		r.AccountBounceRatePct = accountBounce
		r.AccountOpenRatePct = accountOpen
		// Compare each domain with the rest of the account so a domain that
		// carries most of the volume can still stand out.
		if restSent := totalSent - r.Sent; restSent > 0 {
			v := ratePct(totalBounced-r.Bounced, restSent)
			r.RestBounceRatePct = &v
		}
		if restTracked := totalTracked - r.TrackedSent; restTracked > 0 {
			v := ratePct(totalOpens-r.UniqueOpens, restTracked)
			r.RestOpenRatePct = &v
		}
		if r.Sent >= minSent && r.Sent > 0 {
			relativeHigh := r.RestBounceRatePct != nil && *r.RestBounceRatePct > 0 && r.BounceRatePct >= recipientDomainBounceRateRatio**r.RestBounceRatePct
			if r.BounceRatePct > 0 && (relativeHigh || r.HardBounceRatePct >= recipientDomainHardBounceFloorPct) {
				r.Flags = append(r.Flags, recipientDomainFlagHighBounce)
			}
			if r.RestOpenRatePct != nil && *r.RestOpenRatePct > 0 && r.OpenRatePct != nil && *r.OpenRatePct <= recipientDomainOpenRateRatio**r.RestOpenRatePct {
				r.Flags = append(r.Flags, recipientDomainFlagLowOpen)
			}
		}
		r.Flagged = len(r.Flags) > 0
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Flagged != out[j].Flagged {
			return out[i].Flagged
		}
		if out[i].Sent != out[j].Sent {
			return out[i].Sent > out[j].Sent
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

func newNovelRecipientDomainsCmd(flags *rootFlags) *cobra.Command {
	var flagWindow, dbPath string
	var minSent, limit int

	cmd := &cobra.Command{
		Use:   "recipient-domains",
		Short: "Report sends, bounces by type, complaints, suppressions, and opens per recipient mailbox domain from the synced archive, flagging outlier domains",
		Long: strings.Trim(`
Use this command to see delivery outcomes grouped by recipient mailbox domain (gmail.com, outlook.com, a customer's company domain), for example when several users at one provider report missing mail. Do NOT use this command for one recipient's history; use 'diagnose' instead. Do NOT use it for your own sending domains' DKIM, SPF, or Return-Path status; use 'domains health' instead.

Reads the local archive populated by 'sync' (messages, bounces, opens,
suppressions from every synced server). Sent counts each recipient of each
message once. Bounced excludes spam complaints, which are counted separately.
Suppressed counts addresses the archive recorded as suppressed on any stream
as of the last sync; one lifted since then still counts until it is gone from
a fresh archive, so confirm a single address with 'suppressions check'. Open
rate counts unique opens of the window's messages sent with open tracking. A domain with at least --min-sent sends
is flagged high-bounce-rate when its bounce rate is at least 2x the rest of the
account's or its hard-bounce rate is at least 5%, and low-open-rate when its
open rate is at most half the rest of the account's.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli recipient-domains --window 7d --min-sent 20 --agent
  postmark-pp-cli recipient-domains --window 30d --min-sent 5 --limit 10 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "recipient-domains")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			windowDur, err := parsePositiveDuration(flagWindow, "window", "7d, 4w, or 48h")
			if err != nil {
				return err
			}
			if minSent < 0 || limit < 0 {
				return usageErr(errors.New("--min-sent and --limit must be zero or more"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("postmark-pp-cli")
			}
			if localMirrorMissing(cmd.ErrOrStderr(), dbPath, "messages,bounces,opens,streams") {
				return recipientDomainsOutput(cmd, flags, make([]recipientDomainRow, 0))
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer db.Close()
			unsynced := false
			for _, rt := range []string{"messages", "bounces", "opens", "suppressions"} {
				if hintIfUnsynced(cmd, db, rt) {
					unsynced = true
					break
				}
				hintIfStale(cmd, db, rt, flags.maxAge)
			}
			msgs, bounces, opens, sups, err := recipientDomainsLoad(ctx, db)
			if err != nil {
				return err
			}
			rows := recipientDomainsCompute(msgs, bounces, opens, sups, time.Now().Add(-windowDur), minSent)
			// The unsynced hint already names the sync command; a second,
			// different one would contradict it.
			if len(rows) == 0 && !unsynced {
				syncCmd := "postmark-pp-cli sync"
				if cmd.Flags().Changed("db") {
					syncCmd += " --db " + dbPath
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no synced messages in the last %s; run '%s' to add recent mail (with an account token, once per server with --server <name>)\n", flagWindow, syncCmd)
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return recipientDomainsOutput(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "How far back to count activity (e.g. 7d, 30d, 4w)")
	cmd.Flags().IntVar(&minSent, "min-sent", 20, "Minimum sends before a domain can be flagged")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum domains to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local archive path (default: the CLI's data.db)")
	return cmd
}

// recipientDomainsLoad reads the four resource types, draining each result
// set before the next query.
func recipientDomainsLoad(ctx context.Context, db *store.Store) ([]recipientDomainMessage, []recipientDomainBounce, []recipientDomainOpen, []recipientDomainSuppression, error) {
	query := func(sql string, scan func(cols []string) error, n int) error {
		return queryArchive(ctx, db, "local archive", sql, n, scan)
	}
	msgs := make([]recipientDomainMessage, 0)
	err := query(`SELECT id, COALESCE(json_extract(data,'$.Recipients'),'[]'), COALESCE(json_extract(data,'$.ReceivedAt'),''), CASE WHEN json_extract(data,'$.TrackOpens') THEN '1' ELSE '0' END FROM resources WHERE resource_type = 'messages'`, func(c []string) error {
		m := recipientDomainMessage{MessageID: c[0], TrackOpens: c[3] == "1"}
		if err := json.Unmarshal([]byte(c[1]), &m.Recipients); err != nil {
			return nil
		}
		t, ok := postmarkParseTime(c[2])
		if !ok {
			return nil
		}
		m.ReceivedAt = t
		msgs = append(msgs, m)
		return nil
	}, 4)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	bounces := make([]recipientDomainBounce, 0)
	err = query(`SELECT COALESCE(json_extract(data,'$.Email'),''), COALESCE(json_extract(data,'$.Type'),''), COALESCE(json_extract(data,'$.BouncedAt'),'') FROM resources WHERE resource_type = 'bounces'`, func(c []string) error {
		if t, ok := postmarkParseTime(c[2]); ok {
			bounces = append(bounces, recipientDomainBounce{Email: c[0], Type: c[1], BouncedAt: t})
		}
		return nil
	}, 3)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	opens := make([]recipientDomainOpen, 0)
	err = query(`SELECT COALESCE(json_extract(data,'$.Recipient'),''), COALESCE(json_extract(data,'$.MessageID'),''), COALESCE(json_extract(data,'$.ReceivedAt'),'') FROM resources WHERE resource_type = 'opens'`, func(c []string) error {
		if t, ok := postmarkParseTime(c[2]); ok {
			opens = append(opens, recipientDomainOpen{Recipient: c[0], MessageID: c[1], ReceivedAt: t})
		}
		return nil
	}, 3)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sups := make([]recipientDomainSuppression, 0)
	err = query(`SELECT COALESCE(json_extract(data,'$.EmailAddress'),''), COALESCE(json_extract(data,'$.SuppressionReason'),'') FROM resources WHERE resource_type = 'suppressions'`, func(c []string) error {
		sups = append(sups, recipientDomainSuppression{EmailAddress: c[0], Reason: c[1]})
		return nil
	}, 2)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return msgs, bounces, opens, sups, nil
}

func recipientDomainsOutput(cmd *cobra.Command, flags *rootFlags, rows []recipientDomainRow) error {
	for _, r := range rows {
		if r.Suppressed > 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "note: suppression counts come from the local archive as of the last sync; run 'postmark-pp-cli suppressions check <email>' for an address's current status.")
			break
		}
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	out := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(out, "No recipient-domain activity in the local archive for this window.")
		return nil
	}
	fmt.Fprintf(out, "account bounce rate %.2f%%; a domain is flagged when its bounce rate is at least %.0fx the rest of the account or its hard-bounce rate is at least %.0f%%\n\n", rows[0].AccountBounceRatePct, recipientDomainBounceRateRatio, recipientDomainHardBounceFloorPct)
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "DOMAIN\tSENT\tBOUNCED\tBOUNCE%\tHARD%\tCOMPLAINTS\tSUPPRESSED\tOPEN%\tFLAGS")
	for _, r := range rows {
		open := "-"
		if r.OpenRatePct != nil {
			open = fmt.Sprintf("%.1f", *r.OpenRatePct)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.2f\t%.2f\t%d\t%d\t%s\t%s\n", r.Domain, r.Sent, r.Bounced, r.BounceRatePct, r.HardBounceRatePct, r.Complaints, r.Suppressed, open, strings.Join(r.Flags, ","))
	}
	return tw.Flush()
}
