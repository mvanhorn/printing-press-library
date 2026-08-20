package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newRulesCmd registers the `rules` subcommand tree. All endpoints come from
// the internal api.lunchmoney.app backend — see internal/internalapi/captures/rules.md.
//
// The public Lunch Money v2 API does NOT expose rules; this command group is the
// only way to manage them programmatically.
func newRulesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage transaction rules (auto-categorize, mark-reviewed, etc.) — internal API",
		Long: `Manage Lunch Money transaction rules. Uses the undocumented web-UI backend
because the public v2 API does not expose rules.

Supported actions on rules create:
  --category-id N             Set category to N (mutually exclusive with --clear-category)
  --clear-category            Strip category (set_uncategorized: true)
  --mark-reviewed             Set status to cleared
  --mark-unreviewed           Set status to uncleared
  --set-payee TEXT            Rename payee on match
  --set-notes TEXT            Set notes on match
  --add-tag-id N              Add tag (repeatable)
  --send-email                Send notification email
  --should-delete             Delete the transaction on match
  --skip-recurring            Suppress recurring auto-link
  --dont-run-rules            Suppress suggested-rule promotion
  --stop                      Stop processing other rules after this one fires
  --desc TEXT                 Internal label for the rule
  --priority N                Rule priority (lower = higher precedence; default 10)

Run 'lunch-money-pp-cli internal auth set-cookie ...' once before using these commands,
or set LUNCHMONEY_INTERNAL_COOKIE for a non-persistent session-cookie override.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	cmd.AddCommand(newRulesListCmd(flags))
	cmd.AddCommand(newRulesCreateCmd(flags))
	cmd.AddCommand(newRulesUpdateCmd(flags))
	cmd.AddCommand(newRulesDeleteCmd(flags))
	cmd.AddCommand(newRulesApplyCmd(flags))
	return cmd
}

func newInternalClient() (*internalapi.Client, error) {
	c, err := internalapi.New(internalapi.DefaultCookiePath())
	if err != nil {
		return nil, err
	}
	if !c.HasSession() {
		return nil, fmt.Errorf("no internal-API session — run 'lunch-money-pp-cli internal auth set-cookie' first or set LUNCHMONEY_INTERNAL_COOKIE")
	}
	return c, nil
}

// PATCH: Keep rules commands testable without touching the live internal API.
var newInternalClientForRules = newInternalClient

func newRulesListCmd(flags *rootFlags) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all transaction rules",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClientForRules()
			if err != nil {
				return err
			}
			rules, err := c.ListRules(100)
			if err != nil {
				return err
			}
			if asJSON || flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rules)
			}
			// Human table — show payee match + action summary (decoded)
			fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-10s %-4s %-28s %-40s\n",
				"RULE_ID", "CRIT_ID", "PRI", "PAYEE_MATCH", "ACTION")
			for _, r := range rules {
				payee := r.PayeeDisplay()
				if payee != "" && r.CriteriaPayeeNameMatch != nil {
					payee = *r.CriteriaPayeeNameMatch + ":" + payee
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10d %-10d %-4d %-28s %-40s\n",
					r.RuleID, r.RuleCriteriaID, r.CriteriaPriority,
					truncRule(payee, 28), truncRule(r.ActionSummary(), 40))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d rules\n", len(rules))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit JSON")
	return cmd
}

func truncRule(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ruleCreateFlags holds all the action flags for create/update.
type ruleCreateFlags struct {
	payee, match, desc string
	categoryID         int64
	priority           int
	apply              bool
	clearCategory      bool
	markReviewed       bool
	markUnreviewed     bool
	setPayee, setNotes string
	addTagIDs          []int64
	sendEmail          bool
	shouldDelete       bool
	skipRecurring      bool
	dontRunRules       bool
	stop               bool
}

func (f *ruleCreateFlags) bindCreate(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.payee, "payee", "", "Payee match string (required for create; update preserves existing payee unless set)")
	cmd.Flags().StringVar(&f.match, "match", "contain", "Match mode: contain | exact | start | end | regex")
	cmd.Flags().StringVar(&f.desc, "desc", "", "Internal label for the rule")
	cmd.Flags().Int64Var(&f.categoryID, "category-id", 0, "Set category to this ID")
	cmd.Flags().IntVar(&f.priority, "priority", 10, "Rule priority (lower = higher precedence)")
	cmd.Flags().BoolVar(&f.apply, "apply", false, "Apply rule to past transactions after creating")
	cmd.Flags().BoolVar(&f.clearCategory, "clear-category", false, "Strip category on match (set_uncategorized)")
	cmd.Flags().BoolVar(&f.markReviewed, "mark-reviewed", false, "Mark transaction as reviewed (cleared)")
	cmd.Flags().BoolVar(&f.markUnreviewed, "mark-unreviewed", false, "Mark transaction as unreviewed (uncleared)")
	cmd.Flags().StringVar(&f.setPayee, "set-payee", "", "Rename payee to this value on match")
	cmd.Flags().StringVar(&f.setNotes, "set-notes", "", "Set notes to this value on match")
	cmd.Flags().Int64SliceVar(&f.addTagIDs, "add-tag-id", nil, "Add this tag id (repeatable)")
	cmd.Flags().BoolVar(&f.sendEmail, "send-email", false, "Send notification email on match")
	cmd.Flags().BoolVar(&f.shouldDelete, "should-delete", false, "Delete the transaction on match (dangerous)")
	cmd.Flags().BoolVar(&f.skipRecurring, "skip-recurring", false, "Suppress recurring-item auto-link on match")
	cmd.Flags().BoolVar(&f.dontRunRules, "dont-run-rules", false, "Suppress suggested-rule promotion")
	cmd.Flags().BoolVar(&f.stop, "stop", false, "Stop processing other rules after this one fires")
}

func (f *ruleCreateFlags) toSpec() (internalapi.RuleSpec, error) {
	if f.payee == "" {
		return internalapi.RuleSpec{}, fmt.Errorf("--payee is required")
	}
	if f.categoryID != 0 && f.clearCategory {
		return internalapi.RuleSpec{}, fmt.Errorf("--category-id and --clear-category are mutually exclusive")
	}
	if f.markReviewed && f.markUnreviewed {
		return internalapi.RuleSpec{}, fmt.Errorf("--mark-reviewed and --mark-unreviewed are mutually exclusive")
	}
	hasAction := f.categoryID != 0 || f.clearCategory || f.markReviewed || f.markUnreviewed ||
		f.setPayee != "" || f.setNotes != "" || len(f.addTagIDs) > 0 ||
		f.sendEmail || f.shouldDelete || f.skipRecurring || f.dontRunRules
	if !hasAction {
		return internalapi.RuleSpec{}, fmt.Errorf("provide at least one action (e.g. --category-id, --clear-category, --mark-unreviewed)")
	}
	spec := internalapi.RuleSpec{
		Conditions: internalapi.RuleConditions{
			Payee:    &internalapi.RulePayee{Name: f.payee, Match: f.match},
			OnPlaid:  true,
			OnCSV:    true,
			OnAPI:    true,
			OnManual: true,
			Priority: strconv.Itoa(f.priority),
		},
	}
	a := &spec.Actions
	if f.categoryID != 0 {
		a.CategoryID = &f.categoryID
	}
	if f.clearCategory {
		t := true
		a.SetUncategorized = &t
	}
	if f.markReviewed {
		t := true
		a.MarkAsReviewed = &t
	}
	if f.markUnreviewed {
		t := true
		a.MarkAsUnreviewed = &t
	}
	if f.setPayee != "" {
		a.Payee = &f.setPayee
	}
	if f.setNotes != "" {
		a.Notes = &f.setNotes
	}
	if len(f.addTagIDs) > 0 {
		a.AddTagIDs = f.addTagIDs
	}
	if f.sendEmail {
		t := true
		a.ShouldSendEmail = &t
	}
	if f.shouldDelete {
		t := true
		a.ShouldDelete = &t
	}
	if f.skipRecurring {
		t := true
		a.SkipRecurring = &t
	}
	if f.dontRunRules {
		t := true
		a.DontRunRules = &t
	}
	if f.stop {
		t := true
		a.StopProcessingOthers = &t
	}
	if f.desc != "" {
		a.Description = &f.desc
	}
	return spec, nil
}

func (f *ruleCreateFlags) toUpdateSpec(existing *internalapi.Rule, cmd *cobra.Command) (internalapi.RuleSpec, error) {
	if existing == nil {
		return internalapi.RuleSpec{}, fmt.Errorf("existing rule is required")
	}
	spec := existing.ToSpec()
	changed := cmd.Flags().Changed

	if changed("payee") || changed("match") {
		name := f.payee
		if name == "" {
			if spec.Conditions.Payee == nil {
				return internalapi.RuleSpec{}, fmt.Errorf("--payee is required because the existing rule has no payee condition")
			}
			name = spec.Conditions.Payee.Name
		}
		match := f.match
		if !changed("match") {
			match = "contain"
			if spec.Conditions.Payee != nil && spec.Conditions.Payee.Match != "" {
				match = spec.Conditions.Payee.Match
			}
		}
		spec.Conditions.Payee = &internalapi.RulePayee{Name: name, Match: match}
	}
	if changed("priority") {
		spec.Conditions.Priority = strconv.Itoa(f.priority)
	}

	a := &spec.Actions
	if changed("category-id") {
		if f.categoryID == 0 {
			a.CategoryID = nil
		} else {
			v := f.categoryID
			a.CategoryID = &v
			a.SetUncategorized = nil
		}
	}
	if changed("clear-category") {
		if f.clearCategory {
			t := true
			a.SetUncategorized = &t
			a.CategoryID = nil
		} else {
			a.SetUncategorized = nil
		}
	}
	if a.CategoryID != nil && a.SetUncategorized != nil && *a.SetUncategorized {
		return internalapi.RuleSpec{}, fmt.Errorf("--category-id and --clear-category are mutually exclusive")
	}

	if changed("mark-reviewed") {
		setBoolAction(&a.MarkAsReviewed, f.markReviewed)
		if f.markReviewed {
			a.MarkAsUnreviewed = nil
		}
	}
	if changed("mark-unreviewed") {
		setBoolAction(&a.MarkAsUnreviewed, f.markUnreviewed)
		if f.markUnreviewed {
			a.MarkAsReviewed = nil
		}
	}
	if a.MarkAsReviewed != nil && *a.MarkAsReviewed && a.MarkAsUnreviewed != nil && *a.MarkAsUnreviewed {
		return internalapi.RuleSpec{}, fmt.Errorf("--mark-reviewed and --mark-unreviewed are mutually exclusive")
	}
	if changed("set-payee") {
		a.Payee = stringPtr(f.setPayee)
	}
	if changed("set-notes") {
		a.Notes = stringPtr(f.setNotes)
	}
	if changed("desc") {
		a.Description = stringPtr(f.desc)
	}
	if changed("add-tag-id") {
		a.AddTagIDs = append([]int64(nil), f.addTagIDs...)
	}
	if changed("send-email") {
		setBoolAction(&a.ShouldSendEmail, f.sendEmail)
	}
	if changed("should-delete") {
		setBoolAction(&a.ShouldDelete, f.shouldDelete)
	}
	if changed("skip-recurring") {
		setBoolAction(&a.SkipRecurring, f.skipRecurring)
	}
	if changed("dont-run-rules") {
		setBoolAction(&a.DontRunRules, f.dontRunRules)
	}
	if changed("stop") {
		setBoolAction(&a.StopProcessingOthers, f.stop)
	}
	if !ruleActionsHaveAny(*a) {
		return internalapi.RuleSpec{}, fmt.Errorf("provide at least one action (e.g. --category-id, --clear-category, --mark-unreviewed)")
	}
	return spec, nil
}

func setBoolAction(dst **bool, value bool) {
	if !value {
		*dst = nil
		return
	}
	t := true
	*dst = &t
}

func stringPtr(s string) *string {
	return &s
}

func ruleActionsHaveAny(a internalapi.RuleActions) bool {
	return a.CategoryID != nil ||
		(a.SetUncategorized != nil && *a.SetUncategorized) ||
		a.Payee != nil ||
		a.Notes != nil ||
		len(a.AddTagIDs) > 0 ||
		(a.MarkAsReviewed != nil && *a.MarkAsReviewed) ||
		(a.MarkAsUnreviewed != nil && *a.MarkAsUnreviewed) ||
		(a.ShouldSendEmail != nil && *a.ShouldSendEmail) ||
		(a.ShouldDelete != nil && *a.ShouldDelete) ||
		len(a.ShouldSplit) > 0 ||
		(a.SkipRecurring != nil && *a.SkipRecurring) ||
		(a.DontRunRules != nil && *a.DontRunRules) ||
		(a.StopProcessingOthers != nil && *a.StopProcessingOthers)
}

func newRulesCreateCmd(flags *rootFlags) *cobra.Command {
	var fl ruleCreateFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction rule",
		Example: `  rules create --payee "Gregory" --match contain --category-id 1768448 --stop --desc "Lawyer fees"
  rules create --payee "ONLINE DOMESTIC WIRE TRANSFER" --match contain --clear-category --mark-unreviewed --priority 20 --desc "Force review of unknown wires"
  rules create --payee "Amazon" --match contain --add-tag-id 262245 --set-notes "verify"
  rules create --payee "ZZZZ_DELETE_ME" --match exact --should-delete --priority 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := fl.toSpec()
			if err != nil {
				return err
			}
			// PATCH: Honor global dry-run before constructing the internal
			// cookie client so rule writes can be audited safely offline.
			if dryRunOK(flags) {
				return writeRulesDryRun(cmd, "create", 0, spec, nil)
			}
			c, err := newInternalClientForRules()
			if err != nil {
				return err
			}
			r, err := c.CreateRule(spec)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created rule_id=%d criteria_id=%d action=%s\n",
				r.RuleID, r.RuleCriteriaID, r.ActionSummary())
			if fl.apply {
				results, err := c.ApplyRules([]int64{r.RuleCriteriaID}, nil)
				if err != nil {
					return fmt.Errorf("rule created but apply failed: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied to %d transactions\n", len(results))
			}
			return nil
		},
	}
	fl.bindCreate(cmd)
	return cmd
}

