package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
	"github.com/spf13/cobra"
)

func newArchiveCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Manage the accumulating archive",
	}
	cmd.AddCommand(
		newArchiveStatusCmd(opts),
		newArchiveEnableCmd(opts),
		newArchiveDisableCmd(opts),
		newArchiveClobberCmd(opts),
		newArchiveResetCmd(opts),
		newArchiveVacuumCmd(opts),
	)
	return cmd
}

func newArchiveEnableCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "enable",
		Short:       "Enable the accumulating archive from the current snapshot",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			counts, alreadyEnabled, err := store.EnableArchiveFromSource(snapshot, time.Now().UTC())
			if err != nil {
				if os.IsNotExist(err) {
					return ErrNoSnapshot
				}
				return err
			}
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			row := map[string]any{
				"archive_enabled": status.Enabled,
				"archive_path":    status.ArchivePath,
				"baseline_at":     status.BaselineAt,
				"already_enabled": alreadyEnabled,
				"archive_visits":  counts.Total,
				"appended":        counts.Appended,
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, []map[string]any{row})
		},
	}
}

func newArchiveDisableCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "disable",
		Short:       "Disable archive mode while keeping the archive file",
		Annotations: map[string]string{"pp:typed-exit-codes": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := store.DisableArchive()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, status)
		},
	}
}

func newArchiveClobberCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "clobber",
		Short:       "Replace the archive with a fresh current snapshot baseline",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			result, err := store.ClobberArchiveFromSource(snapshot, time.Now().UTC())
			if err != nil {
				if os.IsNotExist(err) {
					return ErrNoSnapshot
				}
				return err
			}
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			row := map[string]any{
				"archive_enabled":    status.Enabled,
				"archive_path":       status.ArchivePath,
				"baseline_at":        result.BaselineAt,
				"old_archive_visits": result.OldVisits,
				"new_archive_visits": result.NewVisits,
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, []map[string]any{row})
		},
	}
}

func newArchiveResetCmd(opts *RootOptions) *cobra.Command {
	var force bool
	var purge bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Disable archive mode and move or purge archive.db",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			output.DefaultToJSONIfNotTTY(&opts.Output)
			if !force {
				plan, err := store.PlanArchiveReset()
				if err != nil {
					return err
				}
				if err := output.Render(opts.Output, plan); err != nil {
					return err
				}
				return errors.Join(ErrUsage, fmt.Errorf("archive reset requires --force"))
			}
			result, err := store.ResetArchive(purge, time.Now().UTC())
			if err != nil {
				return err
			}
			return output.Render(opts.Output, result)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "perform the guarded reset")
	cmd.Flags().BoolVar(&purge, "purge", false, "delete archive.db instead of moving it to a .bak")
	return cmd
}

func newArchiveVacuumCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "vacuum",
		Short:       "Compact archive.db with VACUUM and PRAGMA optimize",
		Annotations: map[string]string{"pp:typed-exit-codes": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := store.VacuumArchive()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, result)
		},
	}
}

func newArchiveStatusCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show accumulating archive status",
		Annotations: map[string]string{"pp:typed-exit-codes": "0", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, status)
		},
	}
}
