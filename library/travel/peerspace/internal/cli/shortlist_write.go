// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-07-16: favorite write path) — create board + favorite listing via POST /v1/projects/attachments
// Novel commands backed by browser-captured favorite flows (HAR).
// Live-validated 2026-07-16: writes require Authorization: Bearer <PSAccess cookie>
// and add-to-board uses project as a bare board-id string (not project_id).

package cli

// pp:client-call — live POST to Peerspace favorites attachment endpoint

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

const favBoardNS = "FAV_BOARD"
const favAttachmentsPath = "/v1/projects/attachments"

// favCreateBoardBody is the HAR-verified create-board + save-listing payload.
// Observed 2026-07-16:
//
//	POST /v1/projects/attachments
//	{"ns":"FAV_BOARD","value":"<listing_id>","project":{"name":"...","activity":"...","location":"..."}}
type favCreateBoardBody struct {
	NS      string           `json:"ns"`
	Value   string           `json:"value"`
	Project favProjectCreate `json:"project"`
}

type favProjectCreate struct {
	Name     string `json:"name"`
	Activity string `json:"activity,omitempty"`
	Location string `json:"location,omitempty"`
}

// favAddListingBody attaches a listing to an existing favorite board.
// Live-validated 2026-07-16 against www.peerspace.com:
//
//	{"ns":"FAV_BOARD","value":"<listing_id>","project":"<board_id>"}
//
// Note: project is a JSON string (board id), not an object and not project_id.
// Using project_id returns HTTP 401 Invalid Permissions even with valid auth.
type favAddListingBody struct {
	NS      string `json:"ns"`
	Value   string `json:"value"`
	Project string `json:"project"`
}

func buildFavCreateBoardBody(listingID, name, activity, location string) favCreateBoardBody {
	return favCreateBoardBody{
		NS:    favBoardNS,
		Value: strings.TrimSpace(listingID),
		Project: favProjectCreate{
			Name:     strings.TrimSpace(name),
			Activity: strings.TrimSpace(activity),
			Location: strings.TrimSpace(location),
		},
	}
}

func buildFavAddListingBody(listingID, boardID string) favAddListingBody {
	return favAddListingBody{
		NS:      favBoardNS,
		Value:   strings.TrimSpace(listingID),
		Project: strings.TrimSpace(boardID),
	}
}

// psAccessBearerFromCookieHeader extracts the PSAccess cookie and returns
// "Bearer <value>" for Peerspace mutation endpoints. The SPA sends this on
// every authenticated request; cookie-only POSTs return 401 Invalid Permissions.
func psAccessBearerFromCookieHeader(cookieHeader string) string {
	cookieHeader = strings.TrimSpace(cookieHeader)
	if cookieHeader == "" {
		return ""
	}
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(name), "PSAccess") {
			continue
		}
		raw, err := url.QueryUnescape(strings.TrimSpace(val))
		if err != nil || raw == "" {
			raw = strings.TrimSpace(val)
		}
		if raw == "" {
			return ""
		}
		return "Bearer " + raw
	}
	return ""
}

