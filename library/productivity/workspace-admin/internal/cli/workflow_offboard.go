// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: workflow offboard.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

type offboardStep struct {
	Step   string `json:"step"`
	Status string `json:"status"` // ok | skipped | error | planned
	Detail string `json:"detail,omitempty"`
}

type offboardLedger struct {
	User     string         `json:"user"`
	Manager  string         `json:"manager,omitempty"`
	Executed bool           `json:"executed"`
	Steps    []offboardStep `json:"steps"`
}

func newNovelWorkflowOffboardCmd(flags *rootFlags) *cobra.Command {
	var flagManager string
	var flagSuspendedOU string
	var flagExecute bool

	cmd := &cobra.Command{
		Use:   "offboard <user>",
		Short: "Run a departing user's full lifecycle: suspend, sign out, revoke tokens, transfer Drive, delegate mail, remove from groups.",
		Long: "Execute a departing user's entire lifecycle in one command with per-step error isolation and a\n" +
			"completion ledger: suspend, sign out, revoke OAuth tokens, move to a suspended OU, delegate the\n" +
			"mailbox and transfer Drive ownership to a manager, and remove from all groups.\n\n" +
			"Safe by default: prints the ordered plan and makes NO changes unless --execute is passed.\n" +
			"Do NOT use single endpoint commands for a complete offboard; this sequences them and records\n" +
			"what completed.",
		Example:     "  workspace-admin-pp-cli workflow offboard departing@yourdomain.com --manager manager@yourdomain.com --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run the full offboard sequence (plan-only unless --execute)")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a user email is required"))
			}
			user := args[0]
			if !validUserKey(user) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%q is not a valid user email or directory ID", user))
			}
			execute := flagExecute && !cliutil.IsVerifyEnv()

			ledger := offboardLedger{User: user, Manager: flagManager, Executed: execute}

			// Plan-only path: describe the ordered steps, change nothing.
			if !execute {
				ledger.Steps = offboardPlan(user, flagManager, flagSuspendedOU)
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, ledger)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Offboard plan for %s (no changes made; pass --execute to run):\n", user)
				for i, s := range ledger.Steps {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s — %s\n", i+1, s.Step, s.Detail)
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ledger.Steps = runOffboard(ctx, c, user, flagManager, flagSuspendedOU)
			return flags.printJSON(cmd, ledger)
		},
	}
	cmd.Flags().StringVar(&flagManager, "manager", "", "Manager who receives Drive ownership and mailbox delegation")
	cmd.Flags().StringVar(&flagSuspendedOU, "suspended-ou", "", "Org unit path to move the user into (e.g. /Suspended)")
	cmd.Flags().BoolVar(&flagExecute, "execute", false, "Actually perform the offboard (default: plan only)")
	return cmd
}

// offboardPlan returns the ordered, human-readable plan without making calls.
func offboardPlan(user, manager, ou string) []offboardStep {
	steps := []offboardStep{
		{Step: "suspend", Status: "planned", Detail: "suspend " + user},
		{Step: "sign-out", Status: "planned", Detail: "revoke active sessions for " + user},
		{Step: "revoke-tokens", Status: "planned", Detail: "revoke all third-party OAuth tokens for " + user},
	}
	if ou != "" {
		steps = append(steps, offboardStep{Step: "move-ou", Status: "planned", Detail: "move " + user + " to " + ou})
	}
	if manager != "" {
		steps = append(steps,
			offboardStep{Step: "delegate-mail", Status: "planned", Detail: "delegate " + user + " mailbox to " + manager},
			offboardStep{Step: "transfer-drive", Status: "planned", Detail: "transfer " + user + " Drive ownership to " + manager},
		)
	}
	steps = append(steps, offboardStep{Step: "remove-from-groups", Status: "planned", Detail: "remove " + user + " from all groups"})
	return steps
}

