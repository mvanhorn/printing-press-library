// Hand-authored (Slice 2): listings bulk-edit.
// pp:data-source auto

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if p := s2FindChild(root, "listings"); p != nil {
			addNovelCommandIfAbsent(p, newS2ListingsBulkEditCmd(flags))
		}
	})
}

// bulkMinInterval spaces consecutive Etsy requests so a run stays at or under
// 2 requests per second (Etsy's app limit is 5 QPS; bulk writes stay well clear).
const bulkDefaultInterval = 500 * time.Millisecond

var bulkMinInterval = bulkDefaultInterval

type bulkThrottle struct {
	interval time.Duration
	last     time.Time
}

func (t *bulkThrottle) wait(ctx context.Context) error {
	if !t.last.IsZero() {
		if d := t.interval - time.Since(t.last); d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	t.last = time.Now()
	return nil
}

func bulkAPIStatus(err error) int {
	var ae *client.APIError
	if errors.As(err, &ae) {
		return ae.StatusCode
	}
	return 0
}

func bulkShopID(flagVal string) string {
	return bulkShopIDForCmd(nil, flagVal)
}

func bulkShopIDForCmd(cmd *cobra.Command, flagVal string) string {
	if v := strings.TrimSpace(flagVal); v != "" {
		return v
	}
	return shopxShopID(cmd)
}

// bulkCurrentFromListingJSON extracts the editable fields of a listing object.
func bulkCurrentFromListingJSON(raw []byte) *bulkCurrent {
	m, err := s2DecodeObject(raw)
	if err != nil {
		return &bulkCurrent{}
	}
	c := &bulkCurrent{Found: true, Alt: map[int64]string{}}
	c.Title, _ = m["title"].(string)
	strs := func(v any) []string {
		out := make([]string, 0)
		if arr, ok := v.([]any); ok {
			for _, e := range arr {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
		}
		return out
	}
	c.Tags, c.Materials = strs(m["tags"]), strs(m["materials"])
	if f, ok := s2MoneyFloat(m["price"]); ok {
		c.Price = &f
	}
	return c
}

// bulkStoreCurrent loads listings (and image alt text) from the local mirror.
func bulkStoreCurrent(db *store.Store, ids []int64) map[int64]*bulkCurrent {
	out := map[int64]*bulkCurrent{}
	if db == nil {
		return out
	}
	for _, id := range ids {
		var data []byte
		err := db.DB().QueryRow(`SELECT data FROM listings WHERE listing_id = ? OR id = ? LIMIT 1`, id, strconv.FormatInt(id, 10)).Scan(&data)
		if err != nil {
			continue
		}
		c := bulkCurrentFromListingJSON(data)
		bulkInventoryCurrent(c, []byte(s2StoredInventory(db, id)))
		if rs, err := db.DB().Query(`SELECT listing_image_id, COALESCE(alt_text,'') FROM listing_images WHERE listing_id = ?`, id); err == nil {
			for rs.Next() {
				var img int64
				var alt string
				if rs.Scan(&img, &alt) == nil {
					c.Alt[img] = alt
				}
			}
			_ = rs.Close()
		}
		out[id] = c
	}
	return out
}

func bulkListingIDs(items []bulkPlanItem) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0)
	for _, it := range items {
		if id := int64(it.ListingID); id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// bulkOpKeys derives the journal op keys a plan needs for one listing.
func bulkOpKeys(items []bulkPlanItem, listing int64) []string {
	seen := map[string]bool{}
	keys := make([]string, 0)
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, it := range items {
		if int64(it.ListingID) != listing {
			continue
		}
		switch it.Field {
		case bulkFieldTitle, bulkFieldTags, bulkFieldMaterials:
			add(fmt.Sprintf("%d|patch", listing))
		case bulkFieldPrice:
			add(fmt.Sprintf("%d|price", listing))
		case bulkFieldAltText:
			add(fmt.Sprintf("%d|alt|%d", listing, int64(it.ImageID)))
		}
	}
	return keys
}

// bulkEstimateCalls is a conservative per-run request count: per listing one
// GET, two snapshot GETs when not yet snapshotted, one PATCH, two for a price
// write (inventory GET + PUT) and one per image alt text.
func bulkEstimateCalls(items []bulkPlanItem, snapshotted map[int64]bool, done map[string]bool) int {
	n := 0
	for _, id := range bulkListingIDs(items) {
		pending := 0
		for _, k := range bulkOpKeys(items, id) {
			if done[k] {
				continue
			}
			switch {
			case strings.HasSuffix(k, "|patch"):
				pending++
			case strings.HasSuffix(k, "|price"):
				pending += 2
			default:
				pending++
			}
		}
		if pending == 0 {
			continue
		}
		n += 1 + pending
		if !snapshotted[id] {
			n += 2
		}
	}
	return n
}

func bulkJobID(items []bulkPlanItem) string {
	sum := sha256.Sum256(bulkPlanItemsCanonical(items))
	return "plan-" + hex.EncodeToString(sum[:])[:12]
}

type bulkPreview struct {
	Mode         string        `json:"mode"`
	Source       string        `json:"source"`
	Job          string        `json:"job"`
	Listings     int           `json:"listings"`
	Changes      []bulkChange  `json:"changes"`
	Problems     []bulkProblem `json:"problems"`
	PlannedCalls int           `json:"planned_calls"`
	Budget       string        `json:"budget_warning,omitempty"`
	Resumable    int           `json:"already_done,omitempty"`
	Next         string        `json:"next,omitempty"`
}

type bulkOpResult struct {
	OpKey  string `json:"op_key"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type bulkApplyResult struct {
	Mode        string         `json:"mode"`
	Job         string         `json:"job"`
	Done        int            `json:"done"`
	SkippedDone int            `json:"skipped_done"`
	Failed      int            `json:"failed"`
	Calls       int            `json:"http_calls"`
	Results     []bulkOpResult `json:"results"`
	Problems    []bulkProblem  `json:"problems,omitempty"`
	Resume      string         `json:"resume,omitempty"`
}

func bulkItemsFromFlags(listings string, all bool, db *store.Store, shop string, field string, set, add, remove []string, image int) ([]bulkPlanItem, error) {
	field = strings.ToLower(strings.TrimSpace(field))
	n := 0
	for _, l := range [][]string{set, add, remove} {
		if len(l) > 0 {
			n++
		}
	}
	if field == "" || n != 1 {
		return nil, usageErr(fmt.Errorf("flag mode needs --field and exactly one of --set, --add or --remove (or pass --plan FILE)"))
	}
	op, vals := bulkOpSet, set
	if len(add) > 0 {
		op, vals = bulkOpAdd, add
	} else if len(remove) > 0 {
		op, vals = bulkOpRemove, remove
	}
	ids := make([]int64, 0)
	if strings.TrimSpace(listings) != "" {
		for _, p := range strings.Split(listings, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil || id <= 0 {
				return nil, usageErr(fmt.Errorf("--listings entry %q is not a listing id", p))
			}
			ids = append(ids, id)
		}
	}
	if all {
		shop = strings.TrimSpace(shop)
		if shop == "" {
			return nil, usageErr(fmt.Errorf("%s; --all is shop-scoped", shopxNoteShopUnset))
		}
		if db == nil {
			return nil, usageErr(fmt.Errorf("--all needs a synced local store; run `etsy-pp-cli sync` first"))
		}
		rs, err := db.DB().Query(`SELECT COALESCE(listing_id, CAST(id AS INTEGER)) FROM listings WHERE state = 'active' AND CAST(json_extract(data,'$.shop_id') AS TEXT) = ?`, shop)
		if err == nil {
			for rs.Next() {
				var id int64
				if rs.Scan(&id) == nil && id > 0 {
					ids = append(ids, id)
				}
			}
			_ = rs.Close()
		}
	}
	if len(ids) == 0 {
		return nil, usageErr(fmt.Errorf("flag mode needs --listings ID[,ID...] or --all"))
	}
	var value any
	switch field {
	case bulkFieldTags, bulkFieldMaterials:
		arr := make([]any, 0, len(vals))
		for _, v := range vals {
			arr = append(arr, v)
		}
		value = arr
	case bulkFieldPrice:
		f, err := strconv.ParseFloat(strings.TrimSpace(vals[0]), 64)
		if err != nil {
			return nil, usageErr(fmt.Errorf("--price value %q is not a number", vals[0]))
		}
		value = f
	default:
		value = vals[0]
	}
	items := make([]bulkPlanItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, bulkPlanItem{ListingID: bulkFlexInt(id), Field: field, Op: op, Value: value, ImageID: bulkFlexInt(image)})
	}
	return items, nil
}

func newS2ListingsBulkEditCmd(flags *rootFlags) *cobra.Command {
	var planPath, field, listingsCSV, dbPath, shopFlag, resume string
	var set, add, remove []string
	var all, apply, ignoreBudget, restart, continueOnError, flatten bool
	var image int
	cmd := &cobra.Command{
		Use:   "bulk-edit",
		Short: "Preview, then apply, validated edits to many listings (tags, title, price, materials, image alt text).",
		Long: strings.Trim(`
Edit many listings from a plan file (JSON or CSV: listing_id, field, op, value,
image_id, reason) or from flags (--field with --set/--add/--remove). Every
value is checked against Etsy's limits first (title up to 140 characters, up to
13 tags of up to 20 characters, up to 13 materials) and the offending listing
and field are named. The default run is a PREVIEW: an old->new diff against the
local store with zero API calls. --apply --yes executes it: each listing is
snapshotted first (see 'listings snapshot'), writes are throttled to 2 requests
per second, every item is journaled so --resume JOB continues where it
stopped, and the daily Etsy call budget is checked up front (--ignore-budget to
override). Price changes rewrite the whole inventory (like 'inventory set-qty').
Needs the listings_w OAuth scope. 'listings audit --plan-out' writes plans in
this format.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli listings bulk-edit --plan audit-fixes.json
  etsy-pp-cli listings bulk-edit --listings 1000000001,1000000002 --field tags --add "linen apron"
  etsy-pp-cli listings bulk-edit --plan audit-fixes.json --apply --yes
`, "\n"),
		// etsy-ext: declare filesystem sinks for the MCP walker.
		Annotations: map[string]string{"mcp:read-flags": "plan", "mcp:read-only": "false", "pp:data-source": "auto", "pp:happy-args": "--plan=example-value", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("restart") && cmd.Flags().Changed("resume") {
				return usageErr(fmt.Errorf("--restart and --resume can't be combined (--restart is deprecated)"))
			}
			// etsy-ext: MCP callers cannot bypass budget checks.
			if cliutil.IsMCPChildEnv() {
				if ignoreBudget {
					return usageErr(fmt.Errorf("--ignore-budget is not available over MCP"))
				}
			}

			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flags.yes && !apply {
				return usageErr(fmt.Errorf("--yes only confirms --apply; add --apply to execute the plan, or drop --yes to preview"))
			}
			if flags.dryRun {
				apply = false
				if planPath == "" && field == "" {
					// Nothing to preview yet (verify/dogfood sweeps pass no input).
					return writeDryRun(cmd.OutOrStdout(), flags, "listings bulk-edit")
				}
			}
			if apply && !flags.yes {
				return usageErr(fmt.Errorf("--apply changes live listings and needs --yes to confirm; run without --apply to preview the diff"))
			}
			if planPath == "" && field == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--plan FILE or --field is required"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-pp-cli")
			}
			var mirror *store.Store
			if _, statErr := os.Stat(dbPath); statErr == nil {
				var err error
				if mirror, err = store.OpenReadOnly(dbPath); err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				defer mirror.Close()
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no local mirror at %s; old values are unknown until you run 'etsy-pp-cli sync'\n", dbPath)
			}
			var items []bulkPlanItem
			source := "flags"
			if planPath != "" {
				plan, err := bulkLoadPlanFile(planPath)
				if err != nil {
					return usageErr(err)
				}
				items, source = plan.Items, planPath
			} else {
				var err error
				if items, err = bulkItemsFromFlags(listingsCSV, all, mirror, bulkShopIDForCmd(cmd, shopFlag), field, set, add, remove, image); err != nil {
					return err
				}
			}
			problems := bulkValidateStatic(items)
			cur := bulkStoreCurrent(mirror, bulkListingIDs(items))
			changes, rp := bulkResolve(items, cur)
			var vp []bulkProblem
			changes, vp = bulkApplyVariationRules(changes, cur, flatten)
			rp = append(rp, vp...)
			static := map[string]bool{}
			for _, p := range problems {
				static[fmt.Sprintf("%d|%s", p.ListingID, p.Field)] = true
			}
			for _, p := range rp {
				if !static[fmt.Sprintf("%d|%s", p.ListingID, p.Field)] {
					problems = append(problems, p)
				}
			}
			problems = bulkDedupeProblems(problems)
			job := bulkJobID(items)
			if resume != "" {
				if resume != job {
					return usageErr(fmt.Errorf("--resume %s does not match this plan (job %s)", resume, job))
				}
				if mirror == nil {
					return usageErr(fmt.Errorf("job %s does not exist", resume))
				}
				status, err := mirror.BulkJobStatus(job)
				if err != nil {
					return err
				}
				if status == "" {
					return usageErr(fmt.Errorf("job %s does not exist", job))
				}
				if status == "done" {
					return usageErr(fmt.Errorf("job %s already finished; run without --resume to apply the plan again", job))
				}
			}
			// Journal state is used only for explicit resume.
			done := map[string]bool{}
			snapshotted := map[int64]bool{}
			if mirror != nil && resume != "" {
				bulkReadJournal(mirror, job, done, snapshotted)
			}
			planned := bulkEstimateCalls(items, snapshotted, done)
			if len(problems) > 0 {
				out := bulkPreview{Mode: "invalid", Source: source, Job: job, Listings: len(bulkListingIDs(items)), Changes: changes, Problems: problems, PlannedCalls: planned}
				if err := bulkEmit(cmd, flags, out); err != nil {
					return err
				}
				return usageErr(fmt.Errorf("plan rejected: %d problem(s); first: %s", len(problems), problems[0].String()))
			}
			budgetErr := checkBudget(cmd, planned, ignoreBudget)
			if !apply {
				out := bulkPreview{Mode: "preview", Source: source, Job: job, Listings: len(bulkListingIDs(items)), Changes: changes, Problems: problems, PlannedCalls: planned, Resumable: len(done), Next: "re-run with --apply --yes to execute (snapshots each listing first, 2 requests/second, resumable)"}
				if budgetErr != nil {
					out.Budget = budgetErr.Error()
				}
				return bulkEmit(cmd, flags, out)
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "listings bulk-edit --apply")
			}
			if budgetErr != nil {
				return budgetErr
			}
			shop := bulkShopIDForCmd(cmd, shopFlag)
			if shop == "" {
				return usageErr(fmt.Errorf("a shop id is required to write: set ETSY_SHOP_ID or pass --shop-id"))
			}
			if mirror != nil {
				_ = mirror.Close()
				mirror = nil
			}
			return bulkApply(cmd, flags, bulkApplyInput{Items: items, Source: source, Job: job, Shop: shop, DBPath: dbPath, Restart: restart, Resume: resume, Flatten: flatten, ContinueOnError: continueOnError})
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "Plan file (.json or .csv) listing the edits")
	cmd.Flags().StringVar(&field, "field", "", "Field for flag mode: tags, title, price, materials or alt_text")
	cmd.Flags().StringArrayVar(&set, "set", nil, "Value to set (repeat for several tags/materials)")
	cmd.Flags().StringArrayVar(&add, "add", nil, "Tag/material to add (repeatable)")
	cmd.Flags().StringArrayVar(&remove, "remove", nil, "Tag/material to remove (repeatable)")
	cmd.Flags().StringVar(&listingsCSV, "listings", "", "Comma-separated listing ids for flag mode")
	cmd.Flags().BoolVar(&all, "all", false, "Flag mode: every active listing in the local store")
	cmd.Flags().IntVar(&image, "image-id", 0, "Listing image id for --field alt_text")
	cmd.Flags().BoolVar(&apply, "apply", false, "Execute the edits (needs --yes); default is a preview")
	cmd.Flags().BoolVar(&ignoreBudget, "ignore-budget", false, "Run even if the planned calls exceed the remaining daily Etsy budget")
	cmd.Flags().StringVar(&resume, "resume", "", "Continue an unfinished job")
	cmd.Flags().BoolVar(&flatten, "flatten-variation-prices", false, "Allow replacing all variation prices with one price")
	cmd.Flags().BoolVar(&restart, "restart", false, "Discard the journal for this plan and redo every item")
	_ = cmd.Flags().MarkDeprecated("restart", "fresh starts are now the default; use --resume JOB to continue")
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "Keep going after a failed item (default stops at the first failure)")
	cmd.Flags().StringVar(&shopFlag, "shop-id", "", "Shop id for writes (default: ETSY_SHOP_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func bulkReadJournal(db *store.Store, job string, done map[string]bool, snapshotted map[int64]bool) {
	rs, err := db.DB().Query(`SELECT op_key, listing_id, kind, status FROM bulk_edit_items WHERE job_id = ?`, job)
	if err != nil {
		return
	}
	defer rs.Close()
	for rs.Next() {
		var k, kind, st string
		var lid int64
		if rs.Scan(&k, &lid, &kind, &st) != nil || st != store.BulkStatusDone {
			continue
		}
		if kind == "snapshot" {
			snapshotted[lid] = true
		} else {
			done[k] = true
		}
	}
}

func bulkEmit(cmd *cobra.Command, flags *rootFlags, v any) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	w := cmd.OutOrStdout()
	switch t := v.(type) {
	case bulkPreview:
		fmt.Fprintf(w, "%s: %d listing(s), %d change(s), ~%d API call(s) to apply\n", t.Mode, t.Listings, len(t.Changes), t.PlannedCalls)
		for _, c := range t.Changes {
			label := c.Field
			if c.ImageID > 0 {
				label += fmt.Sprintf("[image %d]", c.ImageID)
			}
			fmt.Fprintf(w, "  listing %d %s: %s -> %s", c.ListingID, label, bulkShow(c.Old), bulkShow(c.New))
			if c.Note != "" {
				fmt.Fprintf(w, "  (%s)", c.Note)
			}
			fmt.Fprintln(w)
			for _, o := range c.Offerings {
				fmt.Fprintf(w, "    offering %s (%s): %g -> %g\n", o.OfferingID, o.SKU, o.Old, o.New)
			}
		}
		for _, p := range t.Problems {
			fmt.Fprintf(w, "  PROBLEM %s\n", p.String())
		}
		if t.Next != "" {
			fmt.Fprintln(w, t.Next)
		}
		return nil
	default:
		return printJSONFiltered(w, v, flags)
	}
}

func bulkShow(v any) string {
	if v == nil {
		return "(unknown)"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

type bulkApplyInput struct {
	Items           []bulkPlanItem
	Source, Job     string
	Shop, DBPath    string
	Restart         bool
	Resume          string
	Flatten         bool
	ContinueOnError bool
}

type bulkRunner struct {
	cmd        *cobra.Command
	flags      *rootFlags
	c          *client.Client
	db         *store.Store
	ctx        context.Context
	thr        *bulkThrottle
	calls      int
	shop       string
	job        string
	results    []bulkOpResult
	res        bulkApplyResult
	firstErr   error
	journalErr error
}

func (r *bulkRunner) get(path string) (json.RawMessage, error) {
	if err := r.thr.wait(r.ctx); err != nil {
		return nil, err
	}
	r.calls++
	return r.c.Get(r.ctx, path, nil)
}

func (r *bulkRunner) fail(op, msg string, err error) {
	if err := r.db.BulkMarkOp(r.job, op, store.BulkStatusFailed, msg, 0); err != nil && r.journalErr == nil {
		r.journalErr = err
	}
	r.res.Failed++
	r.results = append(r.results, bulkOpResult{OpKey: op, Status: "failed", Error: msg})
	if r.firstErr == nil {
		r.firstErr = err
	}
}

func (r *bulkRunner) succeed(op string, snap int64) {
	if err := r.db.BulkMarkOp(r.job, op, store.BulkStatusDone, "", snap); err != nil && r.journalErr == nil {
		r.journalErr = err
	}
	r.res.Done++
	r.results = append(r.results, bulkOpResult{OpKey: op, Status: "done"})
}

// fatal reports whether an error should stop the whole run regardless of
// --continue-on-error (auth, scope and rate-limit failures will not heal).
func bulkIsDailyQuota(err error) bool {
	return isDailyQuotaErr(err)
}

func bulkFatal(err error) bool {
	if bulkIsDailyQuota(err) {
		return true
	}
	switch bulkAPIStatus(err) {
	case 401, 403, 429:
		return true
	}
	return false
}

func bulkApply(cmd *cobra.Command, flags *rootFlags, in bulkApplyInput) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := store.OpenWithContext(ctx, in.DBPath)
	if err != nil {
		return fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	// read-before-write / snapshot / refresh reads must see live data, never the 5-minute response cache
	c.NoCache = true
	ids := bulkListingIDs(in.Items)
	prior, err := db.BulkJobStatus(in.Job)
	if err != nil {
		return err
	}
	if in.Resume == "" && prior == "running" {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: job %s was interrupted; use --resume %s to continue it (starting fresh)\n", in.Job, in.Job)
	}
	if err := db.BulkStartJob(in.Job, in.Source, len(in.Items), in.Resume == ""); err != nil {
		return err
	}
	for _, id := range ids {
		for _, k := range append(bulkOpKeys(in.Items, id), fmt.Sprintf("%d|snapshot", id)) {
			kind := "patch"
			switch {
			case strings.HasSuffix(k, "|snapshot"):
				kind = "snapshot"
			case strings.HasSuffix(k, "|price"):
				kind = "price"
			case strings.Contains(k, "|alt|"):
				kind = "alt_text"
			}
			if err := db.BulkRegisterOp(store.BulkOp{JobID: in.Job, OpKey: k, ListingID: id, Kind: kind}); err != nil {
				return err
			}
		}
	}
	r := &bulkRunner{cmd: cmd, flags: flags, c: c, db: db, ctx: ctx, thr: &bulkThrottle{interval: bulkMinInterval}, shop: in.Shop, job: in.Job}
	r.res = bulkApplyResult{Mode: "apply", Job: in.Job}
	stop := false
	for _, id := range ids {
		if stop {
			break
		}
		stop = r.applyListing(id, in)
	}
	status := "done"
	if r.res.Failed > 0 {
		status = "failed"
	}
	if ctx.Err() != nil || r.journalErr != nil {
		status = "running"
	}
	if err := db.BulkFinishJob(in.Job, status); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: finishing job: %v\n", err)
	}
	r.res.Calls = r.calls
	r.res.Results = r.results
	if r.res.Results == nil {
		r.res.Results = make([]bulkOpResult, 0)
	}
	if r.res.Failed > 0 {
		r.res.Resume = fmt.Sprintf("fix the cause and re-run with --resume %s to continue; completed items are skipped (re-running without --resume starts over)", in.Job)
	}
	if ctx.Err() != nil {
		r.res.Resume = "interrupted: " + ctx.Err().Error() + "; " + r.res.Resume
	}
	if err := printJSONFiltered(cmd.OutOrStdout(), r.res, flags); err != nil {
		return err
	}
	if r.journalErr != nil {
		return fmt.Errorf("journal write failed: %w", r.journalErr)
	}
	if r.firstErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "bulk-edit stopped: %d done, %d failed. %s\n", r.res.Done, r.res.Failed, r.res.Resume)
		return classifyAPIErrorOnly(r.firstErr)
	}
	return nil
}

// applyListing runs every pending op of one listing. It returns true when the
// whole run should stop.
func (r *bulkRunner) applyListing(id int64, in bulkApplyInput) (stop bool) {
	defer func() {
		if r.journalErr != nil {
			stop = true
		}
	}()
	if r.journalErr != nil {
		return true
	}
	keys := bulkOpKeys(in.Items, id)
	pending := make([]string, 0, len(keys))
	for _, k := range keys {
		st, _ := r.db.BulkOpStatus(r.job, k)
		if st == store.BulkStatusDone {
			r.res.SkippedDone++
			r.results = append(r.results, bulkOpResult{OpKey: k, Status: "skipped_done"})
			continue
		}
		pending = append(pending, k)
	}
	if len(pending) == 0 {
		return false
	}
	fail := func(k string, err error) bool {
		r.fail(k, err.Error(), err)
		return r.journalErr != nil || bulkFatal(err) || !in.ContinueOnError
	}
	failAll := func(err error) bool {
		for _, k := range pending {
			r.fail(k, err.Error(), err)
		}
		return r.journalErr != nil || bulkFatal(err) || !in.ContinueOnError
	}
	idStr := strconv.FormatInt(id, 10)
	rawListing, err := r.get("/listings/" + idStr)
	if err != nil {
		return failAll(err)
	}
	// Snapshot first, once per job, so a resumed run never overwrites the
	// pre-write copy.
	snapKey := fmt.Sprintf("%d|snapshot", id)
	var inventory json.RawMessage
	if st, _ := r.db.BulkOpStatus(r.job, snapKey); st != store.BulkStatusDone {
		inv, ierr := r.get("/listings/" + idStr + "/inventory")
		if ierr != nil {
			return failAll(fmt.Errorf("snapshot failed, nothing written: %w", ierr))
		}
		inventory = inv
		pers, perr := r.get("/listings/" + idStr + "/personalization")
		if perr != nil && bulkAPIStatus(perr) != 404 {
			return failAll(fmt.Errorf("snapshot failed, nothing written: %w", perr))
		}
		snapID, serr := r.db.SaveListingSnapshot(id, "bulk-edit:"+r.job, string(rawListing), string(inv), string(pers))
		if serr != nil {
			return failAll(fmt.Errorf("snapshot failed, nothing written: %w", serr))
		}
		r.succeed(snapKey, snapID)
		if r.journalErr != nil {
			return true
		}
	}
	// Re-resolve against the live listing so add/remove work on current data.
	cur := map[int64]*bulkCurrent{id: bulkCurrentFromListingJSON(rawListing)}
	if !cur[id].Found {
		return failAll(fmt.Errorf("could not parse the live listing %d", id))
	}
	listingItems := make([]bulkPlanItem, 0)
	for _, it := range in.Items {
		if int64(it.ListingID) == id {
			listingItems = append(listingItems, it)
		}
	}
	changes, probs := bulkResolve(listingItems, cur)
	if len(probs) > 0 {
		return failAll(fmt.Errorf("live validation failed: %s", probs[0].String()))
	}
	patch := url.Values{}
	price, hasPrice := 0.0, false
	alts := map[int64]string{}
	for _, ch := range changes {
		switch ch.Field {
		case bulkFieldTitle:
			patch.Set("title", fmt.Sprint(ch.New))
		case bulkFieldTags:
			patch.Set("tags", strings.Join(ch.New.([]string), ","))
		case bulkFieldMaterials:
			patch.Set("materials", strings.Join(ch.New.([]string), ","))
		case bulkFieldPrice:
			price, hasPrice = ch.New.(float64), true
		case bulkFieldAltText:
			alts[ch.ImageID] = fmt.Sprint(ch.New)
		}
	}
	for _, k := range pending {
		if err := r.thr.wait(r.ctx); err != nil {
			r.fail(k, err.Error(), err)
			return true
		}
		var data json.RawMessage
		var opErr error
		switch {
		case strings.HasSuffix(k, "|patch"):
			r.calls++
			data, _, opErr = r.c.PatchFormWithParams(r.ctx, "/shops/"+url.PathEscape(r.shop)+"/listings/"+idStr, nil, patch)
			if opErr == nil && json.Valid(data) {
				_ = r.db.UpsertListings(data)
			}
		case strings.HasSuffix(k, "|price"):
			if !hasPrice {
				opErr = fmt.Errorf("no price value resolved")
				break
			}
			if inventory == nil {
				var gerr error
				if inventory, gerr = r.get("/listings/" + idStr + "/inventory"); gerr != nil {
					opErr = gerr
					break
				}
				if err := r.thr.wait(r.ctx); err != nil {
					opErr = err
					break
				}
			}
			live := &bulkCurrent{}
			bulkInventoryCurrent(live, inventory)
			obj, _ := s2DecodeObject(inventory)
			if props, _ := obj["price_on_property"].([]any); len(props) > 0 && !in.Flatten {
				opErr = fmt.Errorf("listing %d has per-variation prices (price_on_property %v); a price change would flatten all %d live offerings to one price; re-run with --flatten-variation-prices", id, props, len(live.Offerings))
				break
			}
			body, berr := s2InventoryPutBody(inventory, func(_, _, out map[string]any) { out["price"] = price })
			if berr != nil {
				opErr = berr
				break
			}
			if in.Flatten {
				body["price_on_property"] = []any{}
			}
			r.calls++
			data, _, opErr = r.c.PutWithParams(r.ctx, "/listings/"+idStr+"/inventory", nil, body)
		case strings.Contains(k, "|alt|"):
			img, _ := strconv.ParseInt(k[strings.LastIndex(k, "|")+1:], 10, 64)
			r.calls++
			data, _, opErr = r.c.PostMultipartWithParams(r.ctx, "/shops/"+url.PathEscape(r.shop)+"/listings/"+idStr+"/images", nil,
				map[string]string{"listing_image_id": strconv.FormatInt(img, 10), "alt_text": alts[img]}, map[string]string{})
		}
		_ = data
		if opErr != nil {
			if fail(k, opErr) {
				return true
			}
			continue
		}
		r.succeed(k, 0)
		if r.journalErr != nil {
			return true
		}
	}
	return false
}
