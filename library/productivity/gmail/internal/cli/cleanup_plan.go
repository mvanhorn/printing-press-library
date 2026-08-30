// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `cleanup` command family: the preview half of the
// preview -> confirm -> apply -> undo engine. `cleanup plan` freezes an
// immutable, sha-named plan file plus a one-time nonce; nothing here ever
// mutates the mailbox. The shared builder (buildAndWritePlan) is also the
// engine behind `rules run`.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// planGroupRequest is one group's inputs before freezing: a query OR an
// explicit id list, plus the action. Sender/UnsubURL are set only by
// `unsub plan` (carried verbatim into the frozen group).
type planGroupRequest struct {
	Rule     string
	Query    string
	IDs      []string
	Action   cleanupPlanAction
	Sender   string
	UnsubURL string
}

// ruleCount is the per-group preview row.
type ruleCount struct {
	Rule   string `json:"rule"`
	Action string `json:"action"`
	Count  int    `json:"count"`
}

// planOutput is what plan/rules-run print (via printJSONFiltered).
type planOutput struct {
	Account        string               `json:"account"`
	AccountEmail   string               `json:"account_email"`
	Source         string               `json:"source"`
	Action         string               `json:"action"`
	Total          int                  `json:"total"`
	PerGroup       []ruleCount          `json:"per_group"`
	Samples        []cleanupPlanSample  `json:"samples"`
	PlanSha        string               `json:"plan_sha,omitempty"`
	PlanPath       string               `json:"plan_path,omitempty"`
	Nonce          string               `json:"nonce,omitempty"`
	NonceExpiresAt string               `json:"nonce_expires_at,omitempty"`
	Watermark      cleanupPlanWatermark `json:"watermark"`
	ApplyHint      string               `json:"apply_hint,omitempty"`
	Note           string               `json:"note,omitempty"`
}

