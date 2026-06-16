package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
	"github.com/mvanhorn/printing-press-library/library/marketing/ads-intel/internal/store"
	"github.com/spf13/cobra"
)

const applyConfirmPrefix = "APPLY GOOGLE ADS EXACT NEGATIVES "
const undoConfirmPrefix = "UNDO GOOGLE ADS EXACT NEGATIVE "

type applyOptions struct {
	draftPaths       []string
	allowAccounts    []string
	policyPath       string
	maxChangesPerRun int
	liveApproved     bool
	confirm          string
	childBinary      string
}

type applyPolicy struct {
	SchemaVersion            string   `json:"schema_version,omitempty"`
	AllowedGoogleAdsAccounts []string `json:"allowed_google_ads_accounts"`
	MaxChangesPerRun         int      `json:"max_changes_per_run,omitempty"`
}

type negativeDraft struct {
	DraftType        string                `json:"draft_type"`
	ReadOnly         bool                  `json:"read_only"`
	Finding          finding               `json:"finding"`
	Target           negativeDraftTarget   `json:"target"`
	Evidence         negativeDraftEvidence `json:"evidence"`
	ApplyConstraints map[string]any        `json:"apply_constraints"`
	Raw              map[string]any        `json:"-"`
	ConfirmedLocal   bool                  `json:"-"`
}

type negativeDraftTarget struct {
	Platform     string `json:"platform"`
	CustomerID   string `json:"customer_id"`
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name,omitempty"`
	AdGroupID    string `json:"ad_group_id,omitempty"`
	AdGroupName  string `json:"ad_group_name,omitempty"`
	KeywordText  string `json:"keyword_text"`
	MatchType    string `json:"match_type"`
}

type negativeDraftEvidence struct {
	SourceCheck         string  `json:"source_check"`
	Spend               float64 `json:"spend"`
	Conversions         float64 `json:"conversions"`
	Clicks              int     `json:"clicks,omitempty"`
	MinimumSpend        float64 `json:"minimum_spend"`
	RequiredConversions float64 `json:"required_conversions"`
}

type resolvedNegativeTarget struct {
	CustomerID   string `json:"customer_id"`
	Scope        string `json:"scope"`
	CampaignID   string `json:"campaign_id"`
	CampaignName string `json:"campaign_name,omitempty"`
	AdGroupID    string `json:"ad_group_id,omitempty"`
	AdGroupName  string `json:"ad_group_name,omitempty"`
	KeywordText  string `json:"keyword_text"`
	MatchType    string `json:"match_type"`
}

type negativeCriterion struct {
	ResourceName string `json:"resource_name"`
	Scope        string `json:"scope"`
	CampaignID   string `json:"campaign_id,omitempty"`
	AdGroupID    string `json:"ad_group_id,omitempty"`
	KeywordText  string `json:"keyword_text"`
	MatchType    string `json:"match_type"`
}

type mutationResult struct {
	ResourceName string `json:"resource_name"`
	Raw          string `json:"raw,omitempty"`
}

type applyPlan struct {
	DraftPath string                 `json:"draft_path"`
	Draft     negativeDraft          `json:"draft"`
	Target    resolvedNegativeTarget `json:"target"`
	Operation map[string]any         `json:"operation"`
	Inverse   map[string]any         `json:"inverse"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Existing  *negativeCriterion     `json:"existing,omitempty"`
	Result    *mutationResult        `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Snapshot  string                 `json:"snapshot_path,omitempty"`
	Reversal  string                 `json:"reversal_id,omitempty"`
}

type applyResult struct {
	SchemaVersion string      `json:"schema_version"`
	Mode          string      `json:"mode"`
	AccountID     string      `json:"account_id"`
	ChangeCap     int         `json:"max_changes_per_run"`
	Plans         []applyPlan `json:"plans"`
	UndoNote      string      `json:"undo_note"`
}

type googleAdsApplyClient interface {
	ResolveNegativeTarget(d negativeDraft, dset store.DataSet) (resolvedNegativeTarget, error)
	ListExactNegatives(target resolvedNegativeTarget) ([]negativeCriterion, error)
	AddExactNegative(target resolvedNegativeTarget) (mutationResult, error)
	RemoveNegative(customerID, scope, resourceName string) (mutationResult, error)
}

