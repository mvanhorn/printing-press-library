// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlaygroundsConfigDriftCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source local
	var flagAppUuid string

	cmd := &cobra.Command{
		Use:         "config-drift",
		Short:       "Detect structurally divergent Playgrounds configurations without storing or revealing private values.",
		Example:     "  screencloud-pp-cli playgrounds config-drift --app-uuid 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "compare local configuration shapes", "app_uuid": flagAppUuid, "sent": false})
			}
			if strings.TrimSpace(flagAppUuid) == "" {
				return usageErr(fmt.Errorf("--app-uuid is required"))
			}
			out := map[string]any{"app_uuid": flagAppUuid, "source": "local", "groups": []map[string]any{}, "drift": false, "complete": false, "private_values_included": false}
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
			_, fresh, syncErr := freshCompleteSyncState(s, "app_instances")
			if syncErr != nil || !fresh {
				out["complete"] = false
				out["hint"] = "Run screencloud-pp-cli sync --resources app-instances; configuration evidence must be complete and less than 24 hours old."
				return printValue(cmd, flags, out)
			}
			groups := map[string]map[string]any{}
			matched := 0
			for _, instance := range instances {
				if !objectContains(instance, flagAppUuid) {
					continue
				}
				matched++
				config := valueAt(instance, "config", "configuration")
				fingerprint, paths := structuralFingerprint(config)
				group, ok := groups[fingerprint]
				if !ok {
					group = map[string]any{"fingerprint": fingerprint, "shape": paths, "instance_ids": []string{}}
					groups[fingerprint] = group
				}
				group["instance_ids"] = append(group["instance_ids"].([]string), firstString(instance, "id"))
			}
			rows := make([]map[string]any, 0, len(groups))
			for _, group := range groups {
				group["count"] = len(group["instance_ids"].([]string))
				rows = append(rows, group)
			}
			out["groups"] = rows
			out["drift"] = len(rows) > 1
			out["instance_count"] = matched
			out["group_count"] = len(rows)
			if matched == 0 {
				out["complete"] = false
				out["hint"] = "No synchronized app instance matched --app-uuid; verify the identifier and sync coverage."
				return printValue(cmd, flags, out)
			}
			out["complete"] = true
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagAppUuid, "app-uuid", "", "Playgrounds app UUID to compare across instances")
	return cmd
}
