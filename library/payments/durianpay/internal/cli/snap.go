// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// SNAP command tree: hand-built commands over the internal/snap signing transport.
package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSnapCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "snap",
		Short:       "SNAP (Bank Indonesia standard) APIs: signed transfers, payments, QRIS, VAs, token lifecycle",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSnapKeygenCmd(flags))
	cmd.AddCommand(newNovelSnapSignCmd(flags))
	cmd.AddCommand(newNovelSnapTokenCmd(flags))
	cmd.AddCommand(newSnapBalanceCmd(flags))
	cmd.AddCommand(newSnapInquiryBankCmd(flags))
	cmd.AddCommand(newSnapInquiryEwalletCmd(flags))
	cmd.AddCommand(newSnapTransferCmd(flags))
	cmd.AddCommand(newSnapTransferStatusCmd(flags))
	cmd.AddCommand(newSnapEwalletTransferCmd(flags))
	cmd.AddCommand(newSnapEwalletTransferStatusCmd(flags))
	cmd.AddCommand(newSnapEwalletPayCmd(flags))
	cmd.AddCommand(newSnapEwalletStatusCmd(flags))
	cmd.AddCommand(newSnapEwalletCancelCmd(flags))
	cmd.AddCommand(newSnapEwalletRefundCmd(flags))
	cmd.AddCommand(newSnapQrisGenerateCmd(flags))
	cmd.AddCommand(newSnapQrisQueryCmd(flags))
	cmd.AddCommand(newSnapQrisCancelCmd(flags))
	cmd.AddCommand(newSnapQrisRefundCmd(flags))
	cmd.AddCommand(newSnapVaCreateCmd(flags))
	cmd.AddCommand(newSnapVaUpdateCmd(flags))
	cmd.AddCommand(newSnapVaInquiryCmd(flags))
	return cmd
}
