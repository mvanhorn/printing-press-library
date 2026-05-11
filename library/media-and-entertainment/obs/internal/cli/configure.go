package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/config"
	"github.com/spf13/cobra"
)

var (
	configureHost           string
	configurePort           int
	configurePassword       string
	configureNonInteractive bool
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Set up OBS WebSocket connection settings",
	Long: `Configure obs-pp-cli with your OBS WebSocket server settings.
Settings are saved to ~/.obs-pp/config.json (chmod 600).
No credentials are stored in source code or environment variables.

Non-interactive mode (for agents and scripts):
  obs-pp-cli configure --non-interactive --host localhost --port 4455 --password ""`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if isDryRun() {
			fmt.Printf("[dry-run] Would save config: host=%s port=%d\n", configureHost, configurePort)
			return nil
		}

		if configureNonInteractive {
			cfg := config.Config{Host: configureHost, Port: configurePort, Password: configurePassword}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w\nhint: check permissions on ~/.obs-pp/", err)
			}
			fmt.Printf("✅ Config saved to ~/.obs-pp/config.json\n")
			fmt.Printf("   Host: %s  Port: %d  Auth: %v\n", cfg.Host, cfg.Port, cfg.Password != "")
			return nil
		}

		// Interactive mode
		defaults := config.DefaultConfig()
		r := bufio.NewReader(os.Stdin)

		fmt.Println("OBS WebSocket Configuration")
		fmt.Println("(Press Enter to accept defaults)")
		fmt.Println()

		hostInput := promptLine(r, fmt.Sprintf("Host [%s]: ", defaults.Host))
		if hostInput == "" {
			hostInput = defaults.Host
		}

		portInput := promptLine(r, fmt.Sprintf("Port [%d]: ", defaults.Port))
		portVal := defaults.Port
		if portInput != "" {
			p, err := strconv.Atoi(portInput)
			if err != nil {
				return fmt.Errorf("invalid port: %s\nhint: port must be a number, default is 4455", portInput)
			}
			portVal = p
		}

		fmt.Printf("Password (leave blank if authentication is disabled): ")
		passInput, _ := r.ReadString('\n')
		passInput = strings.TrimSpace(passInput)

		cfg := config.Config{
			Host:     hostInput,
			Port:     portVal,
			Password: passInput,
		}

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("\n✅ Config saved to ~/.obs-pp/config.json\n")
		fmt.Printf("   Host: %s  Port: %d  Auth: %v\n", cfg.Host, cfg.Port, cfg.Password != "")
		fmt.Printf("\nTest your connection with: obs-pp-cli doctor\n")
		return nil
	},
}

func init() {
	configureCmd.Flags().StringVar(&configureHost, "host", "localhost", "OBS WebSocket host address")
	configureCmd.Flags().IntVar(&configurePort, "port", 4455, "OBS WebSocket port number")
	configureCmd.Flags().StringVar(&configurePassword, "password", "", "WebSocket password (omit if authentication is disabled in OBS)")
	configureCmd.Flags().BoolVar(&configureNonInteractive, "non-interactive", false,
		"Skip interactive prompts; use flags directly (for scripts and agents)")
}

func promptLine(r *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := r.ReadString('\n')
	return strings.TrimSpace(input)
}
