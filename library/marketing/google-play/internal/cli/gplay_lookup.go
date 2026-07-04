// Hand-authored Google Play lookup commands (live).
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newAppCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app <appId>",
		Short: "Show full details for a Play Store app",
		Long: "Fetch the complete public listing for an app by package name (e.g. com.dreamgames.royalkingdom): " +
			"installs, exact install count, ratings histogram, IAP range, ads flag, developer contact, version, and recent changes. " +
			"Each fetch also snapshots the listing locally so 'watch-listing' can diff changes over time.",
		Example:     "  google-play-pp-cli app com.dreamgames.royalkingdom --agent --select appId,title,score,realInstalls",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch app details")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required (e.g. com.dreamgames.royalkingdom)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			a, err := c.AppDetails(ctx, args[0])
			if err != nil {
				return classifyGplayErr(err)
			}
			// Best-effort snapshot for watch-listing history; never fail the read.
			snapshotApp(cmd, a)
			return emit(cmd, flags, a)
		},
	}
	cmd.Flags().String("db", "", "Local snapshot database path")
	return cmd
}

func snapshotApp(cmd *cobra.Command, a any) {
	data, err := json.Marshal(a)
	if err != nil {
		return
	}
	var probe struct {
		AppID string `json:"appId"`
	}
	if json.Unmarshal(data, &probe) != nil || probe.AppID == "" {
		return
	}
	s, err := openStoreFor(cmd.Context(), resolveDBFlag(cmd))
	if err != nil {
		return
	}
	defer s.Close()
	_ = s.InsertAppSnapshot(cmd.Context(), probe.AppID, nowUnix(), data)
}

// pp:data-source live
func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "similar <appId>",
		Short:       "List apps similar to a given app",
		Long:        "Fetch the 'similar apps' competitive cluster for an app by package name.",
		Example:     "  google-play-pp-cli similar com.dreamgames.royalkingdom --limit 10 --agent",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch similar apps")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			apps, err := c.Similar(ctx, args[0], limit)
			if err != nil {
				return classifyGplayErr(err)
			}
			return emit(cmd, flags, apps)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum similar apps to return")
	return cmd
}

// pp:data-source live
func newDeveloperCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "developer <devId-or-name>",
		Short:       "List apps published by a developer",
		Long:        "Fetch a developer's published apps by numeric developer id or display name (e.g. \"Dream Games, Ltd.\").",
		Example:     "  google-play-pp-cli developer \"Dream Games, Ltd.\" --agent",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch developer portfolio")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("developer id or name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			apps, err := c.Developer(ctx, args[0], limit)
			if err != nil {
				return classifyGplayErr(err)
			}
			return emit(cmd, flags, apps)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 60, "Maximum apps to return")
	return cmd
}

// pp:data-source live
func newPermissionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permissions <appId>",
		Short:   "List the Android permissions an app declares",
		Example: "  google-play-pp-cli permissions com.dreamgames.royalkingdom --agent",
		Args:    cobra.ArbitraryArgs,
		// An unknown appId returns an empty permission set (HTTP 200), which is
		// indistinguishable from an app that declares no permissions; do not
		// fabricate a not-found error just to satisfy the dogfood error probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch permissions")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			perms, err := c.Permissions(ctx, args[0])
			if err != nil {
				return classifyGplayErr(err)
			}
			return emit(cmd, flags, perms)
		},
	}
	return cmd
}

// pp:data-source live
func newDataSafetyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "datasafety <appId>",
		Short:       "Show an app's data-safety section",
		Long:        "Fetch the developer-declared data-safety summary: data shared, data collected, and the privacy policy URL.",
		Example:     "  google-play-pp-cli datasafety com.dreamgames.royalkingdom --agent",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch data safety")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newGplayClient(cmd, flags)
			ds, err := c.DataSafety(ctx, args[0])
			if err != nil {
				return classifyGplayErr(err)
			}
			return emit(cmd, flags, ds)
		},
	}
	return cmd
}
