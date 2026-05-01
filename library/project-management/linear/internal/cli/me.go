package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"

	"github.com/spf13/cobra"
)

func newMeCmd(flags *rootFlags) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "me",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show current authenticated user",
		Example: `  linear-pp-cli me
  linear-pp-cli me --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var result struct {
				Viewer struct {
					ID           string `json:"id"`
					Name         string `json:"name"`
					DisplayName  string `json:"displayName"`
					Email        string `json:"email"`
					Active       bool   `json:"active"`
					Admin        bool   `json:"admin"`
					Organization struct {
						Name   string `json:"name"`
						URLKey string `json:"urlKey"`
					} `json:"organization"`
				} `json:"viewer"`
			}
			if err := c.QueryInto(client.ViewerQuery, nil, &result); err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result.Viewer)
			}

			v := result.Viewer
			fmt.Printf("Name:  %s\n", v.DisplayName)
			fmt.Printf("Email: %s\n", v.Email)
			fmt.Printf("Org:   %s (%s)\n", v.Organization.Name, v.Organization.URLKey)
			if v.Admin {
				fmt.Println("Role:  Admin")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}
