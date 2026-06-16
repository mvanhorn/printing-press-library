package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-operator-intel/internal/store"
	"github.com/spf13/cobra"
)

const version = "0.1.0-private"

type rootFlags struct {
	asJSON, compact, noInput, yes, noColor, agent bool
	profile, home                                 string
}

func Execute() error             { return NewRootCmd().Execute() }
func NewRootCmd() *cobra.Command { f := &rootFlags{}; return newRootCmd(f) }

func newRootCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "amazon-operator-intel-pp-cli", Short: "Private local-first Amazon operator control tower", Long: "Joins Amazon Seller and Amazon Ads evidence into local-first inventory, profitability, listing, cash, and ad-spend decisions. Fixture/import mode is offline; live sync is opt-in through child CLIs.", SilenceUsage: true, Version: version}
	cmd.PersistentFlags().BoolVar(&f.asJSON, "json", false, "Output JSON")
	cmd.PersistentFlags().BoolVar(&f.compact, "compact", false, "Prefer compact output")
	cmd.PersistentFlags().BoolVar(&f.noInput, "no-input", false, "Disable prompts")
	cmd.PersistentFlags().BoolVar(&f.yes, "yes", false, "Assume yes for safe operations")
	cmd.PersistentFlags().BoolVar(&f.noColor, "no-color", false, "Disable color")
	cmd.PersistentFlags().BoolVar(&f.agent, "agent", false, "Agent mode: --json --compact --no-input --yes --no-color")
	cmd.PersistentFlags().StringVar(&f.profile, "profile", "default", "Profile name")
	cmd.PersistentFlags().StringVar(&f.home, "home", store.DefaultDir(), "State directory")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if f.agent {
			f.asJSON, f.compact, f.noInput, f.yes, f.noColor = true, true, true, true, true
		}
		return nil
	}
	cmd.AddCommand(agentContextCmd(f), doctorCmd(f), sourcesCmd(f), profileCmd(f), syncCmd(f), warRoomCmd(f), restockOrKillCmd(f), adSpendGuardrailCmd(f), skuProfitTruthCmd(f), listingTriageCmd(f), cashLeaksCmd(f), searchTermActionsCmd(f), digestCmd(f), operatorPlanCmd(f), cashCalendarCmd(f), launchReadinessCmd(f), rankDefenseCmd(f), bundleOpportunitiesCmd(f), vendorOpsCmd(f))
	return cmd
}

func st(f *rootFlags) *store.Store { return store.New(f.home) }

func out(cmd *cobra.Command, f *rootFlags, v any, human string) error {
	if f.asJSON || f.agent {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if f.compact {
			return enc.Encode(v)
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return err
	}
	if human == "" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	_, err := io.WriteString(cmd.OutOrStdout(), human)
	return err
}

func agentContextCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Print machine-readable CLI context", RunE: func(cmd *cobra.Command, args []string) error {
		ctx := map[string]any{
			"schema_version":     "amazon-operator-intel.agent-context/v1",
			"name":               "amazon-operator-intel-pp-cli",
			"version":            version,
			"private":            true,
			"local_first":        true,
			"external_api_calls": false,
			"agent_flag":         "sets --json --compact --no-input --yes --no-color",
			"state_dir":          f.home,
			"source_plan":        sourcePlans(),
			"env":                envChecks(),
			"commands":           commandDescriptors(),
			"recommended_workflows": []map[string]any{
				{"name": "morning control tower", "commands": []string{"doctor", "sources doctor", "sync", "war-room", "operator-plan"}},
				{"name": "cash and inventory review", "commands": []string{"sku-profit-truth", "restock-or-kill", "cash-leaks", "cash-calendar"}},
				{"name": "ads cleanup", "commands": []string{"ad-spend-guardrail", "search-term-actions", "rank-defense"}},
				{"name": "launch and merchandising", "commands": []string{"launch-readiness", "bundle-opportunities", "listing-triage"}},
				{"name": "vendor-style ops", "commands": []string{"vendor-ops readiness", "vendor-ops deductions", "vendor-ops po-watch", "vendor-ops scorecard"}},
			},
		}
		return out(cmd, f, ctx, "")
	}}
}

func commandDescriptors() []map[string]any {
	names := []string{"agent-context", "doctor", "sources doctor", "profile save/list/show/delete", "sync", "war-room", "restock-or-kill", "ad-spend-guardrail", "sku-profit-truth", "listing-triage", "cash-leaks", "search-term-actions", "digest daily", "digest weekly", "operator-plan", "cash-calendar", "launch-readiness", "rank-defense", "bundle-opportunities", "vendor-ops readiness", "vendor-ops deductions", "vendor-ops po-watch", "vendor-ops scorecard"}
	descriptions := map[string]string{
		"agent-context":                 "schema-versioned local-first context, source plan, readiness, commands, and workflows",
		"doctor":                        "local state, child binary, config presence, profile count, and optional deep child doctors",
		"sources doctor":                "per-source child readiness without printing secrets",
		"profile save/list/show/delete": "manage non-secret marketplace, seller, ads profile, COGS, local store, report, and target defaults",
		"sync":                          "load fixture/import data or opt into read-only child CLI sync",
		"war-room":                      "morning control tower with sales, profit, risks, and top actions",
		"restock-or-kill":               "SKU decision queue for restock, conserve, pause ads, liquidate, or fix listing",
		"ad-spend-guardrail":            "read-only ad waste and cash-burn findings joined to inventory and listings",
		"sku-profit-truth":              "per-SKU contribution margin after ads, fees, returns, reimbursements, and COGS",
		"listing-triage":                "listing defects ranked by expected business impact",
		"cash-leaks":                    "wasted spend, storage, stranded inventory, reimbursements, settlement, returns, and low-margin leaks",
		"search-term-actions":           "promote, negate, rank, targeting, and branded-dependency actions from ads and Brand Analytics",
		"digest daily":                  "daily owner report safe for empty datasets",
		"digest weekly":                 "weekly owner report safe for empty datasets",
		"operator-plan":                 "one-week execution plan composed from other command scores",
		"cash-calendar":                 "cash pressure forecast across inventory, ads, revenue, fees, and reimbursements",
		"launch-readiness":              "new ASIN/SKU readiness with inventory, listing, ads, margin, keyword, and 14-day checklist",
		"rank-defense":                  "terms to defend, reduce, or stop based on rank, margin, cash, and inventory",
		"bundle-opportunities":          "market-basket bundle and cross-sell recommendations with rejection reasons",
		"vendor-ops readiness":          "local vendor-file readiness and future source extension status",
		"vendor-ops deductions":         "local deduction/chargeback dispute ranking",
		"vendor-ops po-watch":           "local purchase-order ship-window and fill-rate risk",
		"vendor-ops scorecard":          "vendor-style operational risk summary from local files",
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]any{"name": name, "safe_for_agents": true, "description": descriptions[name]})
	}
	return out
}

func doctorCmd(f *rootFlags) *cobra.Command {
	var deep bool
	c := &cobra.Command{Use: "doctor", Short: "Check local readiness without network calls", RunE: func(cmd *cobra.Command, args []string) error {
		profiles, _ := st(f).ListProfiles()
		checks := sourceChecks()
		result := map[string]any{"ok": true, "version": version, "state_dir": f.home, "profiles": len(profiles), "sources": checks, "env": envChecks()}
		if deep {
			result["deep"] = runDeepDoctors()
		}
		lines := []string{fmt.Sprintf("amazon-operator-intel-pp-cli %s", version), "state: " + f.home, fmt.Sprintf("profiles: %d", len(profiles)), "sources:"}
		for _, c := range checks {
			path := "missing"
			if c.Path != "" {
				path = c.Path
			}
			lines = append(lines, fmt.Sprintf("- %s binary=%s config=%s", c.Name, path, presentConfig(c.RequiredConfig)))
		}
		if deep {
			lines = append(lines, "deep child doctors: requested")
		}
		return out(cmd, f, result, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().BoolVar(&deep, "deep", false, "Also run child CLI doctor commands")
	return c
}

func sourcesCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sources", Short: "Inspect local source adapters"}
	cmd.AddCommand(&cobra.Command{Use: "doctor", Short: "Check Seller and Ads child adapter readiness", RunE: func(cmd *cobra.Command, args []string) error {
		checks := sourceChecks()
		lines := []string{"source\tbinary\tfound\trequired_config\tplanned_commands"}
		for _, c := range checks {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%v\t%s\t%s", c.Name, c.ChildBinary, c.Found, strings.Join(c.RequiredConfig, ","), strings.Join(c.PlannedCommands, " | ")))
		}
		return out(cmd, f, checks, strings.Join(lines, "\n")+"\n")
	}})
	return cmd
}

type sourceCheck struct {
	Name            string   `json:"name"`
	ChildBinary     string   `json:"child_binary"`
	Path            string   `json:"path,omitempty"`
	Found           bool     `json:"found"`
	PlannedCommands []string `json:"planned_commands"`
	RequiredConfig  []string `json:"required_config"`
	Note            string   `json:"note"`
}

func sourceChecks() []sourceCheck {
	defs := []sourceCheck{
		{Name: "seller", ChildBinary: "amazon-seller-pp-cli", PlannedCommands: []string{"amazon-seller-pp-cli fba-inventory --agent --marketplace-ids <marketplace> --granularity-type Marketplace --granularity-id <marketplace>", "amazon-seller-pp-cli profitability sku-pnl --agent --marketplace-id <marketplace> --days <days>", "amazon-seller-pp-cli listing-intel health-audit --agent --marketplace-id <marketplace>", "amazon-seller-pp-cli brand-analytics search-terms --agent --marketplace-id <marketplace> --period WEEK"}, RequiredConfig: []string{"marketplace_id"}, Note: "uses SP-API credentials owned by amazon-seller-pp-cli"},
		{Name: "ads", ChildBinary: "amazon-ads-pp-cli", PlannedCommands: []string{"amazon-ads-pp-cli profiles list --agent", "amazon-ads-pp-cli portfolio-dashboard --agent --report <campaign-performance.csv>", "amazon-ads-pp-cli product-ad-profitability --agent --report <product-performance.csv>", "amazon-ads-pp-cli search-term-mining --agent --report <search-term-report.csv>", "amazon-ads-pp-cli wasted-spend --agent --report <search-term-report.csv>"}, RequiredConfig: []string{"profile_id"}, Note: "uses Amazon Ads OAuth/profile config owned by amazon-ads-pp-cli"},
	}
	for i := range defs {
		if path, err := exec.LookPath(defs[i].ChildBinary); err == nil {
			defs[i].Found = true
			defs[i].Path = path
		}
	}
	return defs
}

func sourcePlans() []map[string]any {
	checks := sourceChecks()
	out := make([]map[string]any, 0, len(checks))
	for _, c := range checks {
		out = append(out, map[string]any{"source": c.Name, "child_binary": c.ChildBinary, "planned_commands": c.PlannedCommands, "required_config": c.RequiredConfig, "note": c.Note})
	}
	return out
}

type envCheck struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
}

func envChecks() []envCheck {
	names := []string{"AMAZON_OPERATOR_INTEL_HOME", "AMAZON_MARKETPLACE_ID", "AMAZON_SELLER_ID", "AMAZON_ADS_PROFILE_ID", "SP_API_LWA_CLIENT_ID", "SP_API_LWA_CLIENT_SECRET", "SP_API_REFRESH_TOKEN", "AMAZON_ADS_CLIENT_ID", "AMAZON_ADS_CLIENT_SECRET", "AMAZON_ADS_REFRESH_TOKEN"}
	out := make([]envCheck, 0, len(names))
	for _, name := range names {
		out = append(out, envCheck{Name: name, Present: os.Getenv(name) != ""})
	}
	return out
}

func presentConfig(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		env := map[string]string{"marketplace_id": "AMAZON_MARKETPLACE_ID", "profile_id": "AMAZON_ADS_PROFILE_ID"}[x]
		status := "absent"
		if x == "marketplace_id" || os.Getenv(env) != "" {
			status = "present"
		}
		parts = append(parts, x+":"+status)
	}
	return strings.Join(parts, ",")
}