// runOffboard performs the offboard with per-step error isolation. Each step is
// recorded; a failing step does not abort later steps.
func runOffboard(ctx context.Context, c *client.Client, user, manager, ou string) []offboardStep {
	var steps []offboardStep
	rec := func(step, detail string, err error) {
		s := offboardStep{Step: step, Detail: detail, Status: "ok"}
		if err != nil {
			s.Status = "error"
			s.Detail = err.Error()
		}
		steps = append(steps, s)
	}

	// 1. Suspend.
	_, _, err := c.Put(ctx, wsDirectoryBase+"/users/"+user, map[string]any{"suspended": true})
	rec("suspend", "suspended "+user, err)

	// 2. Sign out (revoke sessions).
	_, _, err = c.Post(ctx, wsDirectoryBase+"/users/"+user+"/signOut", nil)
	rec("sign-out", "revoked sessions", err)

	// 3. Revoke OAuth tokens.
	{
		var revoked, failed int
		var lastErr error
		if data, terr := c.Get(ctx, wsDirectoryBase+"/users/"+user+"/tokens", nil); terr == nil {
			var env struct {
				Items []tokenRow `json:"items"`
			}
			if json.Unmarshal(data, &env) == nil {
				for _, t := range env.Items {
					if _, _, derr := c.Delete(ctx, wsDirectoryBase+"/users/"+user+"/tokens/"+t.ClientID); derr != nil {
						failed++
						lastErr = derr
					} else {
						revoked++
					}
				}
			}
		} else {
			lastErr = terr
		}
		steps = append(steps, offboardStep{
			Step:   "revoke-tokens",
			Status: stepStatus(lastErr, failed),
			Detail: fmt.Sprintf("revoked %d tokens, %d failed", revoked, failed),
		})
	}

	// 4. Move OU.
	if ou != "" {
		_, _, err = c.Put(ctx, wsDirectoryBase+"/users/"+user, map[string]any{"orgUnitPath": ou})
		rec("move-ou", "moved to "+ou, err)
	}

	// 5 & 6. Manager-dependent steps.
	if manager != "" {
		_, _, err = c.Post(ctx, "https://gmail.googleapis.com/gmail/v1/users/"+user+"/settings/delegates",
			map[string]any{"delegateEmail": manager})
		rec("delegate-mail", "delegated mailbox to "+manager, err)

		rec("transfer-drive", "created Drive ownership transfer to "+manager, transferDrive(ctx, c, user, manager))
	}

	// 7. Remove from all groups. The Directory groups.list endpoint pages at
	// ~200 groups; walk every page before removing so a user in more than one
	// page of groups is fully removed rather than left in the remainder while
	// the step still reports success.
	{
		var removed, failed int
		var lastErr error
		pageToken := ""
		for {
			params := map[string]string{"userKey": user, "fields": "nextPageToken,groups(email)", "maxResults": "200"}
			if pageToken != "" {
				params["pageToken"] = pageToken
			}
			data, gerr := c.Get(ctx, wsDirectoryBase+"/groups", params)
			if gerr != nil {
				lastErr = gerr
				break
			}
			var env struct {
				NextPageToken string `json:"nextPageToken"`
				Groups        []struct {
					Email string `json:"email"`
				} `json:"groups"`
			}
			if json.Unmarshal(data, &env) != nil {
				break
			}
			for _, g := range env.Groups {
				if _, _, derr := c.Delete(ctx, wsDirectoryBase+"/groups/"+g.Email+"/members/"+user); derr != nil {
					failed++
					lastErr = derr
				} else {
					removed++
				}
			}
			if env.NextPageToken == "" || len(env.Groups) == 0 {
				break
			}
			pageToken = env.NextPageToken
		}
		steps = append(steps, offboardStep{
			Step:   "remove-from-groups",
			Status: stepStatus(lastErr, failed),
			Detail: fmt.Sprintf("removed from %d groups, %d failed", removed, failed),
		})
	}

	return steps
}

// transferDrive resolves the user IDs and the Drive transfer application, then
// creates a Data Transfer moving Drive ownership from user to manager.
func transferDrive(ctx context.Context, c *client.Client, user, manager string) error {
	oldID, err := lookupUserID(ctx, c, user)
	if err != nil {
		return fmt.Errorf("resolving %s id: %w", user, err)
	}
	newID, err := lookupUserID(ctx, c, manager)
	if err != nil {
		return fmt.Errorf("resolving %s id: %w", manager, err)
	}
	driveAppID, err := lookupDriveTransferAppID(ctx, c)
	if err != nil {
		return err
	}
	body := map[string]any{
		"oldOwnerUserId": oldID,
		"newOwnerUserId": newID,
		"applicationDataTransfers": []map[string]any{
			{"applicationId": driveAppID},
		},
	}
	_, _, err = c.Post(ctx, "https://admin.googleapis.com/admin/datatransfer/v1/transfers", body)
	return err
}

func lookupUserID(ctx context.Context, c *client.Client, user string) (string, error) {
	data, err := c.Get(ctx, wsDirectoryBase+"/users/"+user, map[string]string{"fields": "id"})
	if err != nil {
		return "", err
	}
	var u struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &u); err != nil || u.ID == "" {
		return "", fmt.Errorf("no id for %s", user)
	}
	return u.ID, nil
}

func lookupDriveTransferAppID(ctx context.Context, c *client.Client) (string, error) {
	data, err := c.Get(ctx, "https://admin.googleapis.com/admin/datatransfer/v1/applications", map[string]string{"customerId": "my_customer"})
	if err != nil {
		return "", err
	}
	var env struct {
		Applications []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"applications"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", err
	}
	for _, a := range env.Applications {
		if a.Name == "Drive and Docs" {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("Drive transfer application not found")
}

func stepStatus(err error, failed int) string {
	if err != nil || failed > 0 {
		return "error"
	}
	return "ok"
}
