// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// bindLocalFrameworkSearch retains the generated FTS/type/DB callback, while
// preventing the provider's catalog listing from being used as a text search.
func bindLocalFrameworkSearch(root *cobra.Command, flags *rootFlags) {
	cmd, _, err := root.Find([]string{"search"})
	if err != nil || cmd == nil || cmd.RunE == nil {
		return
	}
	localSearch := cmd.RunE
	cmd.Short = "Search synced geography with local FTS; use catalog search for regional products"
	cmd.Long = "Search the geography records in the framework store populated by sync. Auto and local modes both use local FTS, preserving --type, --db and --limit. Explicit live mode is unsupported; use catalog search QUERY --region REGION for bounded source products. Selected catalog observations are separate; use catalog saved for that evidence."
	cmd.Example = "  tabiwa-pp-cli search 岡山 --type geography --data-source local --json\n  tabiwa-pp-cli search 岡山 --db /tmp/tabiwa-geography.db --limit 3 --json"
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["pp:data-source"] = "local"
	cmd.Annotations["pp:happy-args"] = "query=岡山;--type=geography;--limit=3"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, "local"); err != nil {
			return usageErr(fmt.Errorf("root search has no live text-search equivalent; use catalog search QUERY --region REGION: %w", err))
		}
		requested := flags.dataSource
		flags.dataSource = "local"
		defer func() { flags.dataSource = requested }()
		return localSearch(cmd, args)
	}
}
