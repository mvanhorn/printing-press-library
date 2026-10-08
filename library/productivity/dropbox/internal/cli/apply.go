// pp:data-source live
package cli

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type opCounts struct {
	OK      int `json:"ok"`
	Failed  int `json:"failed"`
	Unknown int `json:"unknown"`
	Skipped int `json:"skipped"`
}
type applyCounts struct {
	Mkdir  opCounts `json:"mkdir"`
	Move   opCounts `json:"move"`
	Delete opCounts `json:"delete"`
	Revoke opCounts `json:"revoke"`
}
type applyFailure struct {
	Seq   int    `json:"seq"`
	Op    string `json:"op"`
	Error string `json:"error"`
}
type applyResult struct {
	BatchID           string         `json:"batch_id"`
	Status            string         `json:"status"`
	Counts            applyCounts    `json:"counts"`
	Failures          []applyFailure `json:"failures"`
	RestoreWindowDays int            `json:"restore_window_days"`
}
type applyPreview struct {
	Would struct {
		Mkdir         int   `json:"mkdir"`
		MoveBatches   []int `json:"move_batches"`
		DeleteBatches []int `json:"delete_batches"`
		Revoke        int   `json:"revoke"`
	} `json:"would"`
	Check               dropbox.CheckReport `json:"check"`
	RequiresFlags       []string            `json:"requires_flags"`
	Irreversible        []string            `json:"irreversible"`
	IrreversibleOrLarge []string            `json:"irreversible_or_large"`
}

func batchSizes(n int) []int {
	out := make([]int, 0)
	for n > 0 {
		size := n
		if size > 1000 {
			size = 1000
		}
		out = append(out, size)
		n -= size
	}
	return out
}
func makeApplyPreview(p dropbox.Plan, r dropbox.CheckReport, allowDev, allowCross, allowNonempty, allowUnshare bool) applyPreview {
	v := applyPreview{Check: r, RequiresFlags: requiredApplyFlags(r, allowDev, allowCross, allowNonempty, allowUnshare), Irreversible: make([]string, 0), IrreversibleOrLarge: make([]string, 0)}
	moves, deletes := 0, 0
	covered := map[int]bool{}
	for _, item := range r.Results {
		if item.Status == "covered" {
			covered[item.Seq] = true
		}
		if item.Code == "delete_nonempty_attested" || item.Code == "delete_nonempty_allowed" {
			v.IrreversibleOrLarge = append(v.IrreversibleOrLarge, item.Code+": "+item.Message)
		}
	}
	for i, op := range p.Ops {
		if covered[i+1] {
			continue
		}
		switch op.Op {
		case "mkdir":
			v.Would.Mkdir++
		case "move":
			moves++
		case "delete":
			deletes++
		case "revoke_link":
			v.Would.Revoke++
		}
	}
	v.Would.MoveBatches = batchSizes(moves)
	v.Would.DeleteBatches = batchSizes(deletes)
	if v.Would.Revoke > 0 {
		v.Irreversible = append(v.Irreversible, "revoke_link")
		v.IrreversibleOrLarge = append(v.IrreversibleOrLarge, "revoke_link")
	}
	return v
}

func requiredApplyFlags(report dropbox.CheckReport, allowDev, allowCross, allowNonempty, allowUnshare bool) []string {
	needed := map[string]bool{}
	for _, item := range report.Results {
		switch item.Code {
		case "dev_dir":
			if !allowDev {
				needed["--allow-dev-dirs"] = true
			}
		case "cross_share":
			if !allowCross {
				needed["--allow-cross-share"] = true
			}
		case "delete_nonempty_folder":
			if !allowNonempty {
				needed["--allow-nonempty-delete"] = true
			}
		case "shared_folder_delete":
			if !allowUnshare {
				needed["--allow-unshare"] = true
			}
		}
	}
	out := make([]string, 0, 4)
	for _, name := range []string{"--allow-dev-dirs", "--allow-cross-share", "--allow-nonempty-delete", "--allow-unshare"} {
		if needed[name] {
			out = append(out, name)
		}
	}
	return out
}
func newBatchID() string {
	var suffix [2]byte
	_, _ = rand.Read(suffix[:])
	return time.Now().UTC().Format("20060102-150405-") + hex.EncodeToString(suffix[:])
}
func restoreDays(accountType string) int {
	if accountType == "business" {
		return 180
	}
	return 30
}