func runDeepDoctors() []map[string]any {
	out := []map[string]any{}
	for _, bin := range []string{"amazon-seller-pp-cli", "amazon-ads-pp-cli"} {
		row := map[string]any{"child_binary": bin}
		if _, err := exec.LookPath(bin); err != nil {
			row["found"] = false
			row["ok"] = false
			row["error"] = "binary not found"
			out = append(out, row)
			continue
		}
		c := exec.Command(bin, "--agent", "doctor")
		b, err := c.CombinedOutput()
		row["found"] = true
		row["ok"] = err == nil
		row["output"] = strings.TrimSpace(string(b))
		if err != nil {
			row["error"] = err.Error()
		}
		out = append(out, row)
	}
	return out
}

func profileCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage local non-secret profiles"}
	var p store.Profile
	save := &cobra.Command{Use: "save", Short: "Save a profile", RunE: func(cmd *cobra.Command, args []string) error {
		if p.Name == "" {
			p.Name = f.profile
		}
		if err := st(f).SaveProfile(p); err != nil {
			return err
		}
		return out(cmd, f, p, fmt.Sprintf("saved profile %s\n", p.Name))
	}}
	save.Flags().StringVar(&p.Name, "name", "", "Profile name (defaults to --profile)")
	save.Flags().StringVar(&p.MarketplaceID, "marketplace-id", "ATVPDKIKX0DER", "Amazon marketplace id")
	save.Flags().StringVar(&p.SellerID, "seller-id", "", "Seller id (non-secret)")
	save.Flags().StringVar(&p.AdsProfileID, "ads-profile-id", "", "Amazon Ads profile id")
	save.Flags().IntVar(&p.DefaultDays, "days", 30, "Default date window")
	save.Flags().StringVar(&p.COGSFile, "cogs-file", "", "COGS file path")
	save.Flags().StringVar(&p.SellerStoreDB, "seller-store", "", "Optional amazon-seller local store DB path")
	save.Flags().StringVar(&p.AdsReportDir, "ads-report-dir", "", "Optional normalized Amazon Ads report directory")
	save.Flags().Float64Var(&p.TargetACOS, "target-acos", 0, "Default target ACOS")
	save.Flags().Float64Var(&p.TargetMargin, "target-margin", 0, "Default target contribution margin")
	cmd.AddCommand(save)
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List profiles", RunE: func(cmd *cobra.Command, args []string) error {
		ps, err := st(f).ListProfiles()
		if err != nil {
			return err
		}
		lines := []string{}
		for _, p := range ps {
			lines = append(lines, p.Name+"\t"+p.MarketplaceID+"\t"+p.AdsProfileID)
		}
		return out(cmd, f, ps, strings.Join(lines, "\n")+term(lines))
	}})
	cmd.AddCommand(&cobra.Command{Use: "show [name]", Args: cobra.MaximumNArgs(1), Short: "Show a profile", RunE: func(cmd *cobra.Command, args []string) error {
		n := f.profile
		if len(args) > 0 {
			n = args[0]
		}
		p, err := st(f).GetProfile(n)
		if err != nil {
			return err
		}
		human := fmt.Sprintf("%s\nmarketplace_id: %s\nseller_id: %s\nads_profile_id: %s\ndays: %d\n", p.Name, p.MarketplaceID, p.SellerID, p.AdsProfileID, p.DefaultDays)
		return out(cmd, f, p, human)
	}})
	cmd.AddCommand(&cobra.Command{Use: "delete [name]", Args: cobra.MaximumNArgs(1), Short: "Delete a profile", RunE: func(cmd *cobra.Command, args []string) error {
		n := f.profile
		if len(args) > 0 {
			n = args[0]
		}
		if err := st(f).DeleteProfile(n); err != nil {
			return err
		}
		return out(cmd, f, map[string]any{"deleted": n}, fmt.Sprintf("deleted profile %s\n", n))
	}})
	return cmd
}

func term(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return "\n"
}

type syncOptions struct {
	ImportPath, Source, MarketplaceID, SellerID, AdsProfileID, AdsReportDir, SellerStoreDB, COGSFile, StartDate, EndDate string
	Live, Real                                                                                                           bool
	Days                                                                                                                 int
}

func syncCmd(f *rootFlags) *cobra.Command {
	var opts syncOptions
	c := &cobra.Command{Use: "sync", Short: "Import fixture, local JSON, or read-only child CLI data", RunE: func(cmd *cobra.Command, args []string) error {
		opts.Source = strings.ToLower(strings.TrimSpace(opts.Source))
		if opts.Source == "" && (opts.Live || opts.Real) {
			opts.Source = "all"
		}
		if opts.ImportPath != "" && opts.Source != "" {
			return fmt.Errorf("--import cannot be combined with --source/--live/--real")
		}
		if p, err := st(f).GetProfile(f.profile); err == nil {
			applyProfile(&opts, p)
		}
		var d store.DataSet
		var err error
		if opts.ImportPath == "" && opts.Source == "" {
			d = store.Fixture(f.profile)
		} else if opts.ImportPath != "" {
			d, err = importData(f.profile, opts.ImportPath)
		} else {
			base := store.DataSet{Profile: f.profile, SyncedAt: time.Now().UTC()}
			if existing, loadErr := st(f).LoadData(f.profile); loadErr == nil {
				base = existing
			}
			d, err = syncFromChildCLIs(f.profile, opts, base)
		}
		if err != nil {
			return err
		}
		if opts.COGSFile != "" {
			if err := applyCOGSFile(&d, opts.COGSFile); err != nil {
				return err
			}
		}
		if err := st(f).SaveData(d); err != nil {
			return err
		}
		summary := map[string]any{"profile": d.Profile, "source": d.Source, "synced_at": d.SyncedAt, "skus": len(d.SKUs), "campaigns": len(d.Campaigns), "search_terms": len(d.SearchTerms)}
		return out(cmd, f, summary, fmt.Sprintf("synced %d SKUs, %d campaigns, %d search terms for %s from %s\n", len(d.SKUs), len(d.Campaigns), len(d.SearchTerms), d.Profile, d.Source))
	}}
	c.Flags().StringVar(&opts.ImportPath, "import", "", "Import JSON DataSet or entity array instead of fixture")
	c.Flags().StringVar(&opts.Source, "source", "", "Real child CLI source to sync: all, seller, or ads")
	c.Flags().BoolVar(&opts.Live, "live", false, "Use child CLIs instead of the embedded fixture")
	c.Flags().BoolVar(&opts.Real, "real", false, "Use child CLIs instead of the embedded fixture")
	c.Flags().StringVar(&opts.MarketplaceID, "marketplace-id", "", "Marketplace id (default profile/env/US)")
	c.Flags().StringVar(&opts.SellerID, "seller-id", "", "Seller id")
	c.Flags().StringVar(&opts.AdsProfileID, "ads-profile-id", "", "Amazon Ads profile id")
	c.Flags().StringVar(&opts.AdsReportDir, "ads-report-dir", "", "Directory containing normalized Amazon Ads reports")
	c.Flags().StringVar(&opts.SellerStoreDB, "seller-store", "", "amazon-seller local store DB path")
	c.Flags().StringVar(&opts.COGSFile, "cogs-file", "", "COGS file path")
	c.Flags().IntVar(&opts.Days, "days", 30, "Date window")
	c.Flags().StringVar(&opts.StartDate, "start-date", "", "Start date")
	c.Flags().StringVar(&opts.EndDate, "end-date", "", "End date")
	return c
}

func applyProfile(opts *syncOptions, p store.Profile) {
	if opts.MarketplaceID == "" {
		opts.MarketplaceID = p.MarketplaceID
	}
	if opts.SellerID == "" {
		opts.SellerID = p.SellerID
	}
	if opts.AdsProfileID == "" {
		opts.AdsProfileID = p.AdsProfileID
	}
	if opts.AdsReportDir == "" {
		opts.AdsReportDir = p.AdsReportDir
	}
	if opts.SellerStoreDB == "" {
		opts.SellerStoreDB = p.SellerStoreDB
	}
	if opts.COGSFile == "" {
		opts.COGSFile = p.COGSFile
	}
	if opts.Days == 0 {
		opts.Days = p.DefaultDays
	}
}

