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
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
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
	cmd.AddCommand(agentContextCmd(f), doctorCmd(f), sourcesCmd(f), profileCmd(f), syncCmd(f), moversCmd(f), moneyPagesCmd(f), queryRevenueCmd(f), explainDropCmd(f), refreshQueueCmd(f), opportunityGapCmd(f), quickWinsCmd(f), revenueAtRiskCmd(f), refreshBriefCmd(f), cannibalizationCmd(f), topicClustersCmd(f), sourceCoverageCmd(f), internalLinkPlanCmd(f), experimentPlanCmd(f), forecastImpactCmd(f), staleWinnersCmd(f), digestCmd(f))
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
				{"name": "AHREFS_PROJECT", "required": false, "purpose": "default Ahrefs project/target for child CLI sync", "present": envPresent("AHREFS_PROJECT")},
				{"name": "AHREFS_TARGET", "required": false, "purpose": "default Ahrefs target/domain for child CLI sync", "present": envPresent("AHREFS_TARGET")},
			},
			"commands": []map[string]any{
				{"name": "doctor", "safe_for_agents": true, "description": "local readiness, env presence, and optional child binary discovery"},
				{"name": "sources doctor", "safe_for_agents": true, "description": "source-specific child adapter status without printing secrets"},
				{"name": "profile save/list/show/delete", "safe_for_agents": true, "description": "manage local profile metadata"},
				{"name": "sync", "safe_for_agents": true, "description": "load embedded ecommerce fixture, local JSON import, or opt-in child CLI sync; all/live/real require all source configs"},
				{"name": "movers", "safe_for_agents": true, "description": "diff latest snapshot against the previous snapshot for climbers, droppers, new Strike-Zone entrants, and new revenue-at-risk"},
				{"name": "money-pages", "safe_for_agents": true, "description": "rank landing pages by GA4 revenue/conversions"},
				{"name": "query-revenue", "safe_for_agents": true, "description": "sum revenue for matching URLs/titles"},
				{"name": "explain-drop", "safe_for_agents": true, "description": "combine GSC/GA4/Ahrefs deltas into drop explanations"},
				{"name": "refresh-queue", "safe_for_agents": true, "description": "prioritize refreshes from lost clicks, revenue, and link value"},
				{"name": "opportunity-gap", "safe_for_agents": true, "description": "rank high-impression near-ranking pages by CTR gap and business value"},
				{"name": "quick-wins", "safe_for_agents": true, "description": "surface pages with near-page-one positions, weak CTR, and conversion/revenue value"},
				{"name": "revenue-at-risk", "safe_for_agents": true, "description": "rank pages where organic/session declines overlap with revenue or conversion value"},
				{"name": "refresh-brief", "safe_for_agents": true, "description": "generate an agent-ready refresh brief for one URL or topic"},
				{"name": "cannibalization", "safe_for_agents": true, "description": "detect pages competing for the same query/topic and rank by revenue impact"},
				{"name": "topic-clusters", "safe_for_agents": true, "description": "summarize clicks, revenue, backlinks, and decay by inferred topic cluster"},
				{"name": "source-coverage", "safe_for_agents": true, "description": "audit which pages have GSC, GA4, and Ahrefs evidence and what is missing"},
				{"name": "internal-link-plan", "safe_for_agents": true, "description": "recommend source and target pages for internal links by topic, revenue, and link equity"},
				{"name": "experiment-plan", "safe_for_agents": true, "description": "turn one page into title, meta, content, and measurement tests"},
				{"name": "forecast-impact", "safe_for_agents": true, "description": "estimate click and revenue upside from closing CTR gaps"},
				{"name": "stale-winners", "safe_for_agents": true, "description": "find high-value pages to refresh before visible decay"},
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
		inputHashes := map[string]string{}
		sourceVersions := map[string]string{"traffic-intel-pp-cli": version}
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" && (live || real) {
			source = "all"
		}
		if importPath != "" && source != "" {
			return fmt.Errorf("--import cannot be combined with --source/--live/--real")
		}
		if importPath == "" && source == "" {
			d = store.Fixture(f.profile)
			inputHashes["embedded_fixture"] = intelcli.HashJSON(d.Pages)
		} else if importPath != "" {
			b, err := os.ReadFile(importPath)
			if err != nil {
				return err
			}
			inputHashes["import_file"] = intelcli.HashBytes(b)
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
			inputHashes = d.Provenance.InputHashes
			sourceVersions = intelcli.MergeStringMaps(sourceVersions, d.Provenance.SourceCommandVersions)
		}
		ensureSyncProvenance(&d, startDate, endDate, sourceVersions, inputHashes)
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
	switch source {
	case "all":
		if missing := missingChildSources(defs, "gsc", "ga4", "ahrefs"); len(missing) > 0 {
			return store.DataSet{}, fmt.Errorf("--source all requires gsc, ga4, and ahrefs configuration; missing %s", strings.Join(missing, ", "))
		}
	case "gsc", "ga4", "ahrefs":
		def, ok := findChildSource(defs, source)
		if !ok {
			return store.DataSet{}, fmt.Errorf("source %q is not configured; %s", source, sourceConfigHint(source))
		}
		defs = []childSourceDef{def}
	default:
		return store.DataSet{}, fmt.Errorf("unknown source %q (want all, gsc, ga4, or ahrefs)", source)
	}

	merged := map[string]store.PageMetrics{}
	order := []string{}
	used := []string{}
	inputHashes := map[string]string{}
	sourceVersions := map[string]string{}
	for _, def := range defs {
		pages, command, outputHash, childVersion, err := runChildSource(def)
		if err != nil {
			return store.DataSet{}, err
		}
		used = append(used, def.name)
		inputHashes[def.name] = outputHash
		sourceVersions[def.name] = childVersion
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
	start, end := defaultDateRange(opts.StartDate, opts.EndDate)
	return store.DataSet{
		Profile:  profile,
		SyncedAt: now,
		Source:   "child-cli:" + strings.Join(used, "+"),
		Provenance: store.DataProvenance{
			SchemaVersion:         "traffic-intel.provenance/v1",
			DateRange:             store.DateRange{StartDate: start, EndDate: end},
			SourceCommandVersions: sourceVersions,
			InputHashes:           inputHashes,
		},
		Pages: pages,
	}, nil
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
	return defs, nil
}

