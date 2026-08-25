// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Implementations for the approved Cosmos-specific analysis commands.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/cliutil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func runCosmosReview(cmd *cobra.Command, flags *rootFlags, sinceRaw string) error {
	since, err := timeFromRelative(sinceRaw, time.Now())
	if err != nil {
		return err
	}
	uid, err := cosmosUserID(flags, cmd)
	if err != nil {
		return err
	}
	data, err := cosmosCaptured(flags, cmd, newAllsElementsCmd, map[string]any{
		"userId": uid, "filters": nil, "order": nil,
		"pageCursor": nil, "pageSize": 100, "isLoggedIn": true,
	})
	if err != nil {
		return err
	}
	raw := mapsAt(data, "allElementsV2", "items")
	queue := make([]map[string]any, 0)
	mediaOwners := map[string][]string{}
	for _, item := range raw {
		element := normalizeElement(item)
		if created, ok := element["created_at"].(string); ok {
			if parsed, parseErr := time.Parse(time.RFC3339, created); parseErr == nil && parsed.Before(since) {
				continue
			}
		}
		reasons := make([]string, 0, 4)
		if mapString(element, "source_url") == "" {
			reasons = append(reasons, "missing_source_url")
		}
		if mapString(element, "source_author") == "" {
			reasons = append(reasons, "missing_source_author")
		}
		if generated, _ := element["ai_generated"].(bool); generated {
			reasons = append(reasons, "ai_generated")
		}
		if count, ok := element["connections_count"].(float64); ok && count == 0 {
			reasons = append(reasons, "unfiled")
		}
		if media := mapString(element, "media_url"); media != "" {
			mediaOwners[media] = append(mediaOwners[media], idString(element["id"]))
		}
		if len(reasons) > 0 {
			queue = append(queue, map[string]any{"element": element, "reasons": reasons})
		}
	}
	duplicateGroups := make([]map[string]any, 0)
	for media, ids := range mediaOwners {
		if len(ids) > 1 {
			duplicateGroups = append(duplicateGroups, map[string]any{"media_url": media, "element_ids": ids, "count": len(ids)})
		}
	}
	sort.Slice(duplicateGroups, func(i, j int) bool {
		return fmt.Sprint(duplicateGroups[i]["media_url"]) < fmt.Sprint(duplicateGroups[j]["media_url"])
	})
	return printCosmos(cmd, flags, map[string]any{
		"since": since.UTC().Format(time.RFC3339), "reviewed": len(raw), "queue": queue,
		"queue_count": len(queue), "duplicate_media": duplicateGroups,
	})
}

func runCosmosCollectionOverlap(cmd *cobra.Command, flags *rootFlags, args []string) error {
	if len(args) != 2 {
		return usageErr(fmt.Errorf("overlap requires left and right collection IDs"))
	}
	leftID, err := parseID(args[0], "left collection ID")
	if err != nil {
		return err
	}
	rightID, err := parseID(args[1], "right collection ID")
	if err != nil {
		return err
	}
	left, err := cosmosCollectionElements(flags, cmd, leftID, 100)
	if err != nil {
		return err
	}
	right, err := cosmosCollectionElements(flags, cmd, rightID, 100)
	if err != nil {
		return err
	}
	leftByID, rightByID := elementIndex(left), elementIndex(right)
	shared, leftOnly, rightOnly := make([]map[string]any, 0), make([]map[string]any, 0), make([]map[string]any, 0)
	for id, item := range leftByID {
		if _, ok := rightByID[id]; ok {
			shared = append(shared, item)
		} else {
			leftOnly = append(leftOnly, item)
		}
	}
	for id, item := range rightByID {
		if _, ok := leftByID[id]; !ok {
			rightOnly = append(rightOnly, item)
		}
	}
	duplicateMedia := crossCollectionDuplicateMedia(left, right)
	sortElements(shared)
	sortElements(leftOnly)
	sortElements(rightOnly)
	union := len(leftByID) + len(rightByID) - len(shared)
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(len(shared)) / float64(union)
	}
	return printCosmos(cmd, flags, map[string]any{
		"left_collection_id": leftID, "right_collection_id": rightID,
		"shared": shared, "left_only": leftOnly, "right_only": rightOnly,
		"duplicate_media": duplicateMedia, "jaccard_similarity": jaccard,
		"counts": map[string]any{"left": len(left), "right": len(right), "shared": len(shared), "left_only": len(leftOnly), "right_only": len(rightOnly)},
	})
}

