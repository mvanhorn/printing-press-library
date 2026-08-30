// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `unsub run`: the execute half of the unsubscribe engine.
// Takes ONLY --plan <sha> --token <nonce> (exactly like `cleanup apply`;
// the frozen sender list and URLs come from the plan file — never an
// ad-hoc list), re-verifies every one-click condition live per sender plus
// the DKIM coverage check, then performs hardened external HTTPS POSTs
// (grill R1-C3/R2-C3/R3-C5): public-unicast SSRF guard, resolved-IP pinned
// dial, no cookies, no auth, redirects never followed, every attempt
// ledgered in mail_unsub_ledger.
//
// These POSTs go to third-party servers, NOT the Gmail API — they bypass
// the transport allowlist by construction, which is exactly why the guard
// stack and the plan/token gate are mandatory here.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
	"github.com/spf13/cobra"
)

// unsubRunSenderResult is one frozen sender's outcome.
type unsubRunSenderResult struct {
	Sender     string `json:"sender"`
	URL        string `json:"url"`
	MessageID  string `json:"message_id,omitempty"`
	Outcome    string `json:"outcome"` // posted | skipped | unknown
	HTTPStatus int    `json:"http_status,omitempty"`
	Redirect   string `json:"redirect_location,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// unsubRunResult is the JSON envelope `unsub run` prints.
type unsubRunResult struct {
	ApplyID int64                  `json:"apply_id"`
	Account string                 `json:"account"`
	PlanSha string                 `json:"plan_sha"`
	Posted  int                    `json:"posted"`
	Skipped int                    `json:"skipped"`
	Unknown int                    `json:"unknown"`
	Senders []unsubRunSenderResult `json:"senders"`
	State   string                 `json:"state"`
	Note    string                 `json:"note,omitempty"`
}

// unsubLiveHeaders is the live messages.get view the run-time re-check uses.
type unsubLiveHeaders struct {
	listUnsub     []string
	listUnsubPost []string
	dkimSigs      []string
}

// fetchUnsubLiveHeaders GETs one message live (cache bypassed) with
// format=metadata restricted to exactly the three headers the run-time
// check needs (allowlisted messages.get). found=false on a 404.
func fetchUnsubLiveHeaders(ctx context.Context, c *client.Client, id string) (unsubLiveHeaders, bool, error) {
	params := url.Values{}
	params.Set("format", "metadata")
	for _, h := range []string{"DKIM-Signature", "List-Unsubscribe", "List-Unsubscribe-Post"} {
		params.Add("metadataHeaders", h)
	}
	var out unsubLiveHeaders
	data, err := c.GetWithHeadersNoCacheValues(ctx, mailMetadataFetchPath+url.PathEscape(id), params, nil)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return out, false, nil
		}
		return out, false, err
	}
	var msg gmailMessageMeta
	if err := json.Unmarshal(data, &msg); err != nil {
		return out, false, fmt.Errorf("parsing message %s: %w", id, err)
	}
	for _, h := range msg.Payload.Headers {
		switch {
		case strings.EqualFold(h.Name, "List-Unsubscribe"):
			out.listUnsub = append(out.listUnsub, h.Value)
		case strings.EqualFold(h.Name, "List-Unsubscribe-Post"):
			out.listUnsubPost = append(out.listUnsubPost, h.Value)
		case strings.EqualFold(h.Name, "DKIM-Signature"):
			out.dkimSigs = append(out.dkimSigs, h.Value)
		}
	}
	return out, true, nil
}

// checkUnsubRunConditions re-verifies the full one-click ladder for one
// frozen sender at run time from the LIVE headers plus the stored
// Authentication-Results, and performs the DKIM coverage and
// destination-alignment checks. Returns "" when the sender may be posted,
// or a skip reason. Ambiguity of any kind (duplicate headers, duplicate
// URLs, duplicate signatures) is a skip, never a guess.
func checkUnsubRunConditions(frozenURL, sender, storedAuthResults string, live unsubLiveHeaders, allowThirdParty bool) string {
	// (a) exactly ONE List-Unsubscribe header instance, live.
	if len(live.listUnsub) == 0 {
		return "live message no longer carries a List-Unsubscribe header"
	}
	if len(live.listUnsub) > 1 {
		return fmt.Sprintf("ambiguous: %d live List-Unsubscribe headers", len(live.listUnsub))
	}
	info := parseListUnsubscribe(live.listUnsub[0])
	// (a cont.) exactly one https URL inside it.
	if len(info.HTTPSURLs) == 0 {
		return "live List-Unsubscribe carries no https URL"
	}
	if len(info.HTTPSURLs) > 1 {
		return fmt.Sprintf("ambiguous: %d https URLs in the live List-Unsubscribe header", len(info.HTTPSURLs))
	}
	// The live URL must be byte-identical to the URL the plan froze — run
	// only ever POSTs what the owner confirmed.
	if info.HTTPSURLs[0] != frozenURL {
		return "live unsubscribe URL differs from the URL the plan froze — re-plan"
	}
	// (b) List-Unsubscribe-Post: exactly one instance, exact One-Click value.
	if len(live.listUnsubPost) != 1 {
		return fmt.Sprintf("expected exactly one live List-Unsubscribe-Post header, saw %d", len(live.listUnsubPost))
	}
	if !strings.EqualFold(strings.TrimSpace(live.listUnsubPost[0]), oneClickPostValue) {
		return fmt.Sprintf("live List-Unsubscribe-Post is %q, not %q", strings.TrimSpace(live.listUnsubPost[0]), oneClickPostValue)
	}
	// (d) stored Authentication-Results: Gmail's own, dmarc=pass.
	if !authResultsIsGmailDMARCPass(storedAuthResults) {
		return "stored Authentication-Results is not Gmail's own dmarc=pass verdict"
	}
	// (e) DKIM coverage, live: exactly ONE signature, h= covering both
	// one-click headers, d= aligned with the sender's registrable domain.
	if len(live.dkimSigs) == 0 {
		return "live message carries no DKIM-Signature header"
	}
	if len(live.dkimSigs) > 1 {
		return fmt.Sprintf("ambiguous: %d DKIM-Signature headers", len(live.dkimSigs))
	}
	sig := live.dkimSigs[0]
	if !dkimCoversUnsubHeaders(sig) {
		return "DKIM h= tag list does not cover both list-unsubscribe and list-unsubscribe-post"
	}
	d := dkimDomain(sig)
	if d == "" {
		return "DKIM-Signature carries no d= domain"
	}
	if registrableDomain(d) != registrableDomain(emailDomain(sender)) {
		return fmt.Sprintf("DKIM d= domain %s does not align with sender domain %s", d, emailDomain(sender))
	}
	// (f) destination alignment: (e) ties the MESSAGE to the sender; this
	// ties the POST DESTINATION to it. A third-party (ESP-hosted) URL host
	// is posted only under the operator's explicit --allow-third-party.
	if !allowThirdParty && !unsubHostAligned(frozenURL, sender) {
		return fmt.Sprintf("unsubscribe host %q is outside sender domain %q (third-party); re-run with --allow-third-party to permit",
			unsubURLHost(frozenURL), emailDomain(sender))
	}
	return ""
}

func newNovelUnsubRunCmd(flags *rootFlags) *cobra.Command {
	var planSha, token string
	var allowThirdParty bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute a frozen unsubscribe plan: burn its one-time token, re-verify every sender live (incl. DKIM coverage), then POST one-click unsubscribes with SSRF-guarded pinned dialing",
		Long: `Execute exactly the senders (and exact URLs) an 'unsub plan' froze.

Ordering (every step fails closed):
 1. re-hash the plan file and refuse (exit 4) unless it still equals its
    own sha-name, is an unsub plan, and matches this account,
 2. take the advisory apply lock (shared with the cleanup engine; busy =
    exit 7),
 3. atomically verify + burn the one-time token and record the run,
 4. identity preflight: the live mailbox must match the plan's account,
 5. per sender, EVERYTHING is re-verified at run time from a live
    messages.get of the sender's newest unsubscribe-bearing message:
    exactly one List-Unsubscribe header with exactly one https URL that
    byte-matches the frozen URL; List-Unsubscribe-Post exactly
    "List-Unsubscribe=One-Click"; Gmail's own stored dmarc=pass verdict;
    and exactly one DKIM-Signature whose h= list covers both one-click
    headers and whose d= domain aligns (registrable domain) with the
    sender; and the unsubscribe URL's own host must share the sender's
    registrable domain — third-party (ESP-hosted) destinations are
    skipped unless this run passed --allow-third-party ('unsub plan'
    lists them under third_party_hosts). Any failure or ambiguity SKIPS
    that sender with a reason — nothing is ever posted on a guess.
 6. each POST runs the hardened path: the URL host must resolve to ONLY
    public unicast addresses (RFC1918, loopback, link-local, CGNAT, ULA,
    multicast, reserved all refuse), the connection dials the exact
    resolved IP (DNS-rebinding defense) with TLS against the original
    hostname, no cookie jar, no Authorization, body exactly
    "List-Unsubscribe=One-Click" (application/x-www-form-urlencoded), 10
    second deadline, response body capped at 64KB; a 3xx is terminal and
    recorded — redirects are NEVER followed.
 7. every attempt lands in mail_unsub_ledger: an HTTP status, or
    'skipped:<reason>', or 'unknown' (network error after the connection
    was established — NEVER auto-retried; check 'unsub verify' later).

Typed exits: 0 all posted / 3 partial (some skipped or unknown) / 4
refused (token, identity, wrong plan kind) / 5 auth or API failure / 7
lock busy.`,
		Example: `  # Tokens come from 'unsub plan' and live 10 minutes
  gmail-pp-cli unsub run --plan 4f0c...e2 --token 9b31c0de84a1f2b3c4d5e6f708192a3b

  # A week later, see who kept mailing anyway
  gmail-pp-cli unsub verify --account personal`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3,4,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if planSha == "" || token == "" {
				return usageErr(fmt.Errorf("--plan <sha> and --token <nonce> are both required (minted by 'unsub plan'; run never accepts a sender list directly)"))
			}
			authDir := gauthConfigDirFrom(flags.authDir)

			plan, err := loadPlanFile(authDir, planSha)
			if err != nil {
				return err
			}
			if !plan.planIsUnsub() {
				return refusedErr(fmt.Errorf("plan %s is a %q plan — 'unsub run' executes only plans frozen by 'unsub plan' (mailbox plans run via 'cleanup apply')",
					strings.ToLower(strings.TrimSpace(planSha)), plan.actionSummary()))
			}
			if flags.account != "" && flags.account != plan.Account {
				return refusedErr(fmt.Errorf("--account %q does not match the plan's account %q — a plan runs only for the account it froze", flags.account, plan.Account))
			}
			flags.account = plan.Account
			prof, err := gauthProfile(flags, plan.Account)
			if err != nil {
				return err
			}
			if !strings.EqualFold(prof.Email, plan.AccountEmail) {
				return refusedErr(fmt.Errorf("plan was frozen for %s but profile %q is now configured for %s — re-plan", plan.AccountEmail, plan.Account, prof.Email))
			}
			if dryRunOK(flags) {
				return nil
			}

			release, err := acquireApplyLock(authDir)
			if err != nil {
				return err
			}
			defer release()

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			sha := strings.ToLower(strings.TrimSpace(planSha))
			applyID, err := db.AuthorizeMailApply(token, sha, plan.Account, "unsub", time.Now())
			if err != nil {
				if isNonceRefusal(err) {
					return refusedErr(fmt.Errorf("run token refused: %w (mint a fresh one with 'unsub plan')", err))
				}
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if err := verifyLiveIdentity(ctx, c, flags, plan.Account); err != nil {
				_ = db.SetMailApplyState(applyID, store.MailApplyStateRefused)
				return classifyEngineAPIError(err)
			}
			if err := db.SetMailApplyState(applyID, store.MailApplyStateApplying); err != nil {
				return err
			}

			res := unsubRunResult{ApplyID: applyID, Account: plan.Account, PlanSha: sha}
			for _, g := range plan.Groups {
				sr := unsubRunSenderResult{Sender: g.Sender, URL: g.UnsubURL}

				skip := func(reason string) {
					sr.Outcome = "skipped"
					sr.Reason = reason
					res.Skipped++
					if _, lerr := db.InsertMailUnsubAttempt(store.MailUnsubAttempt{
						Account: plan.Account, Sender: g.Sender, URL: g.UnsubURL,
						PlanSha: sha, Status: "skipped:" + reason,
					}); lerr != nil {
						sr.Reason += fmt.Sprintf(" (ledger write failed: %v)", lerr)
					}
					res.Senders = append(res.Senders, sr)
				}

				newest, err := db.NewestUnsubMeta(plan.Account, g.Sender)
				if errors.Is(err, sql.ErrNoRows) {
					skip("no stored unsubscribe-bearing message for this sender")
					continue
				}
				if err != nil {
					return fmt.Errorf("reading newest unsubscribe message for %s: %w", g.Sender, err)
				}
				sr.MessageID = newest.ID

				// The plan froze exactly one message id per sender; if the
				// store's newest unsubscribe-bearing message has changed
				// since, the stored Authentication-Results below would come
				// from a message the operator never confirmed — skip and
				// re-plan.
				if len(g.IDs) != 1 || newest.ID != g.IDs[0] {
					skip("newest unsubscribe-bearing message changed since the plan froze — re-plan")
					continue
				}

				var live unsubLiveHeaders
				var found bool
				lerr := engineCall(ctx, func(cctx context.Context) error {
					var ferr error
					live, found, ferr = fetchUnsubLiveHeaders(cctx, c, newest.ID)
					return ferr
				})
				if lerr != nil {
					if isEngineAuthError(lerr) {
						// Auth died mid-run: stop with honest counts; senders
						// not yet attempted stay unattempted (re-plan after
						// 'accounts auth' — the token is burned).
						res.State = store.MailApplyStatePartial
						res.Note = "authorization failed mid-run; remaining senders were not attempted — re-auth and mint a fresh plan"
						_ = db.SetMailApplyState(applyID, store.MailApplyStatePartial)
						if perr := printJSONFiltered(cmd.OutOrStdout(), res, flags); perr != nil {
							return perr
						}
						return classifyEngineAPIError(lerr)
					}
					skip(fmt.Sprintf("live header fetch failed: %v", lerr))
					continue
				}
				if !found {
					skip("message no longer exists in the mailbox")
					continue
				}
				if reason := checkUnsubRunConditions(g.UnsubURL, g.Sender, newest.AuthResults, live, allowThirdParty); reason != "" {
					skip(reason)
					continue
				}

				// Durable pre-POST intent: the row exists as 'unknown' before
				// anything leaves this machine, so a crash mid-POST leaves the
				// honest ambiguous record ('unknown' is never auto-retried).
				attemptID, err := db.InsertMailUnsubAttempt(store.MailUnsubAttempt{
					Account: plan.Account, Sender: g.Sender, URL: g.UnsubURL,
					PlanSha: sha, Status: "unknown",
				})
				if err != nil {
					return fmt.Errorf("recording unsubscribe attempt for %s: %w", g.Sender, err)
				}

				post := performOneClickPost(ctx, g.UnsubURL)
				switch {
				case post.SkipReason != "":
					reason := post.SkipReason
					if post.Err != nil {
						reason = fmt.Sprintf("%s: %v", post.SkipReason, post.Err)
					}
					if err := db.SetMailUnsubAttemptStatus(attemptID, "skipped:"+post.SkipReason); err != nil {
						return err
					}
					sr.Outcome = "skipped"
					sr.Reason = reason
					res.Skipped++
				case post.Unknown:
					// Leave the ledger row 'unknown' — the POST may have been
					// received; never auto-retried.
					sr.Outcome = "unknown"
					if post.Err != nil {
						sr.Reason = post.Err.Error()
					}
					res.Unknown++
				default:
					if err := db.SetMailUnsubAttemptStatus(attemptID, strconv.Itoa(post.Status)); err != nil {
						return err
					}
					sr.Outcome = "posted"
					sr.HTTPStatus = post.Status
					sr.Redirect = post.RedirectLoc
					res.Posted++
				}
				res.Senders = append(res.Senders, sr)
			}

			if res.Skipped == 0 && res.Unknown == 0 {
				res.State = store.MailApplyStateDone
			} else {
				res.State = store.MailApplyStatePartial
				res.Note = "skipped senders are never posted on a guess — fix the reason (or unsubscribe manually) and re-plan; 'unknown' attempts are never auto-retried"
			}
			if err := db.SetMailApplyState(applyID, res.State); err != nil {
				return err
			}
			if perr := printJSONFiltered(cmd.OutOrStdout(), res, flags); perr != nil {
				return perr
			}
			if res.State != store.MailApplyStateDone {
				return partialApplyErr(fmt.Errorf("unsub run finished partial: posted %d, skipped %d, unknown %d", res.Posted, res.Skipped, res.Unknown))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&planSha, "plan", "", "The plan sha printed by 'unsub plan' (also the plan's file name)")
	cmd.Flags().StringVar(&token, "token", "", "The one-time run token minted with that plan (10-minute expiry, single use)")
	cmd.Flags().BoolVar(&allowThirdParty, "allow-third-party", false, "Permit one-click POSTs whose URL host is outside the sender's registrable domain (ESP-hosted unsubscribe endpoints); skipped by default")
	return cmd
}
