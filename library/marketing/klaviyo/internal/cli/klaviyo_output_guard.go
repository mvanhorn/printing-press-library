// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

func validateMCPDestinations(cmd *cobra.Command, flags *rootFlags) error {
	if flags == nil || strings.TrimSpace(os.Getenv(mcpBoundProfileEnv)) == "" {
		return nil
	}
	for _, override := range []struct{ name, value string }{{"receipt-file", flags.receiptFile}, {"audit-dir", flags.auditDir}} {
		if override.value != "" || (cmd != nil && cmd.Flags().Changed(override.name)) {
			return fmt.Errorf("MCP tools may not override --%s", override.name)
		}
	}
	if strings.HasPrefix(flags.deliverSpec, "file:") || strings.HasPrefix(flags.deliverSpec, "webhook:") {
		return errors.New("MCP tools may not use --deliver file: or webhook:")
	}
	return nil
}

// Resolve missing output leaves too, including dangling symlinks into protected trees.

func resolveOutputPath(name string) (string, error) {
	name, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err == nil || !os.IsNotExist(err) {
		return resolved, err
	}
	if info, statErr := os.Lstat(name); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(name), target)
		}
		return resolveOutputPath(target)
	}
	parent, err := resolveOutputPath(filepath.Dir(name))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(name)), nil
}

func validateOutputDestination(destination string, flags *rootFlags) error {
	paths, err := platform.PathsFor("output-guard", "klaviyo-pp-cli", "klaviyo")
	if err != nil {
		return err
	}
	legacyRoot, err := config.RootDir()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// Protect every tenant, including profiles other than the selected one.
	protected := []string{
		filepath.Dir(filepath.Dir(filepath.Dir(paths.ProfileFile))),
		filepath.Dir(filepath.Dir(filepath.Dir(paths.DataFile))),
		filepath.Join(legacyRoot, "klaviyo-pp-cli"),
		filepath.Join(home, ".config", "klaviyo-pp-cli"),
		filepath.Join(home, ".klaviyo-pp-cli"),
		os.Getenv("KLAVIYO_CONFIG"),
	}
	if flags != nil {
		protected = append(protected, flags.configPath)
		if session := flags.platformSession; session != nil {
			protected = append(protected, session.Paths.ProfileFile, session.Paths.ConfigFile, session.Paths.DataFile)
		}
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return nil
	}
	resolved, err := resolveOutputPath(destination)
	if err != nil {
		// Ordinary filesystem failures belong to the final write, after command output and gate errors.
		return nil
	}
	for _, protectedPath := range protected {
		if protectedPath == "" {
			continue
		}
		root, err := filepath.Abs(protectedPath)
		if err != nil {
			continue
		}
		realRoot, err := resolveOutputPath(root)
		if err != nil {
			realRoot = root
		}
		for _, candidate := range []string{absolute, resolved} {
			for _, parent := range []string{root, realRoot} {
				if outputWithinProtectedPath(candidate, parent) {
					return errors.New("refusing output destination: configuration, credentials, and profile databases are protected")
				}
			}
		}
	}
	return nil
}

func outputWithinProtectedPath(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return true
	}
	// Filesystem identity also catches case aliases on case-insensitive volumes.
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false
	}
	for {
		if info, err := os.Stat(candidate); err == nil && os.SameFile(rootInfo, info) {
			return true
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return false
		}
		candidate = parent
	}
}

func validateReceiptDestinations(flags *rootFlags, writer *platform.ReceiptWriter) error {
	receipt := flags.receiptFile
	if receipt == "" {
		receipt = flags.platformSession.Paths.ReceiptsDir
		if writer != nil {
			receipt = filepath.Join(receipt, writer.Receipt.StartedAt.Format("2006/01/02"), writer.Receipt.RunID+".json")
		}
	}
	destinations := []string{receipt, flags.auditDir}
	if flags.auditDir != "" {
		destinations = append(destinations, filepath.Join(flags.auditDir, "index.jsonl"), filepath.Join(flags.auditDir, "index.jsonl.lock"), filepath.Join(flags.auditDir, "receipts"))
		if writer != nil {
			destinations = append(destinations, filepath.Join(flags.auditDir, "receipts", writer.Receipt.RunID+".json"))
		}
	}
	for _, destination := range destinations {
		if destination != "" {
			if err := validateOutputDestination(destination, flags); err != nil {
				return err
			}
		}
	}
	return nil
}
