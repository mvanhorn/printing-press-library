package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
	"github.com/spf13/cobra"
)

var version = "0.1.0-private"

type rootFlags struct {
	asJSON, compact, noInput, yes, noColor, agent bool
	profile, home                                 string
}

func Execute() error             { return NewRootCmd().Execute() }
func NewRootCmd() *cobra.Command { f := &rootFlags{}; return newRootCmd(f) }
func newRootCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "ecommerce-intel-pp-cli", Short: "Private Printing Press ecommerce intelligence CLI", Long: "Local-first Shopify/commerce intelligence CLI joining Shopify, Klaviyo, GA4, GSC, Ahrefs, fixtures, and imports for revenue, SEO, inventory, email, GEO, and answer-engine workflows.", SilenceUsage: true, Version: version}
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
	cmd.AddCommand(agentContextCmd(f), doctorCmd(f), sourcesCmd(f), profileCmd(f), syncCmd(f), dashboardCmd(f), opportunitiesCmd(f), actionPlanCmd(f), moneyPagesCmd(f), moneyProductsCmd(f), queryRevenueCmd(f), explainDropCmd(f), productActionsCmd(f), categoryActionsCmd(f), emailActionsCmd(f), inventoryRiskCmd(f), sourceCoverageCmd(f), merchandisingLinkPlanCmd(f), experimentPlanCmd(f), forecastImpactCmd(f), restockWinnersCmd(f), cannibalizationCmd(f), categoryClustersCmd(f), digestCmd(f), geoAuditCmd(f))
	return cmd
}
func st(f *rootFlags) *store.Store { return store.New(f.home) }
func out(cmd *cobra.Command, f *rootFlags, v any, human string) error {
	if f.asJSON || f.agent {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	if human == "" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	_, err := io.WriteString(cmd.OutOrStdout(), human)
	return err
}

func agentContextCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Print machine-readable CLI context", RunE: func(cmd *cobra.Command, args []string) error {
		ctx := map[string]any{"schema_version": "ecommerce-intel.agent-context/v1", "name": "ecommerce-intel-pp-cli", "version": version, "private": true, "local_first": true, "external_api_calls": false, "agent_flag": "sets --json --compact --no-input --yes --no-color", "state_dir": f.home, "geo_answer_engine_support": []string{"llms.txt/agents.md discovery", "structured data", "product facts", "reviews/FAQ answer blocks", "collection buying guides", "merchant feed parity", "ChatGPT readiness", "Perplexity readiness", "Google AI Overviews readiness"}, "geo_check_ids": []string{"storefront_llms_txt", "storefront_structured_data", "storefront_product_facts", "pdp_product_schema", "pdp_product_facts", "pdp_faq_answers", "category_guides", "category_faqs", "answerability_score"}, "env": envMeta(), "commands": agentCommands(), "source_plan": sourceChecks()}
		return out(cmd, f, ctx, "")
	}}
}

func doctorCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local readiness without network calls", RunE: func(cmd *cobra.Command, args []string) error {
		ps, _ := st(f).ListProfiles()
		checks := sourceChecks()
		result := map[string]any{"ok": true, "version": version, "state_dir": f.home, "profiles": len(ps), "sources": checks, "env": envChecks()}
		lines := []string{fmt.Sprintf("ecommerce-intel-pp-cli %s", version), "state: " + f.home, fmt.Sprintf("profiles: %d", len(ps)), "sources:"}
		for _, c := range checks {
			path := "missing"
			if c.Path != "" {
				path = c.Path
			}
			lines = append(lines, fmt.Sprintf("- %s binary=%s env=%s planned=%s", c.Name, path, presentList(c.Env), c.PlannedCommand))
		}
		return out(cmd, f, result, strings.Join(lines, "\n")+"\n")
	}}
}
func sourcesCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sources", Short: "Inspect local source adapters"}
	cmd.AddCommand(&cobra.Command{Use: "doctor", Short: "Check Shopify, Klaviyo, GA4, GSC, and Ahrefs adapter readiness", RunE: func(cmd *cobra.Command, args []string) error {
		checks := sourceChecks()
		lines := []string{"source\tbinary\tenv_present\tplanned_command"}
		for _, c := range checks {
			bin := "missing"
			if c.Path != "" {
				bin = c.Path
			}
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s", c.Name, bin, presentList(c.Env), c.PlannedCommand))
		}
		return out(cmd, f, checks, strings.Join(lines, "\n")+"\n")
	}})
	return cmd
}

