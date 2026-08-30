// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/spf13/cobra"
)

func newRecipesSharingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sharing", Short: "Inspect and manage recipe sharing links"}
	cmd.AddCommand(newRecipesSharingListCmd(flags))
	cmd.AddCommand(newRecipesSharingRequestCmd(flags))
	cmd.AddCommand(newRecipesSharingCancelCmd(flags))
	cmd.AddCommand(newRecipesSharingAcceptCmd(flags))
	cmd.AddCommand(newRecipesSharingUnlinkCmd(flags))
	return cmd
}

func newRecipesSharingListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List pending recipe-link requests and linked users",
		Example:     "  anylist-pp-cli recipes sharing list --json",
		Annotations: map[string]string{"pp:endpoint": "recipes.sharing.list", "pp:method": "POST", "pp:path": "/data/user-data/get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "pending": []any{}, "to_confirm": []any{}, "linked_users": []any{}}, "Dry run: would read recipe sharing state\n")
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return err
			}
			view := recipeSharingView(data)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return writeRecipeSharingText(cmd, view)
		},
	}
	return cmd
}

func newRecipesSharingRequestCmd(flags *rootFlags) *cobra.Command {
	var email string
	var apply bool
	cmd := &cobra.Command{
		Use:         "request",
		Short:       "Request recipe sharing with an email address",
		Example:     "  anylist-pp-cli recipes sharing request --email person@example.com --apply --json",
		Annotations: map[string]string{"pp:endpoint": "recipes.sharing.request", "pp:method": "POST", "pp:path": "/data/user-recipe-data/request-recipe-link-v2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := validateRecipeLinkEmail(email)
			if err != nil {
				return err
			}
			if !apply || flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "email": target, "apply": apply}, fmt.Sprintf("Dry run: would request recipe sharing with %q (pass --apply to write)\n", target))
			}
			cfg, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading current recipe sharing state: %w", err)
			}
			if _, err := resolveRecipeLinkRequest(data, "", target); err == nil {
				return fmt.Errorf("a pending recipe-link request for %q already exists", target)
			} else if !strings.Contains(err.Error(), "not found") {
				return err
			}
			request := &pb.PBRecipeLinkRequest{Identifier: uuid.NewString(), RequestingUserId: cfg.UserID, ConfirmingEmail: target}
			response, err := client.RequestRecipeLink(cmd.Context(), request)
			if err != nil {
				return err
			}
			verified, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying recipe-link request: %w", err)
			}
			if _, err := resolveRecipeLinkRequest(verified, request.Identifier, ""); err != nil {
				return fmt.Errorf("recipe-link request verification failed: %w", err)
			}
			result := map[string]any{"requested": true, "verified": true, "request": recipeLinkRequestView(request), "status_code": response.GetStatusCode()}
			return printWriteResult(cmd, flags, result, fmt.Sprintf("Requested recipe sharing with %q (request %s)\n", target, request.Identifier))
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Target user's exact email address")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live request")
	return cmd
}

func newRecipesSharingCancelCmd(flags *rootFlags) *cobra.Command {
	var requestID string
	var email string
	var apply bool
	cmd := &cobra.Command{
		Use:         "cancel",
		Short:       "Cancel a pending recipe-link request",
		Example:     "  anylist-pp-cli recipes sharing cancel --id REQUEST_ID --apply",
		Annotations: map[string]string{"pp:endpoint": "recipes.sharing.cancel", "pp:method": "POST", "pp:path": "/data/user-recipe-data/cancel-recipe-link-request"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(requestID) != "" && strings.TrimSpace(email) != "" {
				return fmt.Errorf("id and email are mutually exclusive")
			}
			if strings.TrimSpace(requestID) == "" && strings.TrimSpace(email) == "" {
				return fmt.Errorf("one of --id or --email is required")
			}
			if strings.TrimSpace(email) != "" {
				var err error
				email, err = validateRecipeLinkEmail(email)
				if err != nil {
					return err
				}
			}
			if !apply || flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "id": strings.TrimSpace(requestID), "email": email, "apply": apply}, "Dry run: would cancel the selected pending recipe-link request (pass --apply to write)\n")
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading pending recipe-link requests: %w", err)
			}
			request, err := resolveRecipeLinkRequest(data, requestID, email)
			if err != nil {
				return err
			}
			if _, err := client.CancelRecipeLink(cmd.Context(), request); err != nil {
				return err
			}
			verified, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying recipe-link cancellation: %w", err)
			}
			if _, err := resolveRecipeLinkRequest(verified, request.Identifier, ""); err == nil {
				return fmt.Errorf("recipe-link cancellation verification failed: request %q remains pending", request.Identifier)
			} else if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("recipe-link cancellation verification failed: %w", err)
			}
			return printWriteResult(cmd, flags, map[string]any{"cancelled": true, "verified": true, "request": recipeLinkRequestView(request)}, fmt.Sprintf("Cancelled recipe-link request %s\n", request.Identifier))
		},
	}
	cmd.Flags().StringVar(&requestID, "id", "", "Exact pending recipe-link request ID")
	cmd.Flags().StringVar(&email, "email", "", "Exact confirming email for a pending request")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live cancellation")
	return cmd
}

