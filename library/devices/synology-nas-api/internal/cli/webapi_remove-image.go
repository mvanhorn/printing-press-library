package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWebapiRemoveImageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-image",
		Short: "Remove a Docker image",
		Long: `This command is a stub. The DSM REST API for container writes returns errors
(114) on DSM 7.3.2. Container Manager's web UI uses WebSocket or Docker
socket proxy for these operations, not the REST API.

Use docker CLI via SSH instead:
  ssh <nas> docker rmi <image>`,
		DisableFlagParsing: true,
		Annotations: map[string]string{"pp:endpoint": "webapi.remove-image", "pp:method": "POST", "pp:stub": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("stub: image removal is not available via the DSM REST API on DSM 7.3.2.\nUse docker CLI via SSH instead.")
		},
	}
	return cmd
}