func importData(profile, path string) (store.DataSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return store.DataSet{}, err
	}
	var d store.DataSet
	if err := json.Unmarshal(b, &d); err == nil && (len(d.SKUs) > 0 || len(d.Campaigns) > 0 || len(d.SearchTerms) > 0 || len(d.VendorDeductions) > 0) {
		if d.Profile == "" {
			d.Profile = profile
		}
		if d.Source == "" {
			d.Source = path
		}
		stampLocalImport(&d, path)
		return d, nil
	}
	var skus []store.SKU
	if err := json.Unmarshal(b, &skus); err == nil && len(skus) > 0 {
		d = store.DataSet{Profile: profile, Source: path, SyncedAt: time.Now().UTC(), SKUs: skus}
		stampLocalImport(&d, path)
		return d, nil
	}
	return store.DataSet{}, fmt.Errorf("import %s must be a DataSet or []SKU JSON file", path)
}

func stampLocalImport(d *store.DataSet, path string) {
	ev := store.SourceEvidence{Present: true, Source: "local-import", ImportedFrom: path, SyncedAt: time.Now().UTC()}
	for i := range d.SKUs {
		d.SKUs[i].Source.LocalImport = ev
	}
	for i := range d.Campaigns {
		d.Campaigns[i].Source.LocalImport = ev
	}
	for i := range d.SearchTerms {
		d.SearchTerms[i].Source.LocalImport = ev
	}
	for i := range d.PurchaseOrders {
		d.PurchaseOrders[i].Source.LocalImport = ev
	}
	for i := range d.VendorDeductions {
		d.VendorDeductions[i].Source.LocalImport = ev
		d.VendorDeductions[i].Source.VendorFiles = ev
	}
}

func load(f *rootFlags) (store.DataSet, error) {
	d, err := st(f).LoadData(f.profile)
	if err != nil {
		return d, fmt.Errorf("no local data for profile %q: run sync first", f.profile)
	}
	return d, nil
}

func loadOrEmpty(f *rootFlags) store.DataSet {
	d, err := st(f).LoadData(f.profile)
	if err != nil {
		return store.DataSet{Profile: f.profile, Source: "empty", SyncedAt: time.Now().UTC()}
	}
	return d
}

func syncFromChildCLIs(profile string, opts syncOptions, base store.DataSet) (store.DataSet, error) {
	if opts.MarketplaceID == "" {
		opts.MarketplaceID = firstNonEmpty(os.Getenv("AMAZON_MARKETPLACE_ID"), "ATVPDKIKX0DER")
	}
	if opts.AdsProfileID == "" {
		opts.AdsProfileID = os.Getenv("AMAZON_ADS_PROFILE_ID")
	}
	if opts.Days == 0 {
		opts.Days = 30
	}
	if days := daysFromRange(opts.StartDate, opts.EndDate); days > 0 {
		opts.Days = days
	}
	switch opts.Source {
	case "all":
		if opts.MarketplaceID == "" || opts.AdsProfileID == "" {
			return store.DataSet{}, fmt.Errorf("--source all requires marketplace_id and ads_profile_id before running child commands")
		}
		if err := validateAdsReports(opts.AdsReportDir); err != nil {
			return store.DataSet{}, err
		}
	case "seller":
		if opts.MarketplaceID == "" {
			return store.DataSet{}, fmt.Errorf("--source seller requires --marketplace-id or AMAZON_MARKETPLACE_ID")
		}
	case "ads":
		if opts.AdsProfileID == "" {
			return store.DataSet{}, fmt.Errorf("--source ads requires --ads-profile-id or AMAZON_ADS_PROFILE_ID")
		}
		if err := validateAdsReports(opts.AdsReportDir); err != nil {
			return store.DataSet{}, err
		}
	default:
		return store.DataSet{}, fmt.Errorf("unknown source %q (want all, seller, or ads)", opts.Source)
	}

	used := []string{}
	if opts.Source == "all" || opts.Source == "seller" {
		seller, err := runSellerSync(profile, opts)
		if err != nil {
			return store.DataSet{}, err
		}
		base = mergeData(base, seller)
		used = append(used, "seller")
	}
	if opts.Source == "all" || opts.Source == "ads" {
		ads, err := runAdsSync(profile, opts)
		if err != nil {
			return store.DataSet{}, err
		}
		base = mergeData(base, ads)
		used = append(used, "ads")
	}
	base.Profile = profile
	base.Source = "child-cli:" + strings.Join(used, "+")
	base.SyncedAt = time.Now().UTC()
	return base, nil
}

func validateAdsReports(dir string) error {
	if dir == "" {
		dir = "."
	}
	missing := []string{}
	for _, name := range []string{"campaign-performance.csv", "product-performance.csv", "search-term-report.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("--source ads requires normalized report files in --ads-report-dir; missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func runSellerSync(profile string, opts syncOptions) (store.DataSet, error) {
	commands := [][]string{
		{"fba-inventory", "--agent", "--marketplace-ids", opts.MarketplaceID, "--granularity-type", "Marketplace", "--granularity-id", opts.MarketplaceID},
		{"profitability", "sku-pnl", "--agent", "--marketplace-id", opts.MarketplaceID, "--days", strconv.Itoa(opts.Days)},
		{"listing-intel", "health-audit", "--agent", "--marketplace-id", opts.MarketplaceID},
		{"brand-analytics", "search-terms", "--agent", "--marketplace-id", opts.MarketplaceID, "--period", "WEEK"},
	}
	out := store.DataSet{Profile: profile, SyncedAt: time.Now().UTC(), Source: "child-cli:seller"}
	for _, args := range commands {
		rows, command, err := runChild("seller", "amazon-seller-pp-cli", args)
		if err != nil {
			return out, err
		}
		out.SKUs = append(out.SKUs, parseSellerSKUs(rows, command)...)
		out.SearchTerms = append(out.SearchTerms, parseSellerSearchTerms(rows, command)...)
	}
	return out, nil
}

func runAdsSync(profile string, opts syncOptions) (store.DataSet, error) {
	reportDir := firstNonEmpty(opts.AdsReportDir, ".")
	commands := [][]string{
		{"profiles", "list", "--agent"},
		{"portfolio-dashboard", "--agent", "--report", reportDir + "/campaign-performance.csv"},
		{"product-ad-profitability", "--agent", "--report", reportDir + "/product-performance.csv"},
		{"search-term-mining", "--agent", "--report", reportDir + "/search-term-report.csv"},
		{"wasted-spend", "--agent", "--report", reportDir + "/search-term-report.csv"},
	}
	out := store.DataSet{Profile: profile, SyncedAt: time.Now().UTC(), Source: "child-cli:ads"}
	for _, args := range commands {
		rows, command, err := runChildWithEnv("ads", "amazon-ads-pp-cli", args, []string{"AMAZON_ADS_PROFILE_ID=" + opts.AdsProfileID})
		if err != nil {
			return out, err
		}
		out.SKUs = append(out.SKUs, parseAdsSKUs(rows, command)...)
		out.Campaigns = append(out.Campaigns, parseAdsCampaigns(rows, command)...)
		out.SearchTerms = append(out.SearchTerms, parseAdsSearchTerms(rows, command)...)
	}
	return out, nil
}

func runChild(source, binary string, args []string) ([]map[string]any, string, error) {
	return runChildWithEnv(source, binary, args, nil)
}

func runChildWithEnv(source, binary string, args []string, env []string) ([]map[string]any, string, error) {
	command := binary + " " + strings.Join(args, " ")
	c := exec.Command(binary, args...)
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	b, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, command, fmt.Errorf("%s failed: %w: %s", source, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, command, fmt.Errorf("%s failed: %w", source, err)
	}
	var v any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, command, fmt.Errorf("%s returned invalid JSON: %w", source, err)
	}
	rows := findRows(v)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if m, ok := row.(map[string]any); ok {
			out = append(out, flattenMap(m, ""))
		}
	}
	return out, command, nil
}

func daysFromRange(start, end string) int {
	if start == "" || end == "" {
		return 0
	}
	a, err := time.Parse("2006-01-02", start)
	if err != nil {
		return 0
	}
	b, err := time.Parse("2006-01-02", end)
	if err != nil {
		return 0
	}
	days := int(b.Sub(a).Hours()/24) + 1
	if days < 1 {
		return 0
	}
	return days
}

func parseSellerSKUs(rows []map[string]any, command string) []store.SKU {
	ev := store.SourceEvidence{Present: true, Source: "child-cli", ChildCLICommand: command, SyncedAt: time.Now().UTC()}
	out := []store.SKU{}
	for _, m := range rows {
		skuID := firstString(m, "sku", "seller_sku", "sellerSku", "fnsku")
		asin := firstString(m, "asin", "child_asin")
		if skuID == "" && asin == "" {
			continue
		}
		s := store.SKU{SKU: skuID, ASIN: asin, Title: firstString(m, "title", "item_name", "product_name"), FBAAvailable: firstInt(m, "fba_available", "available", "available_quantity", "quantity_available", "fulfillable_quantity", "inventory"), DaysOfCover: firstFloat(m, "days_of_cover", "daysCover", "cover_days"), UnitsSold: firstInt(m, "units_sold", "ordered_units", "units"), Revenue: firstFloat(m, "revenue", "sales", "ordered_product_sales"), Profit: firstFloat(m, "profit", "contribution_profit", "net_profit"), ContributionMargin: firstFloat(m, "contribution_margin", "margin"), ReturnRate: firstFloat(m, "return_rate", "refund_rate"), AgingDays: firstInt(m, "aging_days", "age_days", "oldest_inventory_days"), Suppressed: firstBool(m, "suppressed", "is_suppressed"), ListingScore: firstFloat(m, "listing_score", "health_score"), Source: store.MetricSources{Seller: ev, Listings: ev}}
		if s.ContributionMargin == 0 && s.Revenue > 0 {
			s.ContributionMargin = s.Profit / s.Revenue
		}
		out = append(out, s)
	}
	return out
}

func parseSellerSearchTerms(rows []map[string]any, command string) []store.SearchTerm {
	ev := store.SourceEvidence{Present: true, Source: "child-cli", ChildCLICommand: command, SyncedAt: time.Now().UTC()}
	out := []store.SearchTerm{}
	for _, m := range rows {
		term := firstString(m, "term", "search_term", "query", "keyword")
		if term == "" {
			continue
		}
		out = append(out, store.SearchTerm{Term: term, SKU: firstString(m, "sku", "seller_sku"), ASIN: firstString(m, "asin"), OrganicRank: firstInt(m, "organic_rank", "rank"), ClickShare: firstFloat(m, "click_share"), Source: store.MetricSources{BrandAnalytics: ev}})
	}
	return out
}

