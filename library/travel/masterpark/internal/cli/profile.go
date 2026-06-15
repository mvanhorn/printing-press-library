package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

// credFlags holds the generic, 1Password-independent credential overrides.
type credFlags struct {
	username        string
	password        string
	usernameCommand string
	passwordCommand string
}

// addCredFlags registers the generic credential flags on a command.
func addCredFlags(cmd *cobra.Command, cf *credFlags) {
	cmd.Flags().StringVar(&cf.username, "username", "", "username (overrides env/config)")
	cmd.Flags().StringVar(&cf.password, "password", "", "password (overrides env/config; never printed or stored)")
	cmd.Flags().StringVar(&cf.usernameCommand, "username-command", "",
		"command whose stdout is the username, e.g. \"op read op://Agent/Masterparking/username\" (run directly, no shell)")
	cmd.Flags().StringVar(&cf.passwordCommand, "password-command", "",
		"command whose stdout is the password (run directly, no shell; output never printed or stored)")
}

// input converts the flags into a config.CredInput for resolution.
func (cf *credFlags) input() config.CredInput {
	return config.CredInput{
		Username:        cf.username,
		Password:        cf.password,
		UsernameCommand: cf.usernameCommand,
		PasswordCommand: cf.passwordCommand,
	}
}

// parseLoginProfile extracts a non-secret customer profile + vehicles from a
// verifyLogin response payload. The parsing logic lives in the config package
// (config.ProfileFromLoginData) so it sits next to the Profile type; this thin
// wrapper keeps the cli call sites concise. It never extracts a password.
func parseLoginProfile(data json.RawMessage) *config.Profile {
	return config.ProfileFromLoginData(data)
}

// loginAndProfile performs verifyLogin and returns the parsed non-secret
// profile from the response. It returns an error if login is rejected.
func loginAndProfile(ctx context.Context, c *client.Client, username, password, location string) (*config.Profile, error) {
	if location == "" {
		location = "2515-1-889"
	}
	payload := map[string]interface{}{
		"action":   "np_ajax",
		"method":   "verifyLogin",
		"login":    username,
		"password": password,
		"location": location,
	}
	resp, err := c.Ajax(ctx, payload)
	if err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("verifyLogin rejected credentials")
	}
	return parseLoginProfile(resp.Data), nil
}

func newAuthSyncProfileCmd(g *globalOpts) *cobra.Command {
	cf := &credFlags{}
	var lot string
	cmd := &cobra.Command{
		Use:   "sync-profile",
		Short: "Log in and save the non-secret customer profile + vehicles to config (never stores the password)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := g.loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := g.ctx()
			defer cancel()

			creds, err := config.Resolve(ctx, f, config.OnePassword{}, cf.input())
			if err != nil {
				return err
			}
			if creds.Username == "" || creds.Password == "" {
				return fmt.Errorf("credentials required: pass --username/--password, --username-command/--password-command, set %s/%s, or configure 1Password",
					config.EnvUsername, config.EnvPassword)
			}
			location, err := client.ResolveLot(lot)
			if err != nil {
				return err
			}

			// Verifier guard: never hit live endpoints under PRINTING_PRESS_VERIFY.
			if IsVerifyEnv() {
				result := map[string]interface{}{
					"status":      "verify-noop",
					"verify_noop": true,
					"username":    creds.Username,
				}
				if g.json {
					return printJSON(result)
				}
				fmt.Println("VERIFY NO-OP (PRINTING_PRESS_VERIFY=1): not contacting MasterPark; profile not synced.")
				return nil
			}

			c := g.newClient()
			profile, err := loginAndProfile(ctx, c, creds.Username, creds.Password, location)
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			// Persist non-secret metadata only.
			f.Username = creds.Username
			if cf.usernameCommand != "" {
				f.UsernameCommand = cf.usernameCommand
			}
			if cf.passwordCommand != "" {
				f.PasswordCommand = cf.passwordCommand
			}
			if profile != nil {
				f.Profile = profile
			}
			if err := config.Save(g.configPath, f); err != nil {
				return err
			}

			vehicleCount := 0
			if profile != nil {
				vehicleCount = len(profile.Vehicles)
			}
			result := map[string]interface{}{
				"status":        "ok",
				"username":      creds.Username,
				"profile_saved": profile != nil,
				"vehicles":      vehicleCount,
			}
			if g.json {
				return printJSON(result)
			}
			fmt.Printf("Saved profile for %s (%d vehicle(s)). Password was not stored.\n", creds.Username, vehicleCount)
			return nil
		},
	}
	addCredFlags(cmd, cf)
	cmd.Flags().StringVar(&lot, "lot", "B", "lot used for the verifyLogin location (B, G, or a codeID)")
	return cmd
}
