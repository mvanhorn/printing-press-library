package commands

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/config"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Set up OBS WebSocket connection",
	Long: `Configure obs-pp-cli with your OBS WebSocket server settings.
Settings are saved to ~/.obs-pp/config.json (chmod 600).
No credentials are stored in source code or environment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check for flags first (non-interactive mode for agents)
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		password, _ := cmd.Flags().GetString("password")
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

		if nonInteractive {
			cfg := config.Config{Host: host, Port: port, Password: password}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
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

		hostInput := prompt(r, fmt.Sprintf("Host [%s]: ", defaults.Host))
		if hostInput == "" {
			hostInput = defaults.Host
		}

		portInput := prompt(r, fmt.Sprintf("Port [%d]: ", defaults.Port))
		portVal := defaults.Port
		if portInput != "" {
			p, err := strconv.Atoi(portInput)
			if err != nil {
				return fmt.Errorf("invalid port: %s", portInput)
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
		fmt.Printf("\nTest your connection with: obs-pp-cli scene list\n")
		return nil
	},
}

func init() {
	configureCmd.Flags().String("host", "localhost", "OBS WebSocket host")
	configureCmd.Flags().Int("port", 4455, "OBS WebSocket port")
	configureCmd.Flags().String("password", "", "OBS WebSocket password (omit if auth disabled)")
	configureCmd.Flags().Bool("non-interactive", false, "Skip prompts, use flags directly")
}

func prompt(r *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := r.ReadString('\n')
	return strings.TrimSpace(input)
}
