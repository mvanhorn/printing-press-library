// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/store"
	"github.com/spf13/cobra"
)

func newNovelPlaygroundsPreviewDriftCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source local
	var flagOlderThan string
	var refresh bool
	var appUUID string
	var spaceID string

	cmd := &cobra.Command{
		Use:         "preview-drift",
		Short:       "Find unpublished previews, production-ahead conflicts, and preview work that has waited too long.",
		Example:     "  screencloud-pp-cli playgrounds preview-drift --older-than 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			threshold, err := parseAge(flagOlderThan)
			if err != nil {
				return usageErr(err)
			}
			if refresh {
				if strings.TrimSpace(appUUID) == "" || strings.TrimSpace(spaceID) == "" {
					return usageErr(fmt.Errorf("--refresh requires --app-uuid and --space-id"))
				}
				if flags.dryRun {
					return printValue(cmd, flags, map[string]any{"operation": "refresh sanitized production and preview timestamps", "app_uuid": appUUID, "space_id": spaceID, "sent": false})
				}
				if !flags.yes {
					return usageErr(fmt.Errorf("--refresh mints a short-lived management JWT; rerun with --yes after reviewing the target"))
				}
				if err := refreshPlaygroundsTimestampMetadata(cmd, flags, appUUID, spaceID); err != nil {
					return err
				}
			}
			out := map[string]any{"source": "local", "older_than": threshold.String(), "findings": []map[string]any{}, "complete": false}
			s, err := loadStoreReadOnly()
			if err != nil {
				out["hint"] = "Run sync and import sanitized Playgrounds metadata first."
				return printValue(cmd, flags, out)
			}
			defer s.Close()
			organizationMatch, organizationErr := mirrorMatchesCurrentOrganization(cmd.Context(), flags, s)
			if organizationErr != nil || !organizationMatch {
				out["hint"] = "Run screencloud-pp-cli sync or refresh with the current credential; local evidence belongs to a different or unverifiable organization."
				return printValue(cmd, flags, out)
			}
			objects, err := listLocalObjects(s, "playgrounds_metadata")
			if err != nil {
				return err
			}
			if len(objects) == 0 {
				out["hint"] = "No sanitized Playgrounds timestamp metadata is available; import or sync it before treating the queue as clean."
				return printValue(cmd, flags, out)
			}
			expected := map[string]struct{}{}
			if strings.TrimSpace(appUUID) != "" {
				expected[appUUID] = struct{}{}
			} else {
				instances, err := listLocalObjects(s, "app_instances")
				if err != nil {
					return err
				}
				_, fresh, stateErr := freshCompleteSyncState(s, "app_instances")
				if stateErr != nil || !fresh {
					out["hint"] = "Run a complete app-instances sync less than 24 hours before treating preview metadata as fleet-complete."
					return printValue(cmd, flags, out)
				}
				for _, instance := range instances {
					if id := firstString(instance, "playgroundsAppUuid", "appUuid", "app_uuid"); id != "" {
						expected[id] = struct{}{}
					}
				}
			}
			if len(expected) == 0 {
				out["hint"] = "No expected Playgrounds app UUIDs are available; specify --app-uuid or sync app instances."
				return printValue(cmd, flags, out)
			}
			observed := map[string]struct{}{}
			findings := []map[string]any{}
			staleMetadata := []string{}
			for _, object := range objects {
				appUUID := firstString(object, "app_uuid", "appUuid", "id")
				if _, ok := expected[appUUID]; !ok {
					continue
				}
				if !freshEvidenceTimestamp(valueAt(object, "refreshed_at", "refreshedAt")) {
					staleMetadata = append(staleMetadata, appUUID)
					continue
				}
				usable := false
				detailed := false
				for _, resource := range []string{"files", "data"} {
					productionValue := valueAt(object, "production_"+resource+"_last_modified")
					previewValue := valueAt(object, "preview_"+resource+"_last_modified")
					if productionValue == nil && previewValue == nil {
						continue
					}
					detailed = true
					production := parseFlexibleTime(productionValue)
					preview := parseFlexibleTime(previewValue)
					if !production.IsZero() || !preview.IsZero() {
						usable = true
					}
					appendPreviewDriftFindings(&findings, appUUID, resource, production, preview, threshold)
				}
				if !detailed {
					production := parseFlexibleTime(valueAt(object, "production_last_modified", "productionLastModified"))
					preview := parseFlexibleTime(valueAt(object, "preview_last_modified", "previewLastModified"))
					usable = !production.IsZero() || !preview.IsZero()
					appendPreviewDriftFindings(&findings, appUUID, "workspace", production, preview, threshold)
				}
				if usable {
					observed[appUUID] = struct{}{}
				}
			}
			out["findings"] = findings
			out["finding_count"] = len(findings)
			missing := []string{}
			for id := range expected {
				if _, ok := observed[id]; !ok {
					missing = append(missing, id)
				}
			}
			out["missing_metadata_app_uuids"] = missing
			out["stale_metadata_app_uuids"] = staleMetadata
			out["expected_app_count"] = len(expected)
			out["observed_app_count"] = len(observed)
			out["complete"] = len(missing) == 0
			if len(missing) > 0 {
				out["hint"] = "Refresh sanitized timestamps for every missing app before treating the queue as clean."
			}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "7d", "Age threshold such as 12h, 7d, or 2w")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh sanitized lastModified metadata for one app before analysis")
	cmd.Flags().StringVar(&appUUID, "app-uuid", "", "Playgrounds app UUID to refresh")
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint the short-lived management token")
	return cmd
}