func refuseHarnessApply(cmd *cobra.Command, flags *rootFlags, action string) error {
	reason := action + " --yes is disabled under the Printing Press verify/dogfood harness"
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), struct {
			Refused bool   `json:"refused"`
			Reason  string `json:"reason"`
		}{true, reason}, flags)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), reason)
	return err
}

type dropboxBatchPoster struct {
	client  *client.Client
	headers map[string]string
}

func (p dropboxBatchPoster) Write(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return p.retry(ctx, func() (json.RawMessage, error) {
		raw, _, err := p.client.PostWithHeaders(ctx, path, body, p.headers)
		return raw, err
	})
}
func (p dropboxBatchPoster) Read(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return p.retry(ctx, func() (json.RawMessage, error) {
		raw, _, err := p.client.PostQueryWithParamsAndHeaders(ctx, path, nil, body, p.headers)
		return raw, err
	})
}
func (p dropboxBatchPoster) retry(ctx context.Context, call func() (json.RawMessage, error)) (json.RawMessage, error) {
	var raw json.RawMessage
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		raw, err = call()
		if err == nil {
			return raw, nil
		}
		var api *client.APIError
		if !errors.As(err, &api) || api.StatusCode != 429 && !dropbox.HasSummaryPrefix(err, "too_many_write_operations") {
			return nil, err
		}
		wait := time.Duration(100<<attempt) * time.Millisecond
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, err
}

type numberedOp struct {
	seq     int
	op      dropbox.Op
	tag     string
	entryID string
	rev     string
}

func groupedOps(p dropbox.Plan, kind string) []numberedOp {
	out := make([]numberedOp, 0)
	for i, op := range p.Ops {
		if op.Op == kind {
			out = append(out, numberedOp{seq: i + 1, op: op})
		}
	}
	return out
}
func caseOnlyMove(op dropbox.Op) bool {
	return op.Op == "move" && strings.EqualFold(op.From, op.To) && op.From != op.To
}
func resolveApplyItem(ctx context.Context, db *store.Store, item numberedOp) (numberedOp, error) {
	if item.op.Op == "mkdir" {
		item.tag = "folder"
		return item, nil
	}
	if item.op.Op != "move" && item.op.Op != "delete" {
		return item, nil
	}
	p := item.op.Path
	if item.op.Op == "move" {
		p = item.op.From
	}
	err := db.DB().QueryRowContext(ctx, `SELECT COALESCE(tag,''),COALESCE(id,''),COALESCE(rev,'') FROM dbx_files WHERE path_lower=?`, dropbox.PathKey(p)).Scan(&item.tag, &item.entryID, &item.rev)
	if err == sql.ErrNoRows {
		return item, fmt.Errorf("source is missing from refreshed index: %s", p)
	}
	if err != nil {
		return item, err
	}
	return item, nil
}
func batchEntries(raw json.RawMessage, want int) []error {
	var payload struct {
		Entries []json.RawMessage `json:"entries"`
	}
	_ = json.Unmarshal(raw, &payload)
	out := make([]error, want)
	for i := range out {
		if i >= len(payload.Entries) {
			out[i] = fmt.Errorf("batch omitted entry result")
			continue
		}
		var e struct {
			Tag string `json:".tag"`
		}
		if err := json.Unmarshal(payload.Entries[i], &e); err != nil {
			out[i] = err
		} else if e.Tag != "success" {
			out[i] = fmt.Errorf("Dropbox batch entry: %s", payload.Entries[i])
		}
	}
	return out
}
func (r *applyResult) record(kind string, seq int, state string, err error) {
	var c *opCounts
	switch kind {
	case "mkdir":
		c = &r.Counts.Mkdir
	case "move":
		c = &r.Counts.Move
	case "delete":
		c = &r.Counts.Delete
	default:
		c = &r.Counts.Revoke
	}
	switch state {
	case "skipped":
		c.Skipped++
	case "unknown":
		c.Unknown++
		r.Failures = append(r.Failures, applyFailure{seq, kind, err.Error()})
	case "failed":
		c.Failed++
		r.Failures = append(r.Failures, applyFailure{seq, kind, err.Error()})
	default:
		c.OK++
	}
}

func newNovelApplyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var yes, allowCrossShare, allowDevDirs, allowNonemptyDelete, allowUnshare, noRefresh bool
	var maxOps int
	cmd := &cobra.Command{Use: "apply <file>", Short: "Preview or execute a Dropbox change plan", Long: "Check a plan, then create folders, move and soft-delete entries in serialized batches, and journal results. Add --yes to execute remote changes.", Example: strings.Trim(`
  dropbox-pp-cli apply photos.json --agent
  dropbox-pp-cli apply photos.json --yes --agent`, "\n"), Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "file=testdata/dogfood-preview-plan.json", "pp:preview-happy-path": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "apply")
		}
		if len(args) != 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("apply requires one plan file"))
		}
		if maxOps <= 0 {
			return usageErr(fmt.Errorf("--max-ops must be positive"))
		}
		if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "live" {
			return usageErr(fmt.Errorf("apply requires live data source"))
		}
		p, err := dropbox.ReadPlan(args[0])
		if err != nil {
			return usageErr(err)
		}
		if len(p.Ops) == 0 {
			if yes {
				return printJSONFiltered(cmd.OutOrStdout(), applyResult{Status: "noop", Failures: make([]applyFailure, 0)}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), makeApplyPreview(p, dropbox.CheckReport{OK: true, Results: make([]dropbox.CheckResult, 0)}, allowDevDirs, allowCrossShare, allowNonemptyDelete, allowUnshare), flags)
		}
		if yes && cliutil.IsAnyHarness() {
			return refuseHarnessApply(cmd, flags, "apply")
		}
		ctx := cmd.Context()
		cancel := func() {}
		if cmd.Flags().Changed("timeout") {
			ctx, cancel = boundCtx(ctx, flags)
		}
		defer cancel()
		if dbPath == "" {
			dbPath = defaultDBPath("dropbox-pp-cli")
		}
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			report := dropbox.CheckReport{OK: false, Results: []dropbox.CheckResult{{Seq: 0, Op: "plan", Status: "error", Code: "index_missing", Message: "no local index; run: dropbox-pp-cli index"}}, Errors: 1}
			if !yes {
				_ = printJSONFiltered(cmd.OutOrStdout(), makeApplyPreview(p, report, allowDevDirs, allowCrossShare, allowNonemptyDelete, allowUnshare), flags)
			}
			return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
		} else if err != nil {
			return err
		}
		db, err := store.OpenWithContext(ctx, dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.EnsureDropboxSchema(ctx); err != nil {
			return err
		}
		if !yes {
			report, err := checkPlanAtIndex(ctx, db, p, dropbox.CheckOptions{MaxOps: maxOps, AllowNonemptyDelete: allowNonemptyDelete, AllowUnshare: allowUnshare, RequireComplete: noRefresh})
			if err != nil {
				return err
			}
			preview := makeApplyPreview(p, report, allowDevDirs, allowCrossShare, allowNonemptyDelete, allowUnshare)
			if err := printJSONFiltered(cmd.OutOrStdout(), preview, flags); err != nil {
				return err
			}
			if !report.OK {
				return usageErr(fmt.Errorf("plan has %d errors", report.Errors))
			}
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		info, err := verifiedDropboxAccount(ctx, c, db)
		if err != nil {
			return err
		}
		headers := pathRootHeaders(info)
		stale, _, err := db.GetDropboxMeta(ctx, "index_stale")
		if err != nil {
			return err
		}
		if !noRefresh || stale == "1" {
			if err := refreshDropboxIndex(ctx, cmd, c, db, headers); err != nil {
				return err
			}
			if err := db.SetDropboxMeta(ctx, "index_stale", "0"); err != nil {
				return err
			}
		}
		report, err := checkPlanAtIndex(ctx, db, p, dropbox.CheckOptions{MaxOps: maxOps, AllowNonemptyDelete: allowNonemptyDelete, AllowUnshare: allowUnshare, RequireComplete: noRefresh})
		if err != nil {
			return err
		}
		required := requiredApplyFlags(report, allowDevDirs, allowCrossShare, allowNonemptyDelete, allowUnshare)
		if len(required) > 0 {
			_ = printJSONFiltered(cmd.OutOrStdout(), report, flags)
			return usageErr(fmt.Errorf("plan requires %s", strings.Join(required, ", ")))
		}
		if !report.OK {
			_ = printJSONFiltered(cmd.OutOrStdout(), report, flags)
			return usageErr(fmt.Errorf("plan has %d errors", report.Errors))
		}
		result, err := executeApply(ctx, c, db, headers, p, args[0], info.AccountType, applyExecutionOptions{AccountID: info.AccountID, AllowNonemptyDelete: allowNonemptyDelete})
		if printErr := printJSONFiltered(cmd.OutOrStdout(), result, flags); printErr != nil {
			return printErr
		}
		return err
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().BoolVar(&yes, "yes", false, "Execute the plan")
	cmd.Flags().BoolVar(&allowCrossShare, "allow-cross-share", false, "Allow moves across shared folder contexts")
	cmd.Flags().BoolVar(&allowDevDirs, "allow-dev-dirs", false, "Allow changes inside development dependency folders")
	cmd.Flags().BoolVar(&allowNonemptyDelete, "allow-nonempty-delete", false, "Allow deleting folders with indexed file descendants")
	cmd.Flags().BoolVar(&allowUnshare, "allow-unshare", false, "Allow deleting shared folders")
	cmd.Flags().IntVar(&maxOps, "max-ops", 5000, "Maximum operations in a plan")
	cmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "Use the current local index without incremental refresh")
	return cmd
}

