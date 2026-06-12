package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
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

func newArchiveStatusCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show accumulating archive status",
		Example:     "  safari-history-pp-cli archive status\n  safari-history-pp-cli archive status --json",
		Annotations: map[string]string{"pp:typed-exit-codes": "0", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			row := map[string]any{
				"enabled":     status.Enabled,
				"baseline_at": status.BaselineAt,
				"url_count":   status.URLCount,
				"visit_count": status.VisitCount,
				"path":        status.Path,
				"size_bytes":  status.SizeBytes,
			}
			if status.Enabled && status.VisitCount > 0 {
				row["cached_store"] = "queryable"
				row["note"] = "Archive is queryable offline; live Safari source access is only needed to refresh it with sync --accumulate."
			} else if status.VisitCount > 0 {
				row["cached_store"] = "present_disabled"
				row["note"] = "Archive has cached rows but archive mode is disabled; enable archive mode before expecting read tools to use archive.db."
			} else if status.SizeBytes > 0 {
				row["cached_store"] = "empty"
			} else {
				row["cached_store"] = "missing"
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, row)
		},
	}
}

func newArchiveEnableCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "enable",
		Short:       "Enable the accumulating archive from the current snapshot",
		Example:     "  safari-history-pp-cli sync\n  safari-history-pp-cli archive enable",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath, snapshot, err := archiveAndSnapshotPaths()
			if err != nil {
				return err
			}
			if _, err := os.Stat(snapshot); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: snapshot missing at %s", ErrNoSnapshot, snapshot)
				}
				return err
			}
			alreadyEnabled, err := store.IsArchiveEnabled()
			if err != nil {
				return err
			}
			if !alreadyEnabled {
				if err := store.EnableArchiveFromSource(archivePath, snapshot, time.Now().UTC()); err != nil {
					if os.IsNotExist(err) {
						return ErrNoSnapshot
					}
					return err
				}
			}
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			row := map[string]any{
				"message":         archiveEnabledMessage(alreadyEnabled),
				"enabled":         status.Enabled,
				"already_enabled": alreadyEnabled,
				"baseline_at":     status.BaselineAt,
				"url_count":       status.URLCount,
				"visit_count":     status.VisitCount,
				"path":            status.Path,
				"size_bytes":      status.SizeBytes,
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
		Example:     "  safari-history-pp-cli archive disable",
		Annotations: map[string]string{"pp:typed-exit-codes": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath, err := store.ArchivePath()
			if err != nil {
				return err
			}
			if err := store.DisableArchive(archivePath); err != nil {
				return err
			}
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, status)
		},
	}
}

func newArchiveClobberCmd(opts *RootOptions) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "clobber",
		Short:   "Replace the archive with a fresh current snapshot baseline",
		Example: "  safari-history-pp-cli archive clobber --force",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath, snapshot, err := archiveAndSnapshotPaths()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			if !force {
				status, err := store.ReadArchiveStatus()
				if err != nil {
					return err
				}
				plan := []map[string]any{{
					"requires_force": true,
					"path":           archivePath,
					"size_bytes":     status.SizeBytes,
					"enabled":        status.Enabled,
					"visit_count":    status.VisitCount,
				}}
				if err := output.Render(opts.Output, plan); err != nil {
					return err
				}
				return errors.Join(ErrUsage, fmt.Errorf("archive clobber requires --force"))
			}
			if err := store.ClobberArchiveFromSource(archivePath, snapshot, time.Now().UTC()); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: snapshot missing at %s", ErrNoSnapshot, snapshot)
				}
				return err
			}
			status, err := store.ReadArchiveStatus()
			if err != nil {
				return err
			}
			return output.Render(opts.Output, status)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "required: confirm destroying accumulated archive history not present in the current snapshot")
	return cmd
}

func newArchiveResetCmd(opts *RootOptions) *cobra.Command {
	var force bool
	var purge bool
	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Disable archive mode and move or purge archive.db",
		Example: "  safari-history-pp-cli archive reset --force\n  safari-history-pp-cli archive reset --force --purge",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath, err := store.ArchivePath()
			if err != nil {
				return err
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			if !force {
				status, err := store.ReadArchiveStatus()
				if err != nil {
					return err
				}
				plan := []map[string]any{{
					"requires_force": true,
					"would_purge":    purge,
					"would_move_to":  archivePath + ".bak",
					"path":           archivePath,
					"size_bytes":     status.SizeBytes,
					"enabled":        status.Enabled,
					"visit_count":    status.VisitCount,
				}}
				if err := output.Render(opts.Output, plan); err != nil {
					return err
				}
				return errors.Join(ErrUsage, fmt.Errorf("archive reset requires --force"))
			}
			if err := store.ResetArchive(archivePath, purge); err != nil {
				return err
			}
			row := []map[string]any{{
				"reset":       true,
				"purged":      purge,
				"path":        archivePath,
				"file_exists": fileExists(archivePath),
			}}
			return output.Render(opts.Output, row)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "perform the guarded reset")
	cmd.Flags().BoolVar(&purge, "purge", false, "delete archive.db instead of moving it to a .bak")
	return cmd
}

func newArchiveVacuumCmd(opts *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "vacuum",
		Short:       "Compact archive.db with VACUUM",
		Example:     "  safari-history-pp-cli archive vacuum",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath, err := store.ArchivePath()
			if err != nil {
				return err
			}
			before := fileSize(archivePath)
			if err := store.VacuumArchive(archivePath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%w: archive not initialized: run 'archive enable' first", ErrNoSnapshot)
				}
				return err
			}
			after := fileSize(archivePath)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, []map[string]any{{
				"path":              archivePath,
				"size_bytes_before": before,
				"size_bytes_after":  after,
			}})
		},
	}
}

func archiveAndSnapshotPaths() (string, string, error) {
	archivePath, err := store.ArchivePath()
	if err != nil {
		return "", "", err
	}
	snapshot, err := snapshotPath()
	if err != nil {
		return "", "", err
	}
	return archivePath, snapshot, nil
}

func archiveEnabledMessage(alreadyEnabled bool) string {
	if alreadyEnabled {
		return "archive already enabled"
	}
	return "archive enabled"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}
