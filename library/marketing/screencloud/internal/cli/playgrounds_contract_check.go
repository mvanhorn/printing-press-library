// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlaygroundsContractCheckCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source live
	var flagAppUuid string
	var spaceID, screenID string
	fixtureAppUUID, fixtureSpaceID := playgroundsFixtureIDs()

	cmd := &cobra.Command{
		Use:         "contract-check",
		Short:       "Mint ephemeral scoped JWTs and verify Playgrounds management/viewer reads without changing content.",
		Example:     "  screencloud-pp-cli playgrounds contract-check --app-uuid " + fixtureAppUUID + " --space-id " + fixtureSpaceID + " --agent --yes",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--app-uuid=" + fixtureAppUUID + ";--space-id=" + fixtureSpaceID},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "read-only Studio and Playgrounds contract check", "app_uuid": flagAppUuid, "space_id": spaceID, "sent": false})
			}
			if strings.TrimSpace(flagAppUuid) == "" {
				return usageErr(fmt.Errorf("--app-uuid is required"))
			}
			if strings.TrimSpace(spaceID) == "" {
				return usageErr(fmt.Errorf("--space-id is required"))
			}
			managementToken, _, err := mintScopedJWT(cmd.Context(), flags, "management", spaceID, "")
			if err != nil {
				return err
			}
			viewerToken, _, err := mintScopedJWT(cmd.Context(), flags, "viewer", spaceID, screenID)
			if err != nil {
				return err
			}
			c, err := newPlaygroundsClient(flags)
			if err != nil {
				return err
			}
			pathID := url.PathEscape(flagAppUuid)
			_, unauthFilesErr := c.GetWithHeadersNoCache(cmd.Context(), "/files/"+pathID, nil, nil)
			_, unauthDataErr := c.GetWithHeadersNoCache(cmd.Context(), "/data/"+pathID, nil, nil)
			files, filesErr := getPlaygroundsObject(cmd, flags, managementToken, "/files/"+pathID)
			data, dataErr := getPlaygroundsObject(cmd, flags, managementToken, "/data/"+pathID)
			viewerRaw, viewerErr := c.GetWithHeadersNoCache(cmd.Context(), "/apps/"+pathID, nil, bearerHeader(viewerToken))
			checks := []map[string]any{
				{"check": "unauthenticated_files_read_rejected", "passed": isHTTPStatus(unauthFilesErr, 401) || isHTTPStatus(unauthFilesErr, 403)},
				{"check": "unauthenticated_data_read_rejected", "passed": isHTTPStatus(unauthDataErr, 401) || isHTTPStatus(unauthDataErr, 403)},
				{"check": "management_files_read", "passed": filesErr == nil},
				{"check": "files_shape", "passed": filesErr == nil && files["files"] != nil && files["lastModified"] != nil},
				{"check": "management_data_read", "passed": dataErr == nil},
				{"check": "data_shape", "passed": dataErr == nil && data["data"] != nil && data["lastModified"] != nil},
				{"check": "viewer_package_read", "passed": viewerErr == nil},
				{"check": "viewer_package_html", "passed": viewerErr == nil && (strings.Contains(strings.ToLower(string(viewerRaw[:min(len(viewerRaw), 256)])), "<!doctype html") || strings.Contains(strings.ToLower(string(viewerRaw[:min(len(viewerRaw), 256)])), "<html"))},
			}
			passed := 0
			for _, check := range checks {
				if check["passed"].(bool) {
					passed++
				}
			}
			out := map[string]any{"app_uuid": flagAppUuid, "passed": passed == len(checks), "passed_checks": passed, "total_checks": len(checks), "checks": checks, "content_included": false, "contract_risk": "medium_bundle_derived"}
			if err := printValue(cmd, flags, out); err != nil {
				return err
			}
			if passed != len(checks) {
				return apiErr(fmt.Errorf("Playgrounds contract check failed"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagAppUuid, "app-uuid", "", "Playgrounds app UUID to inspect")
	cmd.Flags().StringVar(&spaceID, "space-id", "", "Space UUID used to mint scoped tokens")
	cmd.Flags().StringVar(&screenID, "screen-id", "", "Optional screen UUID for viewer scope")
	return cmd
}