func runCosmosCollectionCoverage(cmd *cobra.Command, flags *rootFlags, collectionRaw, query, limitRaw string) error {
	if strings.TrimSpace(collectionRaw) == "" || strings.TrimSpace(query) == "" {
		return usageErr(fmt.Errorf("--collection and --query are required"))
	}
	collectionID, err := parseID(collectionRaw, "collection ID")
	if err != nil {
		return err
	}
	limit, err := strconv.Atoi(limitRaw)
	if err != nil {
		return usageErr(fmt.Errorf("--limit must be a number"))
	}
	if _, err := boundedLimit(limit, 40); err != nil {
		return err
	}
	existing, err := cosmosCollectionElements(flags, cmd, collectionID, 100)
	if err != nil {
		return err
	}
	data, err := cosmosCaptured(flags, cmd, newGlobalsElementsCmd, map[string]any{
		"userId": nil, "searchTerm": query, "contentType": nil, "origin": nil,
		"pageCursor": nil, "order": nil, "color": nil,
	})
	if err != nil {
		return err
	}
	searchRaw := mapsAt(data, "searchElements", "items")
	if len(searchRaw) > limit {
		searchRaw = searchRaw[:limit]
	}
	existingIDs := elementIndex(existing)
	existingMedia := map[string]bool{}
	existingSources := map[string]bool{}
	for _, item := range existing {
		existingMedia[mapString(item, "media_url")] = true
		existingSources[mapString(item, "source_url")] = true
	}
	candidates, alreadySaved, mediaDuplicates := make([]map[string]any, 0), make([]map[string]any, 0), make([]map[string]any, 0)
	for _, raw := range searchRaw {
		item := normalizeElement(raw)
		if _, ok := existingIDs[idString(item["id"])]; ok {
			alreadySaved = append(alreadySaved, item)
			continue
		}
		media := mapString(item, "media_url")
		source := mapString(item, "source_url")
		if (media != "" && existingMedia[media]) || (source != "" && existingSources[source]) {
			mediaDuplicates = append(mediaDuplicates, item)
			continue
		}
		candidates = append(candidates, item)
	}
	coverage := 0.0
	if len(searchRaw) > 0 {
		coverage = float64(len(alreadySaved)+len(mediaDuplicates)) / float64(len(searchRaw))
	}
	return printCosmos(cmd, flags, map[string]any{
		"collection_id": collectionID, "query": query, "coverage_ratio": coverage,
		"promising_unsaved": candidates, "already_saved": alreadySaved, "duplicate_media_or_source": mediaDuplicates,
		"counts": map[string]any{"searched": len(searchRaw), "promising_unsaved": len(candidates), "already_saved": len(alreadySaved), "duplicate_media_or_source": len(mediaDuplicates)},
	})
}

func runCosmosProvenanceAudit(cmd *cobra.Command, flags *rootFlags, collectionRaw string) error {
	if strings.TrimSpace(collectionRaw) == "" {
		return usageErr(fmt.Errorf("--collection is required"))
	}
	collectionID, err := parseID(collectionRaw, "collection ID")
	if err != nil {
		return err
	}
	items, err := cosmosCollectionElements(flags, cmd, collectionID, 100)
	if err != nil {
		return err
	}
	missingURL, missingAuthor := make([]map[string]any, 0), make([]map[string]any, 0)
	hostCounts := map[string]int{}
	for _, item := range items {
		source := mapString(item, "source_url")
		if source == "" {
			missingURL = append(missingURL, item)
		} else if parsed, parseErr := urlHost(source); parseErr == nil && parsed != "" {
			hostCounts[parsed]++
		}
		if mapString(item, "source_author") == "" {
			missingAuthor = append(missingAuthor, item)
		}
	}
	type hostCount struct {
		Host  string  `json:"host"`
		Count int     `json:"count"`
		Share float64 `json:"share"`
	}
	concentration := make([]hostCount, 0, len(hostCounts))
	for host, count := range hostCounts {
		share := 0.0
		if len(items) > 0 {
			share = float64(count) / float64(len(items))
		}
		concentration = append(concentration, hostCount{Host: host, Count: count, Share: share})
	}
	sort.Slice(concentration, func(i, j int) bool {
		if concentration[i].Count == concentration[j].Count {
			return concentration[i].Host < concentration[j].Host
		}
		return concentration[i].Count > concentration[j].Count
	})
	return printCosmos(cmd, flags, map[string]any{
		"collection_id": collectionID, "elements": len(items), "missing_source_url": missingURL,
		"missing_source_author": missingAuthor, "source_concentration": concentration,
		"counts": map[string]any{"missing_source_url": len(missingURL), "missing_source_author": len(missingAuthor), "distinct_source_hosts": len(hostCounts)},
	})
}

