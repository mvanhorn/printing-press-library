// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/store"
	"github.com/spf13/cobra"
)

type settingsDiffEntry struct {
	Field string `json:"field"`
	Left  any    `json:"left,omitempty"`
	Right any    `json:"right,omitempty"`
}

type settingsDiffResult struct {
	Left         string              `json:"left"`
	Right        string              `json:"right"`
	Diff         []settingsDiffEntry `json:"diff"`
	Count        int                 `json:"count"`
	MissingIndex string              `json:"missing_index,omitempty"`
}

func newNovelSettingsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "diff <index-a> <index-b>",
		Short:       "Field-level comparison of settings between two indices (or a settings file vs an index).",
		Example:     "  algolia-pp-cli settings diff algolia_movie_sample_dataset staging_movies",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "index-a=algolia_movie_sample_dataset;index-b=algolia_movie_sample_dataset", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "settings diff")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two index names are required"))
			}
			leftName, rightName := args[0], args[1]
			if flagDB == "" {
				flagDB = defaultDBPath("algolia-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: algolia-pp-cli sync --resources indexes to populate the local database.\n", flagDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), settingsDiffResult{Left: leftName, Right: rightName, Diff: make([]settingsDiffEntry, 0)}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), flagDB)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "indexes") {
				hintIfStale(cmd, db, "indexes", flags.maxAge)
			}

			// Load both settings blobs from the synced indexes resource.
			leftRaw, errLeft := db.Get("indexes", leftName)
			rightRaw, errRight := db.Get("indexes", rightName)
			if errLeft != nil || leftRaw == nil {
				leftRaw, errLeft = fetchLiveSettings(cmd, flags, leftName)
			}
			if errRight != nil || rightRaw == nil {
				rightRaw, errRight = fetchLiveSettings(cmd, flags, rightName)
			}
			// A genuinely missing index (404) is a valid one-sided-diff state;
			// any other fetch failure (auth, network, rate limit) is an
			// operational error and must not masquerade as a missing index.
			if errLeft != nil && leftRaw == nil {
				if !isNotFoundError(errLeft) {
					return apiErr(fmt.Errorf("fetching settings for index %q: %w", leftName, errLeft))
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: settings for index %q not found (live fetch: %v); showing one-sided diff from %q\n", leftName, errLeft, rightName)
				rightSettings := unwrapSettingsObject(rightRaw)
				res := settingsDiffResult{Left: leftName, Right: rightName, Diff: make([]settingsDiffEntry, 0), Count: 0, MissingIndex: leftName}
				if len(rightSettings) > 0 {
					for k, v := range rightSettings {
						res.Diff = append(res.Diff, settingsDiffEntry{Field: k, Left: nil, Right: v})
					}
					res.Count = len(res.Diff)
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Settings for index %q not found; %q has %d settings fields.\n", leftName, rightName, len(rightSettings))
				return nil
			}
			if errRight != nil && rightRaw == nil {
				if !isNotFoundError(errRight) {
					return apiErr(fmt.Errorf("fetching settings for index %q: %w", rightName, errRight))
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: settings for index %q not found (live fetch: %v); showing one-sided diff from %q\n", rightName, errRight, leftName)
				leftSettings := unwrapSettingsObject(leftRaw)
				res := settingsDiffResult{Left: leftName, Right: rightName, Diff: make([]settingsDiffEntry, 0), Count: 0, MissingIndex: rightName}
				if len(leftSettings) > 0 {
					for k, v := range leftSettings {
						res.Diff = append(res.Diff, settingsDiffEntry{Field: k, Left: v, Right: nil})
					}
					res.Count = len(res.Diff)
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Settings for index %q not found; %q has %d settings fields.\n", rightName, leftName, len(leftSettings))
				return nil
			}

			leftSettings := unwrapSettingsObject(leftRaw)
			rightSettings := unwrapSettingsObject(rightRaw)

			diff := make([]settingsDiffEntry, 0)
			allKeys := make(map[string]bool)
			for k := range leftSettings {
				allKeys[k] = true
			}
			for k := range rightSettings {
				allKeys[k] = true
			}
			for k := range allKeys {
				l, lok := leftSettings[k]
				r, rok := rightSettings[k]
				if lok && rok && jsonEqual(l, r) {
					continue
				}
				diff = append(diff, settingsDiffEntry{Field: k, Left: l, Right: r})
			}

			res := settingsDiffResult{Left: leftName, Right: rightName, Diff: diff, Count: len(diff)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(diff) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Settings identical between %q and %q.\n", leftName, rightName)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Settings diff between %q and %q (%d fields):\n", leftName, rightName, len(diff))
			for _, d := range diff {
				l, _ := json.Marshal(d.Left)
				r, _ := json.Marshal(d.Right)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n    %s: %s\n    %s: %s\n", d.Field, leftName, string(l), rightName, string(r))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func fetchLiveSettings(cmd *cobra.Command, flags *rootFlags, indexName string) (json.RawMessage, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, getErr := c.Get(ctx, "/1/indexes/"+indexName+"/settings", map[string]string{})
	if getErr != nil {
		return nil, getErr
	}
	return data, nil
}

func unwrapSettingsObject(raw json.RawMessage) map[string]any {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		if inner, ok := obj["settings"].(map[string]any); ok {
			return inner
		}
		return obj
	}
	return map[string]any{}
}

func jsonEqual(a, b any) bool {
	aj, errA := json.Marshal(a)
	bj, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(aj) == string(bj)
}

// isNotFoundError reports whether err is a genuine HTTP 404 (index does not
// exist) rather than an auth, network, or rate-limit failure.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return strings.Contains(err.Error(), "HTTP 404")
}
