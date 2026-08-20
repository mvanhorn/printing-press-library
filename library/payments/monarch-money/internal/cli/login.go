package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var email, password, mfa string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Monarch Money and save a local session token.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				email = os.Getenv("MONARCH_EMAIL")
			}
			if password == "" {
				password = os.Getenv("MONARCH_PASSWORD")
			}
			if email == "" || password == "" {
				return fmt.Errorf("email and password are required; pass --email/--password or set MONARCH_EMAIL/MONARCH_PASSWORD")
			}
			token, err := loginRequest(email, password, mfa)
			if err != nil {
				return err
			}
			if err := saveToken(token); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Session saved to %s\n", sessionPath())
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Monarch email address (or MONARCH_EMAIL)")
	cmd.Flags().StringVar(&password, "password", "", "Monarch password; prefer MONARCH_PASSWORD to avoid shell history and process-list exposure")
	cmd.Flags().StringVar(&mfa, "mfa", "", "MFA/TOTP code when required")
	return cmd
}