func findChildSource(defs []childSourceDef, source string) (childSourceDef, bool) {
	for _, def := range defs {
		if def.name == source {
			return def, true
		}
	}
	return childSourceDef{}, false
}

func missingChildSources(defs []childSourceDef, sources ...string) []string {
	missing := []string{}
	for _, source := range sources {
		if _, ok := findChildSource(defs, source); !ok {
			missing = append(missing, source)
		}
	}
	return missing
}

func sourceConfigHint(source string) string {
	switch source {
	case "gsc":
		return "provide --site, a saved profile site, or GSC_SITE_URL"
	case "ga4":
		return "provide --ga-property, a saved profile GA property, or GA4_PROPERTY_ID"
	case "ahrefs":
		return "provide --ahrefs-target, a saved profile Ahrefs project, AHREFS_TARGET, or AHREFS_PROJECT"
	default:
		return "provide --site/--ga-property/--ahrefs-target, a saved profile, or source env vars"
	}
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

func ensureSyncProvenance(d *store.DataSet, startDate, endDate string, sourceVersions, inputHashes map[string]string) {
	if d.Profile == "" {
		d.Profile = "default"
	}
	if d.SyncedAt.IsZero() {
		d.SyncedAt = time.Now().UTC()
	}
	if d.Provenance.SchemaVersion == "" {
		d.Provenance.SchemaVersion = "traffic-intel.provenance/v1"
	}
	if d.Provenance.DateRange.StartDate == "" && d.Provenance.DateRange.EndDate == "" {
		start, end := defaultDateRange(startDate, endDate)
		d.Provenance.DateRange = store.DateRange{StartDate: start, EndDate: end}
	}
	d.Provenance.SourceCommandVersions = intelcli.MergeStringMaps(d.Provenance.SourceCommandVersions, sourceVersions)
	d.Provenance.InputHashes = intelcli.MergeStringMaps(d.Provenance.InputHashes, inputHashes)
	if len(d.Provenance.InputHashes) == 0 {
		d.Provenance.InputHashes = map[string]string{"dataset": intelcli.HashJSON(d.Pages)}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runChildSource(def childSourceDef) ([]store.PageMetrics, string, string, string, error) {
	command := def.binary + " " + strings.Join(def.args, " ")
	c := exec.Command(def.binary, def.args...)
	b, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, command, "", "", fmt.Errorf("%s failed: %w: %s", def.name, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, command, "", "", fmt.Errorf("%s failed: %w", def.name, err)
	}
	pages, err := parseChildPages(def.name, b)
	if err != nil {
		return nil, command, "", "", fmt.Errorf("%s returned invalid JSON: %w", def.name, err)
	}
	return pages, command, intelcli.HashBytes(b), intelcli.ChildCLIVersion(def.binary), nil
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
		for _, key := range []string{"results", "data", "response", "result"} {
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
		if len(explanations) > 0 {
			if err := st(f).AppendLearning(d.Profile, fmt.Sprintf("explain-drop found %d dropping pages for query %q; top target: %v", len(explanations), q, explanations[0]["url"])); err != nil {
				return err
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

func opportunityGapCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "opportunity-gap", Short: "Rank high-upside organic opportunities", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, opportunityScore)
		rows := []map[string]any{}
		lines := []string{"url\tscore\tposition\tctr_gap\trevenue\tquery"}
		for _, p := range ps {
			score := opportunityScore(p)
			if score <= 0 {
				continue
			}
			row := pageOpportunityRow(p, len(rows)+1, score)
			rows = append(rows, row)
			lines = append(lines, fmt.Sprintf("%s\t%.1f\t%.1f\t%.3f\t%.2f\t%s", p.URL, score, p.Position, ctrGap(p), p.Revenue, primaryTopic(p)))
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func quickWinsCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "quick-wins", Short: "Find near-page-one pages with weak CTR and business value", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, quickWinScore)
		rows := []map[string]any{}
		lines := []string{"url\tscore\tposition\tctr\texpected_ctr\tnext_action"}
		for _, p := range ps {
			score := quickWinScore(p)
			if score <= 0 {
				continue
			}
			expected := expectedCTR(p.Position)
			row := map[string]any{
				"rank":         len(rows) + 1,
				"url":          p.URL,
				"title":        p.Title,
				"query":        primaryTopic(p),
				"score":        score,
				"position":     p.Position,
				"ctr":          normalizedCTR(p.CTR),
				"expected_ctr": expected,
				"ctr_gap":      ctrGap(p),
				"revenue":      p.Revenue,
				"conversions":  p.Conversions,
				"next_action":  "test title/meta angle, add internal links, and refresh above-the-fold answer",
			}
			rows = append(rows, row)
			lines = append(lines, fmt.Sprintf("%s\t%.1f\t%.1f\t%.3f\t%.3f\t%s", p.URL, score, p.Position, normalizedCTR(p.CTR), expected, row["next_action"]))
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func revenueAtRiskCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "revenue-at-risk", Short: "Rank organic declines that overlap with revenue value", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, revenueRiskScore)
		rows := []map[string]any{}
		lines := []string{"url\tscore\tlost_clicks\tlost_sessions\tlost_revenue\trevenue"}
		for _, p := range ps {
			score := revenueRiskScore(p)
			if score <= 0 {
				continue
			}
			row := map[string]any{
				"rank":          len(rows) + 1,
				"url":           p.URL,
				"title":         p.Title,
				"query":         primaryTopic(p),
				"score":         score,
				"lost_clicks":   max(0, p.PreviousClicks-p.Clicks),
				"lost_sessions": max(0, p.PreviousSessions-p.Sessions),
				"lost_revenue":  maxf(0, p.PreviousRevenue-p.Revenue),
				"revenue":       p.Revenue,
				"conversions":   p.Conversions,
				"ref_domains":   p.RefDomains,
				"next_action":   riskAction(p),
			}
			rows = append(rows, row)
			lines = append(lines, fmt.Sprintf("%s\t%.1f\t%d\t%d\t%.2f\t%.2f", p.URL, score, row["lost_clicks"], row["lost_sessions"], row["lost_revenue"], p.Revenue))
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func refreshBriefCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "refresh-brief <url-or-topic>", Args: cobra.ExactArgs(1), Short: "Generate an agent-ready refresh brief for one page", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		p, ok := findPage(d.Pages, args[0])
		if !ok {
			return fmt.Errorf("no page matching %q in profile %q", args[0], f.profile)
		}
		brief := refreshBrief(p)
		human := fmt.Sprintf("Refresh brief: %s\nquery: %s\nlikely issue: %s\nnext: %s\n", p.URL, primaryTopic(p), brief["likely_issue"], firstAction(brief))
		return out(cmd, f, brief, human)
	}}
}

func cannibalizationCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "cannibalization", Short: "Detect pages competing for the same query/topic", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		groups := topicGroups(d.Pages, true)
		rows := []map[string]any{}
		for topic, ps := range groups {
			if len(ps) < 2 {
				continue
			}
			sort.Slice(ps, func(i, j int) bool { return businessValue(ps[i]) > businessValue(ps[j]) })
			totalRevenue, totalClicks := 0.0, 0
			pageRows := []map[string]any{}
			for _, p := range ps {
				totalRevenue += p.Revenue
				totalClicks += p.Clicks
				pageRows = append(pageRows, map[string]any{"url": p.URL, "title": p.Title, "clicks": p.Clicks, "position": p.Position, "revenue": p.Revenue, "query": primaryTopic(p)})
			}
			rows = append(rows, map[string]any{
				"topic":              topic,
				"pages":              pageRows,
				"page_count":         len(ps),
				"total_revenue":      totalRevenue,
				"total_clicks":       totalClicks,
				"canonical_url":      ps[0].URL,
				"recommended_action": "choose a canonical page, consolidate overlapping intent, and add internal links from weaker pages",
				"score":              totalRevenue + float64(totalClicks),
			})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["score"].(float64) > rows[j]["score"].(float64) })
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"topic\tpages\ttotal_revenue\tcanonical_url"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%.2f\t%s", row["topic"], row["page_count"], row["total_revenue"], row["canonical_url"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func topicClustersCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "topic-clusters", Short: "Summarize traffic, revenue, and decay by topic cluster", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		groups := topicGroups(d.Pages, false)
		rows := []map[string]any{}
		for topic, ps := range groups {
			clicks, impressions, sessions, conversions, refDomains := 0, 0, 0, 0, 0
			lostClicks, lostSessions := 0, 0
			revenue, lostRevenue := 0.0, 0.0
			topURL := ""
			topValue := -1.0
			for _, p := range ps {
				clicks += p.Clicks
				impressions += p.Impressions
				sessions += p.Sessions
				conversions += p.Conversions
				refDomains += p.RefDomains
				lostClicks += max(0, p.PreviousClicks-p.Clicks)
				lostSessions += max(0, p.PreviousSessions-p.Sessions)
				revenue += p.Revenue
				lostRevenue += maxf(0, p.PreviousRevenue-p.Revenue)
				if value := businessValue(p); value > topValue {
					topValue = value
					topURL = p.URL
				}
			}
			rows = append(rows, map[string]any{
				"topic":            topic,
				"page_count":       len(ps),
				"clicks":           clicks,
				"impressions":      impressions,
				"sessions":         sessions,
				"conversions":      conversions,
				"revenue":          revenue,
				"ref_domains":      refDomains,
				"lost_clicks":      lostClicks,
				"lost_sessions":    lostSessions,
				"lost_revenue":     lostRevenue,
				"top_url":          topURL,
				"recommended_next": clusterAction(lostClicks, lostRevenue, revenue),
				"score":            revenue + float64(clicks) + lostRevenue + float64(lostClicks*2),
			})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["score"].(float64) > rows[j]["score"].(float64) })
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"topic\tpages\tclicks\trevenue\tlost_clicks\ttop_url"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%d\t%d\t%.2f\t%d\t%s", row["topic"], row["page_count"], row["clicks"], row["revenue"], row["lost_clicks"], row["top_url"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func sourceCoverageCmd(f *rootFlags) *cobra.Command {
	var limit int
	var missingOnly bool
	c := &cobra.Command{Use: "source-coverage", Short: "Audit page coverage across GSC, GA4, and Ahrefs", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := []map[string]any{}
		counts := map[string]int{"gsc": 0, "ga4": 0, "ahrefs": 0, "complete": 0, "partial": 0}
		for _, p := range d.Pages {
			row := coverageRow(p)
			for _, source := range []string{"gsc", "ga4", "ahrefs"} {
				if row[source+"_present"].(bool) {
					counts[source]++
				}
			}
			if len(row["missing_sources"].([]string)) == 0 {
				counts["complete"]++
			} else {
				counts["partial"]++
			}
			if missingOnly && len(row["missing_sources"].([]string)) == 0 {
				continue
			}
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool {
			return len(rows[i]["missing_sources"].([]string)) > len(rows[j]["missing_sources"].([]string))
		})
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		result := map[string]any{"profile": d.Profile, "pages": len(d.Pages), "summary": counts, "rows": rows}
		lines := []string{"url\tcoverage\tmissing_sources"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%.2f\t%s", row["url"], row["coverage_score"], strings.Join(row["missing_sources"].([]string), ",")))
		}
		return out(cmd, f, result, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 25, "Rows to return")
	c.Flags().BoolVar(&missingOnly, "missing-only", false, "Only show pages missing one or more sources")
	return c
}

func internalLinkPlanCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "internal-link-plan", Short: "Recommend internal links from source pages to opportunity targets", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := internalLinkPlan(d.Pages)
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		lines := []string{"from_url\tto_url\tanchor\treason"}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s", row["from_url"], row["to_url"], row["anchor"], row["reason"]))
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func experimentPlanCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "experiment-plan <url-or-topic>", Args: cobra.ExactArgs(1), Short: "Create title, meta, content, and measurement tests for one page", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		p, ok := findPage(d.Pages, args[0])
		if !ok {
			return fmt.Errorf("no page matching %q in profile %q", args[0], f.profile)
		}
		plan := experimentPlan(p)
		if err := st(f).AppendLearning(d.Profile, fmt.Sprintf("experiment-plan generated for %s; success metric: %v", p.URL, plan["primary_success_metric"])); err != nil {
			return err
		}
		human := fmt.Sprintf("Experiment plan: %s\nprimary query: %s\nsuccess metric: %s\n", p.URL, primaryTopic(p), plan["primary_success_metric"])
		return out(cmd, f, plan, human)
	}}
}

func forecastImpactCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "forecast-impact", Short: "Estimate clicks and revenue from closing CTR gaps", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, forecastScore)
		rows := []map[string]any{}
		lines := []string{"url\test_click_gain\test_revenue_gain\tctr_gap"}
		for _, p := range ps {
			row := forecastRow(p, len(rows)+1)
			if row["estimated_click_gain"].(float64) <= 0 {
				continue
			}
			rows = append(rows, row)
			lines = append(lines, fmt.Sprintf("%s\t%.1f\t%.2f\t%.3f", row["url"], row["estimated_click_gain"], row["estimated_revenue_gain"], row["ctr_gap"]))
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		return out(cmd, f, rows, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows to return")
	return c
}

func staleWinnersCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "stale-winners", Short: "Find high-value pages to refresh before visible decay", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		ps := sortedPages(d, staleWinnerScore)
		rows := []map[string]any{}
		lines := []string{"url\tscore\trevenue\tclicks\tpreventive_action"}
		for _, p := range ps {
			score := staleWinnerScore(p)
			if score <= 0 {
				continue
			}
			row := map[string]any{
				"rank":              len(rows) + 1,
				"url":               p.URL,
				"title":             p.Title,
				"query":             primaryTopic(p),
				"score":             score,
				"revenue":           p.Revenue,
				"clicks":            p.Clicks,
				"sessions":          p.Sessions,
				"conversions":       p.Conversions,
				"ref_domains":       p.RefDomains,
				"stale_days":        staleDays(p),
				"preventive_action": staleWinnerAction(p),
			}
			rows = append(rows, row)
			lines = append(lines, fmt.Sprintf("%s\t%.1f\t%.2f\t%d\t%s", p.URL, score, p.Revenue, p.Clicks, row["preventive_action"]))
			if limit > 0 && len(rows) >= limit {
				break
			}
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
		digest := map[string]any{"profile": d.Profile, "synced_at": d.SyncedAt, "pages": len(d.Pages), "clicks": clicks, "sessions": sessions, "revenue": revenue, "top_money_page": topMoneyPage, "recommended_next_command": "traffic-intel-pp-cli movers --profile " + d.Profile}
		moverLine := "movers: no prior snapshot yet"
		if snaps, err := st(f).LatestSnapshots(f.profile, 2); err == nil && len(snaps) > 1 {
			movers := buildMovers(snaps[0], snaps[1], 5)
			digest["movers"] = map[string]any{"climbers": len(movers.Climbers), "droppers": len(movers.Droppers), "new_strike_zone_entrants": len(movers.NewStrikeZone), "new_revenue_at_risk": len(movers.NewRevenueAtRisk), "callouts": movers.Callouts}
			moverLine = fmt.Sprintf("movers: %d climbers, %d droppers, %d new Strike Zone entrants, %d new revenue-at-risk", len(movers.Climbers), len(movers.Droppers), len(movers.NewStrikeZone), len(movers.NewRevenueAtRisk))
		}
		if len(d.Pages) == 0 {
			digest["note"] = "no pages in local dataset; run sync --import with page metrics or sync without --import for the fixture"
		}
		return out(cmd, f, digest, fmt.Sprintf("Weekly digest for %s\nAct on what's already moving.\n%s\npages: %d\nclicks: %d\nsessions: %d\nrevenue: %.2f\ntop money page: %s\n", d.Profile, moverLine, len(d.Pages), clicks, sessions, revenue, topMoneyPage))
	}})
	return cmd
}

