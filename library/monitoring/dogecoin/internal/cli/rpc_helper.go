package cli

import (
	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/config"
	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/rpc"
)

func newRPCClientFromConfig(cfg *config.Config) *rpc.Client {
	return rpc.New(cfg)
}
