package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/browser"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/linkedin"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/profile"
	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	var dbPath, fixture string
	var browserCDP, managedBrowser, chromeSession bool
	var cdpURL, profileURL, targetURLMatch, expectName, browserBin, browserUserDataDir string
	var browserPort, timeoutSeconds, loginWaitSeconds int
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture current profile sections into the local mirror from a fixture or user-controlled browser session.",
		Example: `profilepress snapshot --fixture profile.json
profilepress snapshot --browser-cdp --profile-url https://www.linkedin.com/in/example/ --expect-name "Example Person"
profilepress snapshot --managed-browser --profile-url https://www.linkedin.com/in/example/ --expect-name "Example Person"
profilepress snapshot --chrome-session --profile-url https://www.linkedin.com/in/example/ --expect-name "Example Person"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fixture != "" && (browserCDP || managedBrowser || chromeSession) {
				return errors.New("choose either --fixture or browser/session capture, not both")
			}
			modes := 0
			for _, enabled := range []bool{fixture != "", browserCDP, managedBrowser, chromeSession} {
				if enabled {
					modes++
				}
			}
			if modes > 1 {
				return errors.New("choose only one snapshot mode: --fixture, --browser-cdp, --managed-browser, or --chrome-session")
			}
			if managedBrowser {
				browserCDP = true
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
			} else if chromeSession {
				ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeoutSeconds+15)*time.Second)
				defer cancel()
				page, err := browser.CaptureWithChromeSession(ctx, profileURL, "linkedin.com", time.Duration(timeoutSeconds)*time.Second)
				if err != nil {
					return fmt.Errorf("capture LinkedIn profile with local Chrome session: %w", err)
				}
				loaded, loadedSource, err := linkedin.ParseProfilePageText(page, expectName)
				if err != nil {
					return err
				}
				sections = loaded
				source = loadedSource
			} else if browserCDP {
				ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeoutSeconds)*time.Second)
				defer cancel()
				if managedBrowser {
					session, err := browser.LaunchManaged(ctx, browser.ManagedOptions{Binary: browserBin, UserDataDir: browserUserDataDir, Port: browserPort, StartURL: profileURL, Timeout: time.Duration(timeoutSeconds) * time.Second})
					if err != nil {
						return fmt.Errorf("launch managed ProfilePress browser: %w", err)
					}
					cdpURL = session.CDPURL
					if targetURLMatch == "" {
						targetURLMatch = profileURL
					}
				}
				client := browser.NewClient(cdpURL, time.Duration(timeoutSeconds)*time.Second)
				page, err := client.CapturePageText(ctx, profileURL, targetURLMatch, 4*time.Second)
				if err != nil {
					return fmt.Errorf("capture LinkedIn profile through Chrome CDP: %w\nStart Chrome with remote debugging, for example: google-chrome --remote-debugging-port=9222", err)
				}
				loaded, loadedSource, err := linkedin.ParseProfilePageText(page, expectName)
				if err != nil && managedBrowser && loginWaitSeconds > 0 {
					fmt.Fprintf(os.Stderr, "ProfilePress browser is open. If LinkedIn asks you to sign in, complete sign-in there; waiting up to %ds for %q...\n", loginWaitSeconds, expectName)
					loaded, loadedSource, err = waitForManagedProfile(ctx, client, expectName, time.Duration(loginWaitSeconds)*time.Second)
				}
				if err != nil {
					return err
				}
				sections = loaded
				source = loadedSource
			} else {
				return errors.New("snapshot requires --fixture, --browser-cdp, --managed-browser, or --chrome-session")
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
	cmd.Flags().BoolVar(&chromeSession, "chrome-session", false, "capture by importing the existing local Chrome LinkedIn session without driving Chrome")
	cmd.Flags().BoolVar(&managedBrowser, "managed-browser", false, "launch/reuse a ProfilePress-owned isolated Chrome profile with local CDP enabled")
	cmd.Flags().StringVar(&cdpURL, "cdp-url", "http://127.0.0.1:9222", "Chrome DevTools Protocol base URL")
	cmd.Flags().StringVar(&browserBin, "browser-bin", "", "Chrome/Chromium binary for --managed-browser")
	cmd.Flags().StringVar(&browserUserDataDir, "browser-user-data-dir", "", "isolated browser profile directory for --managed-browser")
	cmd.Flags().IntVar(&browserPort, "browser-port", 0, "local debugging port for --managed-browser; 0 chooses a free port")
	cmd.Flags().StringVar(&profileURL, "profile-url", "", "LinkedIn profile URL to open or verify before capture")
	cmd.Flags().StringVar(&targetURLMatch, "target-url-match", "", "substring used to select an existing Chrome target")
	cmd.Flags().StringVar(&expectName, "expect-name", "", "expected profile name; capture fails if absent from title/body")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 15, "browser/session capture timeout in seconds")
	cmd.Flags().IntVar(&loginWaitSeconds, "login-wait", 0, "seconds to wait for login in --managed-browser fallback")
	return cmd
}