func pageOpportunityRow(p store.PageMetrics, rank int, score float64) map[string]any {
	return map[string]any{
		"rank":             rank,
		"url":              p.URL,
		"title":            p.Title,
		"query":            primaryTopic(p),
		"score":            score,
		"impressions":      p.Impressions,
		"clicks":           p.Clicks,
		"position":         p.Position,
		"ctr":              normalizedCTR(p.CTR),
		"expected_ctr":     expectedCTR(p.Position),
		"ctr_gap":          ctrGap(p),
		"revenue":          p.Revenue,
		"conversions":      p.Conversions,
		"ref_domains":      p.RefDomains,
		"recommended_next": "refresh title/meta, expand query coverage, and add internal links from related revenue pages",
	}
}

func coverageRow(p store.PageMetrics) map[string]any {
	gsc := hasGSC(p)
	ga4 := hasGA4(p)
	ahrefs := hasAhrefs(p)
	missing := []string{}
	if !gsc {
		missing = append(missing, "gsc")
	}
	if !ga4 {
		missing = append(missing, "ga4")
	}
	if !ahrefs {
		missing = append(missing, "ahrefs")
	}
	coverage := float64(3-len(missing)) / 3
	return map[string]any{
		"url":             p.URL,
		"title":           p.Title,
		"query":           primaryTopic(p),
		"gsc_present":     gsc,
		"ga4_present":     ga4,
		"ahrefs_present":  ahrefs,
		"coverage_score":  coverage,
		"missing_sources": missing,
		"next_action":     coverageAction(missing),
	}
}