// buildAndWritePlan is the single plan machinery (used by `cleanup plan`
// and `rules run`):
//
//  1. run an incremental sync (full on first use) and record the resulting
//     store watermark,
//  2. freeze the message-id list — queries resolve against the LIVE API
//     (messages.list), never the store; ids passed explicitly are taken
//     verbatim — deduplicated first-group-wins, capped at limit per group,
//  3. snapshot per-id labelIds from the store (fetching any id the synced
//     window missed),
//  4. write the immutable sha-named plan file (0600),
//  5. mint a one-time nonce bound to (plan_sha, account), expiring in 10
//     minutes — unless mintNonce is false (`rules run --plan-only`).
//
// A plan that freezes zero ids writes no file and mints no nonce.
func buildAndWritePlan(ctx context.Context, c *client.Client, db *store.Store, flags *rootFlags,
	account, accountEmail, source string, reqs []planGroupRequest, limit int, mintNonce bool, errW io.Writer) (*planOutput, error) {

	if limit <= 0 {
		limit = cleanupPlanDefaultLimit
	}

	// 1. Sync + watermark.
	watermark, err := runPlanSync(ctx, c, db, account, errW)
	if err != nil {
		return nil, classifyEngineAPIError(err)
	}

	// 2. Freeze ids.
	seen := map[string]bool{}
	groups := make([]cleanupPlanGroup, 0, len(reqs))
	for _, req := range reqs {
		var ids []string
		switch {
		case req.Query != "":
			listed, err := mailListMessageIDs(ctx, c, req.Query, limit)
			if err != nil {
				return nil, classifyEngineAPIError(fmt.Errorf("resolving query %q: %w", req.Query, err))
			}
			ids = listed
		default:
			ids = req.IDs
		}
		kept := make([]string, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			kept = append(kept, id)
		}
		groups = append(groups, cleanupPlanGroup{
			Rule:     req.Rule,
			Query:    req.Query,
			Action:   req.Action,
			IDs:      sortedCopy(kept),
			Count:    len(kept),
			Sender:   req.Sender,
			UnsubURL: req.UnsubURL,
		})
	}

	total := 0
	perGroup := make([]ruleCount, 0, len(groups))
	for _, g := range groups {
		total += g.Count
		perGroup = append(perGroup, ruleCount{Rule: g.Rule, Action: g.Action.Type, Count: g.Count})
	}

	out := &planOutput{
		Account:      account,
		AccountEmail: accountEmail,
		Source:       source,
		Total:        total,
		PerGroup:     perGroup,
		Samples:      []cleanupPlanSample{},
		Watermark:    watermark,
	}

	if total == 0 {
		out.Note = "nothing matched — no plan file written, no token minted"
		return out, nil
	}

	// 3. Snapshots (store first; live fetch for anything the synced window
	// missed, upserted so the store stays coherent).
	plan := &cleanupPlan{
		SchemaVersion: cleanupPlanSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Source:        source,
		Account:       account,
		AccountEmail:  accountEmail,
		Groups:        groups,
		Watermark:     watermark,
		Counts:        cleanupPlanCounts{Total: total, Groups: len(groups)},
	}
	allIDs := plan.allIDs()
	metas := make(map[string]store.MailMeta, len(allIDs))
	var missing []string
	for _, id := range allIDs {
		m, err := db.GetMailMeta(account, id)
		if errors.Is(err, sql.ErrNoRows) {
			missing = append(missing, id)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading snapshot for %s: %w", id, err)
		}
		metas[id] = m
	}
	if len(missing) > 0 {
		fetched, _, err := mailFetchMetadata(ctx, c, account, missing, errW)
		if err != nil {
			return nil, classifyEngineAPIError(fmt.Errorf("fetching metadata for %d ids outside the synced window: %w", len(missing), err))
		}
		if _, err := db.UpsertMailMeta(fetched); err != nil {
			return nil, fmt.Errorf("upserting fetched snapshots: %w", err)
		}
		for _, m := range fetched {
			metas[m.ID] = m
		}
		var unresolved []string
		for _, id := range missing {
			if _, ok := metas[id]; !ok {
				unresolved = append(unresolved, id)
			}
		}
		if len(unresolved) > 0 {
			return nil, usageErr(fmt.Errorf("%d id(s) could not be resolved in this mailbox (first: %s) — a plan only freezes ids it can snapshot", len(unresolved), unresolved[0]))
		}
	}
	for _, id := range allIDs {
		m := metas[id]
		labels := m.LabelIDs
		if labels == nil {
			labels = []string{}
		}
		plan.Snapshots = append(plan.Snapshots, cleanupPlanSnapshot{ID: id, ThreadID: m.ThreadID, LabelIDs: labels})
	}

	// Per-id intended deltas (grill R1-C5): trash groups introduce +TRASH;
	// label groups introduce their exact add/remove lists.
	for _, g := range plan.Groups {
		for _, id := range g.IDs {
			d := cleanupPlanDelta{ID: id}
			if g.Action.Type == "trash" {
				d.Add = []string{"TRASH"}
			} else {
				d.Add = g.Action.Add
				d.Remove = g.Action.Remove
			}
			plan.Deltas = append(plan.Deltas, d)
		}
	}

	// Samples: first 10 frozen ids, group order.
	for i, id := range allIDs {
		if i >= 10 {
			break
		}
		m := metas[id]
		from := m.FromEmail
		if from == "" {
			from = m.FromName
		}
		plan.Samples = append(plan.Samples, cleanupPlanSample{
			ID:      id,
			From:    from,
			Subject: m.Subject,
			Date:    msToRFC3339(m.InternalDate),
		})
	}

	// 4. Immutable plan file.
	authDir := gauthConfigDir(flags)
	sha, path, err := writePlanFile(authDir, plan)
	if err != nil {
		return nil, err
	}
	out.PlanSha = sha
	out.PlanPath = path
	out.Samples = plan.Samples
	out.Action = plan.actionSummary()

	// 5. One-time nonce.
	if mintNonce {
		nonce, err := mintHex(16) // 128-bit
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(cleanupNonceTTL)
		if err := db.CreateMailNonce(nonce, sha, account, expires); err != nil {
			return nil, fmt.Errorf("recording nonce: %w", err)
		}
		out.Nonce = nonce
		out.NonceExpiresAt = expires.Format(time.RFC3339)
		out.ApplyHint = fmt.Sprintf("gmail-pp-cli cleanup apply --plan %s --token %s", sha, nonce)
	} else {
		out.Note = "plan-only: no token minted; re-run without --plan-only to get an applyable token"
	}
	return out, nil
}

// gauthConfigDir resolves the auth dir the same way every gauth caller does.
func gauthConfigDir(flags *rootFlags) string {
	return gauthConfigDirFrom(flags.authDir)
}

// resolveLabelRefs maps user-supplied label references (raw label IDs or
// case-insensitive unique label names) to label IDs against the live
// labels list, so a plan freezes only labels that actually exist.
func resolveLabelRefs(ctx context.Context, c *client.Client, refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	data, err := c.GetWithHeadersNoCache(ctx, "/gmail/v1/users/me/labels", nil, nil)
	if err != nil {
		return nil, classifyEngineAPIError(fmt.Errorf("listing labels to resolve references: %w", err))
	}
	var page struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("parsing labels list: %w", err)
	}
	byID := map[string]bool{}
	byName := map[string][]string{}
	for _, l := range page.Labels {
		byID[l.ID] = true
		key := strings.ToLower(l.Name)
		byName[key] = append(byName[key], l.ID)
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if byID[ref] {
			out = append(out, ref)
			continue
		}
		matches := byName[strings.ToLower(ref)]
		switch len(matches) {
		case 1:
			out = append(out, matches[0])
		case 0:
			return nil, usageErr(fmt.Errorf("label %q matches no label id or name — run 'gmail-pp-cli labels list' (or create it with 'labels create --name %s')", ref, ref))
		default:
			return nil, usageErr(fmt.Errorf("label name %q is ambiguous (%d labels share it case-insensitively) — pass the label ID instead", ref, len(matches)))
		}
	}
	return out, nil
}

