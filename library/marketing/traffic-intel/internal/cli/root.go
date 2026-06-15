package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
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
				{"name": "AHREFS_PROJECT", "required": false, "purpose": "default Ahrefs project id/name for child CLI sync", "present": envPresent("AHREFS_PROJECT")},
			},
			"commands": []map[string]any{
				{"name": "doctor", "safe_for_agents": true, "description": "local readiness, env presence, and optional child binary discovery"},
				{"name": "sources doctor", "safe_for_agents": true, "description": "source-specific child adapter status without printing secrets"},
				{"name": "profile save/list/show/delete", "safe_for_agents": true, "description": "manage local profile metadata"},
				{"name": "sync", "safe_for_agents": true, "description": "load embedded ecommerce fixture or local JSON import"},
				{"name": "money-pages", "safe_for_agents": true, "description": "rank landing pages by GA4 revenue/conversions"},
				{"name": "query-revenue", "safe_for_agents": true, "description": "sum revenue for matching URLs/titles"},
				{"name": "explain-drop", "safe_for_agents": true, "description": "combine GSC/GA4/Ahrefs deltas into drop explanations"},
				{"name": "refresh-queue", "safe_for_agents": true, "description": "prioritize refreshes from lost clicks, revenue, and link value"},
				{"name": "digest weekly", "safe_for_agents": true, "description": "weekly local summary; handles empty datasets"},
			},
			"source_plan": []map[string]string{
				{"source": "gsc", "child_binary": "google-search-console-pp-cli", "planned_command": "google-search-console-pp-cli export pages --json"},
				{"source": "ga4", "child_binary": "google-analytics-pp-cli", "planned_command": "google-analytics-pp-cli export landing-pages --json"},
				{"source": "ahrefs", "child_binary": "ahrefs-pp-cli", "planned_command": "ahrefs-pp-cli export pages --json"},
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
		{Name: "gsc", ChildBinary: "google-search-console-pp-cli", Env: []envCheck{{Name: "GSC_SITE_URL"}}, PlannedCommand: "google-search-console-pp-cli export pages --json", Note: "optional child CLI; future sync adapter; not required for local fixture/import MVP"},
		{Name: "ga4", ChildBinary: "google-analytics-pp-cli", Env: []envCheck{{Name: "GA4_PROPERTY_ID"}}, PlannedCommand: "google-analytics-pp-cli export landing-pages --json", Note: "optional child CLI; future sync adapter; not required for local fixture/import MVP"},
		{Name: "ahrefs", ChildBinary: "ahrefs-pp-cli", Env: []envCheck{{Name: "AHREFS_PROJECT"}}, PlannedCommand: "ahrefs-pp-cli export pages --json", Note: "optional child CLI; future sync adapter; not required for local fixture/import MVP"},
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
	names := []string{"TRAFFIC_INTEL_HOME", "GSC_SITE_URL", "GA4_PROPERTY_ID", "AHREFS_PROJECT"}
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
	c := &cobra.Command{Use: "sync", Short: "Import local fixture or JSON data", RunE: func(cmd *cobra.Command, args []string) error {
		var d store.DataSet
		if importPath == "" {
			d = store.Fixture(f.profile)
		} else {
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
		}
		if err := st(f).SaveData(d); err != nil {
			return err
		}
		return out(cmd, f, map[string]any{"profile": d.Profile, "pages": len(d.Pages), "source": d.Source, "synced_at": d.SyncedAt}, fmt.Sprintf("synced %d pages for %s from %s\n", len(d.Pages), d.Profile, d.Source))
	}}
	c.Flags().StringVar(&importPath, "import", "", "Import JSON DataSet or []PageMetrics instead of fixture")
	return c
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