func hasGSC(p store.PageMetrics) bool {
	return p.Clicks != 0 || p.Impressions != 0 || p.Position != 0 || p.Sources.GSC.QuerySample != ""
}

func hasGA4(p store.PageMetrics) bool {
	return p.Sessions != 0 || p.Conversions != 0 || p.Revenue != 0 || p.PreviousSessions != 0 || p.PreviousRevenue != 0
}

func hasAhrefs(p store.PageMetrics) bool {
	return p.Backlinks != 0 || p.RefDomains != 0 || p.Sources.Ahrefs.TopKeyword != ""
}

func coverageAction(missing []string) string {
	if len(missing) == 0 {
		return "coverage complete"
	}
	return "sync " + strings.Join(missing, ",") + " evidence before relying on combined prioritization"
}

func internalLinkPlan(pages []store.PageMetrics) []map[string]any {
	groups := topicGroups(pages, false)
	rows := []map[string]any{}
	for topic, ps := range groups {
		if len(ps) < 2 {
			continue
		}
		targets := append([]store.PageMetrics(nil), ps...)
		sort.Slice(targets, func(i, j int) bool {
			return opportunityScore(targets[i])+revenueRiskScore(targets[i]) > opportunityScore(targets[j])+revenueRiskScore(targets[j])
		})
		sources := append([]store.PageMetrics(nil), ps...)
		sort.Slice(sources, func(i, j int) bool {
			return linkSourceScore(sources[i]) > linkSourceScore(sources[j])
		})
		for _, target := range targets {
			if opportunityScore(target)+revenueRiskScore(target) <= 0 {
				continue
			}
			for _, source := range sources {
				if source.URL == target.URL {
					continue
				}
				rows = append(rows, map[string]any{
					"topic":    topic,
					"from_url": source.URL,
					"to_url":   target.URL,
					"anchor":   linkAnchor(target),
					"score":    linkSourceScore(source) + opportunityScore(target) + revenueRiskScore(target),
					"reason":   "same topic; source has traffic/link equity and target has upside or revenue risk",
				})
				break
			}
		}
	}
	if len(rows) == 0 {
		rows = fallbackInternalLinks(pages)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["score"].(float64) > rows[j]["score"].(float64) })
	return rows
}

