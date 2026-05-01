package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/cliutil"
)

// newOpenCmd builds the side-effect command that opens a Hacker News
// item or its associated URL in the user's browser. It follows the
// side-effect convention: print by default, --launch to act, and
// short-circuit when running under verify (PRINTING_PRESS_VERIFY=1).
func newOpenCmd(flags *rootFlags) *cobra.Command {
	var hn bool
	var launch bool

	cmd := &cobra.Command{
		Use:   "open <id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Print or launch a story URL or HN thread (--launch to open in browser)",
		Long: `Show the URL associated with a Hacker News item, or launch it in the browser.

By default the URL is printed to stdout — useful for piping into a
selector or another tool. Pass --launch to open it in your default
browser. Pass --hn to choose the HN thread URL instead of the story
URL when both exist.`,
		Example: strings.Trim(`
  # Just print the URL
  hackernews-pp-cli open 12345678

  # Open the story link in the browser
  hackernews-pp-cli open 12345678 --launch

  # Open the HN thread instead of the article
  hackernews-pp-cli open 12345678 --hn --launch
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/item/"+id+".json", nil)
			if err != nil {
				return apiErr(err)
			}
			obj := map[string]any{}
			if jerr := json.Unmarshal(data, &obj); jerr != nil {
				return apiErr(fmt.Errorf("parsing item: %w", jerr))
			}
			storyURL, _ := obj["url"].(string)
			thread := fmt.Sprintf("https://news.ycombinator.com/item?id=%s", id)
			target := storyURL
			if hn || target == "" {
				target = thread
			}
			if target == "" {
				return notFoundErr(fmt.Errorf("item %s has no URL", id))
			}

			// Verify short-circuit: never open a real browser under verify.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would launch:", target)
				return nil
			}

			if !launch {
				fmt.Fprintln(cmd.OutOrStdout(), target)
				return nil
			}

			if err := launchURL(target); err != nil {
				return apiErr(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "launched:", target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&hn, "hn", false, "Use the HN thread URL even when the story has its own URL")
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the URL in the default browser instead of just printing it")
	return cmd
}

func launchURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
