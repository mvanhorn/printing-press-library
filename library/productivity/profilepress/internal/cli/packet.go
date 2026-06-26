package cli

import (
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/packet"
	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/store"
	"github.com/spf13/cobra"
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