func fallbackInternalLinks(pages []store.PageMetrics) []map[string]any {
	if len(pages) < 2 {
		return nil
	}
	sources := append([]store.PageMetrics(nil), pages...)
	targets := append([]store.PageMetrics(nil), pages...)
	sort.Slice(sources, func(i, j int) bool { return linkSourceScore(sources[i]) > linkSourceScore(sources[j]) })
	sort.Slice(targets, func(i, j int) bool {
		return opportunityScore(targets[i])+revenueRiskScore(targets[i]) > opportunityScore(targets[j])+revenueRiskScore(targets[j])
	})
	rows := []map[string]any{}
	for _, target := range targets {
		for _, source := range sources {
			if source.URL == target.URL {
				continue
			}
			rows = append(rows, map[string]any{
				"topic":    clusterTopic(target),
				"from_url": source.URL,
				"to_url":   target.URL,
				"anchor":   linkAnchor(target),
				"score":    linkSourceScore(source) + opportunityScore(target) + revenueRiskScore(target),
				"reason":   "best available source page has traffic/link equity and target has upside or risk",
			})
			break
		}
		if len(rows) >= 10 {
			break
		}
	}
	return rows
}

func linkSourceScore(p store.PageMetrics) float64 {
	return float64(p.Clicks) + p.Revenue/25 + float64(p.RefDomains*10) + float64(p.Sessions)/5
}

func linkAnchor(p store.PageMetrics) string {
	return firstNonEmpty(primaryTopic(p), normalizeTopic(p.Title), strings.Trim(strings.ReplaceAll(p.URL, "-", " "), "/"))
}

func experimentPlan(p store.PageMetrics) map[string]any {
	query := primaryTopic(p)
	return map[string]any{
		"url":                    p.URL,
		"title":                  p.Title,
		"query":                  query,
		"likely_issue":           likelyIssue(p),
		"primary_success_metric": experimentMetric(p),
		"title_tests": []string{
			fmt.Sprintf("Lead with %q and a concrete product/category benefit", query),
			"Test a comparison or buying-guide angle for higher-intent searchers",
			"Add freshness or availability language if the page serves seasonal demand",
		},
		"meta_tests": []string{
			"Mirror the primary query and mention the strongest conversion promise",
			"Add shipping, returns, proof, or selection details that reduce click uncertainty",
		},
		"content_tests": []string{
			"Move the clearest answer or product path above the fold",
			"Add an FAQ or comparison section for secondary query intent",
			"Add internal links from related high-authority pages",
		},
		"measurement": map[string]any{
			"baseline_clicks":     p.Clicks,
			"baseline_ctr":        normalizedCTR(p.CTR),
			"baseline_revenue":    p.Revenue,
			"minimum_runtime":     "14 days or one full weekly cycle",
			"guardrail":           "do not change URL while Ahrefs ref_domains/backlinks are present",
			"follow_up_commands":  []string{"traffic-intel-pp-cli forecast-impact --profile <profile>", "traffic-intel-pp-cli refresh-brief --profile <profile> " + strconv.Quote(p.URL)},
			"expected_click_gain": forecastRow(p, 1)["estimated_click_gain"],
		},
	}
}

func experimentMetric(p store.PageMetrics) string {
	switch {
	case p.Revenue > 0:
		return "organic revenue and CTR"
	case p.Conversions > 0:
		return "organic conversions and CTR"
	case p.Impressions > 0:
		return "CTR and clicks"
	default:
		return "sessions and engagement"
	}
}

func forecastScore(p store.PageMetrics) float64 {
	row := forecastRow(p, 0)
	return row["estimated_revenue_gain"].(float64) + row["estimated_click_gain"].(float64)
}

func forecastRow(p store.PageMetrics, rank int) map[string]any {
	gap := ctrGap(p)
	clickGain := float64(p.Impressions) * gap
	revenuePerClick := 0.0
	if p.Clicks > 0 {
		revenuePerClick = p.Revenue / float64(p.Clicks)
	} else if p.Sessions > 0 {
		revenuePerClick = p.Revenue / float64(p.Sessions)
	}
	conversionsPerClick := 0.0
	if p.Clicks > 0 {
		conversionsPerClick = float64(p.Conversions) / float64(p.Clicks)
	}
	return map[string]any{
		"rank":                       rank,
		"url":                        p.URL,
		"title":                      p.Title,
		"query":                      primaryTopic(p),
		"impressions":                p.Impressions,
		"position":                   p.Position,
		"current_ctr":                normalizedCTR(p.CTR),
		"expected_ctr":               expectedCTR(p.Position),
		"ctr_gap":                    gap,
		"estimated_click_gain":       clickGain,
		"revenue_per_click":          revenuePerClick,
		"estimated_revenue_gain":     clickGain * revenuePerClick,
		"estimated_conversion_gain":  clickGain * conversionsPerClick,
		"assumption":                 "forecast assumes current impressions and value per click hold while CTR closes to the position-based expected curve",
		"recommended_validation_cmd": "traffic-intel-pp-cli experiment-plan --profile <profile> " + strconv.Quote(p.URL),
	}
}

