package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

func newAuthCmd(g *globalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage MasterPark credentials",
	}
	cmd.AddCommand(newAuthCheckCmd(g), newAuthFrom1PasswordCmd(g), newAuthSyncProfileCmd(g))
	return cmd
}

func newAuthCheckCmd(g *globalOpts) *cobra.Command {
	cf := &credFlags{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Report which credential sources are available (never prints the password)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := g.loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := g.ctx()
			defer cancel()
			creds, err := config.Resolve(ctx, f, config.OnePassword{}, cf.input())
			var credErr string
			if err != nil {
				// A credential-fetch failure shouldn't make `auth check` fail;
				// report it as diagnostic output and continue with partial state.
				// The resolution error never carries the secret stdout, only the
				// command's exit status, so it is safe to surface.
				credErr = err.Error()
				// Resolve may return early before defaulting unset sources, so
				// normalize both to "none" instead of leaving them empty.
				if creds.UsernameSource == "" {
					creds.UsernameSource = config.SourceNone
				}
				if creds.PasswordSource == "" {
					creds.PasswordSource = config.SourceNone
				}
				fmt.Fprintf(os.Stderr, "credential resolution failed: %s\n", credErr)
			}
			result := map[string]interface{}{
				"username_present": creds.Username != "",
				"username_source":  creds.UsernameSource,
				"password_present": creds.Password != "",
				"password_source":  creds.PasswordSource,
				"username":         creds.Username,
			}
			if credErr != "" {
				result["credential_error"] = credErr
			}
			if g.json {
				return printJSON(result)
			}
			fmt.Printf("username: %s (source: %s)\n", presence(creds.Username != "", creds.Username), creds.UsernameSource)
			fmt.Printf("password: %s (source: %s)\n", presence(creds.Password != "", "********"), creds.PasswordSource)
			return nil
		},
	}
	addCredFlags(cmd, cf)
	return cmd
}

func presence(present bool, shown string) string {
	if present {
		return shown
	}
	return "<missing>"
}

func newAuthFrom1PasswordCmd(g *globalOpts) *cobra.Command {
	var (
		vault         string
		item          string
		usernameField string
		passwordField string
		lot           string
		loginCheck    bool
		save          bool
		syncProfile   bool
	)
	cmd := &cobra.Command{
		Use:   "from-1password",
		Short: "Load credentials from 1Password via the `op` CLI (never prints the password)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := g.ctx()
			defer cancel()
			op := config.OnePassword{}

			username, err := op.FetchField(ctx, vault, item, usernameField)
			if err != nil {
				return err
			}
			password, err := op.FetchField(ctx, vault, item, passwordField)
			if err != nil {
				return err
			}
			if username == "" || password == "" {
				return fmt.Errorf("1Password item %q is missing username or password fields", item)
			}

			// Verifier guard: when a login/profile check is requested, never hit
			// the live verifyLogin endpoint under PRINTING_PRESS_VERIFY. Match
			// `auth sync-profile`: return a verify no-op and write nothing.
			if (loginCheck || syncProfile) && IsVerifyEnv() {
				result := map[string]interface{}{
					"status":      "verify-noop",
					"verify_noop": true,
					"username":    username,
				}
				if g.json {
					return printJSON(result)
				}
				fmt.Printf("Loaded credentials for %s from 1Password (vault=%s, item=%s).\n", username, vault, item)
				fmt.Println("Password loaded into memory only; not printed or stored.")
				fmt.Println("VERIFY NO-OP (PRINTING_PRESS_VERIFY=1): not contacting MasterPark; login not validated and profile not synced.")
				return nil
			}

			loginValidated := false
			var profile *config.Profile
			if loginCheck || syncProfile {
				location, lerr := client.ResolveLot(lot)
				if lerr != nil {
					return lerr
				}
				profile, lerr = loginAndProfile(ctx, g.newClient(), username, password, location)
				if lerr != nil {
					return fmt.Errorf("verifyLogin rejected the credentials from 1Password: %w", lerr)
				}
				loginValidated = true
			}

			if save {
				f, lerr := g.loadConfig()
				if lerr != nil {
					return lerr
				}
				f.Username = username
				f.OnePassword = &config.OnePasswordRef{
					Vault:         vault,
					Item:          item,
					UsernameField: usernameField,
					PasswordField: passwordField,
				}
				if syncProfile && profile != nil {
					f.Profile = profile
				}
				if serr := config.Save(g.configPath, f); serr != nil {
					return serr
				}
			}

			vehicleCount := 0
			if profile != nil {
				vehicleCount = len(profile.Vehicles)
			}
			// profileFetched: --sync-profile logged in and parsed a profile, but
			// this alone does not mean anything was written to disk.
			// profilePersisted: the profile was actually saved to config and is
			// available to later `reserve --use-saved-profile` calls. Only this
			// reports as profile_synced so agents don't assume persistence.
			profileFetched := syncProfile && profile != nil
			profilePersisted := save && profileFetched
			result := map[string]interface{}{
				"status":          "ok",
				"username":        username,
				"password_loaded": true,
				"login_validated": loginValidated,
				"saved_metadata":  save,
				"profile_fetched": profileFetched,
				"profile_synced":  profilePersisted,
				"vehicles":        vehicleCount,
			}
			if g.json {
				return printJSON(result)
			}
			fmt.Printf("Loaded credentials for %s from 1Password (vault=%s, item=%s).\n", username, vault, item)
			fmt.Println("Password loaded into memory only; not printed or stored.")
			if loginValidated {
				fmt.Println("Login validated against verifyLogin.")
			}
			if save {
				fmt.Println("Saved non-secret 1Password reference to config.")
			}
			if profilePersisted {
				fmt.Printf("Saved non-secret profile (%d vehicle(s)).\n", vehicleCount)
			} else if profileFetched {
				fmt.Printf("Fetched non-secret profile (%d vehicle(s)) but did not save it; re-run with --save to persist it for `reserve --use-saved-profile`.\n", vehicleCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "Agent", "1Password vault")
	cmd.Flags().StringVar(&item, "item", "Masterparking", "1Password item")
	cmd.Flags().StringVar(&usernameField, "username-field", "username", "username field label")
	cmd.Flags().StringVar(&passwordField, "password-field", "password", "password field label")
	cmd.Flags().StringVar(&lot, "lot", "B", "lot used for the verifyLogin location (B, G, or a codeID)")
	cmd.Flags().BoolVar(&loginCheck, "login-check", false, "validate credentials by calling verifyLogin")
	cmd.Flags().BoolVar(&save, "save", false, "persist non-secret 1Password reference to config")
	cmd.Flags().BoolVar(&syncProfile, "sync-profile", false, "log in and save the non-secret customer profile + vehicles (implies a login)")
	return cmd
}

// verifyLoginWithClient calls the real verifyLogin ajax method. Returns true
// when the endpoint accepts the credentials. The live bundle calls
// O.login(model.login, model.password, model.location), so the payload posts
// {login, password, location}.
func verifyLoginWithClient(ctx context.Context, c *client.Client, username, password, location string) (bool, error) {
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
		return false, err
	}
	return len(resp.Errors) == 0, nil
}