var newGoogleAdsApplyClient = func(binary string) googleAdsApplyClient {
	return shellGoogleAdsApplyClient{binary: binary}
}

func applyCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "apply", Short: "Dry-run or apply one hardened paid-media mutation"}
	cmd.AddCommand(applyNegativeKeywordCmd(f), applyUndoCmd(f))
	return cmd
}

func applyNegativeKeywordCmd(f *rootFlags) *cobra.Command {
	opts := applyOptions{maxChangesPerRun: 1, childBinary: "google-ads-pp-cli"}
	c := &cobra.Command{
		Use:   "negative-keyword",
		Short: "Apply exact-match Google Ads negative keyword drafts",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.draftPaths = append(opts.draftPaths, args...)
			if opts.policyPath == "" {
				opts.policyPath = filepath.Join(f.home, "apply-policy.json")
			}
			return runApplyNegativeKeyword(cmd, f, opts)
		},
	}
	c.Flags().StringArrayVar(&opts.draftPaths, "draft", nil, "Path to a negative_keyword_candidate draft; repeat for a batch")
	c.Flags().StringArrayVar(&opts.allowAccounts, "allow-account", nil, "Allow a Google Ads customer ID for this run; repeat as needed")
	c.Flags().StringVar(&opts.policyPath, "policy", "", "Apply policy JSON path with allowed_google_ads_accounts and max_changes_per_run")
	c.Flags().IntVar(&opts.maxChangesPerRun, "max-changes-per-run", 1, "Maximum live changes allowed in this run")
	c.Flags().BoolVar(&opts.liveApproved, "live-approved", false, "Actually mutate after all gates pass")
	c.Flags().StringVar(&opts.confirm, "confirm", "", "Typed confirmation required for --live-approved")
	c.Flags().StringVar(&opts.childBinary, "google-ads-binary", "google-ads-pp-cli", "Google Ads child CLI binary")
	return c
}

