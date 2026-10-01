package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func collectAgentCommands(root *cobra.Command) []map[string]any {
	rows := []map[string]any{}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "agent-context" {
			continue
		}
		flags := []map[string]any{}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			flags = append(flags, map[string]any{"name": f.Name, "type": f.Value.Type(), "usage": f.Usage, "default": f.DefValue})
		})
		cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
			flags = append(flags, map[string]any{"name": f.Name, "type": f.Value.Type(), "usage": f.Usage, "default": f.DefValue})
		})
		rows = append(rows, map[string]any{"name": cmd.Name(), "use": cmd.Use, "short": cmd.Short, "runnable": cmd.Runnable(), "annotations": cmd.Annotations, "flags": flags, "subcommands": collectAgentCommands(cmd)})
	}
	return rows
}
