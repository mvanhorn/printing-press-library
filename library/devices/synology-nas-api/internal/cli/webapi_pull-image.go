package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWebapiPullImageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull-image",
		Short: "Pull a Docker image from a registry",
		Long: `This command is a stub. The DSM REST API for container writes returns errors
(103) on DSM 7.3.2. Container Manager's web UI uses WebSocket or Docker
socket proxy for these operations, not the REST API.

Use the SSH-based CLI instead:
  synology-nas-ssh-pp-cli compose pull <project>
  ssh <nas> docker pull <image>`,
		DisableFlagParsing: true,
		Annotations: map[string]string{"pp:endpoint": "webapi.pull-image", "pp:method": "POST", "pp:stub": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("stub: image pull is not available via the DSM REST API on DSM 7.3.2.\nUse the SSH-based CLI or docker CLI via SSH instead.")
		},
	}
	return cmd
}