func runApplyNegativeKeyword(cmd *cobra.Command, f *rootFlags, opts applyOptions) error {
	if len(opts.draftPaths) == 0 {
		return fmt.Errorf("at least one --draft path is required")
	}
	dset, err := load(f)
	if err != nil {
		return err
	}
	conf := confidenceReportForData(dset)
	if conf.BlocksDerivedMetrics() {
		return fmt.Errorf("refusing apply: tracking confidence is %s; fix confidence before mutation", conf.Level)
	}
	policy, err := loadApplyPolicy(opts.policyPath)
	if err != nil {
		return err
	}
	if opts.maxChangesPerRun <= 0 {
		return fmt.Errorf("refusing apply: --max-changes-per-run must be at least 1")
	}
	if policy.MaxChangesPerRun > 0 && !cmd.Flags().Changed("max-changes-per-run") {
		opts.maxChangesPerRun = policy.MaxChangesPerRun
	}
	allowed := allowedAccountSet(policy, opts.allowAccounts)
	client := newGoogleAdsApplyClient(opts.childBinary)

	plans := []applyPlan{}
	accountID := ""
	for _, path := range opts.draftPaths {
		draft, err := loadNegativeDraft(path, dset)
		if err != nil {
			return err
		}
		if err := validateNegativeDraft(draft); err != nil {
			return err
		}
		target := resolveNegativeTargetFromDraft(draft, dset)
		if opts.liveApproved {
			var err error
			target, err = client.ResolveNegativeTarget(draft, dset)
			if err != nil {
				return err
			}
		}
		if target.Scope == "" {
			return fmt.Errorf("could not resolve campaign/ad-group target for exact negative %q", draft.Target.KeywordText)
		}
		if accountID == "" {
			accountID = target.CustomerID
		}
		if accountID != target.CustomerID {
			return fmt.Errorf("refusing apply: one run may target only one account (%s and %s)", accountID, target.CustomerID)
		}
		if !allowed[target.CustomerID] {
			return fmt.Errorf("refusing apply: Google Ads account %s is not in the apply allowlist", target.CustomerID)
		}
		plans = append(plans, applyPlan{
			DraftPath: path,
			Draft:     draft,
			Target:    target,
			Operation: addNegativeOperation(target),
			Inverse:   inverseNegativeOperation(target, ""),
			Status:    "planned",
		})
	}
	if len(plans) > opts.maxChangesPerRun {
		return fmt.Errorf("refusing apply: %d planned changes exceeds max-changes-per-run cap %d", len(plans), opts.maxChangesPerRun)
	}
	if opts.liveApproved {
		want := applyConfirmPrefix + accountID
		if strings.TrimSpace(opts.confirm) != want {
			return fmt.Errorf("refusing live apply: typed confirmation must be exactly %q", want)
		}
	}

	mode := "dry-run"
	var unlock func()
	if opts.liveApproved {
		mode = "live-approved"
		unlock, err = acquireAccountLock(f.home, accountID)
		if err != nil {
			return err
		}
		defer unlock()
	}

	for i := range plans {
		if !opts.liveApproved {
			plans[i].Status = "dry-run"
			plans[i].Message = "dry-run only; no write executed"
			continue
		}
		existing, err := client.ListExactNegatives(plans[i].Target)
		if err != nil {
			plans[i].Status = "failed"
			plans[i].Error = err.Error()
			continue
		}
		snapshotPath, snapErr := writeApplySnapshot(f.home, f.profile, accountID, plans[i].Target, existing)
		if snapErr != nil {
			plans[i].Status = "failed"
			plans[i].Error = snapErr.Error()
			continue
		}
		plans[i].Snapshot = snapshotPath
		if match := findExistingNegative(existing, plans[i].Target); match != nil {
			plans[i].Status = "skipped"
			plans[i].Message = "exact negative already exists; idempotent no-op"
			plans[i].Existing = match
			continue
		}
		res, err := client.AddExactNegative(plans[i].Target)
		if err != nil {
			plans[i].Status = "failed"
			plans[i].Error = err.Error()
			continue
		}
		plans[i].Result = &res
		plans[i].Inverse = inverseNegativeOperation(plans[i].Target, res.ResourceName)
		after, err := client.ListExactNegatives(plans[i].Target)
		if err != nil {
			plans[i].Status = "failed"
			plans[i].Error = "read-after-write verification query failed: " + err.Error()
			continue
		}
		if match := findExistingNegative(after, plans[i].Target); match == nil {
			plans[i].Status = "failed"
			plans[i].Error = "read-after-write verification failed: exact negative was not found"
			continue
		}
		revID, err := writeReversal(f.home, f.profile, accountID, plans[i])
		if err != nil {
			plans[i].Status = "failed"
			plans[i].Error = err.Error()
			continue
		}
		plans[i].Reversal = revID
		plans[i].Status = "applied"
		if err := appendApplyAudit(f.home, "apply", accountID, plans[i]); err != nil {
			plans[i].Status = "failed"
			plans[i].Error = err.Error()
			continue
		}
	}

	res := applyResult{
		SchemaVersion: "ads-intel.apply/v1",
		Mode:          mode,
		AccountID:     accountID,
		ChangeCap:     opts.maxChangesPerRun,
		Plans:         plans,
		UndoNote:      "Undo is best-effort, not guaranteed; negative keywords can change auction eligibility and effects are not a clean rollback.",
	}
	return out(cmd, f, res, formatApplyResult(res))
}

type reversalEntry struct {
	SchemaVersion string                 `json:"schema_version"`
	ID            string                 `json:"id"`
	CreatedAt     time.Time              `json:"created_at"`
	Profile       string                 `json:"profile"`
	AccountID     string                 `json:"account_id"`
	DraftPath     string                 `json:"draft_path"`
	Target        resolvedNegativeTarget `json:"target"`
	Inverse       map[string]any         `json:"inverse"`
	ResourceName  string                 `json:"resource_name"`
	UndoCommand   string                 `json:"undo_command"`
	BestEffort    bool                   `json:"best_effort"`
}