func staleWinnerScore(p store.PageMetrics) float64 {
	value := businessValue(p)
	if value <= 0 {
		return 0
	}
	decayPenalty := float64(max(0, p.PreviousClicks-p.Clicks))*4 + maxf(0, p.PreviousRevenue-p.Revenue)
	preventiveBoost := 100.0
	if decayPenalty > 0 {
		preventiveBoost = 25
	}
	return value + float64(p.Impressions)*0.01 + float64(staleDays(p))*2 + preventiveBoost - decayPenalty*0.1
}

func staleDays(p store.PageMetrics) int {
	if p.UpdatedAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, p.UpdatedAt)
	if err != nil {
		return 0
	}
	days := int(time.Since(t).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func staleWinnerAction(p store.PageMetrics) string {
	switch {
	case staleDays(p) > 60:
		return "refresh examples, proof points, and internal links because the dataset timestamp is old"
	case ctrGap(p) > 0:
		return "preempt decay by testing title/meta and adding links while the page still has value"
	case p.RefDomains > 0:
		return "preserve URL and update content without disrupting backlink equity"
	default:
		return "schedule a light content refresh and re-sync after the next reporting cycle"
	}
}

func opportunityScore(p store.PageMetrics) float64 {
	if p.Impressions <= 0 || p.Position < 4 || p.Position > 20 {
		return 0
	}
	gap := ctrGap(p)
	if gap <= 0 {
		return 0
	}
	return float64(p.Impressions)*gap*(1+businessValue(p)/1000) + float64(p.RefDomains*5)
}

func quickWinScore(p store.PageMetrics) float64 {
	if p.Impressions <= 0 || p.Position < 3 || p.Position > 15 {
		return 0
	}
	gap := ctrGap(p)
	if gap <= 0 {
		return 0
	}
	value := p.Revenue + float64(p.Conversions*100)
	if value <= 0 {
		value = float64(p.Sessions) * 0.5
	}
	return float64(p.Impressions)*gap + value/10 + float64(p.RefDomains)
}

func revenueRiskScore(p store.PageMetrics) float64 {
	lostClicks := max(0, p.PreviousClicks-p.Clicks)
	lostSessions := max(0, p.PreviousSessions-p.Sessions)
	lostRevenue := maxf(0, p.PreviousRevenue-p.Revenue)
	if lostClicks == 0 && lostSessions == 0 && lostRevenue == 0 {
		return 0
	}
	value := p.Revenue + p.PreviousRevenue + float64(p.Conversions*100)
	if value == 0 {
		value = float64(p.Sessions+p.PreviousSessions) * 0.25
	}
	return lostRevenue + float64(lostClicks*5) + float64(lostSessions*2) + value*0.2 + float64(p.RefDomains*10)
}

func businessValue(p store.PageMetrics) float64 {
	return p.Revenue + float64(p.Conversions*125) + float64(p.Sessions)*0.25 + float64(p.RefDomains*15)
}

func normalizedCTR(ctr float64) float64 {
	if ctr > 1 {
		return ctr / 100
	}
	if ctr < 0 {
		return 0
	}
	return ctr
}

func ctrGap(p store.PageMetrics) float64 {
	return maxf(0, expectedCTR(p.Position)-normalizedCTR(p.CTR))
}

func expectedCTR(position float64) float64 {
	switch {
	case position <= 0:
		return 0
	case position <= 1:
		return 0.28
	case position <= 2:
		return 0.15
	case position <= 3:
		return 0.10
	case position <= 4:
		return 0.07
	case position <= 5:
		return 0.05
	case position <= 10:
		return 0.03
	case position <= 20:
		return 0.015
	default:
		return 0.005
	}
}

func findPage(pages []store.PageMetrics, q string) (store.PageMetrics, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return store.PageMetrics{}, false
	}
	key := pageKey(q)
	for _, p := range pages {
		if strings.ToLower(p.URL) == q || pageKey(p.URL) == key {
			return p, true
		}
	}
	for _, p := range pages {
		if pageMatches(p, q) {
			return p, true
		}
	}
	return store.PageMetrics{}, false
}

func pageMatches(p store.PageMetrics, q string) bool {
	hay := strings.ToLower(strings.Join([]string{p.URL, p.Title, p.Sources.GSC.QuerySample, p.Sources.Ahrefs.TopKeyword}, " "))
	return strings.Contains(hay, strings.ToLower(strings.TrimSpace(q)))
}

