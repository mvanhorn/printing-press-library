package cmd

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

// withClient is the standard handler wrapper. It resolves auth, builds a
// Client, runs the handler, and writes the result through the global formatter.
//
// On error, AuthError → exit 2, InputError → exit 3, anything else → exit 1.
// Cobra's RunE will surface the error to main.go which calls ExitCode.
func withClient(fn func(ctx context.Context, c *client.Client, fmt output.Format) (any, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		f, err := output.Parse(globals.format)
		if err != nil {
			return &InputError{Message: err.Error()}
		}
		cl, err := client.New(globals.apiKey, Version)
		if err != nil {
			return err
		}
		result, err := fn(cmd.Context(), cl, f)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		if err := output.Write(result, f); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}
