// pp:data-source local
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/evidence"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/store"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

type sendfoxEvidenceOptions struct {
	input, dbPath, id, email, kit, previous, audience, outputDir string
	limit                                                        int
	window                                                       string
}

func configureSendfoxEvidenceCmd(cmd *cobra.Command, f *rootFlags, name string, o *sendfoxEvidenceOptions) *cobra.Command {
	short := cmd.Short
	cmd.Long = short + "\nReads explicit JSON/YAML (or Markdown YAML frontmatter) snapshots or the local SQLite mirror. No account access. Ready means local checks passed, never approval to send or migrate."
	cmd.Example = "  sendfox-pp-cli workflow " + name + " --input examples/snapshot.json --agent\n  sendfox-pp-cli workflow " + name + " --db ./sendfox.db --json"
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--input=examples/snapshot.json"}
	if name == "growth-report" {
		cmd.Annotations["pp:happy-args"] += ";--previous=examples/snapshot.json"
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "workflow "+name)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unexpected arguments; use --input, --db, --id or --email"))
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("workflow %s has no live equivalent; use --data-source local and supply evidence", name))
		}
		if o.limit < 1 {
			return usageErr(fmt.Errorf("--limit must be positive"))
		}
		if name == "snapshot-save" && strings.TrimSpace(o.outputDir) == "" {
			return usageErr(fmt.Errorf("--out is required for snapshot-save"))
		}
		growthWindow := time.Duration(0)
		if name == "growth-report" {
			parsedWindow, parseErr := cliutil.ParseDurationLoose(o.window)
			if parseErr != nil {
				return usageErr(fmt.Errorf("--window: %w", parseErr))
			}
			growthWindow = parsedWindow
			if growthWindow <= 0 {
				return usageErr(fmt.Errorf("--window must be positive"))
			}
		}
		if o.input != "" && o.dbPath != "" {
			return usageErr(fmt.Errorf("choose --input or --db, not both"))
		}
		var s evidence.Snapshot
		var err error
		if o.input != "" {
			s, err = evidence.Load(o.input)
		} else if o.dbPath != "" {
			s, err = loadSendfoxEvidenceStore(cmd, f, o.dbPath)
		} else {
			return usageErr(fmt.Errorf("--input or --db is required; see examples/snapshot.json"))
		}
		if err != nil {
			return usageErr(err)
		}
		if o.kit != "" {
			v, e := evidence.Load(o.kit)
			if e != nil {
				return usageErr(fmt.Errorf("--kit: %w", e))
			}
			s.Kit = &v
		}
		if o.previous != "" {
			v, e := evidence.Load(o.previous)
			if e != nil {
				return usageErr(fmt.Errorf("--previous: %w", e))
			}
			s.Previous = &v
		}
		var report evidence.Report
		now := time.Now().UTC()
		switch name {
		case "audience-health", "audience-map", "hygiene-report":
			report = evidence.AudienceHealth(s, now, f.maxAge)
		case "campaign-preflight", "launch-plan":
			report = evidence.CampaignPreflight(s, now, f.maxAge)
		case "campaign-review", "campaign-digest":
			report = evidence.CampaignReview(s, o.id, o.audience, now, f.maxAge)
		case "growth-report":
			report = evidence.GrowthReport(s, now, f.maxAge, growthWindow)
		case "contact-dossier":
			if o.id == "" && o.email == "" {
				return usageErr(fmt.Errorf("--id or --email is required for contact-dossier"))
			}
			if o.id != "" && o.email != "" {
				return usageErr(fmt.Errorf("choose --id or --email, not both"))
			}
			report = evidence.ContactDossier(s, o.id, o.email, now, f.maxAge)
		case "automation-plan":
			report = evidence.AutomationPlan(s, now, f.maxAge)
		case "export-bundle", "account-snapshot":
			report = evidence.ExportBundle(s, now, f.maxAge)
		case "snapshot-save":
			report = evidence.ExportBundle(s, now, f.maxAge)
			report.Workflow = "snapshot-save"
		case "migration-readiness":
			report = evidence.MigrationReadiness(s, now, f.maxAge)
		default:
			return fmt.Errorf("unknown evidence workflow %s", name)
		}
		if name == "snapshot-save" {
			artifact, writeErr := evidence.WriteSnapshotHistory(o.outputDir, s, now)
			if writeErr != nil {
				return fmt.Errorf("write durable snapshot history: %w", writeErr)
			}
			report.Data["history_artifact"] = artifact
		}
		truncation := map[string]int{}
		if len(report.Findings) > o.limit {
			truncation["findings"] = len(report.Findings)
			report.Findings = report.Findings[:o.limit]
		}
		for _, key := range []string{"eligible_observed", "exclusions", "matches", "activity", "links"} {
			if rows, ok := report.Data[key].([]evidence.Row); ok && len(rows) > o.limit {
				truncation[key] = len(rows)
				report.Data[key] = rows[:o.limit]
			}
		}
		if len(truncation) > 0 {
			report.Data["output_truncation"] = truncation
			report.Data["output_limit"] = o.limit
		}
		return printSendfoxEvidence(cmd, f, report)
	}
	if name == "contact-dossier" {
		cmd.Annotations["pp:happy-args"] += ";--id=1"
		cmd.Example = "  sendfox-pp-cli workflow contact-dossier --input examples/snapshot.json --id 1 --agent"
	}
	return cmd
}