func refreshDropboxIndex(ctx context.Context, cmd *cobra.Command, c *client.Client, db *store.Store, headers map[string]string) error {
	maxPages := 0
	if cliutil.IsDogfoodEnv() {
		maxPages = 1
	}
	tracked, err := completeIndexRoots(ctx, db.DB())
	if err != nil {
		return err
	}
	var folders []string
	if _, accountIndexed := tracked[""]; accountIndexed {
		if _, err := crawlDropboxRoot(ctx, cmd, c, db, "", "", false, maxPages, headers); err != nil {
			return err
		}
		if folders, err = db.ListDropboxTopFolders(ctx); err != nil {
			return err
		}
	} else {
		// An index built with `index --root` refreshes only the roots it
		// tracks. Discovering every top-level folder here would turn a
		// one-folder apply or undo into a full-account crawl.
		for root := range tracked {
			folders = append(folders, root)
		}
		sort.Strings(folders)
	}
	for _, folder := range folders {
		if _, err := crawlDropboxRoot(ctx, cmd, c, db, folder, folder, false, maxPages, headers); err != nil {
			return err
		}
	}
	return nil
}

type applyExecutionOptions struct {
	AccountID           string
	AllowNonemptyDelete bool
}

func executeApply(ctx context.Context, c *client.Client, db *store.Store, headers map[string]string, p dropbox.Plan, planPath, accountType string, options ...applyExecutionOptions) (r applyResult, runErr error) {
	r = applyResult{BatchID: newBatchID(), Status: "complete", Failures: make([]applyFailure, 0), RestoreWindowDays: restoreDays(accountType)}
	opt := applyExecutionOptions{}
	if len(options) > 0 {
		opt = options[0]
	}
	created, finalized := false, false
	defer func() {
		if runErr != nil && !finalized {
			r.Status = "failed"
		}
		if created && !finalized {
			_ = db.SetDropboxJournalStatus(ctx, r.BatchID, "failed")
		}
		if runErr != nil && !strings.Contains(runErr.Error(), "do not re-apply") {
			runErr = fmt.Errorf("%w; do not re-apply; run `dropbox-pp-cli index` then inspect `dropbox-pp-cli journal %s --ops`", runErr, r.BatchID)
		}
	}()
	batch := store.DropboxJournalBatch{ID: r.BatchID, CreatedAt: time.Now().UTC().Format(time.RFC3339), Source: p.Source, PlanPath: planPath, Status: "running", AccountType: accountType, AccountID: opt.AccountID, RestoreDays: r.RestoreWindowDays}
	if err := db.CreateDropboxJournalBatch(ctx, batch); err != nil {
		return r, err
	}
	created = true
	runner := applyRun{ctx: ctx, db: db, poster: dropboxBatchPoster{c, headers}, result: &r, nextSeq: len(p.Ops) + 1, priorDeletes: make(map[string]bool), failedMoveSources: make(map[string]bool), allowNonemptyDelete: opt.AllowNonemptyDelete}
	if err := runner.runGroups(p); err != nil {
		return r, err
	}
	if err := runner.revokeLinks(p); err != nil {
		return r, err
	}
	if err := runner.finish(); err != nil {
		return r, err
	}
	finalized = true
	if len(r.Failures) > 0 {
		return r, fmt.Errorf("%d operations need inspection; do not re-apply; run `dropbox-pp-cli index` then inspect `dropbox-pp-cli journal %s --ops`", len(r.Failures), r.BatchID)
	}
	return r, nil
}
