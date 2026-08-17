// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelPlaygroundsImpactCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source auto
	var flagDir string
	fixtureAppUUID, _ := playgroundsFixtureIDs()

	cmd := &cobra.Command{
		Use:         "impact <app-uuid>",
		Short:       "See every space, channel, playlist, and screen that could display a reviewed Playgrounds change before publishing.",
		Example:     "  screencloud-pp-cli playgrounds impact 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --dir ./campaign-playground --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<app-uuid>=" + fixtureAppUUID + ";--dir=./fixtures/playgrounds"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "compute local release impact", "directory": flagDir, "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <app-uuid> is required"))
			}
			if strings.TrimSpace(flagDir) == "" {
				return usageErr(fmt.Errorf("--dir is required"))
			}
			hashes, err := hashLocalWorkingCopy(flagDir)
			if err != nil {
				return err
			}
			out := map[string]any{"app_uuid": args[0], "working_copy": flagDir, "file_hashes": hashes, "placements": []map[string]any{}, "complete": false, "source": "local"}
			s, err := loadStoreReadOnly()
			if err != nil {
				out["hint"] = "Run screencloud-pp-cli sync before relying on placement results."
				return printValue(cmd, flags, out)
			}
			defer s.Close()
			organizationMatch, organizationErr := mirrorMatchesCurrentOrganization(cmd.Context(), flags, s)
			if organizationErr != nil || !organizationMatch {
				out["hint"] = "Run screencloud-pp-cli sync with the current credential; local evidence belongs to a different or unverifiable organization."
				return printValue(cmd, flags, out)
			}
			objectsByType := map[string][]map[string]any{}
			oldest := time.Time{}
			missingResources := []string{}
			for _, resourceType := range []string{"app_instances", "associations", "share_associations", "channels", "playlists", "screens", "spaces"} {
				objects, listErr := listLocalObjects(s, resourceType)
				if listErr != nil {
					missingResources = append(missingResources, resourceType)
					continue
				}
				objectsByType[resourceType] = objects
				lastSynced, fresh, stateErr := freshCompleteSyncState(s, resourceType)
				if stateErr != nil || !fresh {
					missingResources = append(missingResources, resourceType)
				} else if oldest.IsZero() || lastSynced.Before(oldest) {
					oldest = lastSynced
				}
			}
			targets := map[string]struct{}{args[0]: {}}
			matchedObjects := map[string]map[string]any{}
			for changed := true; changed; {
				changed = false
				for resourceType, objects := range objectsByType {
					for _, object := range objects {
						if !objectContainsAny(object, targets) {
							continue
						}
						id := firstString(object, "id", "uuid")
						key := resourceType + ":" + id
						matchedObjects[key] = object
						ids := []string{}
						collectIdentifierValues("", object, &ids)
						for _, relatedID := range ids {
							if _, ok := targets[relatedID]; !ok {
								targets[relatedID] = struct{}{}
								changed = true
							}
						}
					}
				}
			}
			placements := []map[string]any{}
			seenPlacements := map[string]struct{}{}
			nestedConnectionGaps := []string{}
			for key, object := range matchedObjects {
				resourceType := strings.SplitN(key, ":", 2)[0]
				if resourceType == "associations" || resourceType == "share_associations" {
					continue
				}
				appendUniquePlacement(&placements, seenPlacements, safeEntitySummary(resourceType, object))
				if resourceType == "app_instances" {
					screens, screensComplete := nestedConnectionNodes(object, "castedScreenByAppInstanceId")
					if !screensComplete {
						nestedConnectionGaps = append(nestedConnectionGaps, firstString(object, "id")+":castedScreenByAppInstanceId")
					}
					for _, screen := range screens {
						appendUniquePlacement(&placements, seenPlacements, safeEntitySummary("screens", screen))
					}
					spaces, spacesComplete := nestedConnectionNodes(object, "sharedSpacesByAppInstanceId")
					if !spacesComplete {
						nestedConnectionGaps = append(nestedConnectionGaps, firstString(object, "id")+":sharedSpacesByAppInstanceId")
					}
					for _, space := range spaces {
						appendUniquePlacement(&placements, seenPlacements, safeEntitySummary("spaces", space))
					}
				}
			}
			out["placements"] = placements
			out["placement_count"] = len(placements)
			out["complete"] = len(missingResources) == 0 && len(nestedConnectionGaps) == 0
			out["missing_mirror_resources"] = missingResources
			out["incomplete_nested_connections"] = nestedConnectionGaps
			if !oldest.IsZero() {
				out["oldest_mirror_freshness"] = oldest.UTC().Format(time.RFC3339)
			}
			if len(placements) == 0 {
				out["note"] = "No matching placement was found in the bounded local mirror; this is not proof that the app is undisplayed."
			}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagDir, "dir", "", "Reviewed local Playgrounds working copy")
	return cmd
}

func objectContainsAny(value any, targets map[string]struct{}) bool {
	for target := range targets {
		if objectContains(value, target) {
			return true
		}
	}
	return false
}

func nestedConnectionNodes(object map[string]any, key string) ([]map[string]any, bool) {
	connection, _ := object[key].(map[string]any)
	values, _ := connection["nodes"].([]any)
	result := []map[string]any{}
	for _, value := range values {
		if node, ok := value.(map[string]any); ok {
			result = append(result, node)
		}
	}
	total, ok := connection["totalCount"].(float64)
	return result, ok && total >= 0 && len(result) >= int(total)
}

func appendUniquePlacement(placements *[]map[string]any, seen map[string]struct{}, placement map[string]any) {
	key := fmt.Sprint(placement["resource_type"], ":", placement["id"])
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*placements = append(*placements, placement)
}
