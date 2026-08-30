// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

const removeListSettingsHandler = "remove-list-settings"

func newListsSettingsClearCmd(flags *rootFlags) *cobra.Command {
	var name string
	var stdinBody, apply bool

	cmd := &cobra.Command{
		Use:         "clear",
		Short:       "Clear stored settings for a shopping list (preview unless --apply)",
		Example:     "  anylist-pp-cli lists settings clear --name Groceries --apply",
		Annotations: map[string]string{"pp:endpoint": "lists.settings.clear", "pp:method": "POST", "pp:path": "/data/list-settings/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				name = stringFromBody(body, "name")
				if name == "" {
					name = stringFromBody(body, "list")
				}
				apply = boolFromBody(body, "apply")
			}
			name = strings.TrimSpace(name)
			if name == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"name\" not set")
			}
			if !apply || flags.dryRun {
				preview := map[string]any{
					"dry_run":        true,
					"name":           name,
					"apply":          apply,
					"handler_id":     removeListSettingsHandler,
					"operation":      "remove-list-settings",
					"cache_sync":     "after_verified_readback",
					"readback_check": "settings_absent",
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would clear settings for %q (pass --apply to write)\n", name)
				return nil
			}

			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			client := anylist.New(cfg)
			data, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists and settings: %w", err)
			}
			list, err := exactLiveShoppingListByName(data, name)
			if err != nil {
				return err
			}
			settings, found := findLiveListSettingsByListID(data, list.GetIdentifier())
			if !found {
				result := map[string]any{"cleared": false, "already_absent": true, "verified": true, "id": list.GetIdentifier(), "name": list.GetName()}
				return printListsSettingsClearResult(cmd, flags, result)
			}

			if err := client.UpdateListSettings(ctx, removeListSettingsHandler, proto.Clone(settings).(*pb.PBListSettings)); err != nil {
				return fmt.Errorf("clearing settings for %q: %w", list.GetName(), err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying cleared settings for %q: %w", list.GetName(), err)
			}
			if _, stillPresent := findLiveListSettingsByListID(verifiedData, list.GetIdentifier()); stillPresent {
				return fmt.Errorf("settings clear verification failed: settings for list %q are still present", list.GetName())
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after clearing settings: %w", err)
			}
			return printListsSettingsClearResult(cmd, flags, map[string]any{
				"cleared":        true,
				"already_absent": false,
				"verified":       true,
				"id":             list.GetIdentifier(),
				"name":           list.GetName(),
				"settings_id":    settings.GetIdentifier(),
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Shopping list name")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the clear; preview is the default")
	return cmd
}

func findLiveListSettingsByListID(data *pb.PBUserDataResponse, listID string) (*pb.PBListSettings, bool) {
	listID = strings.TrimSpace(listID)
	if data == nil || listID == "" {
		return nil, false
	}
	for _, settings := range data.GetListSettingsResponse().GetSettings() {
		if settings != nil && strings.TrimSpace(settings.GetListId()) == listID {
			return settings, true
		}
	}
	return nil, false
}

func printListsSettingsClearResult(cmd *cobra.Command, flags *rootFlags, result map[string]any) error {
	if flags.quiet {
		if cleared, ok := result["cleared"].(bool); ok {
			fmt.Fprintln(cmd.OutOrStdout(), cleared)
		}
		return nil
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if result["cleared"] == true {
		fmt.Fprintf(cmd.OutOrStdout(), "Cleared settings for %q\n", result["name"])
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Settings for %q were already absent\n", result["name"])
	}
	return nil
}