func newNovelShortlistCreateBoardCmd(flags *rootFlags) *cobra.Command {
	var (
		flagName      string
		flagListingID string
		flagActivity  string
		flagLocation  string
	)

	cmd := &cobra.Command{
		Use:   "create-board",
		Short: "Create a Peerspace favorite board and attach a listing (cookie auth).",
		Long: `Create a favorite board via POST /v1/projects/attachments (ns=FAV_BOARD).

The website create-board flow always saves a listing in the same request, so
--listing-id is required. Activity and location are optional metadata on the board.

Requires browser session cookies (auth login --chrome).`,
		Example: `  peerspace-pp-cli shortlist create-board --name "Q3 meetup shortlist" --listing-id 68d468bb44492187e415d4a6 --activity Meetup --location "Paris, France"
  peerspace-pp-cli shortlist create-board --name "Studio picks" --listing-id 68d468bb44492187e415d4a6 --agent`,
		Annotations: map[string]string{
			"pp:endpoint":         "projects.attachments.create",
			"pp:method":           "POST",
			"pp:path":             favAttachmentsPath,
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Do not short-circuit on --dry-run: the client prints the POST
			// preview when c.DryRun is set via flags.newClient.
			name := strings.TrimSpace(flagName)
			listingID := strings.TrimSpace(flagListingID)
			if name == "" {
				return fmt.Errorf("required flag %q not set", "name")
			}
			if listingID == "" {
				return fmt.Errorf("required flag %q not set", "listing-id")
			}

			body := buildFavCreateBoardBody(listingID, name, flagActivity, flagLocation)
			return postFavAttachment(cmd, flags, body)
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "Favorite board name (required)")
	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing/space id to attach when creating the board (required)")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Board activity label (e.g. Meetup, Salon professionnel)")
	cmd.Flags().StringVar(&flagLocation, "location", "", "Board location label (e.g. Paris, France)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("listing-id")
	return cmd
}

func newNovelShortlistAddCmd(flags *rootFlags) *cobra.Command {
	var (
		flagListingID string
		flagBoardID   string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Favorite a space by attaching its listing id to an existing favorite board (cookie auth).",
		Long: `Save a listing onto an existing favorite board via POST /v1/projects/attachments.

Body shape: {"ns":"FAV_BOARD","value":"<listing_id>","project":"<board_id>"}.
Board ids come from projects details / pulse / create-board responses (project _id).

Requires browser session cookies (auth login --chrome). Writes also send
Authorization: Bearer <PSAccess> derived from the imported cookies.`,
		Example: `  peerspace-pp-cli shortlist add --listing-id 68d468bb44492187e415d4a6 --board-id 669152994300a86e4a943da5
  peerspace-pp-cli shortlist add --listing-id 68d468bb44492187e415d4a6 --board-id 669152994300a86e4a943da5 --agent`,
		Annotations: map[string]string{
			"pp:endpoint":         "projects.attachments.favorite",
			"pp:method":           "POST",
			"pp:path":             favAttachmentsPath,
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Do not short-circuit on --dry-run: the client prints the POST
			// preview when c.DryRun is set via flags.newClient.
			listingID := strings.TrimSpace(flagListingID)
			boardID := strings.TrimSpace(flagBoardID)
			if listingID == "" {
				return fmt.Errorf("required flag %q not set", "listing-id")
			}
			if boardID == "" {
				return fmt.Errorf("required flag %q not set", "board-id")
			}

			body := buildFavAddListingBody(listingID, boardID)
			return postFavAttachment(cmd, flags, body)
		},
	}

	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing/space id to favorite (required)")
	cmd.Flags().StringVar(&flagBoardID, "board-id", "", "Existing favorite board / project id (required)")
	_ = cmd.MarkFlagRequired("listing-id")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func postFavAttachment(cmd *cobra.Command, flags *rootFlags, body any) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	// Peerspace mutations require Authorization: Bearer <PSAccess> in addition
	// to the cookie jar. Cookie-only POSTs return 401 Invalid Permissions.
	headers := map[string]string{}
	if c.Config != nil {
		if bearer := psAccessBearerFromCookieHeader(c.Config.CookieCredential()); bearer != "" {
			headers["Authorization"] = bearer
		}
	}
	var data json.RawMessage
	var status int
	if len(headers) > 0 {
		data, status, err = c.PostWithHeaders(ctx, favAttachmentsPath, body, headers)
	} else {
		data, status, err = c.Post(ctx, favAttachmentsPath, body)
	}
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status >= 400 {
		return classifyAPIError(fmt.Errorf("POST %s returned HTTP %d: %s", favAttachmentsPath, status, truncateForErr(data, 240)), flags)
	}

	if flags.asJSON || wantsMachineOutput(flags) || !isTerminal(cmd.OutOrStdout()) {
		return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
	}

	// Human summary when the response matches the create-board shape.
	var env struct {
		Message    string `json:"message"`
		Attachment struct {
			ID        string `json:"_id"`
			ProjectID string `json:"project_id"`
			Value     string `json:"value"`
			NS        string `json:"ns"`
		} `json:"attachment"`
		Body struct {
			Project struct {
				ID   string `json:"_id"`
				Name string `json:"name"`
			} `json:"project"`
		} `json:"body"`
	}
	if json.Unmarshal(data, &env) == nil && env.Attachment.ID != "" {
		boardID := env.Attachment.ProjectID
		boardName := env.Body.Project.Name
		if boardName != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Saved listing %s on board %q (%s)\n", env.Attachment.Value, boardName, boardID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Saved listing %s on board %s (attachment %s)\n", env.Attachment.Value, boardID, env.Attachment.ID)
		}
		return nil
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
}

func truncateForErr(data json.RawMessage, n int) string {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return "(empty body)"
	}
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
