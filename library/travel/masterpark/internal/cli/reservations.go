package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

func newReservationsCmd(g *globalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservations",
		Short: "Work with existing reservations",
	}
	cmd.AddCommand(newReservationsListCmd(g))
	return cmd
}

func newReservationsListCmd(g *globalOpts) *cobra.Command {
	var lot string
	cf := &credFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reservations for the authenticated customer",
		Long: "List reservations returned by MasterPark's profile endpoint.\n\n" +
			"Important: this endpoint is not authoritative for freshly-created reservations.\n" +
			"A successful saveReservation response and the confirmation email are the\n" +
			"source of truth immediately after booking; listReservations may lag or omit\n" +
			"new active reservations.",
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
				return fmt.Errorf("credentials required: pass --username/--password, --username-command/--password-command, set %s/%s, or use `auth from-1password --save`",
					config.EnvUsername, config.EnvPassword)
			}
			location, err := client.ResolveLot(lot)
			if err != nil {
				return err
			}

			// Verifier guard: reservations list authenticates with verifyLogin and
			// then calls listReservations, so it must no-op before touching live
			// MasterPark endpoints under PRINTING_PRESS_VERIFY.
			if IsVerifyEnv() {
				result := map[string]interface{}{
					"status":      "verify-noop",
					"verify_noop": true,
					"command":     "reservations list",
					"lot":         lot,
					"location":    location,
				}
				if g.json {
					return printJSON(result)
				}
				fmt.Println("VERIFY NO-OP (PRINTING_PRESS_VERIFY=1): not contacting MasterPark; reservations not listed.")
				return nil
			}

			c := g.newClient()
			// Establish an authenticated session against the real endpoint.
			ok, err := verifyLoginWithClient(ctx, c, creds.Username, creds.Password, location)
			if err != nil {
				return fmt.Errorf("verifyLogin failed: %w", err)
			}
			if !ok {
				return fmt.Errorf("verifyLogin rejected credentials")
			}

			resp, err := c.Ajax(ctx, map[string]interface{}{
				"action": "np_ajax",
				"method": "listReservations",
			})
			if err != nil {
				return fmt.Errorf("listReservations failed: %w", err)
			}
			if len(resp.Errors) > 0 {
				return fmt.Errorf("listReservations returned errors (endpoint likely needs a captured browser session): %v", resp.Errors)
			}
			return printRawJSON(resp.Data)
		},
	}
	cmd.Flags().StringVar(&lot, "lot", "B", "lot used for the login location (B, G, or a codeID)")
	addCredFlags(cmd, cf)
	return cmd
}