func newRecipesSharingAcceptCmd(flags *rootFlags) *cobra.Command {
	var requestID string
	var apply bool
	cmd := &cobra.Command{
		Use:         "accept",
		Short:       "Accept an incoming recipe-link request",
		Example:     "  anylist-pp-cli recipes sharing accept --id REQUEST_ID --apply",
		Annotations: map[string]string{"pp:endpoint": "recipes.sharing.accept", "pp:method": "POST", "pp:path": "/data/user-recipe-data/accept-recipe-link-request"},
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID = strings.TrimSpace(requestID)
			if requestID == "" {
				return fmt.Errorf("required flag \"id\" not set")
			}
			if !apply || flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "id": requestID, "apply": apply}, "Dry run: would accept the selected incoming recipe-link request (pass --apply to write)\n")
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading incoming recipe-link requests: %w", err)
			}
			request, err := resolveConfirmingRecipeLinkRequest(data, requestID)
			if err != nil {
				return err
			}
			if request.GetRequestingUserId() == "" {
				return fmt.Errorf("incoming recipe-link request %q has no requesting user ID", requestID)
			}
			if _, err := client.AcceptRecipeLink(cmd.Context(), requestID, request.GetRequestingUserId()); err != nil {
				return err
			}
			verified, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying recipe-link acceptance: %w", err)
			}
			if !linkedUserPresent(verified, request.GetRequestingUserId()) {
				return fmt.Errorf("recipe-link acceptance verification failed: user %q is not linked", request.GetRequestingUserId())
			}
			return printWriteResult(cmd, flags, map[string]any{"accepted": true, "verified": true, "request": recipeLinkRequestView(request)}, fmt.Sprintf("Accepted recipe-link request %s\n", requestID))
		},
	}
	cmd.Flags().StringVar(&requestID, "id", "", "Exact incoming recipe-link request ID")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live acceptance")
	return cmd
}

func newRecipesSharingUnlinkCmd(flags *rootFlags) *cobra.Command {
	var userID string
	var email string
	var apply bool
	cmd := &cobra.Command{
		Use:         "unlink",
		Short:       "Unlink recipes from a linked user",
		Example:     "  anylist-pp-cli recipes sharing unlink --user-id USER_ID --apply",
		Annotations: map[string]string{"pp:endpoint": "recipes.sharing.unlink", "pp:method": "POST", "pp:path": "/data/user-recipe-data/unlink-recipes"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(userID) != "" && strings.TrimSpace(email) != "" {
				return fmt.Errorf("user-id and email are mutually exclusive")
			}
			if strings.TrimSpace(userID) == "" && strings.TrimSpace(email) == "" {
				return fmt.Errorf("one of --user-id or --email is required")
			}
			if strings.TrimSpace(email) != "" {
				var err error
				email, err = validateRecipeLinkEmail(email)
				if err != nil {
					return err
				}
			}
			if !apply || flags.dryRun {
				return printJSONOrText(cmd, flags, map[string]any{"dry_run": true, "user_id": strings.TrimSpace(userID), "email": email, "apply": apply}, "Dry run: would unlink the selected recipe-sharing user (pass --apply to write)\n")
			}
			_, client, err := openAuthedRecipeClient(flags)
			if err != nil {
				return err
			}
			data, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading linked recipe users: %w", err)
			}
			resolvedID, err := resolveLinkedRecipeUser(data, userID, email)
			if err != nil {
				return err
			}
			if _, err := client.UnlinkRecipes(cmd.Context(), resolvedID); err != nil {
				return err
			}
			verified, err := client.GetUserData(cmd.Context())
			if err != nil {
				return fmt.Errorf("verifying recipe unlink: %w", err)
			}
			if linkedUserPresent(verified, resolvedID) {
				return fmt.Errorf("recipe unlink verification failed: user %q remains linked", resolvedID)
			}
			return printWriteResult(cmd, flags, map[string]any{"unlinked": true, "verified": true, "user_id": resolvedID}, fmt.Sprintf("Unlinked recipes from user %s\n", resolvedID))
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "Exact linked user ID")
	cmd.Flags().StringVar(&email, "email", "", "Exact linked user email")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable the live unlink")
	return cmd
}

func openAuthedRecipeClient(flags *rootFlags) (*config.Config, *anylist.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return nil, nil, authErr(fmt.Errorf("not authenticated — run 'anylist-pp-cli auth login' first"))
	}
	return cfg, anylist.New(cfg), nil
}