type sourceCheck struct {
	Name           string     `json:"name"`
	ChildBinary    string     `json:"child_binary"`
	Path           string     `json:"path,omitempty"`
	Found          bool       `json:"found"`
	Env            []envCheck `json:"env"`
	PlannedCommand string     `json:"planned_command"`
	Note           string     `json:"note"`
}
type envCheck struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
}

func sourceChecks() []sourceCheck {
	defs := []sourceCheck{{Name: "shopify", ChildBinary: "shopify-pp-cli", Env: []envCheck{{Name: "SHOPIFY_SHOP"}, {Name: "SHOPIFY_ACCESS_TOKEN"}}, PlannedCommand: "shopify-pp-cli products list --agent --all --data-source live", Note: "optional child CLI; used by sync --source shopify/all"}, {Name: "klaviyo", ChildBinary: "klaviyo-pp-cli", Env: []envCheck{{Name: "KLAVIYO_API_KEY"}, {Name: "KLAVIYO_ACCOUNT"}}, PlannedCommand: "klaviyo-pp-cli report flow-comparison --agent --data-source live", Note: "optional email source"}, {Name: "ga4", ChildBinary: "google-analytics-pp-cli", Env: []envCheck{{Name: "GA4_PROPERTY_ID"}}, PlannedCommand: "google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>", Note: "optional analytics source"}, {Name: "gsc", ChildBinary: "google-search-console-pp-cli", Env: []envCheck{{Name: "GSC_SITE_URL"}}, PlannedCommand: "google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions [\"page\"] --start-date <start> --end-date <end>", Note: "optional search source"}, {Name: "ahrefs", ChildBinary: "ahrefs-pp-cli", Env: []envCheck{{Name: "AHREFS_TARGET"}, {Name: "AHREFS_PROJECT"}}, PlannedCommand: "ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date>", Note: "optional link/keyword source"}}
	for i := range defs {
		if path, err := exec.LookPath(defs[i].ChildBinary); err == nil {
			defs[i].Found = true
			defs[i].Path = path
		}
		for j := range defs[i].Env {
			defs[i].Env[j].Present = envPresent(defs[i].Env[j].Name)
		}
	}
	return defs
}
func envMeta() []map[string]any {
	out := []map[string]any{}
	for _, e := range envChecks() {
		out = append(out, map[string]any{"name": e.Name, "present": e.Present, "required": false})
	}
	return out
}
func envChecks() []envCheck {
	names := []string{"ECOMMERCE_INTEL_HOME", "SHOPIFY_SHOP", "SHOPIFY_ACCESS_TOKEN", "KLAVIYO_API_KEY", "KLAVIYO_ACCOUNT", "GA4_PROPERTY_ID", "GSC_SITE_URL", "AHREFS_TARGET", "AHREFS_PROJECT"}
	out := make([]envCheck, 0, len(names))
	for _, n := range names {
		out = append(out, envCheck{Name: n, Present: envPresent(n)})
	}
	return out
}
func envPresent(name string) bool { return os.Getenv(name) != "" }
func presentList(env []envCheck) string {
	parts := []string{}
	for _, e := range env {
		status := "absent"
		if e.Present {
			status = "present"
		}
		parts = append(parts, e.Name+":"+status)
	}
	return strings.Join(parts, ",")
}

func profileCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage local profiles"}
	var name, shop, site, ga, gsc, ahrefs, klaviyo string
	save := &cobra.Command{Use: "save", Short: "Save a profile", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			name = f.profile
		}
		p := store.Profile{Name: name, ShopifyShop: shop, SiteURL: site, GAProperty: ga, GSCProperty: gsc, AhrefsTarget: ahrefs, KlaviyoAccount: klaviyo}
		if err := st(f).SaveProfile(p); err != nil {
			return err
		}
		return out(cmd, f, p, fmt.Sprintf("saved profile %s\n", name))
	}}
	save.Flags().StringVar(&name, "name", "", "Profile name")
	save.Flags().StringVar(&shop, "shopify-shop", "", "Shopify shop/domain")
	save.Flags().StringVar(&site, "site", "", "Site URL")
	save.Flags().StringVar(&ga, "ga-property", "", "GA4 property id")
	save.Flags().StringVar(&gsc, "gsc-property", "", "GSC property/site")
	save.Flags().StringVar(&ahrefs, "ahrefs-target", "", "Ahrefs target")
	save.Flags().StringVar(&klaviyo, "klaviyo-account", "", "Klaviyo account")
	cmd.AddCommand(save)
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List profiles", RunE: func(cmd *cobra.Command, args []string) error {
		ps, err := st(f).ListProfiles()
		if err != nil {
			return err
		}
		lines := []string{}
		for _, p := range ps {
			lines = append(lines, p.Name+"\t"+first(p.ShopifyShop, p.SiteURL))
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
		return out(cmd, f, p, fmt.Sprintf("%s\nshopify_shop: %s\nsite: %s\nga_property: %s\ngsc_property: %s\nahrefs_target: %s\nklaviyo_account: %s\n", p.Name, p.ShopifyShop, p.SiteURL, p.GAProperty, p.GSCProperty, p.AhrefsTarget, p.KlaviyoAccount))
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
func first(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func syncCmd(f *rootFlags) *cobra.Command {
	var importPath, source string
	var live, real bool
	var shop, klaviyoAccount, site, gaProperty, ahrefsTarget, startDate, endDate, ahrefsDate string
	var limit int
	c := &cobra.Command{Use: "sync", Short: "Import local fixture, JSON data, or opt-in child CLI data", RunE: func(cmd *cobra.Command, args []string) error {
		var d store.DataSet
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" && (live || real) {
			source = "all"
		}
		if importPath != "" && source != "" {
			return fmt.Errorf("--import cannot be combined with --source/--live/--real")
		}
		if importPath == "" && source == "" {
			d = store.Fixture(f.profile)
		} else if importPath != "" {
			b, err := os.ReadFile(importPath)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return err
			}
			if d.Profile == "" {
				d.Profile = f.profile
			}
			if d.Source == "" {
				d.Source = importPath
			}
		} else {
			if !validSource(source) {
				return fmt.Errorf("invalid --source %q: use all, shopify, klaviyo, ga4, gsc, or ahrefs", source)
			}
			base, err := st(f).LoadData(f.profile)
			if err != nil {
				base = store.DataSet{Profile: f.profile, Source: "child-cli-empty-base"}
			}
			opts := childSyncOptions{Shop: shop, KlaviyoAccount: klaviyoAccount, Site: site, GAProperty: gaProperty, AhrefsTarget: ahrefsTarget, StartDate: startDate, EndDate: endDate, AhrefsDate: ahrefsDate, Limit: limit}
			if p, err := st(f).GetProfile(f.profile); err == nil {
				if opts.Shop == "" {
					opts.Shop = p.ShopifyShop
				}
				if opts.KlaviyoAccount == "" {
					opts.KlaviyoAccount = p.KlaviyoAccount
				}
				if opts.Site == "" {
					opts.Site = first(p.GSCProperty, p.SiteURL)
				}
				if opts.GAProperty == "" {
					opts.GAProperty = p.GAProperty
				}
				if opts.AhrefsTarget == "" {
					opts.AhrefsTarget = p.AhrefsTarget
				}
			}
			var syncErr error
			d, syncErr = syncFromChildCLIs(f.profile, source, base, opts)
			if syncErr != nil {
				return syncErr
			}
		}
		if err := st(f).SaveData(d); err != nil {
			return err
		}
		return out(cmd, f, map[string]any{"profile": d.Profile, "products": len(d.Products), "pages": len(d.Pages), "categories": len(d.Categories), "emails": len(d.Emails), "source": d.Source, "synced_at": d.SyncedAt}, fmt.Sprintf("synced %d products, %d pages, %d categories for %s from %s\n", len(d.Products), len(d.Pages), len(d.Categories), d.Profile, d.Source))
	}}
	c.Flags().StringVar(&importPath, "import", "", "Import JSON DataSet")
	c.Flags().StringVar(&source, "source", "", "Real child CLI source to sync: all, shopify, klaviyo, ga4, gsc, ahrefs")
	c.Flags().BoolVar(&live, "live", false, "Use child CLIs instead of the embedded fixture (same as --source all)")
	c.Flags().BoolVar(&real, "real", false, "Use child CLIs instead of the embedded fixture (alias for --live)")
	c.Flags().StringVar(&shop, "shop", "", "Shopify shop/domain (or SHOPIFY_SHOP/profile Shopify shop)")
	c.Flags().StringVar(&klaviyoAccount, "klaviyo-account", "", "Klaviyo account/profile hint")
	c.Flags().StringVar(&site, "site", "", "GSC site URL/property (or GSC_SITE_URL/profile site)")
	c.Flags().StringVar(&gaProperty, "ga-property", "", "GA4 property id (or GA4_PROPERTY_ID/profile property)")
	c.Flags().StringVar(&ahrefsTarget, "ahrefs-target", "", "Ahrefs target/domain (or AHREFS_TARGET/AHREFS_PROJECT/profile target)")
	c.Flags().StringVar(&startDate, "start-date", "", "Start date for GSC/GA4 child sync (default: 7 completed days ago)")
	c.Flags().StringVar(&endDate, "end-date", "", "End date for GSC/GA4/Ahrefs child sync (default: yesterday)")
	c.Flags().StringVar(&ahrefsDate, "date", "", "Ahrefs snapshot date (default: --end-date/yesterday)")
	c.Flags().IntVar(&limit, "limit", 1000, "Maximum rows per child source")
	return c
}
func validSource(source string) bool {
	switch source {
	case "all", "shopify", "klaviyo", "ga4", "gsc", "ahrefs":
		return true
	default:
		return false
	}
}
func load(f *rootFlags) (store.DataSet, error) {
	d, err := st(f).LoadData(f.profile)
	if err != nil {
		return d, fmt.Errorf("no local data for profile %q: run sync first", f.profile)
	}
	return d, nil
}

func dashboardCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "dashboard", Short: "Commerce KPI overview", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		revenue, prev := totals(d)
		risk := inventoryRisks(d)
		res := map[string]any{"profile": d.Profile, "revenue": revenue, "previous_revenue": prev, "revenue_delta": revenue - prev, "products": len(d.Products), "pages": len(d.Pages), "categories": len(d.Categories), "email_flows": len(d.Emails), "inventory_risks": len(risk), "geo_score": geoScore(d)}
		return out(cmd, f, res, fmt.Sprintf("Revenue %.2f (delta %.2f)\nproducts: %d\ninventory risks: %d\nGEO score: %d\n", revenue, revenue-prev, len(d.Products), len(risk), geoScore(d)))
	}}
}
func opportunitiesCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "opportunities", Short: "Prioritized revenue, SEO, GEO, email, and inventory opportunities", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := opps(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"type\ttarget\timpact\taction"}
		for _, r := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%.1f\t%s", r["type"], r["target"], r["impact"], r["action"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}
func actionPlanCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "action-plan", Short: "Generate a 7-day ecommerce action plan", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := opps(d)
		plan := []map[string]any{}
		for i, r := range rows {
			if i >= 7 {
				break
			}
			r["day"] = i + 1
			plan = append(plan, r)
		}
		return out(cmd, f, plan, fmt.Sprintf("7-day plan with %d actions\n", len(plan)))
	}}
}
func moneyPagesCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "money-pages", Short: "Rank pages by revenue and search/link context", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := append([]store.Page(nil), d.Pages...)
		sort.Slice(ps, func(i, j int) bool { return ps[i].Revenue > ps[j].Revenue })
		if limit > 0 && len(ps) > limit {
			ps = ps[:limit]
		}
		lines := []string{"url\trevenue\tsessions\tclicks"}
		for _, p := range ps {
			lines = append(lines, fmt.Sprintf("%s\t%.2f\t%d\t%d", p.URL, p.Revenue, p.Sessions, p.SearchClicks))
		}
		return out(cmd, f, ps, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}
func moneyProductsCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "money-products", Short: "Rank products by revenue and margin", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := append([]store.Product(nil), d.Products...)
		sort.Slice(ps, func(i, j int) bool { return ps[i].Revenue*ps[i].GrossMargin > ps[j].Revenue*ps[j].GrossMargin })
		if limit > 0 && len(ps) > limit {
			ps = ps[:limit]
		}
		lines := []string{"handle\trevenue\tmargin\tinventory"}
		for _, p := range ps {
			lines = append(lines, fmt.Sprintf("%s\t%.2f\t%.2f\t%d", p.Handle, p.Revenue, p.GrossMargin, p.Inventory))
		}
		return out(cmd, f, ps, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}
func queryRevenueCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "query-revenue [query-or-url]", Args: cobra.MaximumNArgs(1), Short: "Find revenue for matching products/pages/categories", RunE: func(cmd *cobra.Command, args []string) error {
		q := ""
		if len(args) > 0 {
			q = strings.ToLower(args[0])
		}
		d, err := load(f)
		if err != nil {
			return err
		}
		total := 0.0
		rows := []map[string]any{}
		for _, p := range d.Products {
			if match(q, p.Handle, p.Title, p.URL, p.Category) {
				total += p.Revenue
				rows = append(rows, map[string]any{"type": "product", "target": p.Handle, "revenue": p.Revenue})
			}
		}
		for _, p := range d.Pages {
			if match(q, p.URL, p.Title, p.PrimaryCollection) {
				total += p.Revenue
				rows = append(rows, map[string]any{"type": "page", "target": p.URL, "revenue": p.Revenue})
			}
		}
		return out(cmd, f, map[string]any{"query": q, "revenue": total, "rows": rows}, fmt.Sprintf("query: %s\nrevenue: %.2f\nrows: %d\n", q, total, len(rows)))
	}}
}
func explainDropCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "explain-drop [query-or-url]", Args: cobra.MaximumNArgs(1), Short: "Explain revenue/search/session drops", RunE: func(cmd *cobra.Command, args []string) error {
		q := ""
		if len(args) > 0 {
			q = strings.ToLower(args[0])
		}
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := []map[string]any{}
		for _, p := range d.Products {
			if q != "" && !match(q, p.Handle, p.Title, p.URL, p.Category) {
				continue
			}
			rd := p.Revenue - p.PreviousRevenue
			cd := p.SearchClicks - p.PreviousClicks
			sd := p.Sessions - p.PreviousSessions
			if rd < 0 || cd < 0 || sd < 0 {
				cause := "mixed demand/conversion pressure"
				if cd < 0 && p.SearchPosition > 6 {
					cause = "SEO ranking/click loss"
				} else if p.Inventory < 15 {
					cause = "inventory constraint"
				} else if rd < 0 && p.ConversionRate < .02 {
					cause = "conversion-rate decline"
				}
				rows = append(rows, map[string]any{"product": p.Handle, "revenue_delta": rd, "click_delta": cd, "session_delta": sd, "likely_cause": cause, "next_action": "inspect source metrics, fix PDP evidence, and update campaign/stock actions"})
			}
		}
		return out(cmd, f, rows, fmt.Sprintf("found %d dropping products\n", len(rows)))
	}}
}

