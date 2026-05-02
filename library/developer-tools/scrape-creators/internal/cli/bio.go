package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newBioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bio",
		Short: "Link-in-bio universal resolver",
	}
	cmd.AddCommand(newBioResolveCmd(flags))
	return cmd
}

type bioLink struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

type bioResult struct {
	Service     string    `json:"service"`
	Handle      string    `json:"handle,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	URL         string    `json:"url"`
	Links       []bioLink `json:"links"`
}

func newBioResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <url>",
		Short: "Auto-detect linktree / komi / pillar / linkbio / linkme and return the unified destination list",
		Long: `Looks at the URL hostname, dispatches to the matching ScrapeCreators
endpoint, and returns a uniform link list regardless of which bio service
the creator uses.`,
		Example:     "  scrape-creators-pp-cli bio resolve https://linktr.ee/mrbeast --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			rawURL := args[0]
			if dryRunOK(flags) {
				return nil
			}
			u, err := url.Parse(rawURL)
			if err != nil {
				return fmt.Errorf("invalid url %q: %w", rawURL, err)
			}
			service, path := classifyBioURL(u)
			if service == "" {
				return fmt.Errorf("unrecognized link-in-bio host %q (supported: linktr.ee, komi.io, pillar.io, lnk.bio, link.me)", u.Host)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(path, map[string]string{"url": rawURL})
			if err != nil {
				return classifyAPIError(err)
			}
			result := bioResult{Service: service, URL: rawURL}
			result.Links = extractBioLinks(raw)
			var d map[string]any
			if json.Unmarshal(raw, &d) == nil {
				result.Handle = lookupString(d, []string{"handle", "username", "id"})
				result.DisplayName = lookupString(d, []string{"display_name", "name", "first_name"})
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s — %d links\n", service, result.DisplayName, len(result.Links))
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TITLE\tURL")
			for _, l := range result.Links {
				fmt.Fprintf(tw, "%s\t%s\n", truncate(l.Title, 40), truncate(l.URL, 80))
			}
			return tw.Flush()
		},
	}
	return cmd
}

// classifyBioURL maps a URL to (service-name, endpoint-path).
func classifyBioURL(u *url.URL) (string, string) {
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "linktr.ee") || strings.Contains(host, "linktree"):
		return "linktree", "/v1/linktree"
	case strings.Contains(host, "komi.io") || strings.Contains(host, "komi.live"):
		return "komi", "/v1/komi"
	case strings.Contains(host, "pillar.io") || strings.Contains(host, "pillar.com"):
		return "pillar", "/v1/pillar"
	case strings.Contains(host, "lnk.bio") || strings.Contains(host, "linkbio"):
		return "linkbio", "/v1/linkbio"
	case strings.Contains(host, "link.me") || strings.Contains(host, "linkme"):
		return "linkme", "/v1/linkme"
	}
	return "", ""
}

// extractBioLinks pulls out url+title pairs from any nested structure.
func extractBioLinks(raw json.RawMessage) []bioLink {
	var anyVal any
	if json.Unmarshal(raw, &anyVal) != nil {
		return nil
	}
	links := []bioLink{}
	walkBio(anyVal, &links)
	return links
}

func walkBio(v any, out *[]bioLink) {
	switch x := v.(type) {
	case map[string]any:
		hasURL := lookupString(x, []string{"url", "href", "destination"})
		if hasURL != "" {
			t := lookupString(x, []string{"title", "label", "name", "text"})
			*out = append(*out, bioLink{Title: t, URL: hasURL})
		}
		for _, vv := range x {
			walkBio(vv, out)
		}
	case []any:
		for _, vv := range x {
			walkBio(vv, out)
		}
	}
}
