package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/config"

	"github.com/spf13/cobra"
)

func newSetupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "First-run wizard: configure your SendFox token, default list, and sender details",
		Long: `setup walks you through everything you need to start sending newsletters:

  1. Your Personal Access Token from sendfox.com/account/oauth
  2. Your default subscriber list
  3. Your sender name and email address

All settings are saved to ~/.config/sendfox-pp-cli/config.json and can be
updated at any time by running setup again.`,
		Example:     "  sendfox-pp-cli setup",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil_isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would run setup wizard")
				return nil
			}

			w := cmd.OutOrStdout()
			r := bufio.NewReader(os.Stdin)

			fmt.Fprintln(w, "")
			fmt.Fprintln(w, bold("SendFox CLI Setup"))
			fmt.Fprintln(w, strings.Repeat("─", 40))
			fmt.Fprintln(w, "")

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				cfg = &config.Config{}
			}

			// Step 1: API token
			fmt.Fprintln(w, "Step 1: Personal Access Token")
			fmt.Fprintln(w, "  Get yours at: https://sendfox.com/account/oauth")
			fmt.Fprintln(w, "")

			existingToken := cfg.SendfoxToken
			if existingToken == "" {
				existingToken = os.Getenv("SENDFOX_TOKEN")
			}
			if existingToken != "" {
				fmt.Fprintf(w, "  Current token: %s (press Enter to keep)\n", maskToken(existingToken))
			}

			fmt.Fprint(w, "  Enter token: ")
			token, err := readLine(r)
			if err != nil {
				return fmt.Errorf("reading token: %w", err)
			}
			token = strings.TrimSpace(token)
			if token == "" && existingToken != "" {
				token = existingToken
				fmt.Fprintln(w, "  Keeping existing token.")
			}
			if token == "" {
				return fmt.Errorf("token is required; get yours at https://sendfox.com/account/oauth")
			}

			// Validate the token
			fmt.Fprint(w, "  Validating token... ")
			me, validateErr := validateSendFoxToken(token, cfg.BaseURL)
			if validateErr != nil {
				fmt.Fprintln(w, red("FAILED"))
				return fmt.Errorf("token validation failed: %w", validateErr)
			}
			fmt.Fprintf(w, "%s (account: %s)\n", green("OK"), me)

			// Save token now so subsequent API calls work
			cfg.SendfoxToken = token
			if err := cfg.SaveTokens("", "", token, "", cfg.TokenExpiry); err != nil {
				return fmt.Errorf("saving token: %w", err)
			}

			// Step 2: Default list
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Step 2: Default Subscriber List")
			lists, listErr := fetchAllLists(token, cfg.BaseURL)
			if listErr != nil {
				fmt.Fprintf(w, "  %s Could not fetch lists: %v\n", yellow("WARN"), listErr)
				fmt.Fprintln(w, "  You can set your default list later with: sendfox-pp-cli setup")
			} else if len(lists) == 0 {
				fmt.Fprintln(w, "  No lists found. Create one at sendfox.com, then re-run setup.")
			} else {
				fmt.Fprintln(w, "  Your lists:")
				for i, l := range lists {
					fmt.Fprintf(w, "    %d. %s (ID: %d)\n", i+1, l.Name, l.ID)
				}
				fmt.Fprintln(w, "")
				if cfg.DefaultListID != 0 {
					fmt.Fprintf(w, "  Current default: %s (ID: %d) — press Enter to keep\n", cfg.DefaultListName, cfg.DefaultListID)
				}
				fmt.Fprint(w, "  Choose list number (or press Enter to skip): ")
				choice, readErr := readLine(r)
				if readErr == nil {
					choice = strings.TrimSpace(choice)
					if choice != "" {
						idx, parseErr := strconv.Atoi(choice)
						if parseErr == nil && idx >= 1 && idx <= len(lists) {
							cfg.DefaultListID = lists[idx-1].ID
							cfg.DefaultListName = lists[idx-1].Name
							fmt.Fprintf(w, "  Default list set to: %s\n", cfg.DefaultListName)
						}
					}
				}
			}

			// Step 3: Sender name and email
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Step 3: Sender Details (used when writing AI campaigns)")

			fmt.Fprint(w, "  Your name (From Name): ")
			if cfg.FromName != "" {
				fmt.Fprintf(w, "[%s] ", cfg.FromName)
			}
			fromName, _ := readLine(r)
			fromName = strings.TrimSpace(fromName)
			if fromName != "" {
				cfg.FromName = fromName
			}

			fmt.Fprint(w, "  Your email (From Email): ")
			if cfg.FromEmail != "" {
				fmt.Fprintf(w, "[%s] ", cfg.FromEmail)
			}
			fromEmail, _ := readLine(r)
			fromEmail = strings.TrimSpace(fromEmail)
			if fromEmail != "" {
				cfg.FromEmail = fromEmail
			}

			// Save all settings
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Fprintln(w, "")
			fmt.Fprintln(w, strings.Repeat("─", 40))
			fmt.Fprintln(w, green("Setup complete!"))
			fmt.Fprintf(w, "Config saved to: %s\n", cfg.Path)
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Quick start:")
			fmt.Fprintln(w, `  sendfox-pp-cli write --topic "Your newsletter topic"`)
			fmt.Fprintln(w, `  sendfox-pp-cli campaigns list`)
			fmt.Fprintln(w, "")
			return nil
		},
	}
	return cmd
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return strings.Repeat("*", len(t))
	}
	return t[:4] + strings.Repeat("*", len(t)-8) + t[len(t)-4:]
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

type listSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func validateSendFoxToken(token, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = "https://sendfox.com/api"
	}
	data, err := rawGet(baseURL+"/me", token)
	if err != nil {
		return "", err
	}
	var me struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &me); err != nil {
		return "", fmt.Errorf("unexpected response from /me")
	}
	if me.Email != "" {
		return me.Email, nil
	}
	if me.Name != "" {
		return me.Name, nil
	}
	return "verified", nil
}

func fetchAllLists(token, baseURL string) ([]listSummary, error) {
	if baseURL == "" {
		baseURL = "https://sendfox.com/api"
	}
	data, err := rawGet(baseURL+"/lists", token)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []listSummary `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		// Try direct array
		var direct []listSummary
		if err2 := json.Unmarshal(data, &direct); err2 != nil {
			return nil, fmt.Errorf("parsing lists response: %w", err)
		}
		return direct, nil
	}
	return resp.Data, nil
}

// cliutil_isVerifyEnv checks the PRINTING_PRESS_VERIFY env var.
func cliutil_isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

// rawGet performs a simple authenticated GET request to the SendFox API.
func rawGet(url, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("invalid token (HTTP 401)")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