func urlHost(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www.")), nil
}

func runCosmosElementTrail(cmd *cobra.Command, flags *rootFlags, idRaw, depthRaw, limitRaw string) error {
	if strings.TrimSpace(idRaw) == "" {
		return usageErr(fmt.Errorf("--id is required"))
	}
	rootID, err := parseID(idRaw, "element ID")
	if err != nil {
		return err
	}
	depth, err := strconv.Atoi(depthRaw)
	if err != nil || depth < 1 || depth > 3 {
		return usageErr(fmt.Errorf("--depth must be between 1 and 3"))
	}
	limit, err := strconv.Atoi(limitRaw)
	if err != nil {
		return usageErr(fmt.Errorf("--limit must be a number"))
	}
	if _, err := boundedLimit(limit, 40); err != nil {
		return err
	}
	if cliutil.IsVerifyEnv() {
		depth = min(depth, 1)
		limit = min(limit, 3)
	}
	type frontierNode struct {
		ID    int64
		Depth int
	}
	frontier := []frontierNode{{ID: rootID, Depth: 0}}
	visited := map[string]bool{idString(rootID): true}
	nodes := map[string]map[string]any{idString(rootID): {"id": rootID, "depth": 0}}
	edges := make([]map[string]any, 0)
	truncated := false
	const maxNodes = 100
	const maxExpansions = 8
	expansions := 0
	for len(frontier) > 0 {
		current := frontier[0]
		frontier = frontier[1:]
		if current.Depth >= depth {
			continue
		}
		if expansions >= maxExpansions {
			truncated = true
			break
		}
		expansions++
		data, err := cosmosCaptured(flags, cmd, newSimilarsPromotedCmd, map[string]any{
			"userId": nil, "elementIds": []any{current.ID}, "isLoggedIn": false, "isAdmin": false,
			"pageCursor": nil, "pageSize": limit,
		})
		if err != nil {
			return err
		}
		items := mapsAt(data, "similarElementsV2", "items")
		if len(items) > limit {
			items = items[:limit]
		}
		for _, raw := range items {
			item := normalizeElement(raw)
			childID := idString(item["id"])
			if childID == "" || childID == "<nil>" {
				continue
			}
			edges = append(edges, map[string]any{"from": current.ID, "to": item["id"], "depth": current.Depth + 1})
			if visited[childID] {
				continue
			}
			if len(visited) >= maxNodes {
				truncated = true
				continue
			}
			visited[childID] = true
			item["depth"] = current.Depth + 1
			nodes[childID] = item
			if nextID, parseErr := strconv.ParseInt(childID, 10, 64); parseErr == nil {
				frontier = append(frontier, frontierNode{ID: nextID, Depth: current.Depth + 1})
			}
		}
	}
	nodeList := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, node)
	}
	sortElements(nodeList)
	return printCosmos(cmd, flags, map[string]any{"root_id": rootID, "depth": depth, "nodes": nodeList, "edges": edges, "node_count": len(nodeList), "edge_count": len(edges), "expansions": expansions, "truncated": truncated})
}