func applyUndoCmd(f *rootFlags) *cobra.Command {
	opts := applyOptions{maxChangesPerRun: 1, childBinary: "google-ads-pp-cli"}
	var reversalID string
	c := &cobra.Command{
		Use:   "undo --reversal-id <id>",
		Short: "Best-effort undo for a recorded exact negative keyword apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.policyPath == "" {
				opts.policyPath = filepath.Join(f.home, "apply-policy.json")
			}
			return runApplyUndo(cmd, f, opts, reversalID)
		},
	}
	c.Flags().StringVar(&reversalID, "reversal-id", "", "Reversal ID from a prior live apply")
	c.Flags().StringArrayVar(&opts.allowAccounts, "allow-account", nil, "Allow a Google Ads customer ID for this run; repeat as needed")
	c.Flags().StringVar(&opts.policyPath, "policy", "", "Apply policy JSON path with allowed_google_ads_accounts")
	c.Flags().BoolVar(&opts.liveApproved, "live-approved", false, "Actually remove the recorded exact negative after all gates pass")
	c.Flags().StringVar(&opts.confirm, "confirm", "", "Typed confirmation required for --live-approved")
	c.Flags().StringVar(&opts.childBinary, "google-ads-binary", "google-ads-pp-cli", "Google Ads child CLI binary")
	return c
}

func runApplyUndo(cmd *cobra.Command, f *rootFlags, opts applyOptions, reversalID string) error {
	if strings.TrimSpace(reversalID) == "" {
		return fmt.Errorf("--reversal-id is required")
	}
	entry, err := findReversal(f.home, f.profile, reversalID)
	if err != nil {
		return err
	}
	policy, err := loadApplyPolicy(opts.policyPath)
	if err != nil {
		return err
	}
	if !allowedAccountSet(policy, opts.allowAccounts)[entry.AccountID] {
		return fmt.Errorf("refusing undo: Google Ads account %s is not in the apply allowlist", entry.AccountID)
	}
	if opts.liveApproved {
		want := undoConfirmPrefix + entry.AccountID
		if strings.TrimSpace(opts.confirm) != want {
			return fmt.Errorf("refusing live undo: typed confirmation must be exactly %q", want)
		}
	}
	plan := applyPlan{
		DraftPath: entry.DraftPath,
		Target:    entry.Target,
		Operation: entry.Inverse,
		Inverse:   addNegativeOperation(entry.Target),
		Status:    "dry-run",
		Message:   "best-effort undo dry-run only; no write executed",
		Reversal:  entry.ID,
	}
	mode := "dry-run"
	if opts.liveApproved {
		mode = "live-approved"
		unlock, err := acquireAccountLock(f.home, entry.AccountID)
		if err != nil {
			return err
		}
		defer unlock()
		res, err := newGoogleAdsApplyClient(opts.childBinary).RemoveNegative(entry.AccountID, entry.Target.Scope, entry.ResourceName)
		if err != nil {
			plan.Status = "failed"
			plan.Error = err.Error()
		} else {
			plan.Status = "applied"
			plan.Result = &res
			if err := appendApplyAudit(f.home, "undo", entry.AccountID, plan); err != nil {
				plan.Status = "failed"
				plan.Error = err.Error()
			}
		}
	}
	res := applyResult{
		SchemaVersion: "ads-intel.apply/v1",
		Mode:          mode,
		AccountID:     entry.AccountID,
		ChangeCap:     1,
		Plans:         []applyPlan{plan},
		UndoNote:      "Undo is best-effort, not guaranteed; negative keywords can change auction eligibility and effects are not a clean rollback.",
	}
	return out(cmd, f, res, formatApplyResult(res))
}

func loadApplyPolicy(path string) (applyPolicy, error) {
	p := applyPolicy{SchemaVersion: "ads-intel.apply-policy/v1"}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return p, nil
	}
	return p, json.Unmarshal(b, &p)
}

func allowedAccountSet(policy applyPolicy, flags []string) map[string]bool {
	out := map[string]bool{}
	for _, account := range append(policy.AllowedGoogleAdsAccounts, flags...) {
		if strings.TrimSpace(account) != "" {
			out[strings.TrimSpace(account)] = true
		}
	}
	return out
}

