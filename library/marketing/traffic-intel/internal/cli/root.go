package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/traffic-intel/internal/store"
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
	cmd := &cobra.Command{Use: "traffic-intel-pp-cli", Short: "Private Printing Press traffic intelligence CLI", Long: "Combines local Google Search Console, Google Analytics, and Ahrefs-style traffic intelligence for agent workflows. MVP uses offline fixture/import data and makes no external API calls.", SilenceUsage: true, Version: version}
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
	cmd.AddCommand(agentContextCmd(f), doctorCmd(f), sourcesCmd(f), profileCmd(f), syncCmd(f), moneyPagesCmd(f), queryRevenueCmd(f), explainDropCmd(f), refreshQueueCmd(f), digestCmd(f))
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
		ctx := map[string]any{
			"schema_version":     "traffic-intel.agent-context/v1",
			"name":               "traffic-intel-pp-cli",
			"version":            version,
			"private":            true,
			"local_first":        true,
			"external_api_calls": false,
			"agent_flag":         "sets --json --compact --no-input --yes --no-color",
			"state_dir":          f.home,
			"env": []map[string]any{
				{"name": "TRAFFIC_INTEL_HOME", "required": false, "purpose": "override local state directory", "present": envPresent("TRAFFIC_INTEL_HOME")},
				{"name": "GSC_SITE_URL", "required": false, "purpose": "default Search Console site URL for child CLI sync", "present": envPresent("GSC_SITE_URL")},
				{"name": "GA4_PROPERTY_ID", "required": false, "purpose": "default GA4 property id for child CLI sync", "present": envPresent("GA4_PROPERTY_ID")},
				{"name": "AHREFS_PROJECT", "required": false, "purpose": "default Ahrefs target/domain for child CLI sync", "present": envPresent("AHREFS_PROJECT") || envPresent("AHREFS_TARGET")},
			},
			"commands": []map[string]any{
				{"name": "doctor", "safe_for_agents": true, "description": "local readiness, env presence, and optional child binary discovery"},
				{"name": "sources doctor", "safe_for_agents": true, "description": "source-specific child adapter status without printing secrets"},
				{"name": "profile save/list/show/delete", "safe_for_agents": true, "description": "manage local profile metadata"},
				{"name": "sync", "safe_for_agents": true, "description": "load embedded ecommerce fixture, local JSON import, or requested child CLI source sync"},
				{"name": "money-pages", "safe_for_agents": true, "description": "rank landing pages by GA4 revenue/conversions"},
				{"name": "query-revenue", "safe_for_agents": true, "description": "sum revenue for matching URLs/titles"},
				{"name": "explain-drop", "safe_for_agents": true, "description": "combine GSC/GA4/Ahrefs deltas into drop explanations"},
				{"name": "refresh-queue", "safe_for_agents": true, "description": "prioritize refreshes from lost clicks, revenue, and link value"},
				{"name": "digest weekly", "safe_for_agents": true, "description": "weekly local summary; handles empty datasets"},
			},
			"source_plan": []map[string]string{
				{"source": "gsc", "child_binary": "google-search-console-pp-cli", "planned_command": "google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions [\"page\"] --start-date <start> --end-date <end> --type WEB"},
				{"source": "ga4", "child_binary": "google-analytics-pp-cli", "planned_command": "google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>"},
				{"source": "ahrefs", "child_binary": "ahrefs-pp-cli", "planned_command": "ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date> --select url,sum_traffic,keywords,referring_domains,top_keyword"},
			},
		}
		return out(cmd, f, ctx, "")
	}}
}

func doctorCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local readiness without network calls", RunE: func(cmd *cobra.Command, args []string) error {
		profiles, _ := st(f).ListProfiles()
		checks := sourceChecks()
		result := map[string]any{"ok": true, "version": version, "state_dir": f.home, "profiles": len(profiles), "sources": checks, "env": envChecks()}
		lines := []string{fmt.Sprintf("traffic-intel-pp-cli %s", version), "state: " + f.home, fmt.Sprintf("profiles: %d", len(profiles)), "sources:"}
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
	cmd.AddCommand(&cobra.Command{Use: "doctor", Short: "Check GSC, GA4, and Ahrefs child adapter readiness", RunE: func(cmd *cobra.Command, args []string) error {
		checks := sourceChecks()
		lines := []string{"source	binary	env_present	planned_command"}
		for _, c := range checks {
			bin := "missing"
			if c.Path != "" {
				bin = c.Path
			}
			lines = append(lines, fmt.Sprintf("%s	%s	%s	%s", c.Name, bin, presentList(c.Env), c.PlannedCommand))
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
	defs := []sourceCheck{
		{Name: "gsc", ChildBinary: "google-search-console-pp-cli", Env: []envCheck{{Name: "GSC_SITE_URL"}}, PlannedCommand: "google-search-console-pp-cli webmasters query-search-analytics <site> --agent --dimensions [\"page\"] --start-date <start> --end-date <end> --type WEB", Note: "optional child CLI; used by sync --source gsc/all; not required for fixture/import mode"},
		{Name: "ga4", ChildBinary: "google-analytics-pp-cli", Env: []envCheck{{Name: "GA4_PROPERTY_ID"}}, PlannedCommand: "google-analytics-pp-cli top-pages --agent --property <property> --start <start> --end <end>", Note: "optional child CLI; used by sync --source ga4/all; not required for fixture/import mode"},
		{Name: "ahrefs", ChildBinary: "ahrefs-pp-cli", Env: []envCheck{{Name: "AHREFS_PROJECT"}, {Name: "AHREFS_TARGET"}}, PlannedCommand: "ahrefs-pp-cli site-explorer top-pages --agent --target <target> --date <date> --select url,sum_traffic,keywords,referring_domains,top_keyword", Note: "optional child CLI; used by sync --source ahrefs/all; not required for fixture/import mode"},
	}
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

func envChecks() []envCheck {
	names := []string{"TRAFFIC_INTEL_HOME", "GSC_SITE_URL", "GA4_PROPERTY_ID", "AHREFS_PROJECT", "AHREFS_TARGET"}
	out := make([]envCheck, 0, len(names))
	for _, name := range names {
		out = append(out, envCheck{Name: name, Present: envPresent(name)})
	}
	return out
}

func envPresent(name string) bool { return os.Getenv(name) != "" }

func presentList(env []envCheck) string {
	parts := make([]string, 0, len(env))
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
	var name, site, ga, ahrefs string
	save := &cobra.Command{Use: "save", Short: "Save a profile", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			name = f.profile
		}
		p := store.Profile{Name: name, SiteURL: site, GAProperty: ga, Ahrefs: ahrefs}
		if err := st(f).SaveProfile(p); err != nil {
			return err
		}
		return out(cmd, f, p, fmt.Sprintf("saved profile %s\n", name))
	}}
	save.Flags().StringVar(&name, "name", "", "Profile name (defaults to --profile)")
	save.Flags().StringVar(&site, "site", "", "GSC site URL")
	save.Flags().StringVar(&ga, "ga-property", "", "GA4 property id")
	save.Flags().StringVar(&ahrefs, "ahrefs-project", "", "Ahrefs project id/name")
	cmd.AddCommand(save)
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List profiles", RunE: func(cmd *cobra.Command, args []string) error {
		ps, err := st(f).ListProfiles()
		if err != nil {
			return err
		}
		lines := []string{}
		for _, p := range ps {
			lines = append(lines, p.Name+"\t"+p.SiteURL)
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
		return out(cmd, f, p, fmt.Sprintf("%s\nsite: %s\nga_property: %s\nahrefs_project: %s\n", p.Name, p.SiteURL, p.GAProperty, p.Ahrefs))
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

func syncCmd(f *rootFlags) *cobra.Command {
	var importPath string
	var source string
	var live bool
	var real bool
	var site string
	var gaProperty string
	var ahrefsTarget string
	var startDate string
	var endDate string
	var ahrefsDate string
	var limit int
	c := &cobra.Command{Use: "sync", Short: "Import local fixture, JSON data, or real child CLI data", RunE: func(cmd *cobra.Command, args []string) error {
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
				var pages []store.PageMetrics
				if err2 := json.Unmarshal(b, &pages); err2 != nil {
					return err
				}
				d = store.DataSet{Profile: f.profile, Source: importPath, Pages: pages, SyncedAt: time.Now().UTC()}
			}
			if d.Profile == "" {
				d.Profile = f.profile
			}
			if d.Source == "" {
				d.Source = importPath
			}
		} else {
			opts := childSyncOptions{Site: site, GAProperty: gaProperty, AhrefsTarget: ahrefsTarget, StartDate: startDate, EndDate: endDate, AhrefsDate: ahrefsDate, Limit: limit}
			if p, err := st(f).GetProfile(f.profile); err == nil {
				if opts.Site == "" {
					opts.Site = p.SiteURL
				}
				if opts.GAProperty == "" {
					opts.GAProperty = p.GAProperty
				}
				if opts.AhrefsTarget == "" {
					opts.AhrefsTarget = p.Ahrefs
				}
			}
			var err error
			d, err = syncFromChildCLIs(f.profile, source, opts)
			if err != nil {
				return err
			}
		}
		if err := st(f).SaveData(d); err != nil {
			return err
		}
		return out(cmd, f, map[string]any{"profile": d.Profile, "pages": len(d.Pages), "source": d.Source, "synced_at": d.SyncedAt}, fmt.Sprintf("synced %d pages for %s from %s\n", len(d.Pages), d.Profile, d.Source))
	}}
	c.Flags().StringVar(&importPath, "import", "", "Import JSON DataSet or []PageMetrics instead of fixture")
	c.Flags().StringVar(&source, "source", "", "Real child CLI source to sync: all, gsc, ga4, or ahrefs (default without this flag uses fixture)")
	c.Flags().BoolVar(&live, "live", false, "Use child CLIs instead of the embedded fixture (same as --source all)")
	c.Flags().BoolVar(&real, "real", false, "Use child CLIs instead of the embedded fixture (alias for --live)")
	c.Flags().StringVar(&site, "site", "", "Search Console site URL/property (or GSC_SITE_URL/profile site)")
	c.Flags().StringVar(&gaProperty, "ga-property", "", "GA4 property id (or GA4_PROPERTY_ID/profile property)")
	c.Flags().StringVar(&ahrefsTarget, "ahrefs-target", "", "Ahrefs target/domain (or AHREFS_TARGET/AHREFS_PROJECT/profile ahrefs project)")
	c.Flags().StringVar(&startDate, "start-date", "", "Start date for GSC/GA4 child sync (default: 7 completed days ago)")
	c.Flags().StringVar(&endDate, "end-date", "", "End date for GSC/GA4/Ahrefs child sync (default: yesterday)")
	c.Flags().StringVar(&ahrefsDate, "date", "", "Ahrefs snapshot date (default: --end-date/yesterday)")
	c.Flags().IntVar(&limit, "limit", 1000, "Maximum rows per child source")
	return c
}

type childSyncOptions struct {
	Site         string
	GAProperty   string
	AhrefsTarget string
	StartDate    string
	EndDate      string
	AhrefsDate   string
	Limit        int
}

type childSourceDef struct {
	name   string
	binary string
	args   []string
}

func syncFromChildCLIs(profile, source string, opts childSyncOptions) (store.DataSet, error) {
	defs, err := childSourceDefs(opts)
	if err != nil {
		return store.DataSet{}, err
	}
	if source != "all" {
		found := false
		for _, def := range defs {
			if def.name == source {
				defs = []childSourceDef{def}
				found = true
				break
			}
		}
		if !found {
			return store.DataSet{}, fmt.Errorf("unknown source %q (want all, gsc, ga4, or ahrefs)", source)
		}
	}

	merged := map[string]store.PageMetrics{}
	order := []string{}
	used := []string{}
	for _, def := range defs {
		pages, command, err := runChildSource(def)
		if err != nil {
			return store.DataSet{}, err
		}
		used = append(used, def.name)
		for _, page := range pages {
			key := pageKey(page.URL)
			if key == "" {
				continue
			}
			existing, ok := merged[key]
			if !ok {
				existing = store.PageMetrics{URL: page.URL}
				order = append(order, key)
			}
			merged[key] = mergeSource(existing, page, def.name, command)
		}
	}

	now := time.Now().UTC()
	pages := make([]store.PageMetrics, 0, len(order))
	for _, key := range order {
		p := merged[key]
		if p.UpdatedAt == "" {
			p.UpdatedAt = now.Format(time.RFC3339)
		}
		pages = append(pages, p)
	}
	return store.DataSet{Profile: profile, SyncedAt: now, Source: "child-cli:" + strings.Join(used, "+"), Pages: pages}, nil
}

func childSourceDefs(opts childSyncOptions) ([]childSourceDef, error) {
	start, end := defaultDateRange(opts.StartDate, opts.EndDate)
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}
	site := firstNonEmpty(opts.Site, os.Getenv("GSC_SITE_URL"))
	gaProperty := firstNonEmpty(opts.GAProperty, os.Getenv("GA4_PROPERTY_ID"))
	ahrefsTarget := firstNonEmpty(opts.AhrefsTarget, os.Getenv("AHREFS_TARGET"), os.Getenv("AHREFS_PROJECT"))
	ahrefsDate := firstNonEmpty(opts.AhrefsDate, end)
	defs := []childSourceDef{}
	if site != "" {
		defs = append(defs, childSourceDef{name: "gsc", binary: "google-search-console-pp-cli", args: []string{"webmasters", "query-search-analytics", site, "--agent", "--dimensions", "[\"page\"]", "--start-date", start, "--end-date", end, "--type", "WEB", "--row-limit", strconv.Itoa(opts.Limit)}})
	}
	if gaProperty != "" {
		defs = append(defs, childSourceDef{name: "ga4", binary: "google-analytics-pp-cli", args: []string{"top-pages", "--agent", "--property", gaProperty, "--start", start, "--end", end, "--limit", strconv.Itoa(opts.Limit)}})
	}
	if ahrefsTarget != "" {
		defs = append(defs, childSourceDef{name: "ahrefs", binary: "ahrefs-pp-cli", args: []string{"site-explorer", "top-pages", "--agent", "--target", ahrefsTarget, "--date", ahrefsDate, "--mode", "subdomains", "--protocol", "both", "--limit", strconv.Itoa(opts.Limit), "--select", "url,sum_traffic,keywords,referring_domains,top_keyword"}})
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("no child sources configured; provide --site/--ga-property/--ahrefs-target, a saved profile, or GSC_SITE_URL/GA4_PROPERTY_ID/AHREFS_TARGET")
	}
	return defs, nil
}

func defaultDateRange(start, end string) (string, string) {
	if end == "" {
		end = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if start == "" {
		t, err := time.Parse("2006-01-02", end)
		if err == nil {
			start = t.AddDate(0, 0, -7).Format("2006-01-02")
		} else {
			start = end
		}
	}
	return start, end
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runChildSource(def childSourceDef) ([]store.PageMetrics, string, error) {
	command := def.binary + " " + strings.Join(def.args, " ")
	c := exec.Command(def.binary, def.args...)
	b, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, command, fmt.Errorf("%s failed: %w: %s", def.name, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, command, fmt.Errorf("%s failed: %w", def.name, err)
	}
	pages, err := parseChildPages(def.name, b)
	if err != nil {
		return nil, command, fmt.Errorf("%s returned invalid JSON: %w", def.name, err)
	}
	return pages, command, nil
}

func mergeSource(dst, src store.PageMetrics, source, command string) store.PageMetrics {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.URL == "" || strings.HasPrefix(dst.URL, "/") && strings.HasPrefix(src.URL, "http") {
		dst.URL = src.URL
	}
	switch source {
	case "gsc":
		dst.Clicks, dst.Impressions, dst.CTR, dst.Position, dst.PreviousClicks = src.Clicks, src.Impressions, src.CTR, src.Position, src.PreviousClicks
		dst.Sources.GSC = src.Sources.GSC
		dst.Sources.GSC.ChildCLICommand = command
	case "ga4":
		dst.Sessions, dst.Conversions, dst.Revenue = src.Sessions, src.Conversions, src.Revenue
		dst.PreviousSessions, dst.PreviousRevenue = src.PreviousSessions, src.PreviousRevenue
		dst.Sources.GA4 = src.Sources.GA4
		dst.Sources.GA4.ChildCLICommand = command
	case "ahrefs":
		dst.Backlinks, dst.RefDomains = src.Backlinks, src.RefDomains
		dst.Sources.Ahrefs = src.Sources.Ahrefs
		dst.Sources.Ahrefs.ChildCLICommand = command
	}
	if src.UpdatedAt != "" {
		dst.UpdatedAt = src.UpdatedAt
	}
	return dst
}

func parseChildPages(source string, b []byte) ([]store.PageMetrics, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	rows := findRows(v)
	pages := make([]store.PageMetrics, 0, len(rows))
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			continue
		}
		p := pageFromChildMap(source, flattenMap(m, ""))
		if p.URL != "" {
			pages = append(pages, p)
		}
	}
	return pages, nil
}

func findRows(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		for _, key := range []string{"pages", "rows", "items", "results"} {
			if rows, ok := x[key].([]any); ok {
				return rows
			}
		}
		for _, key := range []string{"data", "response", "result"} {
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
		if arr, ok := v.([]any); ok {
			for i, item := range arr {
				out[fmt.Sprintf("%s.%d", key, i)] = item
			}
		}
		out[key] = v
	}
	return out
}

func pageFromChildMap(source string, m map[string]any) store.PageMetrics {
	url := firstString(m, "url", "page", "pageurl", "page_url", "landingpageplusquerystring", "landing_page_plus_query_string", "landingpage", "landing_page", "landingpagepath", "landing_page_path", "path", "pagepath", "page_path", "keys.0", "dimensionvalues.0.value")
	title := firstString(m, "title", "pagetitle", "page_title")
	updatedAt := firstString(m, "updatedat", "updated_at", "date")
	p := store.PageMetrics{URL: url, Title: title, UpdatedAt: updatedAt}
	switch source {
	case "gsc":
		p.Clicks = firstInt(m, "clicks", "gsc.clicks")
		p.Impressions = firstInt(m, "impressions", "gsc.impressions")
		p.CTR = firstFloat(m, "ctr", "clickthroughrate", "click_through_rate", "gsc.ctr")
		p.Position = firstFloat(m, "position", "avgposition", "avg_position", "averageposition", "average_position", "gsc.position")
		p.PreviousClicks = firstInt(m, "previousclicks", "previous_clicks", "priorclicks", "prior_clicks", "clicksprevious", "clicks_previous", "gsc.previous_clicks")
		p.Sources.GSC = store.GSCMetrics{Clicks: p.Clicks, Impressions: p.Impressions, CTR: p.CTR, Position: p.Position, PreviousClicks: p.PreviousClicks, QuerySample: firstString(m, "query", "querysample", "query_sample", "keyword", "topkeyword", "top_keyword"), ChildCLICommand: "google-search-console-pp-cli webmasters query-search-analytics"}
	case "ga4":
		p.Sessions = firstInt(m, "sessions", "ga4.sessions")
		p.Conversions = firstInt(m, "conversions", "purchases", "transactions", "keyevents", "key_events", "ga4.conversions")
		p.Revenue = firstFloat(m, "revenue", "totalrevenue", "total_revenue", "purchase_revenue", "purchaserevenue", "ga4.revenue")
		p.PreviousSessions = firstInt(m, "previoussessions", "previous_sessions", "priorsessions", "prior_sessions", "sessionsprevious", "sessions_previous", "ga4.previous_sessions")
		p.PreviousRevenue = firstFloat(m, "previousrevenue", "previous_revenue", "priorrevenue", "prior_revenue", "revenueprevious", "revenue_previous", "ga4.previous_revenue")
		p.Sources.GA4 = store.GA4Metrics{Sessions: p.Sessions, Conversions: p.Conversions, Revenue: p.Revenue, PreviousSessions: p.PreviousSessions, PreviousRevenue: p.PreviousRevenue, ChildCLICommand: "google-analytics-pp-cli top-pages"}
	case "ahrefs":
		p.Backlinks = firstInt(m, "backlinks", "backlink_count", "backlinkcount", "ahrefs.backlinks")
		p.RefDomains = firstInt(m, "refdomains", "ref_domains", "referringdomains", "referring_domains", "domains", "ahrefs.ref_domains")
		p.Sources.Ahrefs = store.AhrefsMetrics{Backlinks: p.Backlinks, RefDomains: p.RefDomains, TopKeyword: firstString(m, "topkeyword", "top_keyword", "keyword", "query")}
	}
	return p
}

func pageKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		path := strings.TrimRight(u.Path, "/")
		if path == "" {
			path = "/"
		}
		return strings.ToLower(path)
	}
	path := strings.TrimRight(raw, "/")
	if path == "" {
		path = "/"
	}
	return strings.ToLower(path)
}

func normKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	repl := strings.NewReplacer(" ", "_", "-", "_", "/", "_")
	return repl.Replace(s)
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
			return int(firstNumeric(v))
		}
	}
	return 0
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[normKey(key)]; ok {
			return firstNumeric(v)
		}
	}
	return 0
}

func firstNumeric(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		x = strings.TrimSpace(strings.TrimSuffix(x, "%"))
		f, _ := strconv.ParseFloat(strings.ReplaceAll(x, ",", ""), 64)
		return f
	default:
		return 0
	}
}

func load(f *rootFlags) (store.DataSet, error) {
	d, err := st(f).LoadData(f.profile)
	if err != nil {
		return d, fmt.Errorf("no local data for profile %q: run sync first", f.profile)
	}
	return d, nil
}
func sortedPages(d store.DataSet, score func(store.PageMetrics) float64) []store.PageMetrics {
	ps := append([]store.PageMetrics(nil), d.Pages...)
	sort.Slice(ps, func(i, j int) bool { return score(ps[i]) > score(ps[j]) })
	return ps
}

func moneyPagesCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "money-pages", Short: "Rank pages by revenue and conversion value", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, func(p store.PageMetrics) float64 { return p.Revenue })
		if limit > 0 && len(ps) > limit {
			ps = ps[:limit]
		}
		lines := []string{"url\trevenue\tconversions\tsessions"}
		for _, p := range ps {
			lines = append(lines, fmt.Sprintf("%s\t%.2f\t%d\t%d", p.URL, p.Revenue, p.Conversions, p.Sessions))
		}
		return out(cmd, f, ps, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func queryRevenueCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "query-revenue [query-or-url]", Args: cobra.MaximumNArgs(1), Short: "Find revenue for pages matching a query/url", RunE: func(cmd *cobra.Command, args []string) error {
		q := ""
		if len(args) > 0 {
			q = strings.ToLower(args[0])
		}
		d, err := load(f)
		if err != nil {
			return err
		}
		var rows []store.PageMetrics
		total := 0.0
		for _, p := range d.Pages {
			hay := strings.ToLower(p.URL + " " + p.Title)
			if q == "" || strings.Contains(hay, q) {
				rows = append(rows, p)
				total += p.Revenue
			}
		}
		return out(cmd, f, map[string]any{"query": q, "revenue": total, "rows": rows}, fmt.Sprintf("query: %s\nrevenue: %.2f\nrows: %d\n", q, total, len(rows)))
	}}
}

func explainDropCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "explain-drop [query-or-url]", Args: cobra.MaximumNArgs(1), Short: "Explain traffic/revenue drops from local before/after metrics", RunE: func(cmd *cobra.Command, args []string) error {
		q := ""
		if len(args) > 0 {
			q = strings.ToLower(args[0])
		}
		d, err := load(f)
		if err != nil {
			return err
		}
		explanations := []map[string]any{}
		for _, p := range d.Pages {
			if q != "" && !strings.Contains(strings.ToLower(p.URL+" "+p.Title), q) {
				continue
			}
			clickDelta := p.Clicks - p.PreviousClicks
			sessionDelta := p.Sessions - p.PreviousSessions
			revenueDelta := p.Revenue - p.PreviousRevenue
			if clickDelta < 0 || sessionDelta < 0 || revenueDelta < 0 {
				cause := "mixed signals"
				if clickDelta < 0 && p.Position > 5 {
					cause = "ranking/CTR loss likely from Search Console metrics"
				} else if sessionDelta < 0 {
					cause = "analytics session decline"
				} else if revenueDelta < 0 {
					cause = "conversion or order-value decline"
				}
				explanations = append(explanations, map[string]any{"url": p.URL, "click_delta": clickDelta, "session_delta": sessionDelta, "revenue_delta": revenueDelta, "likely_cause": cause, "next_action": "refresh SERP, inspect changed queries, and update/relink page"})
			}
		}
		return out(cmd, f, explanations, fmt.Sprintf("found %d dropping pages\n", len(explanations)))
	}}
}

func refreshQueueCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "refresh-queue", Short: "Prioritize pages to refresh", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, func(p store.PageMetrics) float64 {
			return float64(max(0, p.PreviousClicks-p.Clicks))*2 + maxf(0, p.PreviousRevenue-p.Revenue)/100 + float64(p.RefDomains)
		})
		if limit > 0 && len(ps) > limit {
			ps = ps[:limit]
		}
		rows := []map[string]any{}
		lines := []string{"url\tpriority\treason"}
		for i, p := range ps {
			reason := "protect revenue/backlinks"
			if p.Clicks < p.PreviousClicks {
				reason = "recover lost search clicks"
			}
			rows = append(rows, map[string]any{"rank": i + 1, "url": p.URL, "reason": reason, "lost_clicks": p.PreviousClicks - p.Clicks, "lost_revenue": p.PreviousRevenue - p.Revenue})
			lines = append(lines, fmt.Sprintf("%s\t%d\t%s", p.URL, i+1, reason))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func digestCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "digest", Short: "Generate digests"}
	cmd.AddCommand(&cobra.Command{Use: "weekly", Short: "Generate weekly traffic intelligence digest", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		revenue := 0.0
		clicks := 0
		sessions := 0
		for _, p := range d.Pages {
			revenue += p.Revenue
			clicks += p.Clicks
			sessions += p.Sessions
		}
		topMoneyPage := ""
		if len(d.Pages) > 0 {
			topMoneyPage = sortedPages(d, func(p store.PageMetrics) float64 { return p.Revenue })[0].URL
		}
		digest := map[string]any{"profile": d.Profile, "synced_at": d.SyncedAt, "pages": len(d.Pages), "clicks": clicks, "sessions": sessions, "revenue": revenue, "top_money_page": topMoneyPage, "recommended_next_command": "traffic-intel-pp-cli refresh-queue --profile " + d.Profile}
		if len(d.Pages) == 0 {
			digest["note"] = "no pages in local dataset; run sync --import with page metrics or sync without --import for the fixture"
		}
		return out(cmd, f, digest, fmt.Sprintf("Weekly digest for %s\npages: %d\nclicks: %d\nsessions: %d\nrevenue: %.2f\ntop money page: %s\n", d.Profile, len(d.Pages), clicks, sessions, revenue, topMoneyPage))
	}})
	return cmd
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
