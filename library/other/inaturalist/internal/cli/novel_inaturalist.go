// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/inaturalist/internal/cliutil"

	"github.com/spf13/cobra"
)

type speciesCount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CommonName  string `json:"common_name,omitempty"`
	IconicTaxon string `json:"iconic_taxon,omitempty"`
	Count       int    `json:"count"`
}

type identificationSnapshot struct {
	ID             string `json:"id"`
	Taxon          string `json:"taxon,omitempty"`
	CommunityTaxon string `json:"community_taxon,omitempty"`
	Quality        string `json:"quality_grade,omitempty"`
	Geoprivacy     string `json:"geoprivacy,omitempty"`
	Obscured       bool   `json:"obscured"`
}

func novelGet(cmd *cobra.Command, flags *rootFlags, path string, params map[string]string) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode iNaturalist response: %w", err)
	}
	return out, nil
}

func requireNovelValue(flags *rootFlags, value, name string) error {
	if flags != nil && flags.dryRun {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return usageErr(fmt.Errorf("--%s is required", name))
	}
	return nil
}

func dateSince(raw string, now time.Time) (string, error) {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(raw), "d"))
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 || days > 3650 {
		return "", usageErr(fmt.Errorf("--since must be a whole-day duration from 1d through 3650d"))
	}
	return now.AddDate(0, 0, -days).Format("2006-01-02"), nil
}

func resultsOf(response map[string]any) []map[string]any {
	items, _ := response["results"].([]any)
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			results = append(results, object)
		}
	}
	return results
}

func valueString(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}

func valueID(object map[string]any, key string) string {
	switch value := object[key].(type) {
	case string:
		return value
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		return ""
	}
}

func valueInt(object map[string]any, key string) int {
	value, _ := object[key].(float64)
	return int(value)
}

func speciesCounts(response map[string]any) []speciesCount {
	counts := make([]speciesCount, 0)
	for _, item := range resultsOf(response) {
		taxon, _ := item["taxon"].(map[string]any)
		if taxon == nil {
			continue
		}
		name := valueString(taxon, "preferred_common_name")
		if name == "" {
			name = valueString(taxon, "name")
		}
		counts = append(counts, speciesCount{ID: valueID(taxon, "id"), Name: name, CommonName: valueString(taxon, "preferred_common_name"), IconicTaxon: valueString(taxon, "iconic_taxon_name"), Count: valueInt(item, "count")})
	}
	return counts
}

func dryRunNovel(cmd *cobra.Command, flags *rootFlags, command string) (bool, error) {
	if flags == nil || !flags.dryRun {
		return false, nil
	}
	return true, flags.printJSON(cmd, map[string]any{"dry_run": true, "command": command, "privacy": "coordinates are never emitted"})
}

func runNearbyHighlights(cmd *cobra.Command, flags *rootFlags, lat, lng, radius string) error {
	if dry, err := dryRunNovel(cmd, flags, "nearby highlights"); dry || err != nil {
		return err
	}
	if err := requireNovelValue(flags, lat, "lat"); err != nil {
		return err
	}
	if err := requireNovelValue(flags, lng, "lng"); err != nil {
		return err
	}
	counts, err := novelGet(cmd, flags, "/observations/species_counts", map[string]string{"lat": lat, "lng": lng, "radius": radius, "per_page": "20", "order": "desc"})
	if err != nil {
		return err
	}
	recent, err := novelGet(cmd, flags, "/observations", map[string]string{"lat": lat, "lng": lng, "radius": radius, "per_page": "30", "order_by": "observed_on", "order": "desc"})
	if err != nil {
		return err
	}
	items := speciesCounts(counts)
	return flags.printJSON(cmd, map[string]any{"area": "supplied radius", "radius_km": radius, "highlights": items, "recent_observations_considered": len(resultsOf(recent)), "ranking": "species counts from the supplied area, paired with a bounded recent-observations check", "privacy": map[string]any{"coordinates_emitted": false, "preserves_geoprivacy_labels": true}})
}