func parseAdsSKUs(rows []map[string]any, command string) []store.SKU {
	ev := store.SourceEvidence{Present: true, Source: "child-cli", ChildCLICommand: command, SyncedAt: time.Now().UTC()}
	out := []store.SKU{}
	for _, m := range rows {
		skuID := firstString(m, "sku", "advertised_sku", "seller_sku")
		asin := firstString(m, "asin", "advertised_asin")
		spend := firstFloat(m, "spend", "cost", "ad_spend")
		sales := firstFloat(m, "sales", "ad_sales", "attributed_sales")
		if skuID == "" && asin == "" && spend == 0 {
			continue
		}
		out = append(out, store.SKU{SKU: skuID, ASIN: asin, AdSpend: spend, AdSales: sales, ACOS: firstNonZeroFloat(firstFloat(m, "acos"), div(spend, sales)), TACOS: firstFloat(m, "tacos"), BreakEvenACOS: firstFloat(m, "break_even_acos", "breakeven_acos"), Source: store.MetricSources{Ads: ev}})
	}
	return out
}

func parseAdsCampaigns(rows []map[string]any, command string) []store.Campaign {
	ev := store.SourceEvidence{Present: true, Source: "child-cli", ChildCLICommand: command, SyncedAt: time.Now().UTC()}
	out := []store.Campaign{}
	for _, m := range rows {
		id := firstString(m, "campaign_id", "campaignid", "id")
		name := firstString(m, "campaign_name", "name")
		spend := firstFloat(m, "spend", "cost")
		if id == "" && name == "" && spend == 0 {
			continue
		}
		out = append(out, store.Campaign{CampaignID: firstNonEmpty(id, name), Name: name, SKU: firstString(m, "sku", "advertised_sku"), ASIN: firstString(m, "asin", "advertised_asin"), Spend: spend, Sales: firstFloat(m, "sales", "ad_sales"), Orders: firstInt(m, "orders", "purchases"), Clicks: firstInt(m, "clicks"), Impressions: firstInt(m, "impressions"), ACOS: firstFloat(m, "acos"), BudgetStatus: firstString(m, "budget_status", "status"), Source: store.MetricSources{Ads: ev}})
	}
	return out
}

func parseAdsSearchTerms(rows []map[string]any, command string) []store.SearchTerm {
	ev := store.SourceEvidence{Present: true, Source: "child-cli", ChildCLICommand: command, SyncedAt: time.Now().UTC()}
	out := []store.SearchTerm{}
	for _, m := range rows {
		term := firstString(m, "search_term", "term", "query", "keyword")
		if term == "" {
			continue
		}
		out = append(out, store.SearchTerm{Term: term, SKU: firstString(m, "sku", "advertised_sku"), ASIN: firstString(m, "asin", "advertised_asin"), Spend: firstFloat(m, "spend", "cost"), Sales: firstFloat(m, "sales", "ad_sales"), Orders: firstInt(m, "orders", "purchases"), Clicks: firstInt(m, "clicks"), Impressions: firstInt(m, "impressions"), AdAction: firstString(m, "action", "recommendation"), Source: store.MetricSources{Ads: ev}})
	}
	return out
}

func mergeData(base, next store.DataSet) store.DataSet {
	now := time.Now().UTC()
	skus := map[string]store.SKU{}
	order := []string{}
	for _, s := range base.SKUs {
		k := skuKey(s)
		if k == "" {
			continue
		}
		skus[k] = s
		order = append(order, k)
	}
	for _, s := range next.SKUs {
		k := skuKey(s)
		if k == "" {
			k = fmt.Sprintf("orphan-%d", len(order)+1)
		}
		if _, ok := skus[k]; !ok {
			order = append(order, k)
		}
		skus[k] = mergeSKU(skus[k], s)
	}
	base.SKUs = base.SKUs[:0]
	for _, k := range order {
		base.SKUs = append(base.SKUs, skus[k])
	}
	base.Campaigns = mergeCampaigns(base.Campaigns, next.Campaigns)
	base.SearchTerms = mergeSearchTerms(base.SearchTerms, next.SearchTerms)
	base.Listings = mergeListings(base.Listings, next.Listings)
	base.PurchaseOrders = append(base.PurchaseOrders, next.PurchaseOrders...)
	base.VendorDeductions = append(base.VendorDeductions, next.VendorDeductions...)
	base.BundleSignals = append(base.BundleSignals, next.BundleSignals...)
	base.LaunchPlans = append(base.LaunchPlans, next.LaunchPlans...)
	base.SyncedAt = now
	return base
}

func skuKey(s store.SKU) string {
	if s.SKU != "" {
		return "sku:" + strings.ToLower(s.SKU)
	}
	if s.ASIN != "" {
		return "asin:" + strings.ToLower(s.ASIN)
	}
	return ""
}

func mergeSKU(a, b store.SKU) store.SKU {
	if a.SKU == "" {
		a.SKU = b.SKU
	}
	if a.ASIN == "" {
		a.ASIN = b.ASIN
	}
	if a.Title == "" {
		a.Title = b.Title
	}
	if b.FBAAvailable != 0 {
		a.FBAAvailable = b.FBAAvailable
	}
	if b.DaysOfCover != 0 {
		a.DaysOfCover = b.DaysOfCover
	}
	if b.Revenue != 0 {
		a.Revenue = b.Revenue
	}
	if b.Profit != 0 {
		a.Profit = b.Profit
	}
	if b.ContributionMargin != 0 {
		a.ContributionMargin = b.ContributionMargin
	}
	if b.AdSpend != 0 {
		a.AdSpend = b.AdSpend
	}
	if b.AdSales != 0 {
		a.AdSales = b.AdSales
	}
	if b.ACOS != 0 {
		a.ACOS = b.ACOS
	}
	if b.BreakEvenACOS != 0 {
		a.BreakEvenACOS = b.BreakEvenACOS
	}
	if b.ListingScore != 0 {
		a.ListingScore = b.ListingScore
	}
	if b.Suppressed {
		a.Suppressed = true
	}
	a.Defects = uniqueStrings(append(a.Defects, b.Defects...))
	a.Source = mergeSources(a.Source, b.Source)
	return a
}

func mergeSources(a, b store.MetricSources) store.MetricSources {
	if b.Seller.Present {
		a.Seller = b.Seller
	}
	if b.Ads.Present {
		a.Ads = b.Ads
	}
	if b.BrandAnalytics.Present {
		a.BrandAnalytics = b.BrandAnalytics
	}
	if b.Listings.Present {
		a.Listings = b.Listings
	}
	if b.Reports.Present {
		a.Reports = b.Reports
	}
	if b.LocalImport.Present {
		a.LocalImport = b.LocalImport
	}
	if b.VendorFiles.Present {
		a.VendorFiles = b.VendorFiles
	}
	return a
}

func mergeCampaigns(a, b []store.Campaign) []store.Campaign {
	seen := map[string]store.Campaign{}
	for _, c := range a {
		seen[firstNonEmpty(c.CampaignID, c.Name)] = c
	}
	for _, c := range b {
		seen[firstNonEmpty(c.CampaignID, c.Name)] = c
	}
	return sortedMapValues(seen)
}

func mergeSearchTerms(a, b []store.SearchTerm) []store.SearchTerm {
	seen := map[string]store.SearchTerm{}
	for _, s := range a {
		seen[termKey(s)] = s
	}
	for _, s := range b {
		k := termKey(s)
		old := seen[k]
		if old.Term == "" {
			seen[k] = s
			continue
		}
		old.Spend += s.Spend
		old.Sales += s.Sales
		old.Orders += s.Orders
		old.Clicks += s.Clicks
		old.Impressions += s.Impressions
		if s.OrganicRank != 0 {
			old.OrganicRank = s.OrganicRank
		}
		if s.AdAction != "" {
			old.AdAction = s.AdAction
		}
		old.Source = mergeSources(old.Source, s.Source)
		seen[k] = old
	}
	out := make([]store.SearchTerm, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return termScore(out[i]) > termScore(out[j]) })
	return out
}

func mergeListings(a, b []store.ListingHealth) []store.ListingHealth {
	seen := map[string]store.ListingHealth{}
	for _, l := range a {
		seen[firstNonEmpty(l.SKU, l.ASIN)] = l
	}
	for _, l := range b {
		seen[firstNonEmpty(l.SKU, l.ASIN)] = l
	}
	out := make([]store.ListingHealth, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	return out
}

func sortedMapValues[T any](m map[string]T) []T {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

func termKey(s store.SearchTerm) string {
	return strings.ToLower(strings.TrimSpace(s.Term + "|" + s.SKU + "|" + s.ASIN))
}

func findRows(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		for _, key := range []string{"skus", "campaigns", "search_terms", "terms", "rows", "items", "results", "data"} {
			if rows, ok := x[key].([]any); ok {
				return rows
			}
		}
		for _, key := range []string{"result", "response", "payload"} {
			if nested, ok := x[key]; ok {
				if rows := findRows(nested); len(rows) > 0 {
					return rows
				}
			}
		}
	}
	return nil
}

func flattenMap(in map[string]any, prefix string) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		key := normKey(k)
		if prefix != "" {
			key = prefix + "." + key
		}
		if m, ok := v.(map[string]any); ok {
			for nk, nv := range flattenMap(m, key) {
				out[nk] = nv
			}
			continue
		}
		out[key] = v
	}
	return out
}

func warRoomCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "war-room", Short: "Morning Amazon operator control tower", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		actions := topActions(d, 5)
		row := map[string]any{"profile": d.Profile, "synced_at": d.SyncedAt, "revenue": totalRevenue(d.SKUs), "profit_estimate": totalProfit(d.SKUs), "inventory_risk_count": countSKUs(d.SKUs, inventoryRisk), "ad_waste_count": len(adWasteRows(d, 0)), "listing_defect_count": countListings(d.Listings), "returns_reimbursement_flags": d.Account.ReturnSpikeCount + d.Account.ReimbursementFlags, "top_actions": actions}
		lines := []string{fmt.Sprintf("revenue %.2f profit %.2f", row["revenue"], row["profit_estimate"]), fmt.Sprintf("inventory risks %d | ad waste %d | listing defects %d", row["inventory_risk_count"], row["ad_waste_count"], row["listing_defect_count"]), "top actions:"}
		for _, a := range actions {
			lines = append(lines, "- "+a["action"].(string))
		}
		return out(cmd, f, row, strings.Join(lines, "\n")+"\n")
	}}
}

func restockOrKillCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "restock-or-kill", Short: "SKU-level inventory and ad decision queue", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := restockRows(d)
		sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
		rows = limitRows(rows, limit)
		return out(cmd, f, rows, table(rows, []string{"sku", "decision", "reason", "estimated_cash_at_risk"}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "Rows to return")
	return c
}

func adSpendGuardrailCmd(f *rootFlags) *cobra.Command {
	var limit int
	var skuFilter string
	c := &cobra.Command{Use: "ad-spend-guardrail", Short: "Read-only ad spend waste and margin guardrails", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := adWasteRows(d, limit)
		if skuFilter != "" {
			rows = filterRows(rows, "sku", skuFilter)
		}
		return out(cmd, f, rows, table(rows, []string{"sku", "issue", "cash_at_risk", "recommendation"}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "Rows to return")
	c.Flags().StringVar(&skuFilter, "sku", "", "Filter by SKU")
	return c
}

func skuProfitTruthCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "sku-profit-truth", Short: "Per-SKU contribution profit after ads and fees", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := []map[string]any{}
		for _, s := range d.SKUs {
			profit := s.Revenue - s.COGS - s.ReferralFees - s.FBAFees - s.StorageFees - s.AdSpend + s.Reimbursements
			rows = append(rows, map[string]any{"sku": s.SKU, "asin": s.ASIN, "revenue": s.Revenue, "ad_sales": s.AdSales, "ad_spend": s.AdSpend, "fees": s.ReferralFees + s.FBAFees + s.StorageFees, "returns_rate": s.ReturnRate, "reimbursements": s.Reimbursements, "cogs": s.COGS, "contribution_profit": profit, "margin_after_ads": div(profit, s.Revenue), "source": s.Source})
		}
		sort.Slice(rows, func(i, j int) bool {
			return asFloat(rows[i]["contribution_profit"]) < asFloat(rows[j]["contribution_profit"])
		})
		rows = limitRows(rows, limit)
		return out(cmd, f, rows, table(rows, []string{"sku", "revenue", "ad_spend", "contribution_profit", "margin_after_ads"}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "Rows to return")
	return c
}

func listingTriageCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "listing-triage", Short: "Rank listing issues by business impact", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := listingRows(d)
		sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
		rows = limitRows(rows, limit)
		return out(cmd, f, rows, table(rows, []string{"sku", "issue", "score", "recommended_action"}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "Rows to return")
	return c
}

func cashLeaksCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "cash-leaks", Short: "Money leaks across ads, inventory, returns, and reimbursements", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := cashLeakRows(d)
		sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["amount"]) > asFloat(rows[j]["amount"]) })
		return out(cmd, f, rows, table(rows, []string{"type", "sku", "amount", "action"}))
	}}
}

func searchTermActionsCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "search-term-actions", Short: "Bridge Ads search terms and Seller Brand Analytics", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := searchTermRows(d)
		sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
		rows = limitRows(rows, limit)
		return out(cmd, f, rows, table(rows, []string{"term", "sku", "action", "reason"}))
	}}
	c.Flags().IntVar(&limit, "limit", 20, "Rows to return")
	return c
}

func digestCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "digest", Short: "Owner-facing daily or weekly report"}
	for _, period := range []string{"daily", "weekly"} {
		p := period
		cmd.AddCommand(&cobra.Command{Use: p, Short: p + " digest", RunE: func(cmd *cobra.Command, args []string) error {
			d := loadOrEmpty(f)
			doc := map[string]any{"period": p, "profile": d.Profile, "what_changed": digestChanged(d, p), "needs_action": topActions(d, 5), "can_wait": monitorRows(d), "verify_next": []string{"amazon-operator-intel-pp-cli --agent war-room", "amazon-operator-intel-pp-cli --agent cash-calendar"}}
			return out(cmd, f, doc, fmt.Sprintf("%s digest: %d actions\n", p, len(doc["needs_action"].([]map[string]any))))
		}})
	}
	return cmd
}

func operatorPlanCmd(f *rootFlags) *cobra.Command {
	var days, maxActions int
	var weekOf, owners string
	var capacity float64
	c := &cobra.Command{Use: "operator-plan", Short: "One-week owner/delegate execution plan", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		_ = days
		if capacity == 0 {
			capacity = 8
		}
		ownerList := splitCSV(firstNonEmpty(owners, "founder,ops,ads,content,finance"))
		actions := planRows(d, ownerList, maxActions)
		doc := map[string]any{"week_of": firstNonEmpty(weekOf, time.Now().Format("2006-01-02")), "capacity_hours": capacity, "actions": groupActions(actions)}
		return out(cmd, f, doc, fmt.Sprintf("operator plan: %d actions\n", len(actions)))
	}}
	c.Flags().IntVar(&days, "days", 7, "Plan horizon")
	c.Flags().StringVar(&weekOf, "week-of", "", "Week start date")
	c.Flags().Float64Var(&capacity, "capacity-hours", 8, "Available capacity")
	c.Flags().StringVar(&owners, "owner", "founder,ops,ads,content,finance", "Comma-separated owner labels")
	c.Flags().IntVar(&maxActions, "max-actions", 10, "Maximum actions")
	return c
}

func cashCalendarCmd(f *rootFlags) *cobra.Command {
	var horizon int
	var cash, budgetCap float64
	var poFile, cogsFile string
	c := &cobra.Command{Use: "cash-calendar", Short: "Forecast cash pressure and tradeoffs", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		if horizon == 0 {
			horizon = 30
		}
		if poFile != "" {
			pos, err := loadPurchaseOrders(poFile)
			if err != nil {
				return err
			}
			d.PurchaseOrders = append(d.PurchaseOrders, pos...)
		}
		if cogsFile != "" {
			if err := applyCOGSFile(&d, cogsFile); err != nil {
				return err
			}
		}
		doc := cashCalendar(d, horizon, cash, budgetCap)
		return out(cmd, f, doc, fmt.Sprintf("cash calendar horizon %d days, crunch dates %d\n", horizon, len(doc["cash_crunch_dates"].([]string))))
	}}
	c.Flags().IntVar(&horizon, "horizon-days", 30, "Forecast horizon")
	c.Flags().Float64Var(&cash, "cash-on-hand", 0, "Starting cash")
	c.Flags().StringVar(&poFile, "planned-po-file", "", "Planned purchase order CSV/JSON")
	c.Flags().Float64Var(&budgetCap, "ad-budget-cap", 0, "Account-wide ad budget cap")
	c.Flags().StringVar(&cogsFile, "cogs-file", "", "COGS file")
	return c
}

func launchReadinessCmd(f *rootFlags) *cobra.Command {
	var asin, skuID, keywordFile, listingFile string
	var targetACOS, budget, cogs float64
	var units int
	c := &cobra.Command{Use: "launch-readiness", Short: "Evaluate new ASIN/SKU launch readiness", RunE: func(cmd *cobra.Command, args []string) error {
		d := loadOrEmpty(f)
		keywords := []string{}
		if keywordFile != "" {
			keywords, _ = loadLines(keywordFile)
		}
		plan := findLaunch(d, skuID, asin)
		if plan.SKU == "" {
			plan = store.LaunchPlan{SKU: skuID, ASIN: asin, TargetACOS: targetACOS, LaunchBudget: budget, InventoryUnits: units, COGS: cogs, Keywords: keywords, ListingScore: 70}
		}
		if targetACOS != 0 {
			plan.TargetACOS = targetACOS
		}
		if budget != 0 {
			plan.LaunchBudget = budget
		}
		if units != 0 {
			plan.InventoryUnits = units
		}
		if cogs != 0 {
			plan.COGS = cogs
		}
		if len(keywords) > 0 {
			plan.Keywords = keywords
		}
		if listingFile != "" {
			plan.ListingScore = maxf(plan.ListingScore, 72)
		}
		doc := launchDecision(plan, findSKU(d, plan.SKU, plan.ASIN))
		return out(cmd, f, doc, fmt.Sprintf("launch decision: %s\n", doc["decision"]))
	}}
	c.Flags().StringVar(&asin, "asin", "", "ASIN")
	c.Flags().StringVar(&skuID, "sku", "", "SKU")
	c.Flags().Float64Var(&targetACOS, "target-acos", 0, "Target ACOS")
	c.Flags().Float64Var(&budget, "launch-budget", 0, "Launch budget")
	c.Flags().IntVar(&units, "inventory-units", 0, "Inventory units")
	c.Flags().Float64Var(&cogs, "cogs", 0, "Unit COGS")
	c.Flags().StringVar(&keywordFile, "keyword-file", "", "Keyword file")
	c.Flags().StringVar(&listingFile, "listing-file", "", "Listing file")
	return c
}

func rankDefenseCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "rank-defense", Short: "Trade off rank defense against cash and inventory", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		doc := rankDefenseRows(d)
		return out(cmd, f, doc, fmt.Sprintf("rank-defense defend=%d reduce=%d do_not_defend=%d\n", len(doc["defend"].([]map[string]any)), len(doc["reduce"].([]map[string]any)), len(doc["do_not_defend"].([]map[string]any))))
	}}
}

func bundleOpportunitiesCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "bundle-opportunities", Short: "Bundle, cross-sell, and virtual kit opportunities", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := bundleRows(d)
		return out(cmd, f, rows, table(rows, []string{"primary_sku", "secondary_sku", "decision", "reason"}))
	}}
}

func vendorOpsCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "vendor-ops", Short: "Local-file vendor operations workflows"}
	var file string
	var fixture bool
	cmd.AddCommand(&cobra.Command{Use: "readiness", Short: "Report vendor source readiness", RunE: func(cmd *cobra.Command, args []string) error {
		d := loadOrEmpty(f)
		doc := map[string]any{"vendor_api_configured": false, "source": "local-import", "deductions": len(d.VendorDeductions), "purchase_orders": len(d.PurchaseOrders), "note": "Vendor Central APIs are not called in this release; use local CSV/JSON files."}
		return out(cmd, f, doc, "vendor ops uses local files; live vendor APIs not configured\n")
	}})
	deductions := &cobra.Command{Use: "deductions", Short: "Rank local deductions and chargebacks", RunE: func(cmd *cobra.Command, args []string) error {
		d := loadOrEmpty(f)
		if fixture {
			d = store.Fixture(f.profile)
		}
		if file != "" {
			rows, err := loadVendorDeductions(file)
			if err != nil {
				return err
			}
			d.VendorDeductions = rows
		}
		rows := deductionRows(d.VendorDeductions)
		return out(cmd, f, rows, table(rows, []string{"id", "type", "amount", "recommendation"}))
	}}
	deductions.Flags().StringVar(&file, "file", "", "CSV/JSON deductions file")
	deductions.Flags().BoolVar(&fixture, "fixture", false, "Use embedded fixture vendor deductions")
	cmd.AddCommand(deductions)
	poWatch := &cobra.Command{Use: "po-watch", Short: "Surface local purchase-order ship-window or fill-rate risk", RunE: func(cmd *cobra.Command, args []string) error {
		d := loadOrEmpty(f)
		if fixture {
			d = store.Fixture(f.profile)
		}
		if file != "" {
			rows, err := loadPurchaseOrders(file)
			if err != nil {
				return err
			}
			d.PurchaseOrders = rows
		}
		rows := poRiskRows(d.PurchaseOrders)
		return out(cmd, f, rows, table(rows, []string{"po_id", "sku", "risk", "recommendation"}))
	}}
	poWatch.Flags().StringVar(&file, "file", "", "CSV/JSON purchase-order file")
	poWatch.Flags().BoolVar(&fixture, "fixture", false, "Use embedded fixture purchase orders")
	cmd.AddCommand(poWatch)
	scorecard := &cobra.Command{Use: "scorecard", Short: "Summarize vendor-style operational risks", RunE: func(cmd *cobra.Command, args []string) error {
		d := loadOrEmpty(f)
		if fixture {
			d = store.Fixture(f.profile)
		}
		doc := map[string]any{"source": "local-import", "deduction_risk_count": len(deductionRows(d.VendorDeductions)), "po_risk_count": len(poRiskRows(d.PurchaseOrders)), "total_deduction_amount": totalDeductions(d.VendorDeductions), "note": "local-file provenance; no live vendor APIs called"}
		return out(cmd, f, doc, fmt.Sprintf("vendor scorecard: %d deduction risks, %d PO risks\n", doc["deduction_risk_count"], doc["po_risk_count"]))
	}}
	scorecard.Flags().BoolVar(&fixture, "fixture", false, "Use embedded fixture vendor data")
	cmd.AddCommand(scorecard)
	return cmd
}

func totalRevenue(skus []store.SKU) float64 {
	total := 0.0
	for _, s := range skus {
		total += s.Revenue
	}
	return total
}
func totalProfit(skus []store.SKU) float64 {
	total := 0.0
	for _, s := range skus {
		total += s.Profit
	}
	return total
}
func inventoryRisk(s store.SKU) bool {
	return s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) || s.FBAAvailable == 0 || s.Stranded || s.AgingDays > 180
}
func countSKUs(skus []store.SKU, pred func(store.SKU) bool) int {
	n := 0
	for _, s := range skus {
		if pred(s) {
			n++
		}
	}
	return n
}
func countListings(listings []store.ListingHealth) int {
	n := 0
	for _, l := range listings {
		if l.Suppressed || len(l.Defects) > 0 || l.Score < 75 {
			n++
		}
	}
	return n
}

func topActions(d store.DataSet, limit int) []map[string]any {
	rows := append(restockRows(d), adWasteRows(d, 0)...)
	rows = append(rows, listingRows(d)...)
	rows = append(rows, cashLeakRows(d)...)
	sort.Slice(rows, func(i, j int) bool {
		return asFloat(rows[i]["score"])+asFloat(rows[i]["cash_at_risk"])+asFloat(rows[i]["amount"]) > asFloat(rows[j]["score"])+asFloat(rows[j]["cash_at_risk"])+asFloat(rows[j]["amount"])
	})
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, r := range rows {
		if r["decision"] == "keep_monitoring" && asFloat(r["score"]) == 0 {
			continue
		}
		action := firstMapString(r, "recommendation", "recommended_action", "action")
		if action == "" {
			action = "review " + firstMapString(r, "sku", "type", "term")
		}
		key := action + "|" + firstMapString(r, "sku", "type", "term")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, map[string]any{"action": action, "sku": r["sku"], "cash_impact": firstNonZeroFloat(asFloat(r["cash_at_risk"]), asFloat(r["amount"])), "source_commands": sourceCommandsFor(r), "validation_command": validationCommandFor(r)})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func restockRows(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, s := range d.SKUs {
		decision := "keep_monitoring"
		reason := "no urgent inventory or margin signal"
		if s.FBAAvailable == 0 || s.Stranded {
			decision, reason = "pause_ads_until_restock", "out of stock or stranded while campaigns may still spend"
		} else if s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) && s.AdSpend > 0 {
			decision, reason = "conserve_inventory", fmt.Sprintf("%.1f days cover and ads are active", s.DaysOfCover)
		} else if s.AgingDays > 180 && s.UnitsSold < 350 {
			decision, reason = "liquidate", "aging inventory with weak velocity"
		} else if s.ListingScore < 70 || s.Suppressed {
			decision, reason = "fix_listing_first", "listing health blocks efficient growth"
		} else if s.ContributionMargin > .18 && s.DaysOfCover < 45 {
			decision, reason = "restock", "profitable SKU with finite cover"
		}
		cash := cashAtRisk(s)
		score := cash + maxf(0, float64(s.UnitsSold))*s.ContributionMargin
		rows = append(rows, map[string]any{"sku": s.SKU, "asin": s.ASIN, "decision": decision, "reason": reason, "estimated_cash_at_risk": cash, "score": score, "follow_up_commands": []string{"amazon-seller-pp-cli inventory-intel restock --marketplace-id " + firstNonEmpty(s.MarketplaceID, "ATVPDKIKX0DER"), "amazon-operator-intel-pp-cli --agent ad-spend-guardrail --sku " + s.SKU}, "source": s.Source})
	}
	return rows
}

func adWasteRows(d store.DataSet, limit int) []map[string]any {
	bySKU := skusByID(d.SKUs)
	rows := []map[string]any{}
	for _, c := range d.Campaigns {
		s := bySKU[c.SKU]
		breakEven := firstNonZeroFloat(s.BreakEvenACOS, .30)
		issue := ""
		rec := ""
		if s.FBAAvailable == 0 || s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) {
			issue, rec = "spending_on_low_or_out_of_stock", "pause or cap campaign until reorder is confirmed"
		} else if c.ACOS > breakEven {
			issue, rec = "above_break_even_acos", "reduce bids/budgets and inspect search terms"
		} else if s.ContributionMargin < .08 && c.Spend > 0 {
			issue, rec = "negative_or_low_contribution_margin", "pause expansion until SKU economics are fixed"
		} else if c.Spend > 250 && c.Sales == 0 {
			issue, rec = "high_spend_no_sales", "add negatives and stop broad waste"
		} else if s.ListingScore < 70 || s.Suppressed {
			issue, rec = "ads_to_weak_listing", "fix detail page before scaling spend"
		}
		if issue == "" {
			continue
		}
		risk := maxf(c.Spend*maxf(0, c.ACOS-breakEven), c.Spend*.25)
		rows = append(rows, map[string]any{"campaign_id": c.CampaignID, "campaign": c.Name, "sku": c.SKU, "asin": c.ASIN, "issue": issue, "cash_at_risk": risk, "recommendation": rec, "dry_run": true, "score": risk, "source": c.Source})
	}
	for _, s := range d.SKUs {
		if s.AdSpend > 0 && s.ASIN != "" && bySKU[s.SKU].Revenue == 0 {
			rows = append(rows, map[string]any{"sku": s.SKU, "asin": s.ASIN, "issue": "orphan_ads_without_seller_inventory", "cash_at_risk": s.AdSpend, "recommendation": "map ASIN/SKU before scaling or pause until inventory evidence exists", "dry_run": true, "score": s.AdSpend, "source": s.Source})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
	return limitRows(rows, limit)
}

func listingRows(d store.DataSet) []map[string]any {
	bySKU := skusByID(d.SKUs)
	rows := []map[string]any{}
	for _, l := range d.Listings {
		s := bySKU[l.SKU]
		if !l.Suppressed && len(l.Defects) == 0 && l.Score >= 75 && l.ConversionRate >= .12 {
			continue
		}
		impact := (s.AdSpend + s.Revenue*.08 + float64(l.Sessions)*1.4) * maxf(.1, (100-l.Score)/100)
		issue := "weak listing"
		if l.Suppressed {
			issue = "suppressed listing"
		} else if l.ConversionRate < .10 && l.Sessions > 1000 {
			issue = "low conversion with traffic"
		} else if len(l.Defects) > 0 {
			issue = strings.Join(l.Defects, "; ")
		}
		rows = append(rows, map[string]any{"sku": l.SKU, "asin": l.ASIN, "issue": issue, "score": impact, "ad_spend": l.AdSpend, "recommended_action": "repair listing content before scaling traffic", "source": l.Source})
	}
	return rows
}

func cashLeakRows(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, r := range adWasteRows(d, 0) {
		rows = append(rows, map[string]any{"type": "wasted_ad_spend", "sku": r["sku"], "amount": r["cash_at_risk"], "action": r["recommendation"], "score": r["score"]})
	}
	for _, s := range d.SKUs {
		if s.AgingDays > 180 || s.StorageFees > 0 {
			rows = append(rows, map[string]any{"type": "storage_or_aging_inventory", "sku": s.SKU, "amount": maxf(s.StorageFees, float64(s.FBAAvailable)*.35), "action": "liquidate, coupon, or stop reordering until cover normalizes", "score": float64(s.AgingDays)})
		}
		if s.Stranded {
			rows = append(rows, map[string]any{"type": "stranded_inventory", "sku": s.SKU, "amount": s.Revenue * .15, "action": "resolve stranded listing/inventory status"})
		}
		if s.ReimbursementDue > 0 {
			rows = append(rows, map[string]any{"type": "reimbursement_opportunity", "sku": s.SKU, "amount": s.ReimbursementDue, "action": "file or verify reimbursement claim"})
		}
		if s.ReturnRate > .08 {
			rows = append(rows, map[string]any{"type": "returns_spike", "sku": s.SKU, "amount": s.Revenue * s.ReturnRate, "action": "inspect returns comments and listing promise"})
		}
		if s.ContributionMargin < .08 && s.AdSpend > 0 {
			rows = append(rows, map[string]any{"type": "low_margin_sku_with_ads", "sku": s.SKU, "amount": s.AdSpend, "action": "pause expansion until fees/COGS/pricing improve"})
		}
	}
	if d.Account.SettlementGap > 0 {
		rows = append(rows, map[string]any{"type": "settlement_discrepancy", "sku": "", "amount": d.Account.SettlementGap, "action": "run settlement reconciliation"})
	}
	return rows
}

func searchTermRows(d store.DataSet) []map[string]any {
	bySKU := skusByID(d.SKUs)
	rows := []map[string]any{}
	for _, t := range d.SearchTerms {
		s := bySKU[t.SKU]
		action := t.AdAction
		reason := "follow child recommendation"
		if t.Orders > 3 && t.Sales > t.Spend*3 && t.OrganicRank > 5 {
			action, reason = "promote_converting_term", "converts profitably and organic rank can improve"
		} else if t.Spend > 150 && t.Orders == 0 {
			action, reason = "add_negative_for_waste", "spend without sales"
		} else if t.OrganicRank > 0 && t.OrganicRank <= 3 && t.Spend > 100 {
			action, reason = "reduce_paid_spend", "organic rank is strong enough to test lower spend"
		} else if s.FBAAvailable == 0 || s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) {
			action, reason = "do_not_defend_low_inventory", "rank defense is not worth stockout cash burn"
		} else if t.OrganicRank > 4 && t.Orders > 0 {
			action, reason = "rank_defense_candidate", "paid demand may protect organic discovery"
		}
		rows = append(rows, map[string]any{"term": t.Term, "sku": t.SKU, "asin": t.ASIN, "spend": t.Spend, "sales": t.Sales, "organic_rank": t.OrganicRank, "action": action, "reason": reason, "score": termScore(t), "source": t.Source})
	}
	return rows
}

