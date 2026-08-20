// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// lookup.go declares the `lookup` command: live contact enrichment for an
// email address via Superhuman's /v2/profile endpoint, with an optional
// photo download via /contact/<email>/photo. Both endpoints accept the
// CLI's existing cookie-derived JWT (no iOS-audience token required).
//
// PATCH(2026-05-27-005 U1): new lookup command sourced from the 2026-05-27
// iOS sniff (verified live against /v2/profile and /contact/<email>/photo).

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
)

// contactProfile mirrors the /v2/profile response. Fields the backend omits
// for sparse contacts decode to their zero value and are dropped from output.
type contactProfile struct {
	Email         string        `json:"email,omitempty"`
	Name          string        `json:"name,omitempty"`
	Bio           string        `json:"bio,omitempty"`
	BioFrom       string        `json:"bioFrom,omitempty"`
	Location      string        `json:"location,omitempty"`
	TimeZone      string        `json:"timeZone,omitempty"`
	Avatar        string        `json:"avatar,omitempty"`
	Links         []contactLink `json:"links,omitempty"`
	MemberSince   string        `json:"memberSince,omitempty"`
	TwitterHandle string        `json:"twitterHandle,omitempty"`
	CanBeReferred bool          `json:"canBeReferred,omitempty"`
	CanRefer      bool          `json:"canRefer,omitempty"`
	SalesData     *salesData    `json:"salesData,omitempty"`
}

type contactLink struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Logo  string `json:"logo,omitempty"`
}

type salesData struct {
	Sources []salesSource `json:"sources,omitempty"`
}

type salesSource struct {
	Name string `json:"name,omitempty"`
}

func newLookupCmd(flags *rootFlags) *cobra.Command {
	var photoPath string
	cmd := &cobra.Command{
		Use:   "lookup <email>",
		Short: "Look up Superhuman's live contact enrichment for an email",
		Long: `Fetch Superhuman's live enrichment for a contact email: name, bio,
location, timezone, avatar URL, social links, Twitter handle, and member-since.

This reads /v2/profile and works with the CLI's existing authentication (no
extra setup). With --photo, it also downloads the contact's photo (JPEG) from
/contact/<email>/photo to the given file path; contacts with no photo on file
report that and exit 0 rather than erroring.`,
		Example: strings.Trim(`
  superhuman-pp-cli lookup alice@example.com
  superhuman-pp-cli lookup alice@example.com --json --select name,location,twitterHandle
  superhuman-pp-cli lookup alice@example.com --photo ./alice.jpg`, "\n"),
		Annotations: map[string]string{
			"pp:endpoint":   "contact.profile",
			"pp:method":     "GET",
			"pp:path":       "/v2/profile",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			email := strings.TrimSpace(args[0])
			if !strings.Contains(email, "@") {
				return usageErr(fmt.Errorf("lookup: %q is not a valid email address", email))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET /v2/profile?email=%s\n", email)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			return runLookup(cmd, flags, c, email, photoPath)
		},
	}
	cmd.Flags().StringVar(&photoPath, "photo", "", "Download the contact's photo (JPEG) to this file path")
	return cmd
}

func runLookup(cmd *cobra.Command, flags *rootFlags, c *client.Client, email, photoPath string) error {
	data, err := c.Get("/v2/profile", map[string]string{"email": email})
	if err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			return authErr(fmt.Errorf("lookup: %w", err))
		}
		return apiErr(fmt.Errorf("lookup: fetch profile for %s: %w", email, err))
	}

	var prof contactProfile
	if err := json.Unmarshal(data, &prof); err != nil {
		return apiErr(fmt.Errorf("lookup: parse profile response: %w", err))
	}
	if prof.Email == "" {
		prof.Email = email
	}

	// Emit the profile before attempting the optional photo so a photo
	// failure can never discard the profile the user asked for. Photo
	// status is reported on stderr, keeping stdout a clean profile payload.
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if err := printJSONFiltered(cmd.OutOrStdout(), prof, flags); err != nil {
			return err
		}
	} else if err := printContactCard(cmd, prof); err != nil {
		return err
	}

	if photoPath != "" {
		return downloadContactPhoto(cmd, c, email, photoPath)
	}
	return nil
}

// downloadContactPhoto writes the contact's JPEG to path. The photo is an
// optional artifact: any failure to FETCH it (404, auth, 5xx, transport) is
// reported on stderr and treated as non-fatal so the already-emitted profile
// is never invalidated by a photo problem. Only a local WRITE failure is
// returned as a hard error, since that is the caller's filesystem and worth
// a non-zero exit.
func downloadContactPhoto(cmd *cobra.Command, c *client.Client, email, path string) error {
	photo, err := c.Get("/contact/"+url.PathEscape(email)+"/photo", nil)
	if err != nil {
		var apiE *client.APIError
		switch {
		case errors.As(err, &apiE) && apiE.StatusCode == 404:
			fmt.Fprintf(cmd.ErrOrStderr(), "no photo on file for %s\n", email)
		case errors.Is(err, auth.ErrUnauthorized):
			fmt.Fprintf(cmd.ErrOrStderr(), "could not download photo for %s: not authorized\n", email)
		default:
			fmt.Fprintf(cmd.ErrOrStderr(), "could not download photo for %s: %v\n", email, err)
		}
		return nil
	}
	if len(photo) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "no photo on file for %s\n", email)
		return nil
	}
	if err := os.WriteFile(path, photo, 0o600); err != nil {
		return fmt.Errorf("lookup --photo: write %s: %w", path, err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bytes to %s\n", len(photo), path)
	return nil
}

func printContactCard(cmd *cobra.Command, p contactProfile) error {
	w := cmd.OutOrStdout()
	if p.Name != "" {
		fmt.Fprintf(w, "%s <%s>\n", p.Name, p.Email)
	} else {
		fmt.Fprintf(w, "%s\n", p.Email)
	}
	if p.Bio != "" {
		from := ""
		if p.BioFrom != "" {
			from = " (via " + p.BioFrom + ")"
		}
		fmt.Fprintf(w, "  bio: %s%s\n", p.Bio, from)
	}
	if p.Location != "" {
		fmt.Fprintf(w, "  location: %s\n", p.Location)
	}
	if p.TimeZone != "" {
		fmt.Fprintf(w, "  timezone: %s\n", p.TimeZone)
	}
	if p.TwitterHandle != "" {
		fmt.Fprintf(w, "  twitter: @%s\n", p.TwitterHandle)
	}
	if p.MemberSince != "" {
		fmt.Fprintf(w, "  member since: %s\n", p.MemberSince)
	}
	for _, l := range p.Links {
		fmt.Fprintf(w, "  %s: %s\n", l.Title, l.URL)
	}
	if p.Avatar != "" {
		fmt.Fprintf(w, "  avatar: %s\n", p.Avatar)
	}
	return nil
}