func loadNegativeDraft(path string, dset store.DataSet) (negativeDraft, error) {
	var raw map[string]any
	b, err := os.ReadFile(path)
	if err != nil {
		return negativeDraft{}, err
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return negativeDraft{}, err
	}
	var d negativeDraft
	if err := json.Unmarshal(b, &d); err != nil {
		return negativeDraft{}, err
	}
	d.Raw = raw
	if d.Target.KeywordText == "" {
		d.Target.KeywordText = strings.TrimPrefix(d.Finding.Title, "Negative keyword draft: ")
	}
	if d.Target.CustomerID == "" {
		d.Target.CustomerID = dset.Account.AccountID
	}
	if d.Target.MatchType == "" {
		d.Target.MatchType = "EXACT"
	}
	for _, row := range dset.SearchTerms {
		if row.Platform == "google" && strings.EqualFold(row.Term, d.Target.KeywordText) {
			if d.Target.CampaignID == "" {
				d.Target.CampaignID = row.CampaignID
				d.Target.CampaignName = row.CampaignName
			}
			if d.Target.AdGroupID == "" {
				d.Target.AdGroupID = row.AdGroupID
				d.Target.AdGroupName = row.AdGroupName
			}
			if d.Evidence.Spend == 0 {
				d.Evidence.Spend = row.Spend
			}
			d.Evidence.Conversions = row.Conversions
			if d.Evidence.Clicks == 0 {
				d.Evidence.Clicks = row.Clicks
			}
			d.ConfirmedLocal = true
			break
		}
	}
	return d, nil
}

func validateNegativeDraft(d negativeDraft) error {
	if d.DraftType != "negative_keyword_candidate" {
		return fmt.Errorf("refusing apply: unsupported draft_type %q", d.DraftType)
	}
	if strings.EqualFold(d.Target.Platform, "") {
		d.Target.Platform = d.Finding.Platform
	}
	if !strings.EqualFold(d.Target.Platform, "google") && !strings.EqualFold(d.Finding.Platform, "google") {
		return fmt.Errorf("refusing apply: only Google Ads negative keyword drafts are supported")
	}
	if !strings.EqualFold(d.Target.MatchType, "EXACT") {
		return fmt.Errorf("refusing apply: only EXACT match negatives are supported")
	}
	if strings.TrimSpace(d.Target.KeywordText) == "" {
		return fmt.Errorf("refusing apply: draft has no keyword_text")
	}
	if !d.ConfirmedLocal {
		return fmt.Errorf("refusing apply: draft term is not present in the current local read-only audit dataset")
	}
	if d.Evidence.Spend <= 10 || d.Evidence.Conversions != 0 {
		return fmt.Errorf("refusing apply: term is not confirmed wasted spend (requires >$10 spend and 0 conversions)")
	}
	return nil
}

func resolveNegativeTargetFromDraft(d negativeDraft, dset store.DataSet) resolvedNegativeTarget {
	target := resolvedNegativeTarget{
		CustomerID:   d.Target.CustomerID,
		CampaignID:   d.Target.CampaignID,
		CampaignName: d.Target.CampaignName,
		AdGroupID:    d.Target.AdGroupID,
		AdGroupName:  d.Target.AdGroupName,
		KeywordText:  d.Target.KeywordText,
		MatchType:    "EXACT",
	}
	if target.CustomerID == "" {
		target.CustomerID = dset.Account.AccountID
	}
	if target.AdGroupID != "" {
		target.Scope = "ad_group"
	} else if target.CampaignID != "" {
		target.Scope = "campaign"
	}
	return target
}

func addNegativeOperation(t resolvedNegativeTarget) map[string]any {
	create := map[string]any{"negative": true, "keyword": map[string]any{"text": t.KeywordText, "matchType": "EXACT"}}
	if t.Scope == "ad_group" {
		create["adGroup"] = fmt.Sprintf("customers/%s/adGroups/%s", t.CustomerID, t.AdGroupID)
		return map[string]any{"endpoint": "customers-ad-group-criteria", "operations": []map[string]any{{"create": create}}}
	}
	create["campaign"] = fmt.Sprintf("customers/%s/campaigns/%s", t.CustomerID, t.CampaignID)
	return map[string]any{"endpoint": "customers-campaign-criteria", "operations": []map[string]any{{"create": create}}}
}

func inverseNegativeOperation(t resolvedNegativeTarget, resourceName string) map[string]any {
	inverse := map[string]any{"endpoint": "customers-campaign-criteria", "operations": []map[string]any{{"remove": resourceName}}}
	if t.Scope == "ad_group" {
		inverse["endpoint"] = "customers-ad-group-criteria"
	}
	if resourceName == "" {
		inverse["description"] = "remove the exact negative criterion created for this term after the create response returns its resource name"
	}
	return inverse
}