// Plan bodies, exclusions and source hashes are evidence, not verbose list
// decoration. Keep them in agent output; --select and --limit bound the result.
func printSendfoxEvidence(cmd *cobra.Command, f *rootFlags, report evidence.Report) error {
	outputFlags := *f
	outputFlags.compact = false
	return outputFlags.printJSON(cmd, report)
}
func loadSendfoxEvidenceStore(cmd *cobra.Command, f *rootFlags, file string) (evidence.Snapshot, error) {
	s := evidence.Snapshot{SchemaVersion: 1, Resources: map[string]evidence.Resource{}}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Fprintln(cmd.ErrOrStderr(), "No local mirror; run sendfox-pp-cli sync --db "+file)
		return s, nil
	}
	ctx, cancel := boundCtx(cmd.Context(), f)
	defer cancel()
	db, err := store.OpenReadOnlyContext(ctx, file)
	if err != nil {
		return s, err
	}
	defer db.Close()
	hintIfUnsynced(cmd, db, "")
	hintIfStale(cmd, db, "", f.maxAge)
	rows, err := db.DB().QueryContext(ctx, `SELECT r.resource_type, r.data, COALESCE(s.last_synced_at,''), COALESCE(s.last_attempt_complete,0) FROM resources r LEFT JOIN sync_state s ON s.resource_type=r.resource_type ORDER BY r.resource_type, r.id`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var kind, raw, observed string
		var complete int
		if err := rows.Scan(&kind, &raw, &observed, &complete); err != nil {
			return s, errors.Join(err, rows.Close())
		}
		var row evidence.Row
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			return s, errors.Join(err, rows.Close())
		}
		kind = strings.ReplaceAll(kind, "-", "_")
		switch kind {
		case "contact_tags":
			kind = "tags"
		case "contacts_unsubscribed":
			kind = "unsubscribed"
		}
		r := s.Resources[kind]
		r.Items = append(r.Items, row)
		r.Source = "sqlite:" + file
		r.ObservedAt = observed
		if t, e := time.Parse("2006-01-02 15:04:05", observed); e == nil {
			r.ObservedAt = t.UTC().Format(time.RFC3339)
		}
		done := complete == 1
		r.Complete = &done
		s.Resources[kind] = r
	}
	if err := rows.Err(); err != nil {
		return s, errors.Join(err, rows.Close())
	}
	if err := rows.Close(); err != nil {
		return s, err
	}
	return s, nil
}
func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		parent, _, err := root.Find([]string{"workflow"})
		if err != nil {
			return
		}
		addNovelCommandIfAbsent(parent, newWorkflowAccountSnapshotCmd(f))
		addNovelCommandIfAbsent(parent, newWorkflowAudienceMapCmd(f))
		addNovelCommandIfAbsent(parent, newWorkflowHygieneReportCmd(f))
		addNovelCommandIfAbsent(parent, newWorkflowCampaignDigestCmd(f))
		addNovelCommandIfAbsent(parent, newWorkflowLaunchPlanCmd(f))
	})
}

func newSendfoxCompatibilityCmd(f *rootFlags, name string) *cobra.Command {
	o := &sendfoxEvidenceOptions{}
	cmd := &cobra.Command{Use: name, Short: "Compatibility path for the current local evidence workflow"}
	cmd.Flags().StringVar(&o.input, "input", "", "Version 1 local JSON/YAML evidence snapshot, or Markdown YAML frontmatter")
	cmd.Flags().StringVar(&o.dbPath, "db", "", "Local SQLite mirror created by sync; no network refresh")
	cmd.Flags().StringVar(&o.id, "id", "", "Contact or campaign ID to select from the supplied evidence")
	cmd.Flags().StringVar(&o.email, "email", "", "Exact normalized email for a single contact dossier")
	cmd.Flags().IntVar(&o.limit, "limit", 100, "Maximum findings, activity, links and recipient samples printed; counts remain complete")
	if name == "migration-readiness" {
		cmd.Flags().StringVar(&o.kit, "kit", "", "Normalized Kit JSON/YAML snapshot, including explicit suppression evidence")
		cmd.Flags().StringVar(&o.previous, "previous", "", "Previous Kit snapshot used to compute the late-change delta")
	}
	if name == "campaign-review" || name == "campaign-digest" {
		cmd.Flags().StringVar(&o.audience, "audience", "non_openers", "Documented engagement cohort: non_openers, openers or clickers")
	}
	if name == "growth-report" {
		cmd.Flags().StringVar(&o.previous, "previous", "", "Previous SendFox snapshot for exact observed list joins, leaves and retention")
		cmd.Flags().StringVar(&o.window, "window", "30d", "Single-snapshot timestamp window for new contacts and unsubscribes (for example 30d or 720h)")
	}
	cmd = configureSendfoxEvidenceCmd(cmd, f, name, o)
	cmd.Annotations["mcp:hidden"] = "true" // Canonical workflow tools provide the same capability without redundant aliases.
	return cmd
}
