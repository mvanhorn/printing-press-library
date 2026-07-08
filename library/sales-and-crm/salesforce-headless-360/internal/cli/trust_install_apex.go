package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newTrustInstallApexCmd(flags *rootFlags) *cobra.Command {
	var orgAlias string
	cmd := &cobra.Command{
		Use:   "install-apex --org <alias>",
		Short: "Deploy the SF360 SafeRead, SafeWrite, and SafeUpsert Apex companions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgAlias == "" {
				return fmt.Errorf("--org is required")
			}
			if _, err := exec.LookPath("sf"); err != nil {
				return fmt.Errorf("sf CLI not found in PATH: install Salesforce CLI and authenticate an admin user for org %s", orgAlias)
			}
			version := exec.Command("sf", "--version")
			version.Stdout = cmd.OutOrStdout()
			version.Stderr = cmd.ErrOrStderr()
			if err := version.Run(); err != nil {
				return fmt.Errorf("checking sf --version: %w", err)
			}
			apexDir, err := findApexProjectDir()
			if err != nil {
				return err
			}
			deploy := exec.Command("sf", "project", "deploy", "start", "--target-org", orgAlias)
			deploy.Dir = apexDir
			deploy.Stdout = cmd.OutOrStdout()
			deploy.Stderr = cmd.ErrOrStderr()
			if err := deploy.Run(); err != nil {
				return fmt.Errorf("deploy Apex companion: %w\nhint: run as a Salesforce admin with permission to deploy Apex classes from %s", err, apexDir)
			}
			if err := markApexInstalled(orgAlias); err != nil {
				return fmt.Errorf("mark Apex installed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Apex companions installed for org=%s: SF360SafeRead, SF360SafeWrite, SF360SafeUpsert\n", orgAlias)
			return nil
		},
	}
	cmd.Flags().StringVar(&orgAlias, "org", "", "Org alias (required)")
	_ = cmd.MarkFlagRequired("org")
	return cmd
}

func findApexProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "apex", "sfdx-project.json")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, "apex"), nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
	}
	return "", fmt.Errorf("could not find apex/sfdx-project.json from %s", wd)
}

func markApexInstalled(orgAlias string) error {
	store, err := loadProfileStore()
	if err != nil {
		return err
	}
	profile := store.Profiles[orgAlias]
	if profile.Name == "" {
		profile.Name = orgAlias
	}
	if profile.Values == nil {
		profile.Values = map[string]string{}
	}
	now := time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.LastUsedAt = now
	profile.Values["apex_safe_read_installed"] = "true"
	profile.Values["apex_safe_read_installed_at"] = now.Format(time.RFC3339)
	profile.Values["apex_safe_write_installed"] = "true"
	profile.Values["apex_safe_write_installed_at"] = now.Format(time.RFC3339)
	profile.Values["apex_safe_upsert_installed"] = "true"
	profile.Values["apex_safe_upsert_installed_at"] = now.Format(time.RFC3339)
	store.Profiles[orgAlias] = profile
	return saveProfileStore(store)
}
