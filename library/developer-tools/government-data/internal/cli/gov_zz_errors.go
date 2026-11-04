package cli

import (
	"errors"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		var walk func(*cobra.Command)
		walk = func(c *cobra.Command) {
			for _, child := range c.Commands() {
				walk(child)
			}
			if c.RunE == nil {
				return
			}
			original := c.RunE
			c.RunE = func(cmd *cobra.Command, args []string) error { return govClassify(original(cmd, args)) }
		}
		for _, c := range root.Commands() {
			switch c.Name() {
			case "catalog", "dataset", "resource", "edgar", "sync":
				walk(c)
			}
		}
	})
}
func govClassify(err error) error {
	if err == nil {
		return nil
	}
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	var throttled *cliutil.RateLimitError
	if errors.As(err, &throttled) {
		return rateLimitErr(err)
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "HTTP 404"):
		return notFoundErr(err)
	case strings.Contains(message, "HTTP 401") || strings.Contains(message, "HTTP 403"):
		return authErr(err)
	case strings.Contains(message, "HTTP 429"):
		return rateLimitErr(err)
	case strings.Contains(message, "SEC_USER_AGENT"):
		return configErr(err)
	default:
		return apiErr(err)
	}
}