func validateRecipeLinkEmail(value string) (string, error) {
	raw := value
	value = strings.TrimSpace(value)
	if raw != value {
		return "", fmt.Errorf("invalid email address %q", raw)
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || !strings.Contains(value, "@") {
		return "", fmt.Errorf("invalid email address %q", value)
	}
	return strings.ToLower(value), nil
}

func resolveRecipeLinkRequest(data *pb.PBUserDataResponse, requestID, email string) (*pb.PBRecipeLinkRequest, error) {
	requestID = strings.TrimSpace(requestID)
	email = strings.ToLower(strings.TrimSpace(email))
	var matches []*pb.PBRecipeLinkRequest
	for _, request := range data.GetRecipeDataResponse().GetPendingRecipeLinkRequests() {
		if requestID != "" && request.GetIdentifier() == requestID {
			matches = append(matches, request)
		} else if email != "" && strings.EqualFold(request.GetConfirmingEmail(), email) {
			matches = append(matches, request)
		}
	}
	return oneRecipeLinkRequest(matches, "pending recipe-link request")
}

func resolveConfirmingRecipeLinkRequest(data *pb.PBUserDataResponse, requestID string) (*pb.PBRecipeLinkRequest, error) {
	var matches []*pb.PBRecipeLinkRequest
	for _, request := range data.GetRecipeDataResponse().GetRecipeLinkRequestsToConfirm() {
		if request.GetIdentifier() == requestID {
			matches = append(matches, request)
		}
	}
	return oneRecipeLinkRequest(matches, "incoming recipe-link request")
}

func oneRecipeLinkRequest(matches []*pb.PBRecipeLinkRequest, label string) (*pb.PBRecipeLinkRequest, error) {
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s not found", label)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s selector is ambiguous", label)
	}
	return matches[0], nil
}

func resolveLinkedRecipeUser(data *pb.PBUserDataResponse, userID, email string) (string, error) {
	userID = strings.TrimSpace(userID)
	email = strings.ToLower(strings.TrimSpace(email))
	var matches []*pb.PBEmailUserIDPair
	for _, user := range data.GetRecipeDataResponse().GetLinkedUsers() {
		if userID != "" && user.GetUserId() == userID {
			matches = append(matches, user)
		} else if email != "" && strings.EqualFold(user.GetEmail(), email) {
			matches = append(matches, user)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("linked recipe user not found")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("linked recipe user selector is ambiguous")
	}
	if matches[0].GetUserId() == "" {
		return "", fmt.Errorf("linked recipe user has no user ID")
	}
	return matches[0].GetUserId(), nil
}

func linkedUserPresent(data *pb.PBUserDataResponse, userID string) bool {
	for _, user := range data.GetRecipeDataResponse().GetLinkedUsers() {
		if user.GetUserId() == userID {
			return true
		}
	}
	return false
}

func recipeLinkRequestView(request *pb.PBRecipeLinkRequest) map[string]any {
	return map[string]any{
		"id":               request.GetIdentifier(),
		"requesting_user":  request.GetRequestingUserId(),
		"requesting_email": request.GetRequestingEmail(),
		"requesting_name":  request.GetRequestingName(),
		"confirming_user":  request.GetConfirmingUserId(),
		"confirming_email": request.GetConfirmingEmail(),
		"confirming_name":  request.GetConfirmingName(),
	}
}

func recipeSharingView(data *pb.PBUserDataResponse) map[string]any {
	rdr := data.GetRecipeDataResponse()
	pending := make([]map[string]any, 0, len(rdr.GetPendingRecipeLinkRequests()))
	for _, request := range rdr.GetPendingRecipeLinkRequests() {
		pending = append(pending, recipeLinkRequestView(request))
	}
	confirm := make([]map[string]any, 0, len(rdr.GetRecipeLinkRequestsToConfirm()))
	for _, request := range rdr.GetRecipeLinkRequestsToConfirm() {
		confirm = append(confirm, recipeLinkRequestView(request))
	}
	linked := make([]map[string]any, 0, len(rdr.GetLinkedUsers()))
	for _, user := range rdr.GetLinkedUsers() {
		linked = append(linked, map[string]any{"user_id": user.GetUserId(), "email": user.GetEmail(), "full_name": user.GetFullName()})
	}
	return map[string]any{"pending": pending, "to_confirm": confirm, "linked_users": linked}
}

func writeRecipeSharingText(cmd *cobra.Command, view map[string]any) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Pending requests:")
	for _, request := range view["pending"].([]map[string]any) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %v (%v)\n", request["confirming_email"], request["id"])
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Requests to confirm:")
	for _, request := range view["to_confirm"].([]map[string]any) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %v (%v)\n", request["requesting_email"], request["id"])
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Linked users:")
	for _, user := range view["linked_users"].([]map[string]any) {
		fmt.Fprintf(cmd.OutOrStdout(), "  %v (%v)\n", user["email"], user["user_id"])
	}
	return nil
}

func printJSONOrText(cmd *cobra.Command, flags *rootFlags, value map[string]any, text string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	fmt.Fprint(cmd.OutOrStdout(), text)
	return nil
}

func printWriteResult(cmd *cobra.Command, flags *rootFlags, value map[string]any, text string) error {
	if flags.quiet {
		return nil
	}
	return printJSONOrText(cmd, flags, value, text)
}
