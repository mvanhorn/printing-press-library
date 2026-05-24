package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWebapiCreateContainerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-container",
		Short: "Create a new Docker container",
		Long: `This command is a stub. The DSM REST API for container writes returns errors
(114/103) on DSM 7.3.2. Container Manager's web UI uses WebSocket or Docker
socket proxy for these operations, not the REST API.

Use the SSH-based CLI instead:
  synology-nas-ssh-pp-cli compose up <project>
  ssh <nas> docker run ...`,
		DisableFlagParsing: true,
		Annotations: map[string]string{"pp:endpoint": "webapi.create-container", "pp:method": "POST", "pp:stub": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("stub: container write operations are not available via the DSM REST API on DSM 7.3.2.\nUse the SSH-based CLI or docker CLI via SSH instead.")
		},
	}
	return cmd
}
