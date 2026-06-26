package cli

import (
	"github.com/spf13/cobra"
	"profilepress-pp-cli/internal/packet"
	"profilepress-pp-cli/internal/store"
)

func newPacketCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "packet", Short: "packet commands"}
	cmd.AddCommand(newPacketexportCmd())
	return cmd
}

func packetByIDOrLatest(db *store.Store, id string) (packet.Packet, error) {
	if id != "" {
		return db.GetPacket(id)
	}
	return db.LatestPacket()
}