func findExistingNegative(items []negativeCriterion, target resolvedNegativeTarget) *negativeCriterion {
	for i := range items {
		item := items[i]
		if strings.EqualFold(item.KeywordText, target.KeywordText) && strings.EqualFold(item.MatchType, "EXACT") && item.Scope == target.Scope {
			if target.Scope == "ad_group" && item.AdGroupID == target.AdGroupID {
				return &item
			}
			if target.Scope == "campaign" && item.CampaignID == target.CampaignID {
				return &item
			}
		}
	}
	return nil
}

func writeApplySnapshot(home, profile, accountID string, target resolvedNegativeTarget, existing []negativeCriterion) (string, error) {
	dir := filepath.Join(home, "snapshots", "apply", intelcli.SafeName(profile))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000")+"-"+intelcli.SafeName(accountID)+".json")
	body := map[string]any{
		"schema_version": "ads-intel.apply-snapshot/v1",
		"created_at":     time.Now().UTC(),
		"account_id":     accountID,
		"target":         target,
		"negatives":      existing,
	}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}

func writeReversal(home, profile, accountID string, plan applyPlan) (string, error) {
	resourceName := ""
	if plan.Result != nil {
		resourceName = plan.Result.ResourceName
	}
	id := intelcli.SafeName(accountID + "-" + plan.Target.Scope + "-" + plan.Target.KeywordText + "-" + time.Now().UTC().Format("20060102T150405"))
	entry := reversalEntry{
		SchemaVersion: "ads-intel.reversal/v1",
		ID:            id,
		CreatedAt:     time.Now().UTC(),
		Profile:       profile,
		AccountID:     accountID,
		DraftPath:     plan.DraftPath,
		Target:        plan.Target,
		Inverse:       plan.Inverse,
		ResourceName:  resourceName,
		UndoCommand:   fmt.Sprintf("ads-intel-pp-cli --profile %s apply undo --reversal-id %s", profile, id),
		BestEffort:    true,
	}
	return id, appendJSONL(filepath.Join(home, "reversals", intelcli.SafeName(profile)+".jsonl"), entry)
}

