// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written support for drive import / episode import. Not a novel-command
// scaffold file itself, so it is not subject to the TODO-refresh gate.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/flow/internal/types"

	"github.com/spf13/cobra"
)

// fetchProjectContents fetches flow.projectInitialData for one project,
// shared by scenes gaps and drive import --tag-scene so both read the same
// live/local-fallback path (via resolveReadWithStrategyAndResponsePath)
// instead of duplicating the tRPC call shape.
func fetchProjectContents(cmd *cobra.Command, flags *rootFlags, projectID string) (types.ProjectContents, error) {
	c, err := flags.newClient()
	if err != nil {
		return types.ProjectContents{}, err
	}

	path := "https://labs.google/fx/api/trpc/flow.projectInitialData"
	params := map[string]string{"input": fmt.Sprintf(`{"json":{"projectId":%q}}`, projectID)}
	data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "project", false, path, params, nil, "result.data.json", cmd.ErrOrStderr())
	if err != nil {
		return types.ProjectContents{}, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}

	var contents types.ProjectContents
	if err := json.Unmarshal(data, &contents); err != nil {
		return types.ProjectContents{}, apiErr(fmt.Errorf("parsing project contents: %w", err))
	}
	return contents, nil
}

// resolveDriveFolder accepts a local filesystem path -- most commonly a
// folder under a mounted Google Drive for Desktop volume (macOS:
// ~/Library/CloudStorage/GoogleDrive-<email>/My Drive/..., or a legacy
// ~/Google Drive mount) -- and returns it if it exists and is a directory.
//
// A bare Google Drive folder ID (the kind shown in a drive.google.com URL)
// cannot be resolved this way: Drive for Desktop's local filesystem view is
// organized by folder name/path, not by Drive ID, and there is no local API
// to translate one to the other. Resolving a raw ID would require the
// separate Google Drive API (its own OAuth2 client, distinct from Flow's
// harvested session token) that this CLI does not implement. Callers get an
// actionable error pointing at the supported path-based flow instead of a
// silent failure.
func resolveDriveFolder(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("a folder value is required")
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", fmt.Errorf(
			"%q is not a local path this CLI can read (%w). This CLI resolves Drive folders through a mounted "+
				"Google Drive for Desktop volume, not the Drive API -- pass the folder's local path instead, e.g. "+
				"\"~/Library/CloudStorage/GoogleDrive-<email>/My Drive/<folder>\" on macOS. A bare Drive folder ID "+
				"cannot be resolved without a separate Google Drive API OAuth setup, which this CLI does not implement",
			value, err,
		)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is a file, not a folder", value)
	}
	return value, nil
}
