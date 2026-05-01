package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/atom"
	"github.com/spf13/cobra"
)

func newGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var live bool
	var external bool
	var urlOnly bool

	cmd := &cobra.Command{
		Use:     "get <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Aliases: []string{"info"},
		Short:   "Show the post for a given slug",
		Long: `Return the stored post for a Product Hunt slug. By default reads from
the local store and falls back to a live /feed scan when the slug isn't
there yet. --live forces a live fetch.

Output is the stable post payload (id, slug, title, tagline, author,
discussion_url, external_url, published, updated).`,
		Example: `  # Look up a slug from your local store
  producthunt-pp-cli get seeknal

  # Force a live /feed scan
  producthunt-pp-cli get cavalry-2 --live

  # Just the external URL (for piping into open)
  producthunt-pp-cli get seeknal --url-only

  # External URL only, as JSON
  producthunt-pp-cli get seeknal --external --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			if !live {
				db, err := openStore(dbPath)
				if err == nil {
					defer db.Close()
					p, err := db.GetPostBySlug(slug)
					if err == nil && p != nil {
						return emitInfo(cmd, flags, postToJSON(*p), p.ExternalURL, p.DiscussionURL, urlOnly, external)
					}
					if err != nil && !errors.Is(err, sql.ErrNoRows) {
						return apiErr(err)
					}
				}
			}

			body, err := fetchFeedBody(flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			feed, err := atom.Parse(body)
			if err != nil {
				return apiErr(fmt.Errorf("parse feed: %w", err))
			}
			for _, e := range feed.Entries {
				if e.Slug == slug {
					pp := atomEntryToPayload(e, 0)
					buf, _ := json.Marshal(pp)
					return emitInfo(cmd, flags, buf, e.ExternalURL, e.DiscussionURL, urlOnly, external)
				}
			}
			return notFoundErr(fmt.Errorf("slug %q not found in store or live feed", slug))
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	cmd.Flags().BoolVar(&live, "live", false, "Force a live /feed fetch, bypass store")
	cmd.Flags().BoolVar(&external, "external", false, "Emit only the external URL")
	cmd.Flags().BoolVar(&urlOnly, "url-only", false, "Emit only the URL (useful for 'producthunt-pp-cli get <slug> --url-only | xargs open')")
	return cmd
}

func emitInfo(cmd *cobra.Command, flags *rootFlags, payload json.RawMessage, externalURL, discussionURL string, urlOnly, external bool) error {
	if urlOnly {
		url := discussionURL
		if external && externalURL != "" {
			url = externalURL
		}
		if url == "" {
			return notFoundErr(fmt.Errorf("no URL available"))
		}
		fmt.Fprintln(cmd.OutOrStdout(), url)
		return nil
	}
	if external {
		if externalURL == "" {
			return notFoundErr(fmt.Errorf("no external URL for this slug"))
		}
		out := map[string]string{"external_url": externalURL}
		buf, _ := json.Marshal(out)
		return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
}

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var external bool
	var dry bool

	cmd := &cobra.Command{
		Use:   "open <slug>",
		Short: "Open the Product Hunt page for a slug in the default browser",
		Long: `Resolve a slug to its canonical Product Hunt URL (or external URL with
--external) and hand it to the OS's default browser via 'open' (macOS),
'xdg-open' (Linux), or 'cmd /c start' (Windows).

Use --dry-run to print the URL without opening.`,
		Example: `  producthunt-pp-cli open seeknal
  producthunt-pp-cli open seeknal --external
  producthunt-pp-cli open seeknal --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			var discussion, ext string

			if db, err := openStore(dbPath); err == nil {
				defer db.Close()
				if p, err := db.GetPostBySlug(slug); err == nil && p != nil {
					discussion = p.DiscussionURL
					ext = p.ExternalURL
				}
			}
			if discussion == "" {
				body, err := fetchFeedBody(flags.timeout)
				if err != nil {
					return apiErr(err)
				}
				feed, err := atom.Parse(body)
				if err != nil {
					return apiErr(fmt.Errorf("parse feed: %w", err))
				}
				for _, e := range feed.Entries {
					if e.Slug == slug {
						discussion = e.DiscussionURL
						ext = e.ExternalURL
						break
					}
				}
			}
			url := discussion
			if external && ext != "" {
				url = ext
			}
			if url == "" {
				return notFoundErr(fmt.Errorf("slug %q not found", slug))
			}

			if dry || flags.dryRun {
				out := map[string]string{"url": url, "would_open": "true"}
				buf, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
			}

			if err := openInBrowser(url); err != nil {
				return apiErr(err)
			}
			out := map[string]string{"url": url, "opened": "true"}
			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	cmd.Flags().BoolVar(&external, "external", false, "Open the product's external URL (via PH redirect) instead of its PH page")
	cmd.Flags().BoolVar(&dry, "dry", false, "Print the URL without opening")
	return cmd
}

func openInBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "linux":
		c = exec.Command("xdg-open", url)
	case "windows":
		c = exec.Command("cmd", "/c", "start", "", url)
	default:
		return fmt.Errorf("unsupported GOOS %q", runtime.GOOS)
	}
	return c.Start()
}
