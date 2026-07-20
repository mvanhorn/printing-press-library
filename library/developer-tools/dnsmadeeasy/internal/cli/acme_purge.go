// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: find _acme-challenge TXT records across all zones and delete
// all matches or only those older than a requested age. Previews by default;
// --apply performs the deletes.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"

	"github.com/spf13/cobra"
)

type acmePurgeItem struct {
	Zone      string `json:"zone"`
	DomainID  string `json:"domain_id"`
	RecordID  string `json:"record_id"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	FirstSeen string `json:"first_seen,omitempty"`
	Status    string `json:"status,omitempty"`
}

type acmePurgeView struct {
	OlderThan      string          `json:"older_than,omitempty"`
	Applied        bool            `json:"applied"`
	Matched        int             `json:"matched"`
	ZonesAffected  int             `json:"zones_affected"`
	Targets        []acmePurgeItem `json:"targets"`
	Deleted        int             `json:"deleted,omitempty"`
	ZonesSucceeded int             `json:"zones_succeeded,omitempty"`
	FetchFailures  []struct {
		Zone  string `json:"zone"`
		Error string `json:"error"`
	} `json:"fetch_failures,omitempty"`
	Note string `json:"note,omitempty"`
}

func newNovelAcmePurgeCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var flagApply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "acme-purge",
		Short: "Find and delete _acme-challenge TXT records across all zones",
		Long: strings.Trim(`
Find every _acme-challenge TXT record across all zones and delete them via the
real multi-record delete endpoint. Previews by default; pass --apply to delete.

--older-than uses the local snapshot history (populated by 'sync-records') to
keep only challenge records first seen before the cutoff. Without a mirror,
--older-than cannot be honored and no records are deleted; run without
--older-than to target all challenge records.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli acme-purge --older-than 24h --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				note := "dry-run: would fetch _acme-challenge TXT records across all zones and preview deletions — no API calls made; re-run without --dry-run for the live plan, then add --apply to delete"
				if flagOlderThan != "" {
					note = fmt.Sprintf("dry-run: would fetch _acme-challenge TXT records across all zones and preview deleting those older than %s — no API calls made; re-run without --dry-run for the live plan, then add --apply to delete", flagOlderThan)
				}
				return emitAcme(cmd, flags, acmePurgeView{OlderThan: flagOlderThan, Targets: []acmePurgeItem{}, Note: note})
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would preview _acme-challenge TXT records")
				return nil
			}
			var cutoff time.Time
			if flagOlderThan != "" {
				d, err := cliutil.ParseDurationLoose(flagOlderThan)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --older-than %q: %w", flagOlderThan, err))
				}
				cutoff = time.Now().Add(-d)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() {
				view := acmePurgeView{OlderThan: flagOlderThan, Targets: []acmePurgeItem{}, Note: "dogfood mode: skipped account-wide fetch"}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			records, _, failures, err := fetchAllZoneRecords(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Optional first-seen lookup from the local snapshot mirror.
			firstSeen := map[string]time.Time{}
			if flagOlderThan != "" {
				if dbPath == "" {
					dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
				}
				fs, ok := loadFirstSeen(ctx, dbPath)
				if !ok {
					return usageErr(fmt.Errorf("--older-than needs local snapshot history; run 'dnsmadeeasy-pp-cli sync-records' first, or re-run without --older-than"))
				}
				firstSeen = fs
			}

			byZone := map[string][]string{}
			zoneName := map[string]string{}
			targets := []acmePurgeItem{}
			for _, r := range records {
				if !strings.EqualFold(r.Type, "TXT") {
					continue
				}
				if !strings.HasPrefix(strings.ToLower(r.Name), "_acme-challenge") {
					continue
				}
				key := r.DomainID + "/" + r.ID.String()
				item := acmePurgeItem{Zone: r.DomainName, DomainID: r.DomainID, RecordID: r.ID.String(), Name: r.Name, Value: r.Value}
				if flagOlderThan != "" {
					seen, ok := firstSeen[key]
					if !ok || seen.After(cutoff) {
						continue // never seen in history, or seen too recently
					}
					item.FirstSeen = seen.UTC().Format(time.RFC3339)
				}
				targets = append(targets, item)
				byZone[r.DomainID] = append(byZone[r.DomainID], r.ID.String())
				zoneName[r.DomainID] = r.DomainName
			}

			view := acmePurgeView{OlderThan: flagOlderThan, Applied: flagApply, Matched: len(targets), ZonesAffected: len(byZone), Targets: targets}
			for zone, msg := range failures {
				view.FetchFailures = append(view.FetchFailures, struct {
					Zone  string `json:"zone"`
					Error string `json:"error"`
				}{Zone: zone, Error: msg})
			}
			if len(targets) == 0 {
				view.Note = "no matching _acme-challenge TXT records found"
				return emitAcme(cmd, flags, view)
			}
			if !flagApply {
				view.Note = "preview only — re-run with --apply to delete these records"
				return emitAcme(cmd, flags, view)
			}

			deleted := 0
			succeededZones := 0
			deleteFailures := 0
			deletedZoneIDs := map[string]bool{}
			for domID, ids := range byZone {
				q := url.Values{}
				for _, id := range ids {
					q.Add("ids", id)
				}
				path := "/dns/managed/" + domID + "/records?" + q.Encode()
				if _, _, err := c.Delete(ctx, path); err != nil {
					deleteFailures++
					view.FetchFailures = append(view.FetchFailures, struct {
						Zone  string `json:"zone"`
						Error string `json:"error"`
					}{Zone: zoneName[domID], Error: err.Error()})
					continue
				}
				deleted += len(ids)
				succeededZones++
				deletedZoneIDs[domID] = true
			}
			for i := range view.Targets {
				view.Targets[i].Status = "failed"
				if deletedZoneIDs[view.Targets[i].DomainID] {
					view.Targets[i].Status = "deleted"
				}
			}
			view.Deleted = deleted
			view.ZonesAffected = succeededZones
			view.ZonesSucceeded = succeededZones
			view.Note = acmeDeleteSummary(deleted, succeededZones, deleteFailures)
			return emitAcme(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "", "Only purge records first seen before this age (e.g. 24h, 7d); needs sync-records history")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Actually delete the records (default previews only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database (for --older-than history)")
	return cmd
}

func acmeDeleteSummary(deleted, succeededZones, failedZones int) string {
	summary := fmt.Sprintf("deleted %d record(s) across %d zone(s)", deleted, succeededZones)
	if failedZones > 0 {
		summary += fmt.Sprintf("; %d zone(s) failed", failedZones)
	}
	return summary
}

func emitAcme(cmd *cobra.Command, flags *rootFlags, view acmePurgeView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(view.Targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		return nil
	}
	for _, t := range view.Targets {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s %s (%s)\n", acmeTargetVerb(view.Applied, t.Status), t.Zone, t.Name, t.RecordID)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d record(s) across %d zone(s). %s\n", view.Matched, view.ZonesAffected, view.Note)
	for _, failure := range view.FetchFailures {
		fmt.Fprintf(cmd.OutOrStdout(), "FAILED  %s: %s\n", failure.Zone, failure.Error)
	}
	return nil
}

func acmeTargetVerb(applied bool, status string) string {
	if !applied {
		return "WOULD DELETE"
	}
	if status == "deleted" {
		return "DELETED"
	}
	return "FAILED"
}

// loadFirstSeen returns the earliest snapshot time per "domainId/recordId"
// from the local mirror. ok=false when the mirror/table is unavailable.
func loadFirstSeen(ctx context.Context, dbPath string) (map[string]time.Time, bool) {
	s, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, false
	}
	defer s.Close()
	rows, err := s.DB().QueryContext(ctx, `SELECT domain_id, record_id, MIN(taken_at) FROM record_snapshots GROUP BY domain_id, record_id`)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var dom, rec, taken string
		if err := rows.Scan(&dom, &rec, &taken); err != nil {
			return nil, false
		}
		ts, err := time.Parse(time.RFC3339Nano, taken)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, taken)
			if err != nil {
				continue
			}
		}
		out[dom+"/"+rec] = ts
	}
	if rows.Err() != nil {
		return nil, false
	}
	return out, true
}