func digestChanged(d store.DataSet, period string) []string {
	if len(d.SKUs) == 0 {
		return []string{"no local dataset loaded yet"}
	}
	return []string{fmt.Sprintf("%d SKUs, %d campaigns, %d search terms in local store", len(d.SKUs), len(d.Campaigns), len(d.SearchTerms)), fmt.Sprintf("%.2f revenue and %.2f estimated profit in current window", totalRevenue(d.SKUs), totalProfit(d.SKUs)), fmt.Sprintf("%s digest generated from %s", period, d.Source)}
}

func monitorRows(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, s := range d.SKUs {
		if !inventoryRisk(s) && s.ContributionMargin > .12 && s.ListingScore >= 75 {
			rows = append(rows, map[string]any{"sku": s.SKU, "reason": "healthy margin, inventory, and listing signals"})
		}
	}
	return limitRows(rows, 5)
}

func planRows(d store.DataSet, owners []string, maxActions int) []map[string]any {
	base := topActions(d, maxActions)
	rows := []map[string]any{}
	for i, a := range base {
		owner := owners[i%len(owners)]
		when := "this_week"
		if i < 3 {
			when = "today"
		} else if owner != "founder" {
			when = "delegate"
		}
		cash := asFloat(a["cash_impact"])
		rows = append(rows, map[string]any{"when": when, "priority": i + 1, "owner": owner, "action": a["action"], "why": "ranked from war-room, restock, ad guardrail, listing triage, cash leak, and search-term outputs", "cash_impact": cash, "estimated_hours": minf(2.5, .4+cash/2000), "risk_if_ignored": "cash leakage, stockout, ranking loss, or margin decay", "source_commands": a["source_commands"], "validation_command": a["validation_command"]})
	}
	rows = append(rows, map[string]any{"when": "blocked_missing_evidence", "priority": len(rows) + 1, "owner": "ops", "action": "Verify any ads-only ASINs without matching seller inventory rows", "why": "orphan ads evidence can hide spend on unmapped products", "cash_impact": orphanAdsSpend(d), "estimated_hours": .5, "risk_if_ignored": "continued spend without inventory or margin proof", "source_commands": []string{"amazon-operator-intel-pp-cli --agent ad-spend-guardrail"}, "validation_command": "amazon-operator-intel-pp-cli --agent sku-profit-truth"})
	return limitRows(rows, maxActions)
}

func groupActions(rows []map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{"today": {}, "this_week": {}, "monitor": {}, "delegate": {}, "blocked_missing_evidence": {}}
	for _, r := range rows {
		w := firstMapString(r, "when")
		out[w] = append(out[w], r)
	}
	return out
}

func cashCalendar(d store.DataSet, horizon int, cash, budgetCap float64) map[string]any {
	if cash == 0 {
		cash = 15000
	}
	weeklyRevenue := totalRevenue(d.SKUs) / 4
	weeklyAdSpend := 0.0
	for _, c := range d.Campaigns {
		weeklyAdSpend += c.Spend / 4
	}
	if budgetCap > 0 && weeklyAdSpend*4 > budgetCap {
		weeklyAdSpend = budgetCap / 4
	}
	weeks := []map[string]any{}
	crunch := []string{}
	running := cash
	for w := 1; w <= maxInt(1, horizon/7); w++ {
		reorder := poCashForWeek(d.PurchaseOrders, w)
		storage := agingFees(d.SKUs) / 4
		reimb := reimbursementInflows(d.SKUs) / 4
		net := weeklyRevenue - weeklyAdSpend - reorder - storage + reimb
		running += net
		weekDate := time.Now().AddDate(0, 0, w*7).Format("2006-01-02")
		if running < 5000 || net < -3000 {
			crunch = append(crunch, weekDate)
		}
		weeks = append(weeks, map[string]any{"week": w, "week_start": weekDate, "expected_revenue": weeklyRevenue, "expected_ad_spend": weeklyAdSpend, "required_reorder_cash": reorder, "likely_storage_aging_charges": storage, "reimbursement_inflows": reimb, "net_cash_pressure": net, "ending_cash": running})
	}
	return map[string]any{"horizon_days": horizon, "cash_on_hand": cash, "weeks": weeks, "cash_crunch_dates": crunch, "recommended_tradeoffs": []string{"cap campaigns above break-even ACOS before cutting profitable rank-defense terms", "sequence reorder cash toward SKUs with positive contribution margin and low cover", "collect reimbursements before funding low-margin launches"}}
}

func launchDecision(p store.LaunchPlan, existing store.SKU) map[string]any {
	decision := "ready"
	risks := []string{}
	if p.InventoryUnits < 200 {
		decision = "not_ready_inventory"
		risks = append(risks, "inventory below two-week launch cushion")
	}
	if p.TargetACOS == 0 || p.TargetACOS > .35 || p.COGS <= 0 {
		decision = "not_ready_margin"
		risks = append(risks, "target ACOS/COGS do not prove margin")
	}
	if p.ListingScore < 75 {
		if decision == "ready" {
			decision = "ready_with_risks"
		}
		risks = append(risks, "listing readiness is below launch threshold")
	}
	if len(p.Keywords) < 3 {
		if decision == "ready" {
			decision = "ready_with_risks"
		}
		risks = append(risks, "keyword/search-term plan is thin")
	}
	if existing.ReturnRate > .08 {
		if decision == "ready" {
			decision = "ready_with_risks"
		}
		risks = append(risks, "similar/live SKU has return risk")
	}
	return map[string]any{"sku": p.SKU, "asin": p.ASIN, "decision": decision, "inventory_readiness": p.InventoryUnits >= 200, "listing_readiness": p.ListingScore, "ad_readiness": p.LaunchBudget >= 300 && p.TargetACOS > 0, "margin_readiness": p.COGS > 0 && p.TargetACOS <= .35, "keyword_search_term_readiness": len(p.Keywords), "review_return_risk": existing.ReturnRate, "risks": risks, "checklist_14_day": []string{"finalize hero image and bullets", "load exact/phrase launch campaigns", "set spend caps by target ACOS", "monitor search terms daily", "verify inventory cover before scaling"}}
}

func rankDefenseRows(d store.DataSet) map[string]any {
	defend, reduce, none := []map[string]any{}, []map[string]any{}, []map[string]any{}
	bySKU := skusByID(d.SKUs)
	for _, t := range d.SearchTerms {
		s := bySKU[t.SKU]
		row := map[string]any{"term": t.Term, "sku": t.SKU, "organic_rank": t.OrganicRank, "spend": t.Spend, "sales": t.Sales, "inventory_days": s.DaysOfCover, "margin": s.ContributionMargin}
		if s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) {
			row["reason"] = "low inventory means rank defense is not worth cash burn"
			none = append(none, row)
		} else if t.OrganicRank > 3 && t.Orders > 0 && s.ContributionMargin > .10 {
			row["reason"] = "paid spend may be protecting organic rank or launch demand"
			defend = append(defend, row)
		} else if t.OrganicRank > 0 && t.OrganicRank <= 3 || t.Orders == 0 {
			row["reason"] = "organic rank or poor conversion supports reducing paid spend"
			reduce = append(reduce, row)
		}
	}
	return map[string]any{"defend": defend, "reduce": reduce, "do_not_defend": none}
}

