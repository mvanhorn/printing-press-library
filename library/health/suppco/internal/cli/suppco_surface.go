package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(configureSuppCoSurface)
}

func configureSuppCoSurface(root *cobra.Command, flags *rootFlags) {
	root.AddCommand(newRegimenCmd(flags))
	showCommand(root, "stack")
	hideCommand(root, "agent-context")
	for _, name := range []string{"api", "export", "feedback", "profile", "search", "sync", "which", "workflow"} {
		removeCommand(root, name)
	}

	flags.dataSource = "live"
	if flag := root.PersistentFlags().Lookup("data-source"); flag != nil {
		flag.DefValue = "live"
		_ = root.PersistentFlags().MarkHidden("data-source")
	}
	for _, name := range []string{
		"compact", "csv", "deliver", "human-friendly", "insecure", "max-age",
		"no-cache", "plain", "profile", "quiet", "select",
	} {
		if root.PersistentFlags().Lookup(name) != nil {
			_ = root.PersistentFlags().MarkHidden(name)
		}
	}

	previousPreRun := root.PersistentPreRunE
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := rejectSuppCoGlobalOptions(flags); err != nil {
			return err
		}
		if err := rejectSuppCoOutputOptions(cmd); err != nil {
			return err
		}
		if previousPreRun != nil {
			if err := previousPreRun(cmd, args); err != nil {
				return err
			}
		}
		if err := rejectSuppCoGlobalOptions(flags); err != nil {
			return err
		}
		return rejectSuppCoOutputOptions(cmd)
	}
}

func rejectSuppCoGlobalOptions(flags *rootFlags) error {
	if flags.deliverSpec != "" {
		return usageErr(fmt.Errorf("--deliver is not available for private SuppCo provider output; redirect stdout explicitly if needed"))
	}
	if flags.insecure {
		return usageErr(fmt.Errorf("--insecure is not available for SuppCo provider requests"))
	}
	if flags.dataSource != "live" {
		return usageErr(fmt.Errorf("SuppCo provider reads require the live stateless data source"))
	}
	if flags.profileName != "" {
		return usageErr(fmt.Errorf("--profile is not available for the SuppCo provider surface"))
	}
	return nil
}

func rejectSuppCoOutputOptions(cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}
	for _, name := range []string{"compact", "csv", "human-friendly", "plain", "quiet", "select"} {
		if cmd.Flags().Changed(name) {
			return usageErr(fmt.Errorf("--%s is not available; SuppCo provider commands emit the complete normalized JSON contract", name))
		}
	}
	return nil
}

func suppCoDateArgs(flags *rootFlags) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) && len(args) == 0 {
			return nil
		}
		return cobra.ExactArgs(1)(cmd, args)
	}
}

func removeCommand(root *cobra.Command, name string) {
	cmd, _, err := root.Find([]string{name})
	if err == nil && cmd != nil && cmd != root {
		root.RemoveCommand(cmd)
	}
}

func showCommand(root *cobra.Command, name string) {
	cmd, _, err := root.Find([]string{name})
	if err == nil && cmd != nil && cmd != root {
		cmd.Hidden = false
	}
}

func hideCommand(root *cobra.Command, name string) {
	cmd, _, err := root.Find([]string{name})
	if err == nil && cmd != nil && cmd != root {
		cmd.Hidden = true
	}
}
