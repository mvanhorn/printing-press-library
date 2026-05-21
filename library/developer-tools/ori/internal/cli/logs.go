// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: tail ~/Library/Logs/openclaw-a2a-server/{stdout,stderr}.log
// without remembering the path. Read-only.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/cliutil"
)

func newLogsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Inspect openclaw-a2a-server log files",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newLogsTailCmd(flags))
	cmd.AddCommand(newLogsPathCmd(flags))
	return cmd
}

func newLogsTailCmd(flags *rootFlags) *cobra.Command {
	var stream string
	var lines int
	var follow bool
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail the a2a-server log (stdout or stderr)",
		Long: `Tails ~/Library/Logs/openclaw-a2a-server/{stdout,stderr}.log. Path is hidden;
just say which --stream you want.`,
		Example: `  ori-pp-cli logs tail
  ori-pp-cli logs tail --stream stderr --lines 100
  ori-pp-cli logs tail --stream stdout --follow`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := logPathFor(stream)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would tail:", path)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would tail:", path)
				return nil
			}
			if _, statErr := os.Stat(path); statErr != nil {
				return notFoundErr(fmt.Errorf("log file not present at %s (a2a-server may not be loaded yet): %w", path, statErr))
			}
			tailArgs := []string{"-n", strconv.Itoa(lines)}
			if follow {
				tailArgs = append(tailArgs, "-F")
			}
			tailArgs = append(tailArgs, path)
			tc := exec.Command("tail", tailArgs...)
			tc.Stdout = cmd.OutOrStdout()
			tc.Stderr = cmd.ErrOrStderr()
			return tc.Run()
		},
	}
	cmd.Flags().StringVar(&stream, "stream", "stdout", "Log stream: stdout or stderr")
	cmd.Flags().IntVar(&lines, "lines", 50, "Lines to print from end before following")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow new output (tail -F)")
	return cmd
}

func newLogsPathCmd(flags *rootFlags) *cobra.Command {
	var stream string
	cmd := &cobra.Command{
		Use:     "path",
		Short:   "Print the log file path for scripting",
		Example: "  ori-pp-cli logs path --stream stderr",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := logPathFor(stream)
			if err != nil {
				return usageErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"stream": stream, "path": path}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&stream, "stream", "stdout", "Log stream: stdout or stderr")
	return cmd
}

func logPathFor(stream string) (string, error) {
	switch stream {
	case "stdout", "stderr":
	default:
		return "", fmt.Errorf("unknown --stream %q (valid: stdout, stderr)", stream)
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		// bufio import kept for symmetry with future log filtering features.
		_ = bufio.NewReader
		return "", fmt.Errorf("HOME not set")
	}
	return filepath.Join(home, "Library", "Logs", "openclaw-a2a-server", stream+".log"), nil
}