func bundleRows(d store.DataSet) []map[string]any {
	bySKU := skusByID(d.SKUs)
	rows := []map[string]any{}
	for _, b := range d.BundleSignals {
		p := bySKU[b.PrimarySKU]
		s := bySKU[b.SecondarySKU]
		decision := "test_bundle"
		reason := "basket confidence, margin, and inventory are viable"
		if !b.InventoryFeasible || p.FBAAvailable == 0 || s.FBAAvailable == 0 {
			decision, reason = "reject", "one component is out of stock or inventory-feasible=false"
		} else if b.CombinedMargin < .20 || p.ContributionMargin < .08 || s.ContributionMargin < .08 {
			decision, reason = "reject", "combined or component margin is too low"
		} else if p.ReturnRate > .08 || s.ReturnRate > .08 {
			decision, reason = "reject", "return-rate risk would compound in bundle"
		}
		rows = append(rows, map[string]any{"primary_asin": b.PrimaryASIN, "secondary_asin": b.SecondaryASIN, "primary_sku": b.PrimarySKU, "secondary_sku": b.SecondarySKU, "basket_confidence": b.Confidence, "combined_margin": b.CombinedMargin, "inventory_feasible": b.InventoryFeasible, "suggested_offer": b.SuggestedOffer, "listing_creative_recommendation": "test bundle image and Sponsored Brands creative", "test_channel": "Amazon bundle, coupon, Sponsored Brands creative, or storefront module", "decision": decision, "reason": reason, "source": b.Source})
	}
	return rows
}

func deductionRows(ds []store.VendorDeduction) []map[string]any {
	rows := []map[string]any{}
	for _, d := range ds {
		score := d.Amount * d.Confidence
		rows = append(rows, map[string]any{"id": d.ID, "type": d.Type, "sku": d.SKU, "amount": d.Amount, "reason": d.Reason, "dispute_by": d.DisputeBy, "confidence": d.Confidence, "score": score, "recommendation": "gather proof and dispute before deadline", "source": d.Source})
	}
	sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["score"]) > asFloat(rows[j]["score"]) })
	return rows
}

func poRiskRows(pos []store.PurchaseOrder) []map[string]any {
	rows := []map[string]any{}
	for _, p := range pos {
		risk := ""
		if strings.Contains(strings.ToLower(p.Status), "risk") {
			risk = p.Status
		} else if p.ExpectedShipDate != "" {
			if t, err := time.Parse("2006-01-02", p.ExpectedShipDate); err == nil && time.Until(t) < 72*time.Hour {
				risk = "ship_window_close"
			}
		}
		if risk == "" {
			continue
		}
		rows = append(rows, map[string]any{"po_id": p.POID, "sku": p.SKU, "units": p.Units, "expected_ship_date": p.ExpectedShipDate, "risk": risk, "recommendation": "confirm ship window, fill rate, and receiving date", "source": p.Source})
	}
	return rows
}

func skusByID(skus []store.SKU) map[string]store.SKU {
	m := map[string]store.SKU{}
	for _, s := range skus {
		m[s.SKU] = s
		if s.ASIN != "" {
			m[s.ASIN] = s
		}
	}
	return m
}

func findSKU(d store.DataSet, skuID, asin string) store.SKU {
	for _, s := range d.SKUs {
		if (skuID != "" && s.SKU == skuID) || (asin != "" && s.ASIN == asin) {
			return s
		}
	}
	return store.SKU{}
}

func findLaunch(d store.DataSet, skuID, asin string) store.LaunchPlan {
	for _, p := range d.LaunchPlans {
		if (skuID != "" && p.SKU == skuID) || (asin != "" && p.ASIN == asin) {
			return p
		}
	}
	return store.LaunchPlan{}
}

func cashAtRisk(s store.SKU) float64 {
	adAbove := s.AdSpend * maxf(0, s.ACOS-firstNonZeroFloat(s.BreakEvenACOS, .30))
	stockout := 0.0
	if s.DaysOfCover > 0 && s.DaysOfCover < float64(s.LeadTimeDays) {
		stockout = maxf(0, s.Profit) * .35
	}
	aging := 0.0
	if s.AgingDays > 180 {
		aging = float64(s.FBAAvailable) * .45
	}
	return adAbove + stockout + aging + s.ReimbursementDue
}

func termScore(t store.SearchTerm) float64 {
	return t.Spend + t.Sales*.1 + float64(t.Orders*50) + float64(maxInt(0, 20-t.OrganicRank))*10
}
func orphanAdsSpend(d store.DataSet) float64 {
	total := 0.0
	by := skusByID(d.SKUs)
	for _, c := range d.Campaigns {
		if _, ok := by[c.SKU]; !ok {
			total += c.Spend
		}
	}
	return total
}
func poCashForWeek(pos []store.PurchaseOrder, week int) float64 {
	total := 0.0
	for _, p := range pos {
		if week <= 2 {
			total += float64(p.Units) * p.UnitCost
		}
	}
	return total
}
func agingFees(skus []store.SKU) float64 {
	total := 0.0
	for _, s := range skus {
		if s.AgingDays > 180 {
			total += maxf(s.StorageFees, float64(s.FBAAvailable)*.35)
		}
	}
	return total
}
func reimbursementInflows(skus []store.SKU) float64 {
	total := 0.0
	for _, s := range skus {
		total += s.ReimbursementDue
	}
	return total
}
func totalDeductions(ds []store.VendorDeduction) float64 {
	total := 0.0
	for _, d := range ds {
		total += d.Amount
	}
	return total
}

func sourceCommandsFor(r map[string]any) []string {
	sku := firstMapString(r, "sku")
	if sku != "" {
		return []string{"amazon-operator-intel-pp-cli --agent sku-profit-truth", "amazon-operator-intel-pp-cli --agent ad-spend-guardrail --sku " + sku}
	}
	return []string{"amazon-operator-intel-pp-cli --agent war-room"}
}
func validationCommandFor(r map[string]any) string {
	if sku := firstMapString(r, "sku"); sku != "" {
		return "amazon-operator-intel-pp-cli --agent restock-or-kill --limit 5"
	}
	return "amazon-operator-intel-pp-cli --agent war-room"
}

func loadLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	return lines, nil
}

func loadPurchaseOrders(path string) ([]store.PurchaseOrder, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []store.PurchaseOrder
	if json.Unmarshal(b, &rows) == nil && len(rows) > 0 {
		return rows, nil
	}
	cr := csv.NewReader(strings.NewReader(string(b)))
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	out := []store.PurchaseOrder{}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		m := csvMap(header, rec)
		out = append(out, store.PurchaseOrder{POID: m["po_id"], SKU: m["sku"], ASIN: m["asin"], Units: atoi(m["units"]), UnitCost: atof(m["unit_cost"]), ExpectedShipDate: m["expected_ship_date"], ExpectedReceiveDate: m["expected_receive_date"], Status: m["status"], Source: store.MetricSources{VendorFiles: store.SourceEvidence{Present: true, Source: "local-import", ImportedFrom: path, SyncedAt: time.Now().UTC()}}})
	}
	return out, nil
}

func applyCOGSFile(d *store.DataSet, path string) error {
	cogs, err := loadCOGS(path)
	if err != nil {
		return err
	}
	for i := range d.SKUs {
		if v, ok := cogs[d.SKUs[i].SKU]; ok {
			d.SKUs[i].COGS = v * float64(maxInt(1, d.SKUs[i].UnitsSold))
		}
	}
	return nil
}

func loadCOGS(path string) (map[string]float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	var direct map[string]float64
	if json.Unmarshal(b, &direct) == nil && len(direct) > 0 {
		return direct, nil
	}
	var rows []map[string]any
	if json.Unmarshal(b, &rows) == nil && len(rows) > 0 {
		for _, r := range rows {
			m := flattenMap(r, "")
			if sku := firstString(m, "sku", "seller_sku"); sku != "" {
				out[sku] = firstFloat(m, "cogs", "unit_cost", "cost")
			}
		}
		return out, nil
	}
	cr := csv.NewReader(strings.NewReader(string(b)))
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		m := csvMap(header, rec)
		sku := firstNonEmpty(m["sku"], m["seller_sku"])
		if sku != "" {
			out[sku] = atof(firstNonEmpty(m["cogs"], m["unit_cost"], m["cost"]))
		}
	}
	return out, nil
}

func loadVendorDeductions(path string) ([]store.VendorDeduction, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []store.VendorDeduction
	if json.Unmarshal(b, &rows) == nil && len(rows) > 0 {
		return rows, nil
	}
	cr := csv.NewReader(strings.NewReader(string(b)))
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	out := []store.VendorDeduction{}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		m := csvMap(header, rec)
		out = append(out, store.VendorDeduction{ID: m["id"], Type: m["type"], SKU: m["sku"], ASIN: m["asin"], Amount: atof(m["amount"]), Reason: m["reason"], DisputeBy: m["dispute_by"], Confidence: atof(m["confidence"]), Source: store.MetricSources{VendorFiles: store.SourceEvidence{Present: true, Source: "local-import", ImportedFrom: path, SyncedAt: time.Now().UTC()}}})
	}
	return out, nil
}

func csvMap(header, rec []string) map[string]string {
	m := map[string]string{}
	for i, h := range header {
		if i < len(rec) {
			m[normKey(h)] = strings.TrimSpace(rec[i])
		}
	}
	return m
}

func table(rows []map[string]any, cols []string) string {
	lines := []string{strings.Join(cols, "\t")}
	for _, r := range rows {
		parts := make([]string, 0, len(cols))
		for _, c := range cols {
			parts = append(parts, fmt.Sprint(r[c]))
		}
		lines = append(lines, strings.Join(parts, "\t"))
	}
	return strings.Join(lines, "\n") + "\n"
}

func limitRows(rows []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(rows) > limit {
		return rows[:limit]
	}
	return rows
}
func filterRows(rows []map[string]any, key, value string) []map[string]any {
	out := []map[string]any{}
	for _, r := range rows {
		if strings.EqualFold(fmt.Sprint(r[key]), value) {
			out = append(out, r)
		}
	}
	return out
}
func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"founder"}
	}
	return out
}
func firstMapString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}
func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		return atof(x)
	default:
		return 0
	}
}
func normKey(s string) string {
	return strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(strings.ToLower(strings.TrimSpace(s)))
}
func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case json.Number:
				return x.String()
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			}
		}
	}
	return ""
}
func firstInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			return int(asFloat(v))
		}
	}
	return 0
}
func firstFloat(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			return asFloat(v)
		}
	}
	return 0
}
func firstBool(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			switch x := v.(type) {
			case bool:
				return x
			case string:
				return strings.EqualFold(x, "true") || x == "1"
			}
		}
	}
	return false
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func firstNonZeroFloat(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
func uniqueStrings(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(strings.TrimSuffix(s, "%")), ",", ""), 64)
	return f
}
func atoi(s string) int { i, _ := strconv.Atoi(strings.TrimSpace(s)); return i }
func div(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
