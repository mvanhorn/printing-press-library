// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "open <slug-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Resolve a Food52 recipe/article slug to its canonical URL (and optionally launch it)",
		Long: strings.TrimSpace(`
Resolves a Food52 recipe slug, story slug, or full Food52 URL to its
canonical URL and prints it. Pair with --launch to also open it in the
default browser.

The default is "print only" rather than "launch" because mock-mode
verifiers and CI/agent harnesses often invoke commands with placeholder
args, and shelling out to the OS browser there would spam browser tabs
with bogus URLs. Add --launch when you actually want the browser to open.

Detection rules:
- If the arg contains "/recipes/" → recipe URL
- If the arg contains "/story/" or "/blog/" → article URL
- If the arg starts with http/https → resolved verbatim
- Otherwise → treated as a recipe slug (the most common case)
`),
		Example: strings.Trim(`
  food52-pp-cli open sarah-fennel-s-best-lunch-lady-brownie-recipe
  food52-pp-cli open sarah-fennel-s-best-lunch-lady-brownie-recipe --launch
  food52-pp-cli open https://food52.com/story/best-mothers-day-gift-ideas
`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := strings.TrimSpace(args[0])
			url := resolveOpenURL(arg)
			if url == "" {
				return fmt.Errorf("could not resolve a Food52 URL from %q", arg)
			}
			fmt.Println(url)
			// Launch only when the user explicitly asked AND we're not in a
			// non-interactive context (--no-input typically signals CI/agent;
			// --dry-run signals "tell me what would happen, don't do it").
			if launch && !flags.noInput && !flags.dryRun {
				return openBrowser(url)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Also launch the resolved URL in the default browser (default: print URL only)")
	return cmd
}

func resolveOpenURL(arg string) string {
	switch {
	case strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://"):
		return arg
	case strings.Contains(arg, "/story/") || strings.Contains(arg, "/blog/"):
		slug := articleSlugFromArg(arg)
		if slug == "" {
			return ""
		}
		return canonicalArticleURL(slug)
	case strings.Contains(arg, "/recipes/"):
		slug := recipeSlugFromArg(arg)
		if slug == "" {
			return ""
		}
		return canonicalRecipeURL(slug)
	default:
		// Treat as a bare recipe slug — the common case.
		return canonicalRecipeURL(arg)
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