func newRulesUpdateCmd(flags *rootFlags) *cobra.Command {
	var fl ruleCreateFlags
	cmd := &cobra.Command{
		Use:   "update <criteria_id>",
		Short: "Update an existing rule by criteria_id (PUT /rules/{criteria_id})",
		Long: `Update a rule's conditions and actions. The internal API supports PUT
on /rules/{criteria_id}; the CLI fetches the existing rule first and preserves
fields you do not override with flags.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: rules update <criteria_id> [flags]")
			}
			cid, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid criteria_id: %w", err)
			}
			// PATCH: Dry-run update previews the criteria id and changed
			// flags without fetching or mutating the live internal API.
			if dryRunOK(flags) {
				return writeRulesDryRun(cmd, "update", cid, nil, changedFlagValues(cmd))
			}
			c, err := newInternalClientForRules()
			if err != nil {
				return err
			}
			existing, err := c.GetRule(cid)
			if err != nil {
				return err
			}
			// PATCH: Preserve existing rule fields so action-only updates do
			// not require retyping the payee/match/priority condition.
			spec, err := fl.toUpdateSpec(existing, cmd)
			if err != nil {
				return err
			}
			r, err := c.UpdateRule(cid, spec)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated criteria_id=%d (rule_id=%d) action=%s\n",
				r.RuleCriteriaID, r.RuleID, r.ActionSummary())
			return nil
		},
	}
	fl.bindCreate(cmd)
	_ = cmd.Flags().MarkHidden("apply")
	return cmd
}

func newRulesDeleteCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <criteria_id>...",
		Short: "Delete one or more rules by criteria_id (bulk)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("provide at least one criteria_id")
			}
			ids := make([]int64, 0, len(args))
			for _, a := range args {
				n, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid criteria_id %q: %w", a, err)
				}
				ids = append(ids, n)
			}
			if dryRunOK(flags) {
				return writeRulesDryRun(cmd, "delete", 0, nil, map[string]any{"criteria_ids": ids})
			}
			confirmed := yes || (flags != nil && flags.yes)
			if !confirmed {
				return usageErr(fmt.Errorf("delete requires --yes to confirm %d rule(s)", len(ids)))
			}
			c, err := newInternalClientForRules()
			if err != nil {
				return err
			}
			if err := c.DeleteRules(ids); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %d rule(s): %s\n", len(ids), strings.Join(args, ","))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}

func newRulesApplyCmd(flags *rootFlags) *cobra.Command {
	var dryRun bool
	var commit bool
	var txArg string
	// PATCH: Tell operators that multi-id apply is serialized because the
	// backend misapplies batched commit requests.
	cmd := &cobra.Command{
		Use:   "apply <criteria_id>...",
		Short: "Run rules against existing transactions (dry-run by default)",
		Long: `Apply rules to historical transactions. Dry-run by default for safety.
Pass --commit to actually mutate transactions.

CAVEAT: dry-run evaluates each rule in isolation — it does NOT simulate
the priority chain or stop_processing_others. To get realistic preview
behavior, apply rules in priority order. Multi-id invocations are serialized
one criteria_id per backend request because the backend is unsafe when batched.`,
		Example: `  rules apply 11062614 --dry-run
  rules apply 11062614 --commit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("provide at least one criteria_id")
			}
			if cmd.Flags().Changed("dry-run") && !dryRun && !commit {
				return usageErr(fmt.Errorf("--dry-run=false requires --commit; omit --commit to preview"))
			}
			if dryRunOK(flags) {
				dryRun = true
				commit = false
			}
			ids := make([]int64, 0, len(args))
			for _, a := range args {
				n, err := strconv.ParseInt(a, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid criteria_id %q: %w", a, err)
				}
				ids = append(ids, n)
			}
			var txIDs []int64
			if txArg != "" {
				for _, p := range strings.Split(txArg, ",") {
					p = strings.TrimSpace(p)
					if p == "" {
						continue
					}
					n, err := strconv.ParseInt(p, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid tx_id %q: %w", p, err)
					}
					txIDs = append(txIDs, n)
				}
			}
			c, err := newInternalClientForRules()
			if err != nil {
				return err
			}
			effectiveDryRun := dryRun && !commit
			var results []map[string]any
			// PATCH: Serialize apply calls one criteria_id per request. The
			// internal backend misroutes batched commit calls to the final
			// criteria id, which can recategorize unrelated transactions.
			for _, id := range ids {
				var batch []map[string]any
				if effectiveDryRun {
					batch, err = c.ApplyRulesDryRun([]int64{id}, txIDs)
				} else {
					batch, err = c.ApplyRules([]int64{id}, txIDs)
				}
				if err != nil {
					return err
				}
				results = append(results, batch...)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			mode := "dry-run"
			if !effectiveDryRun {
				mode = "applied"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d transaction(s) would be / were affected\n", mode, len(results))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "Preview without applying (default true)")
	cmd.Flags().BoolVar(&commit, "commit", false, "Actually apply (overrides --dry-run)")
	cmd.Flags().StringVar(&txArg, "tx-ids", "", "Comma-separated transaction IDs to restrict apply to")
	return cmd
}

// PATCH: Keep internal rules dry-runs structured and independent of live cookies.
func writeRulesDryRun(cmd *cobra.Command, action string, criteriaID int64, spec any, extra map[string]any) error {
	payload := map[string]any{
		"dry_run": true,
		"success": false,
		"status":  0,
		"action":  action,
	}
	if criteriaID != 0 {
		payload["criteria_id"] = criteriaID
	}
	if spec != nil {
		payload["spec"] = spec
	}
	for k, v := range extra {
		payload[k] = v
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
}

func changedFlagValues(cmd *cobra.Command) map[string]any {
	changed := map[string]any{}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		changed[f.Name] = f.Value.String()
	})
	return changed
}
