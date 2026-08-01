// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlaygroundsReadinessCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source local
	var appUUID string

	cmd := &cobra.Command{
		Use:         "readiness",
		Short:       "Find missing, inactive, outdated, dangling, and inconsistent Playgrounds deployments across the organization.",
		Example:     "  screencloud-pp-cli playgrounds readiness --agent --select summary,findings,complete,hint",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := map[string]any{"app_uuid": appUUID, "source": "local", "findings": []map[string]any{}, "summary": map[string]any{"ready": 0, "warning": 0, "critical": 0}, "complete": false}
			s, err := loadStoreReadOnly()
			if err != nil {
				out["hint"] = "Run screencloud-pp-cli sync first."
				return printValue(cmd, flags, out)
			}
			defer s.Close()
			instances, err := listLocalObjects(s, "app_instances")
			if err != nil {
				return err
			}
			_, instancesFresh, syncErr := freshCompleteSyncState(s, "app_instances")
			if syncErr != nil || !instancesFresh {
				out["hint"] = "Run screencloud-pp-cli sync --resources app-instances; readiness evidence must be complete and less than 24 hours old."
				return printValue(cmd, flags, out)
			}
			versions, err := listLocalObjects(s, "app_versions")
			if err != nil {
				return err
			}
			_, versionsFresh, versionsSyncErr := freshCompleteSyncState(s, "app_versions")
			if versionsSyncErr != nil || !versionsFresh {
				out["hint"] = "Run screencloud-pp-cli sync --resources app-instances,app-versions; readiness evidence must be complete and less than 24 hours old."
				return printValue(cmd, flags, out)
			}
			installs, err := listLocalObjects(s, "app_installs")
			if err != nil {
				return err
			}
			spaces, err := listLocalObjects(s, "spaces")
			if err != nil {
				return err
			}
			_, installsFresh, installsSyncErr := freshCompleteSyncState(s, "app_installs")
			_, spacesFresh, spacesSyncErr := freshCompleteSyncState(s, "spaces")
			if installsSyncErr != nil || spacesSyncErr != nil || !installsFresh || !spacesFresh {
				out["hint"] = "Run screencloud-pp-cli sync --resources app-instances,app-installs,app-versions,spaces; relationship evidence must be complete and less than 24 hours old."
				return printValue(cmd, flags, out)
			}
			installByID := map[string]map[string]any{}
			for _, install := range installs {
				if id := firstString(install, "id"); id != "" {
					installByID[id] = install
				}
			}
			spaceIDs := map[string]struct{}{}
			for _, space := range spaces {
				if id := firstString(space, "id"); id != "" {
					spaceIDs[id] = struct{}{}
				}
			}
			metadata, err := listLocalObjects(s, "playgrounds_metadata")
			if err != nil {
				return err
			}
			freshContentEvidence := map[string]struct{}{}
			for _, item := range metadata {
				id := firstString(item, "app_uuid", "appUuid", "id")
				if id == "" || !freshEvidenceTimestamp(valueAt(item, "refreshed_at", "refreshedAt")) || !boolAt(item, "production_available") {
					continue
				}
				if parseFlexibleTime(valueAt(item, "production_files_last_modified", "production_data_last_modified", "production_last_modified")).IsZero() {
					continue
				}
				freshContentEvidence[id] = struct{}{}
			}
			latestByApp := map[string]string{}
			for _, version := range versions {
				if boolAt(version, "isLatest") {
					latestByApp[firstString(version, "appId", "app_id")] = firstString(version, "version")
				}
			}
			findings := []map[string]any{}
			matched := 0
			contentEvidenceMissing := false
			notReady := map[string]struct{}{}
			configShapes := map[string]map[string]struct{}{}
			configInstanceIDs := map[string][]string{}
			for _, instance := range instances {
				instanceAppUUID := firstString(instance, "playgroundsAppUuid", "appUuid", "app_uuid")
				if instanceAppUUID == "" || appUUID != "" && instanceAppUUID != appUUID {
					continue
				}
				matched++
				id := firstString(instance, "id", "appUuid", "app_uuid")
				status := strings.ToUpper(firstString(instance, "status"))
				if status != "" && status != "ACTIVE" {
					findings = append(findings, map[string]any{"severity": "critical", "type": "inactive", "app_instance_id": id, "status": status})
					notReady[id] = struct{}{}
				}
				spaceID := firstString(instance, "spaceId", "space_id")
				installID := firstString(instance, "appInstallId", "app_install_id")
				_, spaceExists := spaceIDs[spaceID]
				install, installExists := installByID[installID]
				if spaceID == "" || installID == "" || !spaceExists || !installExists {
					findings = append(findings, map[string]any{"severity": "warning", "type": "dangling_relationship", "app_instance_id": id})
					notReady[id] = struct{}{}
				} else {
					instanceAppID := firstString(instance, "appId", "app_id")
					if instanceAppID == "" || firstString(install, "appId", "app_id") != instanceAppID || firstString(install, "spaceId", "space_id") != spaceID {
						findings = append(findings, map[string]any{"severity": "warning", "type": "relationship_mismatch", "app_instance_id": id})
						notReady[id] = struct{}{}
					}
				}
				if _, ok := freshContentEvidence[instanceAppUUID]; !ok {
					findings = append(findings, map[string]any{"severity": "warning", "type": "content_evidence_missing", "app_instance_id": id, "app_uuid": instanceAppUUID})
					notReady[id] = struct{}{}
					contentEvidenceMissing = true
				}
				version := firstString(instance, "version")
				latest := latestByApp[firstString(instance, "appId", "app_id")]
				if version != "" && latest != "" && version != latest {
					findings = append(findings, map[string]any{"severity": "warning", "type": "outdated", "app_instance_id": id, "version": version, "latest_version": latest})
					notReady[id] = struct{}{}
				}
				if _, ok := instance["config"]; !ok {
					findings = append(findings, map[string]any{"severity": "warning", "type": "configuration_shape_unavailable", "app_instance_id": id})
					notReady[id] = struct{}{}
				} else {
					fingerprint, _ := structuralFingerprint(instance["config"])
					if configShapes[instanceAppUUID] == nil {
						configShapes[instanceAppUUID] = map[string]struct{}{}
					}
					configShapes[instanceAppUUID][fingerprint] = struct{}{}
					configInstanceIDs[instanceAppUUID] = append(configInstanceIDs[instanceAppUUID], id)
				}
			}
			for playgroundsAppUUID, shapes := range configShapes {
				if len(shapes) > 1 {
					findings = append(findings, map[string]any{"severity": "warning", "type": "inconsistent_configuration_shapes", "app_uuid": playgroundsAppUUID, "shape_count": len(shapes)})
					for _, id := range configInstanceIDs[playgroundsAppUUID] {
						notReady[id] = struct{}{}
					}
				}
			}
			if matched == 0 && appUUID != "" {
				findings = append(findings, map[string]any{"severity": "critical", "type": "missing", "message": "No matching Playgrounds app instance exists in the bounded local mirror."})
			} else if matched == 0 {
				out["findings"] = findings
				out["hint"] = "No Playgrounds app instances were identifiable in the synchronized mirror; supply --app-uuid or verify the Playgrounds installation and app-instance configuration before treating the fleet as healthy."
				return printValue(cmd, flags, out)
			}
			summary := map[string]any{"ready": matched - len(notReady), "warning": 0, "critical": 0}
			for _, finding := range findings {
				severity, _ := finding["severity"].(string)
				summary[severity] = summary[severity].(int) + 1
			}
			out["findings"] = findings
			out["summary"] = summary
			out["complete"] = !contentEvidenceMissing
			if contentEvidenceMissing {
				out["hint"] = "Refresh sanitized production files/data timestamps for every Playgrounds app before treating readiness as complete."
			}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&appUUID, "app-uuid", "", "Restrict the audit to one Playgrounds app UUID")
	return cmd
}
