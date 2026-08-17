package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/fellow-stagg-ekg/internal/client"
	"github.com/spf13/cobra"
)

var version = "2026.7.1"

type config struct {
	baseURL string
	host    string
	port    int
	timeout float64
}

func Execute() error {
	return newRootCmd().Execute()
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return 2
}

func newRootCmd() *cobra.Command {
	cfg := &config{
		port:    envInt("FELLOW_STAGG_PORT", 80),
		timeout: envFloat("FELLOW_STAGG_TIMEOUT", 10),
	}

	rootCmd := &cobra.Command{
		Use:           "fellow-stagg-ekg-pp-cli [command]",
		Short:         "Control a Fellow Stagg EKG kettle over its local HTTP CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	rootCmd.PersistentFlags().StringVar(&cfg.baseURL, "base-url", os.Getenv("FELLOW_STAGG_URL"), "Full kettle base URL, for example http://192.168.1.86")
	rootCmd.PersistentFlags().StringVar(&cfg.host, "host", os.Getenv("FELLOW_STAGG_HOST"), "Kettle host or IP address")
	rootCmd.PersistentFlags().IntVar(&cfg.port, "port", cfg.port, "Kettle HTTP port")
	rootCmd.PersistentFlags().Float64Var(&cfg.timeout, "timeout", cfg.timeout, "Request timeout in seconds")

	rootCmd.AddCommand(newStatusCmd(cfg))
	rootCmd.AddCommand(newStateCmd(cfg))
	rootCmd.AddCommand(newSettingsCmd(cfg))
	rootCmd.AddCommand(newClockCmd(cfg))
	rootCmd.AddCommand(newInfoCmd(cfg))
	rootCmd.AddCommand(newHeatCmd(cfg))
	rootCmd.AddCommand(newOffCmd(cfg))
	rootCmd.AddCommand(newSetTempCmd(cfg))
	rootCmd.AddCommand(newSetSettingCmd(cfg))
	rootCmd.AddCommand(newUnitsCmd(cfg))
	rootCmd.AddCommand(newButtonCmd(cfg))
	rootCmd.AddCommand(newDialCmd(cfg))
	rootCmd.AddCommand(newBeepCmd(cfg))
	rootCmd.AddCommand(newRawCmd(cfg))
	return rootCmd
}

func newClient(cfg *config) (*client.Client, error) {
	baseURL, err := normalizeBaseURL(cfg.baseURL, cfg.host, cfg.port)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(cfg.timeout * float64(time.Second))
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return client.New(baseURL, timeout), nil
}

func normalizeBaseURL(baseURL, host string, port int) (string, error) {
	if baseURL != "" && host != "" {
		return "", fmt.Errorf("use either --base-url or --host, not both")
	}

	source := firstNonEmpty(baseURL, host, os.Getenv("FELLOW_STAGG_HOST"), os.Getenv("FELLOW_STAGG_URL"))
	if source == "" {
		return "", fmt.Errorf("provide --host, --base-url, FELLOW_STAGG_HOST, or FELLOW_STAGG_URL")
	}

	if strings.Contains(source, "://") {
		if strings.TrimSpace(source) == "" {
			return "", fmt.Errorf("invalid kettle URL: %q", source)
		}
		return strings.TrimRight(source, "/"), nil
	}

	if port == 80 {
		return "http://" + source, nil
	}
	return fmt.Sprintf("http://%s:%d", source, port), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func fetchAndPrint(cfg *config, command string) error {
	cli, err := newClient(cfg)
	if err != nil {
		return err
	}
	text, err := cli.Fetch(command)
	if err != nil {
		return err
	}
	printResponse(text)
	return nil
}

func fetchMany(cfg *config, commands ...string) error {
	for i, command := range commands {
		if i > 0 {
			fmt.Println()
		}
		if err := fetchAndPrint(cfg, command); err != nil {
			return err
		}
	}
	return nil
}

func printStatus(cfg *config) error {
	cli, err := newClient(cfg)
	if err != nil {
		return err
	}

	sections := []struct {
		label   string
		command string
	}{
		{label: "state", command: "state"},
		{label: "settings", command: "prtsettings"},
		{label: "firmware", command: "fwinfo"},
	}

	for i, section := range sections {
		text, err := cli.Fetch(section.command)
		if err != nil {
			return err
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(section.label)
		fmt.Println(strings.Repeat("-", len(section.label)))
		if section.command == "fwinfo" && text != "" && !strings.Contains(text, "=") {
			fmt.Println(text)
			continue
		}
		printResponse(text)
	}
	return nil
}

func printResponse(text string) {
	parsed := parseKeyValues(text)
	if len(parsed) > 0 {
		for _, line := range parsed {
			fmt.Println(line)
		}
		return
	}
	if strings.TrimSpace(text) != "" {
		fmt.Println(strings.TrimSpace(text))
	}
}

func parseKeyValues(text string) []string {
	cleaned := strings.TrimSpace(strings.TrimSuffix(text, "."))
	if cleaned == "" {
		return nil
	}

	parts := regexp.MustCompile(`,\s*|\n+`).Split(cleaned, -1)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || !strings.Contains(item, "=") {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", strings.TrimSpace(key), strings.TrimSpace(strings.TrimSuffix(value, "."))))
	}
	return lines
}

func newStatusCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show state, settings, and firmware info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printStatus(cfg)
		},
	}
}

func newStateCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "state",
		Short: "Fetch the kettle state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "state")
		},
	}
}

func newSettingsCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "settings",
		Short: "Fetch the kettle settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "prtsettings")
		},
	}
}

func newClockCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "clock",
		Short: "Fetch the kettle clock",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "prtclock")
		},
	}
}

func newInfoCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Fetch firmware info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "fwinfo")
		},
	}
}

func newHeatCmd(cfg *config) *cobra.Command {
	var temp float64
	cmd := &cobra.Command{
		Use:   "heat",
		Short: "Start heating",
		RunE: func(cmd *cobra.Command, args []string) error {
			if temp != 0 {
				return fetchMany(cfg, fmt.Sprintf("setsettingd settempr %g", temp), "setstate S_Heat")
			}
			return fetchAndPrint(cfg, "setstate S_Heat")
		},
	}
	cmd.Flags().Float64Var(&temp, "temp", 0, "Set the target temperature before heating")
	return cmd
}

func newOffCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Turn the kettle off",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "setstate S_Off")
		},
	}
}

func newSetTempCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "set-temp <temperature>",
		Short: "Set the target temperature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, "setsettingd settempr "+args[0])
		},
	}
}

func newSetSettingCmd(cfg *config) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "set-setting <name> <value>",
		Short: "Call setsetting, setsettingd, or setsettings",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandName := map[string]string{
				"int":    "setsetting",
				"double": "setsettingd",
				"string": "setsettings",
			}[kind]
			if commandName == "" {
				return fmt.Errorf("unknown setting kind: %s", kind)
			}
			return fetchAndPrint(cfg, commandName+" "+args[0]+" "+args[1])
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "int", "Command variant to use")
	return cmd
}

func newUnitsCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "units <value>",
		Short: "Switch the display units",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(args[0]) {
			case "c", "celsius":
				return fetchAndPrint(cfg, "setunitsc")
			case "f", "fahrenheit":
				return fetchAndPrint(cfg, "setunitsf")
			default:
				return fmt.Errorf("unknown units value: %s", args[0])
			}
		},
	}
}

func newButtonCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "button <button> [action]",
		Short: "Press or release kettle buttons",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			suffix := map[string]string{
				"press": "",
				"down":  "d",
				"up":    "u",
			}
			action := "press"
			if len(args) > 1 {
				action = strings.ToLower(args[1])
			}
			end := suffix[action]
			if action != "press" && end == "" {
				return fmt.Errorf("unknown button action: %s", action)
			}
			return fetchAndPrint(cfg, args[0]+end)
		},
	}
}

func newDialCmd(cfg *config) *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "dial <direction>",
		Short: "Rotate the kettle dial",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if count < 1 {
				count = 1
			}
			direction := strings.ToLower(args[0])
			if direction != "left" && direction != "right" {
				return fmt.Errorf("unknown dial direction: %s", args[0])
			}
			commands := make([]string, 0, count)
			for i := 0; i < count; i++ {
				commands = append(commands, direction)
			}
			return fetchMany(cfg, commands...)
		},
	}
	cmd.Flags().IntVar(&count, "count", 1, "Number of dial steps to send")
	return cmd
}

func newBeepCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "beep <pattern>...",
		Short: "Run buzzer commands",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.EqualFold(args[0], "sos") {
				return fetchAndPrint(cfg, "buz sos")
			}
			if len(args) == 3 {
				return fetchAndPrint(cfg, "buz "+strings.Join(args, " "))
			}
			return fmt.Errorf("beep expects either 'sos' or three buz arguments: frequency duty duration")
		},
	}
}

func newRawCmd(cfg *config) *cobra.Command {
	return &cobra.Command{
		Use:   "raw <parts>...",
		Short: "Send an arbitrary command string",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchAndPrint(cfg, strings.Join(args, " "))
		},
	}
}