// parseCleanupAction validates the --action/--add/--remove trio.
func parseCleanupAction(action, addCSV, removeCSV string) (cleanupPlanAction, error) {
	act := cleanupPlanAction{Type: strings.ToLower(strings.TrimSpace(action))}
	add, err := cliutil.ParseStringList(addCSV)
	if err != nil {
		return act, usageErr(fmt.Errorf("parsing --add: %w", err))
	}
	remove, err := cliutil.ParseStringList(removeCSV)
	if err != nil {
		return act, usageErr(fmt.Errorf("parsing --remove: %w", err))
	}
	switch act.Type {
	case "trash":
		if len(add) > 0 || len(remove) > 0 {
			return act, usageErr(fmt.Errorf("--action trash takes no --add/--remove (trash IS the delta)"))
		}
	case "label":
		if len(add) == 0 && len(remove) == 0 {
			return act, usageErr(fmt.Errorf("--action label needs at least one of --add/--remove"))
		}
		act.Add = add
		act.Remove = remove
	default:
		return act, usageErr(fmt.Errorf("--action must be 'trash' or 'label', got %q", action))
	}
	return act, nil
}

func newCleanupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cleanup",
		Short:       "Preview-confirm-undo mailbox cleanup: plan freezes what would change, apply executes exactly that, recover finishes a crashed apply",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCleanupPlanCmd(flags))
	cmd.AddCommand(newCleanupApplyCmd(flags))
	cmd.AddCommand(newCleanupRecoverCmd(flags))
	return cmd
}

func newCleanupPlanCmd(flags *rootFlags) *cobra.Command {
	var q, idsCSV, action, addCSV, removeCSV string
	var limit int

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Freeze an immutable cleanup plan (sha-named, 0600) plus a one-time apply token — mailbox untouched",
		Long: `Freeze exactly what a cleanup would do, without doing any of it.

Steps: (1) an incremental sync runs first and the resulting store watermark
(history_id + timestamp) is recorded; (2) the message-id list is FROZEN —
--q resolves against the live Gmail API (messages.list, capped at --limit,
default 2000), --ids is taken verbatim; (3) each id's current labelIds are
snapshotted; (4) the plan is written to <auth-dir>/plans/<sha>.json (0600)
where the file name IS the sha256 of its exact content; (5) a one-time
128-bit token bound to this plan and account is minted, valid 10 minutes.

Apply will execute exactly the frozen ids — never a re-run of the query —
and refuses (exit 4) if the plan file was edited, the token is wrong, used,
or expired, the live mailbox identity mismatches, or any planned id changed
since the watermark ("plan drifted").

Typed exits: 0 plan written / 2 usage / 4 identity refusal / 5 auth or API
failure.`,
		Example: `  # Trash everything promotional and old; preview counts + samples first
  gmail-pp-cli cleanup plan --account personal --q "category:promotions older_than:1y" --action trash

  # Label two specific messages (label names or IDs both work)
  gmail-pp-cli cleanup plan --account ads --ids 18c1a2b3,18c1a2b4 --action label --add Archive2025 --remove UNREAD

  # Then, within 10 minutes:
  gmail-pp-cli cleanup apply --plan <sha> --token <nonce>`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"mcp:local-write":     "true",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if (q == "") == (idsCSV == "") {
				return usageErr(fmt.Errorf("pass exactly one of --q (live Gmail query) or --ids (explicit id list)"))
			}
			act, err := parseCleanupAction(action, addCSV, removeCSV)
			if err != nil {
				return err
			}
			ids, err := cliutil.ParseStringList(idsCSV)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --ids: %w", err))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive, got %d", limit))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			ctx := cmd.Context()

			// Identity preflight binds the plan to the live mailbox at plan
			// time — catching a profiles.yaml/token mismatch here, before a
			// plan exists, not at apply time.
			if err := verifyLiveIdentity(ctx, c, flags, account); err != nil {
				return classifyEngineAPIError(err)
			}
			prof, err := gauthProfile(flags, account)
			if err != nil {
				return err
			}
			if act.Type == "label" {
				if act.Add, err = resolveLabelRefs(ctx, c, act.Add); err != nil {
					return err
				}
				if act.Remove, err = resolveLabelRefs(ctx, c, act.Remove); err != nil {
					return err
				}
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			out, err := buildAndWritePlan(ctx, c, db, flags, account, prof.Email, "cleanup plan",
				[]planGroupRequest{{Query: q, IDs: ids, Action: act}}, limit, true, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&q, "q", "", "Gmail search query resolved LIVE to freeze the id list (full Gmail search syntax)")
	cmd.Flags().StringVar(&idsCSV, "ids", "", "Explicit comma-separated message ids to freeze instead of --q")
	cmd.Flags().StringVar(&action, "action", "", "What apply will do to every frozen id: trash | label")
	cmd.Flags().StringVar(&addCSV, "add", "", "Labels to add (label action): comma-separated label IDs or names")
	cmd.Flags().StringVar(&removeCSV, "remove", "", "Labels to remove (label action): comma-separated label IDs or names")
	cmd.Flags().IntVar(&limit, "limit", cleanupPlanDefaultLimit, "Maximum ids a --q resolution may freeze")
	_ = cmd.MarkFlagRequired("action")
	return cmd
}