func runHuntCreate(cmd *cobra.Command, flags *rootFlags, placeID, iconicTaxa string, limit int) error {
	if dry, err := dryRunNovel(cmd, flags, "hunt create"); dry || err != nil {
		return err
	}
	if err := requireNovelValue(flags, placeID, "place-id"); err != nil {
		return err
	}
	if limit < 1 || limit > 50 {
		return usageErr(fmt.Errorf("--limit must be between 1 and 50"))
	}
	response, err := novelGet(cmd, flags, "/observations/species_counts", map[string]string{"place_id": placeID, "iconic_taxa": iconicTaxa, "per_page": "200", "order": "desc"})
	if err != nil {
		return err
	}
	seenByGroup := map[string]int{}
	items := make([]speciesCount, 0, limit)
	for _, candidate := range speciesCounts(response) {
		group := candidate.IconicTaxon
		if group == "" {
			group = "other"
		}
		if seenByGroup[group] >= 2 {
			continue
		}
		seenByGroup[group]++
		items = append(items, candidate)
		if len(items) == limit {
			break
		}
	}
	return flags.printJSON(cmd, map[string]any{"place_id": placeID, "hunt": items, "selection_rule": "up to two observed taxa per iconic group, sorted by official species-count results", "privacy": "taxon-only checklist; no observation locations"})
}

func runSeasonalShift(cmd *cobra.Command, flags *rootFlags, placeID string, recentDays, baselineDays int) error {
	if dry, err := dryRunNovel(cmd, flags, "nearby seasonal-shift"); dry || err != nil {
		return err
	}
	if err := requireNovelValue(flags, placeID, "place-id"); err != nil {
		return err
	}
	if recentDays < 1 || recentDays > 365 || baselineDays < 1 || baselineDays > 365 {
		return usageErr(fmt.Errorf("--recent-days and --baseline-days must each be between 1 and 365"))
	}
	now := time.Now().UTC()
	recentStart := now.AddDate(0, 0, -recentDays)
	baselineStart := recentStart.AddDate(0, 0, -baselineDays)
	countsFor := func(start, end time.Time) ([]speciesCount, error) {
		response, err := novelGet(cmd, flags, "/observations/species_counts", map[string]string{
			"place_id": placeID,
			"d1":       start.Format("2006-01-02"),
			"d2":       end.Format("2006-01-02"),
			"per_page": "200",
			"order":    "desc",
		})
		if err != nil {
			return nil, err
		}
		return speciesCounts(response), nil
	}
	recent, err := countsFor(recentStart, now)
	if err != nil {
		return err
	}
	baseline, err := countsFor(baselineStart, recentStart)
	if err != nil {
		return err
	}
	baselineByID := make(map[string]speciesCount, len(baseline))
	for _, item := range baseline {
		baselineByID[item.ID] = item
	}
	changes := make([]map[string]any, 0, len(recent))
	for _, item := range recent {
		prior := baselineByID[item.ID]
		change := item.Count - prior.Count
		if change == 0 {
			continue
		}
		changes = append(changes, map[string]any{
			"taxon_id":       item.ID,
			"name":           item.Name,
			"iconic_taxon":   item.IconicTaxon,
			"recent_count":   item.Count,
			"baseline_count": prior.Count,
			"change":         change,
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		left, right := changes[i]["change"].(int), changes[j]["change"].(int)
		if left < 0 {
			left = -left
		}
		if right < 0 {
			right = -right
		}
		return left > right
	})
	return flags.printJSON(cmd, map[string]any{
		"place_id":        placeID,
		"recent_window":   map[string]string{"start": recentStart.Format("2006-01-02"), "end": now.Format("2006-01-02")},
		"baseline_window": map[string]string{"start": baselineStart.Format("2006-01-02"), "end": recentStart.Format("2006-01-02")},
		"shifts":          changes,
		"privacy":         "taxon counts only; no coordinates or place geometry are emitted",
	})
}

func observationSnapshots(response map[string]any) []identificationSnapshot {
	snapshots := make([]identificationSnapshot, 0)
	for _, item := range resultsOf(response) {
		taxon, _ := item["taxon"].(map[string]any)
		community, _ := item["community_taxon"].(map[string]any)
		if community == nil {
			community = taxon
		}
		obscured, _ := item["obscured"].(bool)
		snapshots = append(snapshots, identificationSnapshot{ID: valueID(item, "id"), Taxon: valueString(taxon, "name"), CommunityTaxon: valueString(community, "name"), Quality: valueString(item, "quality_grade"), Geoprivacy: valueString(item, "geoprivacy"), Obscured: obscured})
	}
	return snapshots
}

func getUserObservations(cmd *cobra.Command, flags *rootFlags, user, since string) ([]identificationSnapshot, error) {
	if err := requireNovelValue(flags, user, "user"); err != nil {
		return nil, err
	}
	d1, err := dateSince(since, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	response, err := novelGet(cmd, flags, "/observations", map[string]string{"user_login": user, "d1": d1, "per_page": "200", "order_by": "updated_at", "order": "desc"})
	if err != nil {
		return nil, err
	}
	return observationSnapshots(response), nil
}

func runIdentificationStatus(cmd *cobra.Command, flags *rootFlags, user, since string) error {
	if dry, err := dryRunNovel(cmd, flags, "observations id-status"); dry || err != nil {
		return err
	}
	snapshots, err := getUserObservations(cmd, flags, user, since)
	if err != nil {
		return err
	}
	buckets := map[string]int{"identified": 0, "needs_id": 0, "no_taxon": 0, "disagreement_or_uncertain": 0}
	for _, item := range snapshots {
		switch {
		case item.Taxon == "":
			buckets["no_taxon"]++
		case item.Quality == "needs_id":
			buckets["needs_id"]++
		case item.CommunityTaxon != "" && item.CommunityTaxon != item.Taxon:
			buckets["disagreement_or_uncertain"]++
		default:
			buckets["identified"]++
		}
	}
	return flags.printJSON(cmd, map[string]any{"user": user, "observations": len(snapshots), "status": buckets, "privacy": "no coordinates or private-place fields"})
}

func snapshotPath(user string) (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(user))))
	path := filepath.Join(dir, "identification-snapshots", fmt.Sprintf("%x.json", digest[:8]))
	return path, nil
}