func productActionsCmd(f *rootFlags) *cobra.Command {
	return simpleRowsCmd(f, "product-actions", "PDP/product action recommendations", func(d store.DataSet) []map[string]any {
		rows := []map[string]any{}
		for _, p := range d.Products {
			act := "optimize PDP"
			if !p.HasStructuredData {
				act = "add Product/Offer structured data"
			} else if !p.HasProductFacts {
				act = "add concise product facts/specs"
			} else if p.Inventory < 15 {
				act = "reorder or throttle paid/email demand"
			} else if p.ConversionRate < .025 {
				act = "test offer, reviews, images, and comparison copy"
			}
			rows = append(rows, map[string]any{"product": p.Handle, "action": act, "impact": p.Revenue})
		}
		sortRows(rows)
		return rows
	})
}
func categoryActionsCmd(f *rootFlags) *cobra.Command {
	return simpleRowsCmd(f, "category-actions", "Collection/category action recommendations", func(d store.DataSet) []map[string]any {
		rows := []map[string]any{}
		for _, c := range d.Categories {
			act := "refresh collection merchandising"
			if !c.HasBuyingGuide {
				act = "publish buying guide and link from collection"
			} else if !c.StructuredData {
				act = "add ItemList/Breadcrumb structured data"
			} else if c.OutOfStock > 0 {
				act = "fix out-of-stock merchandising"
			}
			rows = append(rows, map[string]any{"category": c.Handle, "action": act, "impact": c.Revenue, "gap": c.AnswerabilityGap})
		}
		sortRows(rows)
		return rows
	})
}
func emailActionsCmd(f *rootFlags) *cobra.Command {
	return simpleRowsCmd(f, "email-actions", "Klaviyo/email action recommendations", func(d store.DataSet) []map[string]any {
		rows := []map[string]any{}
		for _, e := range d.Emails {
			act := "maintain and monitor"
			if e.NeedsAction || e.ClickRate < .03 {
				act = "refresh creative, offer, and segmentation"
			}
			if e.ConversionRate < .01 {
				act = "audit checkout friction and product matching"
			}
			rows = append(rows, map[string]any{"flow": e.Name, "action": act, "impact": e.Revenue, "click_rate": e.ClickRate})
		}
		sortRows(rows)
		return rows
	})
}
func inventoryRiskCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "inventory-risk", Short: "Inventory and demand risk", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := inventoryRisks(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"target\taction\timpact"}
		for _, r := range rows {
			target := fmt.Sprint(firstAny(r, "product", "target"))
			lines = append(lines, fmt.Sprintf("%s\t%s\t%v", target, r["action"], r["impact"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}
func digestCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "digest", Short: "Generate digests"}
	cmd.AddCommand(&cobra.Command{Use: "weekly", Short: "Generate weekly ecommerce digest", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rev, prev := totals(d)
		digest := map[string]any{"profile": d.Profile, "synced_at": d.SyncedAt, "revenue": rev, "previous_revenue": prev, "revenue_delta": rev - prev, "top_product": "", "inventory_risks": len(inventoryRisks(d)), "geo_score": geoScore(d), "recommended_next_command": "ecommerce-intel-pp-cli opportunities --profile " + d.Profile}
		if len(d.Products) > 0 {
			ps := append([]store.Product(nil), d.Products...)
			sort.Slice(ps, func(i, j int) bool { return ps[i].Revenue > ps[j].Revenue })
			digest["top_product"] = ps[0].Handle
		}
		return out(cmd, f, digest, fmt.Sprintf("Weekly ecommerce digest for %s\nrevenue: %.2f\ndelta: %.2f\ntop product: %s\nGEO score: %d\n", d.Profile, rev, rev-prev, digest["top_product"], geoScore(d)))
	}})
	return cmd
}
func geoAuditCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "geo-audit", Short: "Assess GEO/answer-engine readiness for ChatGPT, Perplexity, and AI Overviews", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		checks := geoChecks(d)
		gaps := []string{}
		for _, c := range checks {
			if c["status"] != "pass" {
				gaps = append(gaps, fmt.Sprintf("%s: %s", c["id"], c["recommendation"]))
			}
		}
		res := map[string]any{"score": geoScore(d), "llms_txt": d.Storefront.LLMSTxt, "structured_data": d.Storefront.StructuredData, "product_facts": d.Storefront.ProductFacts, "buying_guides": d.Storefront.BuyingGuides, "answerability": d.Storefront.Answerability, "answer_engines": []string{"ChatGPT", "Perplexity", "Google AI Overviews", "Google AI Mode", "Claude"}, "checks": checks, "gaps": gaps, "recommended_content_types": []string{"PDP answer block with who/what/best-for/price/reviews/alternatives", "collection-level buying guide", "comparison guide against common alternatives", "first-party productivity/relationship research", "review-backed FAQ"}}
		return out(cmd, f, res, fmt.Sprintf("GEO score: %d\nchecks: %d\ngaps: %d\n", geoScore(d), len(checks), len(gaps)))
	}}
}
func geoChecks(d store.DataSet) []map[string]any {
	checks := []map[string]any{}
	add := func(id, status, severity, target, rec string) {
		checks = append(checks, map[string]any{"id": id, "status": status, "severity": severity, "target": target, "recommendation": rec})
	}
	passfail := func(ok bool) string {
		if ok {
			return "pass"
		}
		return "fail"
	}
	add("storefront_llms_txt", passfail(d.Storefront.LLMSTxt), "high", d.Storefront.Domain, "publish /llms.txt and /agents.md pointing to canonical product, collection, buying-guide, UCP/agent-commerce, shipping, returns, and support URLs")
	add("storefront_structured_data", passfail(d.Storefront.StructuredData), "high", d.Storefront.Domain, "ensure Organization, WebSite/SearchAction, BreadcrumbList, Product/Offer/AggregateRating, FAQPage, and ItemList schema validate on canonical pages")
	add("storefront_product_facts", passfail(d.Storefront.ProductFacts), "medium", d.Storefront.Domain, "standardize product fact tables: best for, outcomes, contents, dimensions, materials, price, availability, warranty/returns, and evidence/review count")
	add("storefront_buying_guides", passfail(d.Storefront.BuyingGuides), "high", d.Storefront.Domain, "publish authoritative buying/comparison guides for high-intent queries and link them from collections/PDPs")
	if d.Storefront.Answerability < 75 {
		add("answerability_score", "warn", "medium", d.Storefront.Domain, "raise answerability above 75 with concise answer-first copy, FAQs, citations, reviews, and entity-consistent brand/product facts")
	} else {
		add("answerability_score", "pass", "medium", d.Storefront.Domain, "maintain answerability above 75")
	}
	for _, p := range d.Products {
		add("pdp_product_schema", passfail(p.HasStructuredData), "high", p.Handle, "add/validate Product, Offer, AggregateRating/Review, image, SKU/GTIN where available, priceCurrency, availability, and canonical URL")
		add("pdp_product_facts", passfail(p.HasProductFacts), "high", p.Handle, "add answer-engine-ready PDP facts and short answer block for best-for, use cases, differentiators, proof, price, and alternatives")
		add("pdp_faq_answers", passfail(p.HasFAQ), "medium", p.Handle, "add real customer-question FAQ with concise answers and matching FAQPage schema where eligible")
		add("pdp_buying_guide_links", passfail(p.HasBuyingGuideLinks), "medium", p.Handle, "link PDP to relevant buying guide, comparison guide, and collection hub")
	}
	for _, c := range d.Categories {
		add("category_guides", passfail(c.HasBuyingGuide), "high", c.Handle, "add collection intro plus buying guide/comparison module answering which product is best for each use case")
		add("category_faqs", passfail(c.HasFAQ), "medium", c.Handle, "add collection FAQ covering fit, selection criteria, shipping/returns, bundles, and giftability")
		add("category_itemlist_schema", passfail(c.StructuredData), "medium", c.Handle, "add BreadcrumbList and ItemList schema with canonical product URLs")
	}
	return checks
}
func simpleRowsCmd(f *rootFlags, use, short string, fn func(store.DataSet) []map[string]any) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: use, Short: short, RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := fn(d)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"target\taction\timpact"}
		for _, r := range rows {
			target := fmt.Sprint(firstAny(r, "product", "category", "flow", "target"))
			lines = append(lines, fmt.Sprintf("%s\t%s\t%v", target, r["action"], r["impact"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func totals(d store.DataSet) (float64, float64) {
	rev, prev := 0.0, 0.0
	for _, p := range d.Products {
		rev += p.Revenue
		prev += p.PreviousRevenue
	}
	return rev, prev
}
func inventoryRisks(d store.DataSet) []map[string]any {
	rows := []map[string]any{}
	for _, p := range d.Products {
		if p.Inventory < 15 || p.DaysOfStock < 7 || p.Revenue > 10000 && p.DaysOfStock < 21 {
			rows = append(rows, map[string]any{"type": "inventory", "product": p.Handle, "target": p.Handle, "action": "protect revenue with reorder/throttle/substitution plan", "impact": p.Revenue, "inventory": p.Inventory, "days_of_stock": p.DaysOfStock})
		}
	}
	sortRows(rows)
	return rows
}
func opps(d store.DataSet) []map[string]any {
	rows := inventoryRisks(d)
	for _, p := range d.Products {
		if !p.HasStructuredData || !p.HasProductFacts || !p.HasFAQ {
			rows = append(rows, map[string]any{"type": "geo", "target": p.Handle, "action": "make PDP answer-engine ready with schema, facts, FAQ, and buying-guide links", "impact": p.Revenue})
		}
		if p.Revenue < p.PreviousRevenue {
			rows = append(rows, map[string]any{"type": "revenue-drop", "target": p.Handle, "action": "diagnose search/session/conversion drop and refresh PDP/campaigns", "impact": p.PreviousRevenue - p.Revenue})
		}
	}
	for _, e := range d.Emails {
		if e.NeedsAction || e.ClickRate < .03 {
			rows = append(rows, map[string]any{"type": "email", "target": e.Name, "action": "refresh Klaviyo segment, creative, offer, and product blocks", "impact": e.Revenue})
		}
	}
	sortRows(rows)
	return rows
}
func geoScore(d store.DataSet) int {
	score := d.Storefront.Answerability
	if d.Storefront.LLMSTxt {
		score += 10
	}
	if d.Storefront.StructuredData {
		score += 10
	}
	if d.Storefront.ProductFacts {
		score += 10
	}
	if d.Storefront.BuyingGuides {
		score += 10
	}
	missing := 0
	total := 0
	for _, p := range d.Products {
		total += 3
		if !p.HasStructuredData {
			missing++
		}
		if !p.HasProductFacts {
			missing++
		}
		if !p.HasFAQ {
			missing++
		}
	}
	score -= missing * 5
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	if total == 0 && score == 0 {
		return 0
	}
	return score
}
func sortRows(rows []map[string]any) {
	sort.Slice(rows, func(i, j int) bool { return asFloat(rows[i]["impact"]) > asFloat(rows[j]["impact"]) })
}
func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
func firstAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return ""
}
func match(q string, parts ...string) bool {
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.Join(parts, " "))
	return strings.Contains(hay, q)
}
func _now() time.Time { return time.Now() }
