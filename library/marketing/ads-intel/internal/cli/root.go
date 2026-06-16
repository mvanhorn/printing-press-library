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

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
	"github.com/mvanhorn/printing-press-library/library/marketing/ads-intel/internal/store"
	"github.com/spf13/cobra"
)

const version = "0.1.0-private"

type rootFlags struct {
	asJSON, compact, noInput, yes, noColor, agent bool
	profile, home                                 string
}

func Execute() error { return NewRootCmd().Execute() }
func NewRootCmd() *cobra.Command {
	f := &rootFlags{}
	cmd := &cobra.Command{Use: "ads-intel-pp-cli", Short: "Read-only paid media intelligence CLI", SilenceUsage: true, Version: version}
	cmd.PersistentFlags().BoolVar(&f.asJSON, "json", false, "Output JSON")
	cmd.PersistentFlags().BoolVar(&f.compact, "compact", false, "Compact output")
	cmd.PersistentFlags().BoolVar(&f.noInput, "no-input", false, "Disable prompts")
	cmd.PersistentFlags().BoolVar(&f.yes, "yes", false, "Assume yes for safe read-only operations")
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
	cmd.AddCommand(agentContextCmd(f), doctorCmd(f), sourcesCmd(f), profileCmd(f), syncCmd(f), accountStatusCmd(f), confidenceCmd(f), auditCmd(f), quickWinsCmd(f), budgetShiftCmd(f))
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
		ctx := map[string]any{"schema_version": "ads-intel.agent-context/v1", "name": "ads-intel-pp-cli", "version": version, "local_first": true, "read_only": true, "external_api_calls": false, "state_dir": f.home, "commands": []string{"sync", "account-status", "confidence", "audit", "quick-wins", "budget-shift"}, "source_plan": sourceChecks()}
		return out(cmd, f, ctx, "")
	}}
}

func doctorCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local readiness", RunE: func(cmd *cobra.Command, args []string) error {
		ps, _ := st(f).ListProfiles()
		res := map[string]any{"ok": true, "version": version, "state_dir": f.home, "profiles": len(ps), "sources": sourceChecks()}
		return out(cmd, f, res, fmt.Sprintf("ads-intel-pp-cli %s\nstate: %s\nprofiles: %d\n", version, f.home, len(ps)))
	}}
}

type sourceCheck struct {
	Name           string `json:"name"`
	ChildBinary    string `json:"child_binary"`
	Path           string `json:"path,omitempty"`
	Found          bool   `json:"found"`
	PlannedCommand string `json:"planned_command"`
	CapabilityNote string `json:"capability_note"`
}

func sourceChecks() []sourceCheck {
	defs := []sourceCheck{
		{Name: "google", ChildBinary: "google-ads-pp-cli", PlannedCommand: "google-ads-pp-cli sync --agent / google-ads-pp-cli wasted-spend --agent / keyword-roi-tiers --agent", CapabilityNote: "read APIs exist; generic mutate surfaces also exist but ads-intel v1 never calls them"},
		{Name: "meta", ChildBinary: "meta-ads-pp-cli", PlannedCommand: "meta-ads-pp-cli sync --agent / insights get-account <id> --agent / fatigue --agent / learning --agent", CapabilityNote: "read-heavy insights and local analytics are available"},
		{Name: "amazon", ChildBinary: "amazon-ads-pp-cli", PlannedCommand: "amazon-ads-pp-cli sync --agent / reports download --agent / weekly-review --agent", CapabilityNote: "read reports plus many write surfaces exist; ads-intel v1 uses only read/local outputs"},
	}
	for i := range defs {
		if p, err := exec.LookPath(defs[i].ChildBinary); err == nil {
			defs[i].Found = true
			defs[i].Path = p
		}
	}
	return defs
}

func sourcesCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sources", Short: "Inspect source adapters"}
	cmd.AddCommand(&cobra.Command{Use: "doctor", Short: "Check paid child CLI readiness", RunE: func(cmd *cobra.Command, args []string) error {
		return out(cmd, f, sourceChecks(), "")
	}})
	return cmd
}

func profileCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage profiles"}
	var name, account, brand string
	save := &cobra.Command{Use: "save", Short: "Save a profile", RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			name = f.profile
		}
		p := store.Profile{Name: name, AccountID: account, Brand: brand}
		if err := st(f).SaveProfile(p); err != nil {
			return err
		}
		return out(cmd, f, p, fmt.Sprintf("saved profile %s\n", name))
	}}
	save.Flags().StringVar(&name, "name", "", "Profile name")
	save.Flags().StringVar(&account, "account", "", "Primary account/CID")
	save.Flags().StringVar(&brand, "brand", "", "Brand term")
	cmd.AddCommand(save)
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List profiles", RunE: func(cmd *cobra.Command, args []string) error {
		ps, err := st(f).ListProfiles()
		if err != nil {
			return err
		}
		return out(cmd, f, ps, fmt.Sprintf("profiles: %d\n", len(ps)))
	}})
	return cmd
}