func refreshBrief(p store.PageMetrics) map[string]any {
	actions := refreshActions(p)
	return map[string]any{
		"url":          p.URL,
		"title":        p.Title,
		"query":        primaryTopic(p),
		"likely_issue": likelyIssue(p),
		"metrics": map[string]any{
			"clicks":            p.Clicks,
			"previous_clicks":   p.PreviousClicks,
			"click_delta":       p.Clicks - p.PreviousClicks,
			"impressions":       p.Impressions,
			"position":          p.Position,
			"ctr":               normalizedCTR(p.CTR),
			"expected_ctr":      expectedCTR(p.Position),
			"sessions":          p.Sessions,
			"previous_sessions": p.PreviousSessions,
			"revenue":           p.Revenue,
			"previous_revenue":  p.PreviousRevenue,
			"revenue_delta":     p.Revenue - p.PreviousRevenue,
			"conversions":       p.Conversions,
			"ref_domains":       p.RefDomains,
			"backlinks":         p.Backlinks,
		},
		"recommended_actions": actions,
		"suggested_commands": []string{
			"traffic-intel-pp-cli refresh-queue --profile <profile>",
			"traffic-intel-pp-cli opportunity-gap --profile <profile>",
			"traffic-intel-pp-cli query-revenue --profile <profile> " + strconv.Quote(primaryTopic(p)),
		},
	}
}

func refreshActions(p store.PageMetrics) []string {
	actions := []string{}
	if p.Position >= 4 && p.Position <= 20 {
		actions = append(actions, "tighten title/H1 around the primary query and expand the section that answers search intent")
	}
	if ctrGap(p) > 0 {
		actions = append(actions, "test a higher-intent title/meta description because CTR trails the expected curve")
	}
	if p.Clicks < p.PreviousClicks {
		actions = append(actions, "inspect lost GSC queries and restore sections that matched those intents")
	}
	if p.Revenue < p.PreviousRevenue {
		actions = append(actions, "audit product cards, offer clarity, and conversion path because revenue declined")
	}
	if p.RefDomains > 0 {
		actions = append(actions, "preserve the URL and add internal links from related pages to protect backlink equity")
	}
	if len(actions) == 0 {
		actions = append(actions, "refresh examples, add internal links, and re-check query coverage after the next sync")
	}
	return actions
}

func firstAction(brief map[string]any) string {
	actions, _ := brief["recommended_actions"].([]string)
	if len(actions) == 0 {
		return ""
	}
	return actions[0]
}

func likelyIssue(p store.PageMetrics) string {
	switch {
	case p.Clicks < p.PreviousClicks && p.Position > 5:
		return "organic ranking or CTR decline"
	case p.Sessions < p.PreviousSessions:
		return "analytics session decline"
	case p.Revenue < p.PreviousRevenue:
		return "conversion or order-value decline"
	case ctrGap(p) > 0:
		return "SERP CTR underperformance"
	default:
		return "maintenance refresh opportunity"
	}
}

func riskAction(p store.PageMetrics) string {
	switch {
	case p.Revenue < p.PreviousRevenue:
		return "refresh commercial intent and audit conversion path"
	case p.Clicks < p.PreviousClicks:
		return "recover lost queries and strengthen internal links"
	case p.Sessions < p.PreviousSessions:
		return "compare acquisition paths and check analytics landing page changes"
	default:
		return "monitor after next sync"
	}
}

func clusterAction(lostClicks int, lostRevenue, revenue float64) string {
	switch {
	case lostRevenue > 0:
		return "refresh revenue pages and inspect conversion path"
	case lostClicks > 0:
		return "recover search intent coverage and strengthen internal links"
	case revenue > 0:
		return "protect high-value pages with periodic content and link refreshes"
	default:
		return "expand pages with validated query demand"
	}
}

func topicGroups(pages []store.PageMetrics, preferQuery bool) map[string][]store.PageMetrics {
	groups := map[string][]store.PageMetrics{}
	for _, p := range pages {
		topic := ""
		if preferQuery {
			topic = normalizeTopic(firstNonEmpty(p.Sources.GSC.QuerySample, p.Sources.Ahrefs.TopKeyword))
		}
		if topic == "" {
			topic = clusterTopic(p)
		}
		groups[topic] = append(groups[topic], p)
	}
	return groups
}

func clusterTopic(p store.PageMetrics) string {
	if topic := normalizeTopic(firstNonEmpty(p.Sources.GSC.QuerySample, p.Sources.Ahrefs.TopKeyword, p.Title)); topic != "" {
		return topic
	}
	if u, err := url.Parse(p.URL); err == nil {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for _, part := range parts {
			if topic := normalizeTopic(part); topic != "" {
				return topic
			}
		}
	}
	return "uncategorized"
}

func primaryTopic(p store.PageMetrics) string {
	return firstNonEmpty(p.Sources.GSC.QuerySample, p.Sources.Ahrefs.TopKeyword, normalizeTopic(p.Title), p.URL)
}

func normalizeTopic(s string) string {
	tokens := topicTokens(s)
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) > 3 {
		tokens = tokens[:3]
	}
	return strings.Join(tokens, " ")
}

func topicTokens(s string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "best": true, "for": true, "from": true, "how": true, "of": true, "or": true, "page": true, "pages": true, "the": true, "to": true, "with": true,
	}
	parts := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := []string{}
	for _, part := range parts {
		if part == "" || stop[part] {
			continue
		}
		out = append(out, part)
	}
	return out
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