func findReversal(home, profile, id string) (reversalEntry, error) {
	path := filepath.Join(home, "reversals", intelcli.SafeName(profile)+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return reversalEntry{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var entry reversalEntry
		if json.Unmarshal(sc.Bytes(), &entry) == nil && entry.ID == id {
			return entry, nil
		}
	}
	if err := sc.Err(); err != nil {
		return reversalEntry{}, err
	}
	return reversalEntry{}, fmt.Errorf("reversal %q not found", id)
}

func appendApplyAudit(home, action, accountID string, plan applyPlan) error {
	entry := map[string]any{
		"schema_version": "ads-intel.apply-audit/v1",
		"at":             time.Now().UTC(),
		"action":         action,
		"account_id":     accountID,
		"target":         plan.Target,
		"operation":      plan.Operation,
		"inverse":        plan.Inverse,
		"status":         plan.Status,
		"draft_path":     plan.DraftPath,
		"result":         plan.Result,
	}
	return appendJSONL(filepath.Join(home, "audit", "apply.log"), entry)
}

func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func acquireAccountLock(home, accountID string) (func(), error) {
	dir := filepath.Join(home, "locks", "google-ads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, intelcli.SafeName(accountID)+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("account %s is already locked for apply: %w", accountID, err)
	}
	_, _ = fmt.Fprintf(f, "pid=%d at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}

func formatApplyResult(res applyResult) string {
	lines := []string{
		fmt.Sprintf("mode: %s", res.Mode),
		fmt.Sprintf("account: %s", res.AccountID),
		fmt.Sprintf("max changes: %d", res.ChangeCap),
	}
	for _, plan := range res.Plans {
		lines = append(lines,
			fmt.Sprintf("status: %s", plan.Status),
			fmt.Sprintf("target: customer=%s scope=%s campaign=%s ad_group=%s keyword=[%s] match=EXACT", plan.Target.CustomerID, plan.Target.Scope, plan.Target.CampaignID, plan.Target.AdGroupID, plan.Target.KeywordText),
			fmt.Sprintf("operation: %s", mustCompactJSON(plan.Operation)),
			fmt.Sprintf("inverse: %s", mustCompactJSON(plan.Inverse)),
		)
		if plan.Snapshot != "" {
			lines = append(lines, "snapshot: "+plan.Snapshot)
		}
		if plan.Reversal != "" {
			lines = append(lines, "reversal: "+plan.Reversal)
		}
		if plan.Message != "" {
			lines = append(lines, "message: "+plan.Message)
		}
		if plan.Error != "" {
			lines = append(lines, "error: "+plan.Error)
		}
	}
	lines = append(lines, "undo: "+res.UndoNote)
	if res.Mode == "dry-run" {
		lines = append(lines, "DRY-RUN: no writes executed.")
	}
	return strings.Join(lines, "\n") + "\n"
}

func mustCompactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

type shellGoogleAdsApplyClient struct{ binary string }

func (c shellGoogleAdsApplyClient) ResolveNegativeTarget(d negativeDraft, dset store.DataSet) (resolvedNegativeTarget, error) {
	target := resolvedNegativeTarget{
		CustomerID:   d.Target.CustomerID,
		CampaignID:   d.Target.CampaignID,
		CampaignName: d.Target.CampaignName,
		AdGroupID:    d.Target.AdGroupID,
		AdGroupName:  d.Target.AdGroupName,
		KeywordText:  d.Target.KeywordText,
		MatchType:    "EXACT",
	}
	if target.CustomerID == "" {
		target.CustomerID = dset.Account.AccountID
	}
	query := fmt.Sprintf("SELECT campaign.id, campaign.name, ad_group.id, ad_group.name, search_term_view.search_term FROM search_term_view WHERE search_term_view.search_term = '%s' LIMIT 1", gaqlQuote(target.KeywordText))
	data, err := c.runJSON("--json", "--compact", "--no-input", "customers-google-ads", "search", target.CustomerID, "--query", query)
	if err == nil {
		mergeResolvedTargetFromSearch(&target, data)
	}
	if target.AdGroupID != "" {
		target.Scope = "ad_group"
	} else if target.CampaignID != "" {
		target.Scope = "campaign"
	} else {
		return target, fmt.Errorf("could not resolve campaign/ad-group target for exact negative %q", target.KeywordText)
	}
	return target, nil
}

func (c shellGoogleAdsApplyClient) ListExactNegatives(target resolvedNegativeTarget) ([]negativeCriterion, error) {
	query := negativeLookupGAQL(target)
	data, err := c.runJSON("--json", "--compact", "--no-input", "customers-google-ads", "search", target.CustomerID, "--query", query)
	if err != nil {
		return nil, err
	}
	return parseNegativeCriteria(data, target), nil
}

func (c shellGoogleAdsApplyClient) AddExactNegative(target resolvedNegativeTarget) (mutationResult, error) {
	operation := addNegativeOperation(target)
	return c.mutate(target.CustomerID, operation)
}

func (c shellGoogleAdsApplyClient) RemoveNegative(customerID, scope, resourceName string) (mutationResult, error) {
	if strings.TrimSpace(resourceName) == "" {
		return mutationResult{}, fmt.Errorf("recorded reversal has no resource name to remove")
	}
	endpoint := "customers-campaign-criteria"
	if scope == "ad_group" {
		endpoint = "customers-ad-group-criteria"
	}
	return c.mutate(customerID, map[string]any{"endpoint": endpoint, "operations": []map[string]any{{"remove": resourceName}}})
}

func (c shellGoogleAdsApplyClient) mutate(customerID string, operation map[string]any) (mutationResult, error) {
	ops, _ := json.Marshal(operation["operations"])
	endpoint, _ := operation["endpoint"].(string)
	data, err := c.runJSON("--json", "--no-input", endpoint, customerID, "--operations", string(ops), "--partial-failure", "--response-content-type", "RESOURCE_NAME_ONLY")
	if err != nil {
		return mutationResult{}, err
	}
	res := mutationResult{Raw: string(data)}
	res.ResourceName = firstResourceName(data)
	return res, nil
}

func (c shellGoogleAdsApplyClient) runJSON(args ...string) (json.RawMessage, error) {
	bin := c.binary
	if bin == "" {
		bin = "google-ads-pp-cli"
	}
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s failed: %w: %s", bin, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return json.RawMessage(bytes.TrimSpace(out)), nil
}

func gaqlQuote(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func negativeLookupGAQL(t resolvedNegativeTarget) string {
	if t.Scope == "ad_group" {
		return fmt.Sprintf("SELECT ad_group_criterion.resource_name, ad_group.id, ad_group_criterion.keyword.text, ad_group_criterion.keyword.match_type, ad_group_criterion.negative FROM ad_group_criterion WHERE ad_group.id = %s AND ad_group_criterion.negative = true AND ad_group_criterion.keyword.match_type = EXACT AND ad_group_criterion.keyword.text = '%s'", t.AdGroupID, gaqlQuote(t.KeywordText))
	}
	return fmt.Sprintf("SELECT campaign_criterion.resource_name, campaign.id, campaign_criterion.keyword.text, campaign_criterion.keyword.match_type, campaign_criterion.negative FROM campaign_criterion WHERE campaign.id = %s AND campaign_criterion.negative = true AND campaign_criterion.keyword.match_type = EXACT AND campaign_criterion.keyword.text = '%s'", t.CampaignID, gaqlQuote(t.KeywordText))
}

func mergeResolvedTargetFromSearch(target *resolvedNegativeTarget, data json.RawMessage) {
	for _, row := range flattenRows(data) {
		if campaign, ok := row["campaign"].(map[string]any); ok {
			if target.CampaignID == "" {
				target.CampaignID = stringify(campaign["id"])
			}
			if target.CampaignName == "" {
				target.CampaignName = stringify(campaign["name"])
			}
		}
		if adGroup, ok := row["adGroup"].(map[string]any); ok {
			if target.AdGroupID == "" {
				target.AdGroupID = stringify(adGroup["id"])
			}
			if target.AdGroupName == "" {
				target.AdGroupName = stringify(adGroup["name"])
			}
		}
	}
}

func parseNegativeCriteria(data json.RawMessage, target resolvedNegativeTarget) []negativeCriterion {
	out := []negativeCriterion{}
	for _, row := range flattenRows(data) {
		if crit, ok := row["adGroupCriterion"].(map[string]any); ok {
			keyword, _ := crit["keyword"].(map[string]any)
			adGroup, _ := row["adGroup"].(map[string]any)
			out = append(out, negativeCriterion{ResourceName: stringify(crit["resourceName"]), Scope: "ad_group", AdGroupID: stringify(adGroup["id"]), KeywordText: stringify(keyword["text"]), MatchType: strings.ToUpper(stringify(keyword["matchType"]))})
		}
		if crit, ok := row["campaignCriterion"].(map[string]any); ok {
			keyword, _ := crit["keyword"].(map[string]any)
			campaign, _ := row["campaign"].(map[string]any)
			out = append(out, negativeCriterion{ResourceName: stringify(crit["resourceName"]), Scope: "campaign", CampaignID: stringify(campaign["id"]), KeywordText: stringify(keyword["text"]), MatchType: strings.ToUpper(stringify(keyword["matchType"]))})
		}
	}
	return out
}

func flattenRows(data json.RawMessage) []map[string]any {
	var root any
	if json.Unmarshal(data, &root) != nil {
		return nil
	}
	rows := []map[string]any{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			if result, ok := x["results"]; ok {
				walk(result)
				return
			}
			if data, ok := x["data"]; ok {
				walk(data)
				return
			}
			if _, ok := x["campaign"]; ok {
				rows = append(rows, x)
				return
			}
			if _, ok := x["adGroupCriterion"]; ok {
				rows = append(rows, x)
				return
			}
			if _, ok := x["campaignCriterion"]; ok {
				rows = append(rows, x)
				return
			}
		}
	}
	walk(root)
	return rows
}

func firstResourceName(data json.RawMessage) string {
	var root any
	if json.Unmarshal(data, &root) != nil {
		return ""
	}
	var found string
	var walk func(any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			for k, val := range x {
				if strings.EqualFold(k, "resourceName") || strings.EqualFold(k, "resource_name") {
					found = stringify(val)
					return
				}
				walk(val)
			}
		}
	}
	walk(root)
	return found
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.0f", x), ".0"), ".")
	default:
		return fmt.Sprintf("%v", x)
	}
}
