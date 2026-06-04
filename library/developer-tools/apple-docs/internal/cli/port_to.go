// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/client"

	"github.com/spf13/cobra"
)

func newNovelPortToCmd(flags *rootFlags) *cobra.Command {
	var flagMaxHops int

	cmd := &cobra.Command{
		Use:   "port-to <platform> <symbol>",
		Short: "Walk See-Also references until landing on an API available on the target platform",
		Long: strings.TrimSpace(`
Find the API that replaces an unavailable or deprecated symbol on a target
platform.

If the symbol IS already available on the target platform (and isn't
deprecated), the command says so and exits.

Otherwise, it walks the symbol's See-Also references breadth-first, fetching
each referenced page until landing on a symbol that:
  - has metadata.platforms[] containing the target platform, and
  - is not marked deprecated or unavailable on that platform.

The walk is bounded by --max-hops (default 12).

Use this command to find the API that replaces an unavailable symbol on a
specific platform. Do NOT use it for general similar-API lookup unrelated to
platform availability; use 'doc similar' instead. Do NOT use it to enumerate
all deprecated symbols in a framework; use 'deprecation-cliff' instead.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli port-to visionOS uikit/uitableview --agent
  apple-docs-pp-cli port-to macOS uikit/uitouch
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would walk See-Also graph")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<platform> and <symbol> are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := walkForReplacement(cmd, c, args[1], args[0], flagMaxHops)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitJSON(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&flagMaxHops, "max-hops", 12, "Maximum See-Also pages to fetch while walking")
	return cmd
}

type portToResult struct {
	From            string   `json:"from"`
	Platform        string   `json:"platform"`
	Available       bool     `json:"available_on_platform"`
	DeprecatedOnSrc bool     `json:"deprecated_on_platform,omitempty"`
	Replacement     string   `json:"replacement,omitempty"`
	ReplacementURL  string   `json:"replacement_url,omitempty"`
	WalkedSymbols   int      `json:"walked_symbols"`
	WalkChain       []string `json:"walk_chain,omitempty"`
	Note            string   `json:"note,omitempty"`
}

func walkForReplacement(cmd *cobra.Command, c *client.Client, symbol, platform string, maxHops int) (*portToResult, error) {
	root, err := applejson.FetchDoc(cmd.Context(), c, symbol)
	if err != nil {
		return nil, err
	}
	res := &portToResult{
		From:     root.Identifier,
		Platform: platform,
	}
	if root.IsAvailableOn(platform) {
		res.Available = true
		res.Note = fmt.Sprintf("%s is already available on %s; no replacement needed", root.Title, platform)
		return res, nil
	}
	if root.IsDeprecatedOn(platform) {
		res.DeprecatedOnSrc = true
	}

	type qItem struct {
		path  string
		chain []string
	}
	queue := []qItem{}
	visited := map[string]bool{strings.ToLower(strings.Trim(symbol, "/")): true}
	for _, ref := range pickSeeAlso(root) {
		queue = append(queue, qItem{path: ref, chain: []string{ref}})
	}

	walked := 0
	for len(queue) > 0 && walked < maxHops {
		cur := queue[0]
		queue = queue[1:]
		key := strings.ToLower(strings.Trim(cur.path, "/"))
		if visited[key] {
			continue
		}
		visited[key] = true
		walked++

		page, err := applejson.FetchDoc(cmd.Context(), c, cur.path)
		if err != nil {
			continue
		}
		if page.IsAvailableOn(platform) {
			res.Available = false
			res.Replacement = page.Title
			res.ReplacementURL = page.URL
			res.WalkedSymbols = walked
			res.WalkChain = cur.chain
			res.Note = fmt.Sprintf("found replacement after walking %d See-Also reference(s)", walked)
			return res, nil
		}
		for _, ref := range pickSeeAlso(page) {
			nkey := strings.ToLower(strings.Trim(ref, "/"))
			if visited[nkey] {
				continue
			}
			newChain := append(append([]string{}, cur.chain...), ref)
			queue = append(queue, qItem{path: ref, chain: newChain})
		}
	}
	res.Available = false
	res.WalkedSymbols = walked
	res.Note = fmt.Sprintf("no replacement found on %s after walking %d reference(s); try a deeper walk with --max-hops, or use 'doc similar' for a relevance-based hint", platform, walked)
	return res, nil
}