func appendPreviewDriftFindings(findings *[]map[string]any, appUUID, resource string, production, preview time.Time, threshold time.Duration) {
	base := map[string]any{"app_uuid": appUUID, "resource": resource}
	add := func(kind, severity string, extra map[string]any) {
		finding := map[string]any{"app_uuid": base["app_uuid"], "resource": base["resource"], "type": kind, "severity": severity}
		for key, value := range extra {
			finding[key] = value
		}
		*findings = append(*findings, finding)
	}
	if production.IsZero() && !preview.IsZero() {
		add("preview_only", "warning", nil)
	} else if !production.IsZero() && !preview.IsZero() {
		if production.After(preview) {
			add("production_ahead", "critical", nil)
		} else if preview.After(production) {
			add("preview_ahead", "warning", nil)
		}
	}
	if !preview.IsZero() && time.Since(preview) > threshold && (production.IsZero() || !production.Equal(preview)) {
		add("aged_unpublished_preview", "warning", map[string]any{"preview_updated_at": preview.UTC().Format(time.RFC3339)})
	}
}

func refreshPlaygroundsTimestampMetadata(cmd *cobra.Command, flags *rootFlags, appUUID, spaceID string) error {
	token, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
	if err != nil {
		return err
	}
	c, err := newPlaygroundsClient(flags)
	if err != nil {
		return err
	}
	metadata := map[string]any{"id": appUUID, "app_uuid": appUUID, "refreshed_at": time.Now().UTC().Format(time.RFC3339)}
	for _, workspace := range []struct {
		id     string
		prefix string
	}{{appUUID, "production"}, {appUUID + "-preview", "preview"}} {
		available := false
		latest := time.Time{}
		var latestValue any
		for _, resource := range []string{"files", "data"} {
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), "/"+resource+"/"+url.PathEscape(workspace.id), nil, bearerHeader(token))
			if err != nil {
				if isHTTPStatus(err, 404) {
					continue
				}
				return classifyAPIError(err, flags)
			}
			object, err := decodeObject(raw)
			if err != nil {
				return err
			}
			modified, ok := object["lastModified"]
			parsed := parseFlexibleTime(modified)
			if !ok || parsed.IsZero() {
				return apiErr(fmt.Errorf("Playgrounds %s %s response omitted a usable lastModified value", workspace.id, resource))
			}
			available = true
			metadata[workspace.prefix+"_"+resource+"_last_modified"] = modified
			if latest.IsZero() || parsed.After(latest) {
				latest = parsed
				latestValue = modified
			}
		}
		metadata[workspace.prefix+"_available"] = available
		if available {
			metadata[workspace.prefix+"_last_modified"] = latestValue
		} else if workspace.prefix == "production" {
			return apiErr(fmt.Errorf("Playgrounds production workspace %s returned no files or data", workspace.id))
		}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("screencloud-pp-cli"))
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := bindStoreToCurrentOrganization(cmd.Context(), flags, s); err != nil {
		return err
	}
	if err := s.Upsert("playgrounds_metadata", appUUID, raw); err != nil {
		return err
	}
	return s.SaveSyncState("playgrounds_metadata", "playgrounds:lastModified", 1)
}

func parseAge(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(trimmed, "d") || strings.HasSuffix(trimmed, "w") {
		factor := 24 * time.Hour
		if strings.HasSuffix(trimmed, "w") {
			factor = 7 * 24 * time.Hour
		}
		n, err := strconv.ParseFloat(trimmed[:len(trimmed)-1], 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid --older-than %q", value)
		}
		return time.Duration(n * float64(factor)), nil
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --older-than %q", value)
	}
	return duration, nil
}
