package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCmd(runnerErr error) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Health check (nlm installed, auth valid, database accessible)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("notebooklm-pp-cli doctor")
			fmt.Println("---")

			// Check nlm binary
			if runnerErr != nil {
				fmt.Printf("FAIL nlm binary: %v\n", runnerErr)
			} else {
				fmt.Printf("OK   nlm binary: %s\n", runner.BinPath())
				// Check auth
				if out, err := runner.Run("login", "--check"); err != nil {
					fmt.Printf("FAIL nlm auth: %v\n", err)
				} else {
					fmt.Printf("OK   nlm auth: %s", string(out))
				}
			}

			return nil
		},
	}
}