func runCosmosSnapshotDiff(cmd *cobra.Command, flags *rootFlags, fromRaw, toRaw string) error {
	now := time.Now()
	fromTarget, err := timeFromRelative(fromRaw, now)
	if err != nil {
		return err
	}
	toTarget, err := timeFromRelative(toRaw, now)
	if err != nil {
		return err
	}
	snapshots, err := loadCosmosSnapshots(flags, fromTarget, toTarget)
	if err != nil {
		return err
	}
	if len(snapshots) < 2 {
		return printCosmos(cmd, flags, map[string]any{
			"status":      "needs_snapshots",
			"next_action": "run 'cosmos-pp-cli sync --full' at two different times",
			"added":       []any{},
			"removed":     []any{},
			"moved":       []any{},
			"counts":      map[string]any{"added": 0, "removed": 0, "moved": 0},
		})
	}
	from := nearestSnapshot(snapshots, fromTarget)
	to := nearestSnapshot(snapshots, toTarget)
	if from.CapturedAt.Equal(to.CapturedAt) {
		return usageErr(fmt.Errorf("--from and --to resolve to the same snapshot (%s); capture another sync or choose a wider interval", from.CapturedAt.Format(time.RFC3339)))
	}
	if from.CapturedAt.After(to.CapturedAt) {
		from, to = to, from
	}
	if from.AccountID == "" || to.AccountID == "" {
		return usageErr(fmt.Errorf("selected snapshots predate account-scoped history; run sync twice with the current Cosmos version"))
	}
	if from.AccountID != to.AccountID {
		return usageErr(fmt.Errorf("refusing to compare snapshots from different Cosmos accounts"))
	}
	fromMembership, fromItems := snapshotMembership(from)
	toMembership, toItems := snapshotMembership(to)
	added, removed, moved := make([]map[string]any, 0), make([]map[string]any, 0), make([]map[string]any, 0)
	for id, collections := range toMembership {
		before, ok := fromMembership[id]
		if !ok {
			added = append(added, map[string]any{"element": toItems[id], "collections": sortedKeys(collections)})
			continue
		}
		if !sameStringSet(before, collections) {
			moved = append(moved, map[string]any{"element": toItems[id], "from_collections": sortedKeys(before), "to_collections": sortedKeys(collections)})
		}
	}
	for id, collections := range fromMembership {
		if _, ok := toMembership[id]; !ok {
			removed = append(removed, map[string]any{"element": fromItems[id], "collections": sortedKeys(collections)})
		}
	}
	return printCosmos(cmd, flags, map[string]any{
		"account_id": from.AccountID, "from": from.CapturedAt.Format(time.RFC3339), "to": to.CapturedAt.Format(time.RFC3339),
		"added": added, "removed": removed, "moved": moved,
		"counts": map[string]any{"added": len(added), "removed": len(removed), "moved": len(moved)},
	})
}

func elementIndex(items []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any, len(items))
	for _, item := range items {
		result[idString(item["id"])] = item
	}
	return result
}

func crossCollectionDuplicateMedia(left, right []map[string]any) []map[string]any {
	leftMedia := map[string][]string{}
	for _, item := range left {
		if media := mapString(item, "media_url"); media != "" {
			leftMedia[media] = append(leftMedia[media], idString(item["id"]))
		}
	}
	result := make([]map[string]any, 0)
	for _, item := range right {
		media := mapString(item, "media_url")
		if ids := leftMedia[media]; media != "" && len(ids) > 0 {
			result = append(result, map[string]any{"media_url": media, "left_element_ids": ids, "right_element_id": item["id"]})
		}
	}
	return result
}

func sortElements(items []map[string]any) {
	sort.Slice(items, func(i, j int) bool { return idString(items[i]["id"]) < idString(items[j]["id"]) })
}

func nearestSnapshot(snapshots []cosmosSnapshot, target time.Time) cosmosSnapshot {
	best := snapshots[0]
	bestDistance := absDuration(best.CapturedAt.Sub(target))
	for _, candidate := range snapshots[1:] {
		distance := absDuration(candidate.CapturedAt.Sub(target))
		if distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	return best
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func snapshotMembership(snapshot cosmosSnapshot) (map[string]map[string]bool, map[string]map[string]any) {
	membership := map[string]map[string]bool{}
	items := map[string]map[string]any{}
	for collectionID, elements := range snapshot.Elements {
		for _, element := range elements {
			id := idString(element["id"])
			if membership[id] == nil {
				membership[id] = map[string]bool{}
			}
			membership[id][collectionID] = true
			items[id] = element
		}
	}
	return membership, items
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func sameStringSet(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}
