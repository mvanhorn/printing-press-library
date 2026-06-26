package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/browser"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/profile"
	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	var dbPath, fixture string
	var browserCDP bool
	var cdpURL, profileURL, targetURLMatch, expectName string
	var timeoutSeconds int
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture current profile sections into the local mirror from a fixture or user-controlled browser session.",
		Example: `profilepress snapshot --fixture profile.json
profilepress snapshot --browser-cdp --profile-url https://www.linkedin.com/in/example/ --expect-name "Example Person"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fixture != "" && browserCDP {
				return errors.New("choose either --fixture or --browser-cdp, not both")
			}
			var sections []profile.Section
			var source string
			if fixture != "" {
				loaded, loadedSource, err := linkedin.LoadSnapshotFixture(fixture)
				if err != nil {
					return err
				}
				sections = loaded
				source = loadedSource
			} else if browserCDP {
				ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeoutSeconds)*time.Second)
				defer cancel()
				client := browser.NewClient(cdpURL, time.Duration(timeoutSeconds)*time.Second)
				page, err := client.CapturePageText(ctx, profileURL, targetURLMatch, 4*time.Second)
				if err != nil {
					return fmt.Errorf("capture LinkedIn profile through Chrome CDP: %w\nStart Chrome with remote debugging, for example: google-chrome --remote-debugging-port=9222", err)
				}
				loaded, loadedSource, err := linkedin.ParseProfilePageText(page, expectName)
				if err != nil {
					return err
				}
				sections = loaded
				source = loadedSource
			} else {
				return errors.New("snapshot requires --fixture or --browser-cdp; for browser capture start Chrome with --remote-debugging-port=9222")
			}
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snap, err := db.CreateSnapshot(source, sections)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"snapshot_id": snap.ID, "sections": len(snap.Sections), "source": snap.Source})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&fixture, "fixture", "", "JSON fixture with profile sections")
	cmd.Flags().BoolVar(&browserCDP, "browser-cdp", false, "capture from a user-controlled Chrome/Chromium session via local CDP")
	cmd.Flags().StringVar(&cdpURL, "cdp-url", "http://127.0.0.1:9222", "Chrome DevTools Protocol base URL")
	cmd.Flags().StringVar(&profileURL, "profile-url", "", "LinkedIn profile URL to open or verify before capture")
	cmd.Flags().StringVar(&targetURLMatch, "target-url-match", "", "substring used to select an existing Chrome target")
	cmd.Flags().StringVar(&expectName, "expect-name", "", "expected profile name; capture fails if absent from title/body")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 15, "browser capture timeout in seconds")
	return cmd
}