func loadSnapshot(path string) ([]identificationSnapshot, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snapshots []identificationSnapshot
	return snapshots, json.Unmarshal(data, &snapshots)
}

func saveSnapshot(path string, snapshots []identificationSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(snapshots)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(path), data, 0o600)
}

func runIdentificationChanges(cmd *cobra.Command, flags *rootFlags, user, since string) error {
	if dry, err := dryRunNovel(cmd, flags, "observations id-changes"); dry || err != nil {
		return err
	}
	current, err := getUserObservations(cmd, flags, user, since)
	if err != nil {
		return err
	}
	path, err := snapshotPath(user)
	if err != nil {
		return err
	}
	previous, err := loadSnapshot(path)
	if err != nil {
		return fmt.Errorf("read identification snapshot: %w", err)
	}
	priorByID := make(map[string]identificationSnapshot, len(previous))
	for _, item := range previous {
		priorByID[item.ID] = item
	}
	changes := make([]map[string]string, 0)
	for _, item := range current {
		prior, ok := priorByID[item.ID]
		if !ok || prior.CommunityTaxon != item.CommunityTaxon || prior.Taxon != item.Taxon || prior.Quality != item.Quality {
			changes = append(changes, map[string]string{"id": item.ID, "from_taxon": prior.CommunityTaxon, "to_taxon": item.CommunityTaxon, "quality_grade": item.Quality})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i]["id"] < changes[j]["id"] })
	if err := saveSnapshot(path, current); err != nil {
		return fmt.Errorf("save identification snapshot: %w", err)
	}
	return flags.printJSON(cmd, map[string]any{"user": user, "baseline_created": previous == nil, "changes": changes, "snapshot_count": len(current), "privacy": "only IDs, taxon state, quality, geoprivacy labels, and obscured state are persisted; coordinates are never stored"})
}
