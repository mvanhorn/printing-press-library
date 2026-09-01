// PATCH(library): runtime-discoverable machine inventory and target selection commands.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/machines"
	"github.com/spf13/cobra"
)

func init() { registerNovelCommand(registerMachineCommands) }

// pp:data-source local
func registerMachineCommands(root *cobra.Command, flags *rootFlags) {
	addNovelCommandIfAbsent(root, newMachinesCmd(flags))
}
func machineStorePath(flags *rootFlags) string {
	if flags.configPath != "" {
		return flags.configPath + ".targets.json"
	}
	d, _ := cliutil.ConfigDir()
	return filepath.Join(d, "targets.json")
}
func newMachinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "machines", Short: "List and select configured KVM machine targets", Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}}
	list := &cobra.Command{Use: "list", Short: "List machine targets", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		inv := machines.DefaultInventory()
		return flags.printJSON(cmd, inv)
	}}
	use := &cobra.Command{Use: "use <name>", Short: "Persist a target profile after exact validation", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:local-write": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		inv := machines.DefaultInventory()
		t, e := inv.Resolve(args[0])
		if e != nil {
			return e
		}
		if e = (machines.TargetStateStore{Path: machineStorePath(flags)}).Save(t.Name); e != nil {
			return e
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), t.Name)
		return nil
	}}
	current := &cobra.Command{Use: "current", Short: "Show persisted target", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		name, e := (machines.TargetStateStore{Path: machineStorePath(flags)}).Load()
		if e != nil {
			if os.IsNotExist(e) {
				return fmt.Errorf("no target selected")
			}
			return e
		}
		return flags.printJSON(cmd, map[string]string{"selected": name})
	}}
	selectCmd := &cobra.Command{Use: "select <name>", Short: "Select a target and verify a console response", Args: cobra.ExactArgs(1), Annotations: map[string]string{"pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		inv := machines.DefaultInventory()
		target, err := inv.Resolve(args[0])
		if err != nil {
			return err
		}
		if flags.dryRun {
			return flags.printJSON(cmd, map[string]any{"dry_run": true, "target": target.Name, "port": target.Port, "keys": []string{"ControlRight", "ControlRight", fmt.Sprintf("Digit%d", target.Port), "Enter"}})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		lockDir := filepath.Join(filepath.Dir(machineStorePath(flags)), "locks")
		s := machines.Selector{Inventory: inv, DeviceID: c.RequestBaseURL(), LockFactory: func(id string) (machines.Locker, error) {
			return machines.NewDeviceLock(machines.LockPath(lockDir, id))
		}, SendKey: c.KVMDKey, Verify: func(ctx context.Context, t machines.Target) error {
			b, e := c.Get(ctx, "/api/streamer/snapshot", nil)
			if e != nil {
				return e
			}
			if len(b) == 0 {
				return fmt.Errorf("empty verification snapshot")
			}
			return nil
		}}
		rec, err := s.Select(cmd.Context(), args[0], machines.SelectOptions{Settle: 100 * time.Millisecond})
		if err != nil {
			return flags.printJSON(cmd, rec)
		}
		if err = (machines.TargetStateStore{Path: machineStorePath(flags)}).Save(rec.Target.Name); err != nil {
			return err
		}
		return flags.printJSON(cmd, rec)
	}}
	cmd.AddCommand(list, use, current, selectCmd)
	return cmd
}
