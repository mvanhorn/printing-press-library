package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/cliutil"
	"github.com/spf13/cobra"
)

type bambuPrinterProfile struct {
	Name          string `json:"name"`
	Host          string `json:"host,omitempty"`
	Location      string `json:"location,omitempty"`
	SerialEnv     string `json:"serial_env"`
	AccessCodeEnv string `json:"access_code_env"`
}

type bambuPrinterProfileStore struct {
	Profiles map[string]bambuPrinterProfile `json:"profiles"`
}

var envNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
var printerCertificateIdentityPattern = regexp.MustCompile(`(?i)CN[=: ]+[A-Za-z0-9]{12,20}`)

func sanitizePrinterError(err error, serial string) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if serial != "" {
		message = strings.ReplaceAll(message, serial, redactSerial(serial))
	}
	return printerCertificateIdentityPattern.ReplaceAllString(message, "CN=[redacted-printer-id]")
}

func bambuPrinterProfilePath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "printer-profiles.json"), nil
}
func loadBambuPrinterProfiles() (*bambuPrinterProfileStore, error) {
	path, err := bambuPrinterProfilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the CLI config directory plus the fixed printer-profiles.json filename.
	if os.IsNotExist(err) {
		return &bambuPrinterProfileStore{Profiles: map[string]bambuPrinterProfile{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var store bambuPrinterProfileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse printer profiles: %w", err)
	}
	if store.Profiles == nil {
		store.Profiles = map[string]bambuPrinterProfile{}
	}
	return &store, nil
}
func saveBambuPrinterProfiles(store *bambuPrinterProfileStore) error {
	path, err := bambuPrinterProfilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

func resolveNamedBambuPrinter(ctx context.Context, name string) (deviceContext, error) {
	store, err := loadBambuPrinterProfiles()
	if err != nil {
		return deviceContext{}, configErr(err)
	}
	profile, ok := store.Profiles[name]
	if !ok {
		return deviceContext{}, notFoundErr(fmt.Errorf("printer profile %q was not found; run profile printer-list", name))
	}
	serial := strings.TrimSpace(os.Getenv(profile.SerialEnv))
	accessCode := strings.TrimSpace(os.Getenv(profile.AccessCodeEnv))
	if serial == "" || accessCode == "" {
		return deviceContext{}, authErr(fmt.Errorf("printer profile %q requires environment variables %s and %s", name, profile.SerialEnv, profile.AccessCodeEnv))
	}
	if err := bambu.ValidateSerial(serial); err != nil {
		return deviceContext{}, authErr(fmt.Errorf("printer profile %q has invalid serial in %s: %w", name, profile.SerialEnv, err))
	}
	host := strings.TrimSpace(profile.Host)
	discovery := bambu.Discovery{Host: host, Serial: serial, Name: profile.Name}
	if host != "" {
		if err := bambu.ValidatePrivateIP(host); err != nil {
			return deviceContext{}, usageErr(err)
		}
	} else {
		found, err := bambu.Discover(ctx, serial)
		if err != nil {
			return deviceContext{}, apiErr(err)
		}
		if len(found) == 0 {
			return deviceContext{}, notFoundErr(fmt.Errorf("printer profile %q was not found by SSDP", name))
		}
		discovery, host = found[0], found[0].Host
	}
	return deviceContext{Host: host, Serial: serial, AccessCode: accessCode, Discovery: discovery}, nil
}

func newBambuProfileAddCmd(flags *rootFlags) *cobra.Command {
	var name, host, location, serialEnv, accessEnv string
	cmd := &cobra.Command{Use: "printer-add", Short: "Add a printer profile using environment-variable references, never credential values", Example: "  bambu-pp-cli profile printer-add --name workshop --serial-env WORKSHOP_BAMBU_SERIAL --access-code-env WORKSHOP_BAMBU_ACCESS_CODE", RunE: func(cmd *cobra.Command, _ []string) error {
		if name == "" || !envNamePattern.MatchString(serialEnv) || !envNamePattern.MatchString(accessEnv) {
			return usageErr(fmt.Errorf("--name and uppercase --serial-env/--access-code-env names are required"))
		}
		if host != "" {
			if err := bambu.ValidatePrivateIP(host); err != nil {
				return usageErr(err)
			}
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_add": name, "serial_env": serialEnv, "access_code_env": accessEnv}, flags)
		}
		store, err := loadBambuPrinterProfiles()
		if err != nil {
			return configErr(err)
		}
		store.Profiles[name] = bambuPrinterProfile{Name: name, Host: host, Location: location, SerialEnv: serialEnv, AccessCodeEnv: accessEnv}
		if err := saveBambuPrinterProfiles(store); err != nil {
			return configErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), store.Profiles[name], flags)
	}}
	cmd.Flags().StringVar(&name, "name", "", "Stable printer profile name")
	cmd.Flags().StringVar(&host, "host", "", "Optional private-LAN host; empty uses SSDP")
	cmd.Flags().StringVar(&location, "location", "", "Optional human-readable printer location")
	cmd.Flags().StringVar(&serialEnv, "serial-env", "", "Environment variable containing this printer serial")
	cmd.Flags().StringVar(&accessEnv, "access-code-env", "", "Environment variable containing this printer local access code")
	return cmd
}
func newBambuProfileListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "printer-list", Short: "List secret-safe printer profiles and credential variable names", Example: "  bambu-pp-cli profile printer-list --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		store, err := loadBambuPrinterProfiles()
		if err != nil {
			return configErr(err)
		}
		items := make([]bambuPrinterProfile, 0, len(store.Profiles))
		for _, profile := range store.Profiles {
			items = append(items, profile)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return printJSONFiltered(cmd.OutOrStdout(), items, flags)
	}}
}
func newBambuProfileDeleteCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "printer-delete", Short: "Delete a printer profile without touching its environment variables", Example: "  bambu-pp-cli profile printer-delete --name workshop --dry-run", RunE: func(cmd *cobra.Command, _ []string) error {
		if name == "" {
			return usageErr(fmt.Errorf("--name is required"))
		}
		store, err := loadBambuPrinterProfiles()
		if err != nil {
			return configErr(err)
		}
		if _, ok := store.Profiles[name]; !ok {
			return notFoundErr(fmt.Errorf("printer profile %q was not found", name))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_delete": name}, flags)
		}
		delete(store.Profiles, name)
		if err := saveBambuPrinterProfiles(store); err != nil {
			return configErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": name}, flags)
	}}
	cmd.Flags().StringVar(&name, "name", "", "Printer profile name to delete")
	return cmd
}
