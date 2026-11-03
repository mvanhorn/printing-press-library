// Catalog snapshot analysis; local evidence never implies live catalog completeness.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govcache"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, name := range []string{"overlap", "changes", "coverage", "intake"} {
			for _, c := range root.Commands() {
				if c.Name() == name {
					root.RemoveCommand(c)
				}
			}
		}
		root.AddCommand(newGovLocalCmd(flags, "overlap"), newGovLocalCmd(flags, "changes"), newGovLocalCmd(flags, "coverage"))
		parent := &cobra.Command{Use: "intake", Short: "Audit saved catalog metadata and preserve selected resource provenance", Example: "  government-data-pp-cli intake --dry-run --json", RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "intake")
			}
			return parentNoSubcommandRunE(flags)(cmd, args)
		}}
		parent.AddCommand(newGovLocalCmd(flags, "audit"), newGovLocalCmd(flags, "lock"))
		root.AddCommand(parent)
		for _, c := range root.Commands() {
			if c.Name() == "catalog" {
				c.AddCommand(newGovSnapshotsCmd(flags))
			}
		}
	})
}
func newGovLocalCmd(flags *rootFlags, kind string) *cobra.Command {
	var dbPath, snapshot, from, to, output string
	var limit int
	var selected []string
	path := kind
	if kind == "audit" || kind == "lock" {
		path = "intake " + kind
	}
	descriptions := map[string]string{"overlap": "Find exact resource URLs shared across saved catalog records", "changes": "Compare two explicit saved snapshots without claiming dataset deletion", "coverage": "Compare declared formats within an explicit saved topic scope", "audit": "List missing catalog and resource metadata fields in a saved snapshot", "lock": "Write a provenance manifest for explicitly selected saved resources"}
	annotations := map[string]string{"pp:data-source": "local", "pp:verified-by-proof": "catalog-local-lifecycle.md", "pp:happy-args": "--dry-run"}
	if kind != "lock" {
		annotations["mcp:read-only"] = "true"
	} else {
		annotations["mcp:local-write"] = "true"
	}
	cmd := &cobra.Command{Use: kind, Short: descriptions[kind], Annotations: annotations, Example: "  government-data-pp-cli " + path + " --snapshot water-week1 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, path)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("%s takes flags, not positional arguments", path))
		}
		if limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("--limit must be 1..10000"))
		}
		if kind == "changes" {
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("changes requires --from and --to saved snapshot IDs; list them with catalog snapshots"))
			}
		} else if snapshot == "" {
			return usageErr(fmt.Errorf("%s requires --snapshot ID; first run catalog find water --source both --save water-week1", path))
		}
		if kind == "lock" && (len(selected) == 0 || output == "") {
			return usageErr(fmt.Errorf("intake lock requires --resource source:dataset-id:resource-id (repeatable) and --output path (or - for stdout)"))
		}
		if dbPath == "" {
			dbPath = defaultDBPath("government-data-pp-cli")
		}
		db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
		if err != nil {
			return fmt.Errorf("open saved catalog evidence: %w; create a snapshot with catalog find --save ID", err)
		}
		defer db.Close()
		hintCatalogStore(cmd, db, flags)
		load := func(id string) (govdata.Snapshot, error) {
			s, err := govcache.Load(cmd.Context(), db.DB(), id)
			if err != nil {
				return s, notFoundErr(err)
			}
			hintCatalogSnapshot(cmd, s, flags.maxAge)
			return s, nil
		}
		if kind == "changes" {
			a, e := load(from)
			if e != nil {
				return e
			}
			b, e := load(to)
			if e != nil {
				return e
			}
			all := govcache.Diff(a, b)
			n := len(all)
			if len(all) > limit {
				all = all[:limit]
			}
			return flags.printJSON(cmd, map[string]any{"from": scopeHeader(a), "to": scopeHeader(b), "results": all, "total_changes": n, "truncated": n > limit, "interpretation": "Snapshot membership differences do not prove live dataset deletion. Different query scopes may produce different membership."})
		}
		s, err := load(snapshot)
		if err != nil {
			return err
		}
		var results any
		total := 0
		switch kind {
		case "overlap":
			all := govcache.Overlap(s)
			total = len(all)
			if total > limit {
				all = all[:limit]
			}
			results = all
		case "coverage":
			all := govcache.Coverage(s)
			total = len(all)
			if total > limit {
				all = all[:limit]
			}
			results = all
		case "audit":
			all := govcache.Audit(s)
			total = len(all)
			if total > limit {
				all = all[:limit]
			}
			results = all
		case "lock":
			locked, e := govcache.Lock(s, selected)
			if e != nil {
				return usageErr(e)
			}
			manifest := map[string]any{"schema_version": 1, "snapshot": scopeHeader(s), "results": locked, "fingerprint_kind": "sha256-canonical-metadata", "note": "Metadata fingerprints do not verify downloaded file bytes."}
			if output == "-" {
				return flags.printJSON(cmd, manifest)
			}
			b, e := json.MarshalIndent(manifest, "", "  ")
			if e != nil {
				return e
			}
			f, e := os.CreateTemp(filepath.Dir(output), ".intake-*.tmp")
			if e != nil {
				return e
			}
			temp := f.Name()
			defer os.Remove(temp)
			_, writeErr := f.Write(append(b, '\n'))
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
			if e = os.Link(temp, output); e != nil {
				return fmt.Errorf("create --output without overwrite: %w", e)
			}

			return flags.printJSON(cmd, map[string]any{"output": output, "selected_resources": len(locked), "snapshot": s.ID})
		}
		return flags.printJSON(cmd, map[string]any{"snapshot": scopeHeader(s), "results": results, "total": total, "truncated": total > limit, "scope_notice": "Results describe only this saved catalog snapshot, not the full live catalog."})
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite file containing saved catalog snapshots")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum result rows to print (1..10000)")
	if kind == "changes" {
		cmd.Flags().StringVar(&from, "from", "", "Earlier saved snapshot ID to compare")
		cmd.Flags().StringVar(&to, "to", "", "Later saved snapshot ID to compare")
		cmd.Example = "  government-data-pp-cli changes --from water-week1 --to water-week2 --agent"
	} else {
		cmd.Flags().StringVar(&snapshot, "snapshot", "", "Explicit saved catalog snapshot ID to analyze")
	}
	if kind == "lock" {
		cmd.Flags().StringSliceVar(&selected, "resource", nil, "Selected source:dataset-id:resource-id keys (repeatable)")
		cmd.Flags().StringVar(&output, "output", "", "New manifest destination; use - to print without writing a file")
		cmd.Example = "  government-data-pp-cli intake lock --snapshot water-week1 --resource california:a47fde7d-7071-4f89-bd95-d8912ec8c385:resource-id --output intake.json --json"
	}
	switch kind {
	case "overlap":
		cmd.Long = "Use this command to find shared resource URLs across saved catalog records. Do NOT use it to compare records over time; use 'changes' instead."
	case "changes":
		cmd.Long = "Use this command to compare two saved metadata snapshots over time. Do NOT use it to identify cross-catalog resource overlap; use 'overlap' instead."
	case "coverage":
		cmd.Long = "Use this command to compare format availability within a saved topic result set. Do NOT use it to inspect individual metadata omissions; use 'intake audit' instead."
	case "audit":
		cmd.Long = "Use this command to find missing metadata before selecting resources. Do NOT use it to preserve a selected intake manifest; use 'intake lock' instead."
	case "lock":
		cmd.Long = "Use this command to preserve the selected resources and their catalog provenance. Do NOT use it to diagnose missing metadata; use 'intake audit' instead."
	}
	return cmd
}
func scopeHeader(s govdata.Snapshot) map[string]any {
	v := map[string]any{"id": s.ID, "query": s.Query, "source": s.Source, "captured_at": s.CapturedAt, "complete": s.Complete, "failures": s.Failures, "dataset_count": len(s.Datasets)}
	v["scope"] = s.Scope
	return v
}

func newGovSnapshotsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{Use: "snapshots", Short: "List saved catalog snapshot IDs and their capture scopes", Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"}, Example: "  government-data-pp-cli catalog snapshots --limit 10 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "catalog snapshots")
		}
		if len(args) > 0 || limit < 1 || limit > 10000 {
			return usageErr(fmt.Errorf("catalog snapshots takes no positionals and --limit must be 1..10000"))
		}
		if dbPath == "" {
			dbPath = defaultDBPath("government-data-pp-cli")
		}
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return flags.printJSON(cmd, map[string]any{"results": []any{}, "total": 0, "note": "No saved catalog snapshots yet; create one with catalog find --save ID."})
		}

		db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
		if err != nil {
			return notFoundErr(fmt.Errorf("open saved snapshots: %w; create one using catalog find --save ID", err))
		}
		defer db.Close()
		hintCatalogStore(cmd, db, flags)
		all, err := govcache.List(cmd.Context(), db.DB())
		if err != nil {
			if strings.Contains(err.Error(), "no such table: gov_snapshots") {
				return flags.printJSON(cmd, map[string]any{"results": []any{}, "total": 0, "note": "No saved catalog snapshots yet; create one with catalog find --save ID."})
			}
			return err
		}
		total := len(all)
		if total > limit {
			all = all[:limit]
		}
		return flags.printJSON(cmd, map[string]any{"results": all, "total": total, "truncated": total > limit})
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite file containing saved catalog snapshots")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum snapshot summaries to print (1..10000)")
	return cmd
}

// Custom snapshots carry their own capture time. Generated sync hints apply only
// when the generated store actually contains matching dataset sync metadata.
func hintCatalogStore(cmd *cobra.Command, db *store.Store, flags *rootFlags) {
	var count int
	if err := db.DB().QueryRowContext(cmd.Context(), `SELECT count(*) FROM sync_state WHERE resource_type='datasets'`).Scan(&count); err == nil && count > 0 {
		if !hintIfUnsynced(cmd, db, "datasets") {
			hintIfStale(cmd, db, "datasets", flags.maxAge)
		}
	}
}
func hintCatalogSnapshot(cmd *cobra.Command, s govdata.Snapshot, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, s.CapturedAt)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: snapshot %s has an unrecognized captured_at timestamp; verify its capture provenance.\n", s.ID)
		return
	}
	age := time.Since(at)
	if age > maxAge {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: snapshot %s was captured %s ago, older than --max-age=%s. Capture a new snapshot using catalog find --save NEW_ID to compare fresh evidence.\n", s.ID, age.Round(time.Second), maxAge)
	}
}