func syncCmd(f *rootFlags) *cobra.Command {
	var importPath string
	c := &cobra.Command{Use: "sync", Short: "Load fixture or local JSON paid media data", RunE: func(cmd *cobra.Command, args []string) error {
		var d store.DataSet
		if importPath == "" {
			d = store.Fixture(f.profile)
		} else {
			b, err := os.ReadFile(importPath)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &d); err != nil {
				return err
			}
			d.Provenance.InputHashes = intelcli.MergeStringMaps(d.Provenance.InputHashes, map[string]string{"import_file": intelcli.HashBytes(b)})
		}
		ensureProvenance(&d)
		if err := st(f).SaveData(d); err != nil {
			return err
		}
		return out(cmd, f, withHeader(f, d, "sync", map[string]any{"profile": d.Profile, "campaigns": len(d.Campaigns), "search_terms": len(d.SearchTerms), "source": d.Source}), fmt.Sprintf("synced %d campaigns for %s\n", len(d.Campaigns), d.Profile))
	}}
	c.Flags().StringVar(&importPath, "import", "", "Import DataSet JSON")
	return c
}

func ensureProvenance(d *store.DataSet) {
	if d.Profile == "" {
		d.Profile = "default"
	}
	if d.SyncedAt.IsZero() {
		d.SyncedAt = time.Now().UTC()
	}
	if d.Source == "" {
		d.Source = "local-import"
	}
	if d.Provenance.SchemaVersion == "" {
		d.Provenance.SchemaVersion = "ads-intel.provenance/v1"
	}
	if d.Provenance.DateRange.StartDate == "" {
		start, end := intelcli.DefaultDateRange("", "")
		d.Provenance.DateRange = store.DateRange{StartDate: start, EndDate: end}
	}
	d.Provenance.SourceCommandVersions = intelcli.MergeStringMaps(d.Provenance.SourceCommandVersions, map[string]string{"ads-intel-pp-cli": version})
	if len(d.Provenance.InputHashes) == 0 {
		d.Provenance.InputHashes = map[string]string{"dataset": intelcli.HashJSON(d.Campaigns)}
	}
}

func load(f *rootFlags) (store.DataSet, error) {
	d, err := st(f).LoadData(f.profile)
	if err != nil {
		return d, fmt.Errorf("no local data for profile %q: run sync first", f.profile)
	}
	return d, nil
}

func withHeader(f *rootFlags, d store.DataSet, command string, body map[string]any) map[string]any {
	body["account_status"] = accountHeader(d, command)
	return body
}

func accountHeader(d store.DataSet, command string) map[string]any {
	return map[string]any{"profile": d.Profile, "command": command, "account_id": d.Account.AccountID, "account_status": d.Account.Status, "date_range_used": d.Provenance.DateRange, "tracking_confidence": confidenceForData(d), "mode": "read-only"}
}

func confidenceForData(d store.DataSet) string {
	if len(d.Campaigns) == 0 || totalSpendAll(d) == 0 {
		return "Broken"
	}
	hasCost, hasConv, hasRevenue := false, false, false
	for _, c := range d.Campaigns {
		hasCost = hasCost || c.Spend > 0
		hasConv = hasConv || c.Conversions > 0
		hasRevenue = hasRevenue || c.Revenue > 0
	}
	if hasCost && hasConv && hasRevenue {
		return "High"
	}
	if hasCost && (hasConv || hasRevenue) {
		return "Medium"
	}
	return "Low"
}

func accountStatusCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "account-status", Short: "Show account status header", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		return out(cmd, f, accountHeader(d, "account-status"), fmt.Sprintf("account: %s status=%s confidence=%s mode=read-only\n", d.Account.AccountID, d.Account.Status, confidenceForData(d)))
	}}
}

func confidenceCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "confidence", Short: "Show tracking confidence", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		res := withHeader(f, d, "confidence", map[string]any{"tracking_confidence": confidenceForData(d), "checks": []string{"cost rows present", "conversion signal present", "revenue signal present", "learning phase guarded"}})
		return out(cmd, f, res, fmt.Sprintf("tracking confidence: %s\n", confidenceForData(d)))
	}}
}

func auditCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "audit", Short: "Run read-only structural audit", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		audit, err := runAudit(d, f.home)
		if err != nil {
			return err
		}
		res := withHeader(f, d, "audit", map[string]any{"audit": audit})
		lines := []string{fmt.Sprintf("audit: %s score=%.1f", audit.Status, audit.Score)}
		for _, finding := range audit.Findings {
			lines = append(lines, formatFinding(finding))
		}
		return out(cmd, f, res, strings.Join(lines, "\n")+"\n")
	}}
}

func quickWinsCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "quick-wins", Short: "High-severity fixes under 15 minutes", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		audit, err := runAudit(d, f.home)
		if err != nil {
			return err
		}
		return out(cmd, f, withHeader(f, d, "quick-wins", map[string]any{"quick_wins": audit.QuickWins}), fmt.Sprintf("quick wins: %d\n", len(audit.QuickWins)))
	}}
}

func budgetShiftCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "budget-shift", Short: "Read-only cross-channel budget shift view", RunE: func(cmd *cobra.Command, args []string) error {
		d, err := load(f)
		if err != nil {
			return err
		}
		rows := []map[string]any{}
		for platform, spend := range spendByPlatform(d) {
			revenue := 0.0
			for _, c := range d.Campaigns {
				if c.Platform == platform {
					revenue += c.Revenue
				}
			}
			rows = append(rows, map[string]any{"platform": platform, "spend": spend, "revenue": revenue, "roas": percent(revenue, spend), "view": "read-only"})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i]["roas"].(float64) > rows[j]["roas"].(float64) })
		return out(cmd, f, withHeader(f, d, "budget-shift", map[string]any{"rows": rows, "note": "Read-only view; no budget changes are recommended during active learning."}), fmt.Sprintf("budget-shift rows: %d\n", len(rows)))
	}}
}
