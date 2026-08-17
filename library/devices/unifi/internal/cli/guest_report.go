// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.

// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type guestVoucher struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Code                 string `json:"code"`
	Expired              bool   `json:"expired"`
	AuthorizedGuestCount int64  `json:"authorized_guest_count"`
	AuthorizedGuestLimit int64  `json:"authorized_guest_limit,omitempty"`
}

type guestClient struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MAC        string `json:"mac"`
	IP         string `json:"ip,omitempty"`
	Authorized bool   `json:"authorized"`
}

type guestReportView struct {
	Site            string         `json:"site"`
	ActiveVouchers  []guestVoucher `json:"active_vouchers"`
	ExpiredVouchers int            `json:"expired_voucher_count"`
	ConnectedGuests []guestClient  `json:"connected_guests"`
}

func newNovelGuestReportCmd(flags *rootFlags) *cobra.Command {
	var flagSite string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Summarize guest network usage: active vouchers and connected guest clients, from local data.",
		Long: "Joins hotspot vouchers with currently synced clients whose " +
			"access.type is GUEST — no single API response combines these. " +
			"Local-mirror-only: run 'unifi-pp-cli sync' first.",
		Example:     "  unifi-pp-cli guest report --site default --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "guest report")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("unifi-pp-cli")
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), guestReportView{ActiveVouchers: []guestVoucher{}, ConnectedGuests: []guestClient{}}, flags)
				}
				return nil
			}
			defer db.Close()

			siteID, siteName, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), guestReportView{ActiveVouchers: []guestVoucher{}, ConnectedGuests: []guestClient{}}, flags)
					}
					return nil
				}
				return err
			}

			voucherRows, err := resourceRows(ctx, db.DB(), "hotspot", siteID)
			if err != nil {
				return err
			}
			clientRows, err := resourceRows(ctx, db.DB(), "clients", siteID)
			if err != nil {
				return err
			}

			view := guestReportView{Site: siteName, ActiveVouchers: []guestVoucher{}, ConnectedGuests: []guestClient{}}
			for _, id := range sortedKeys(voucherRows) {
				var v struct {
					ID                   string `json:"id"`
					Name                 string `json:"name"`
					Code                 string `json:"code"`
					Expired              bool   `json:"expired"`
					AuthorizedGuestCount int64  `json:"authorizedGuestCount"`
					AuthorizedGuestLimit int64  `json:"authorizedGuestLimit"`
				}
				if json.Unmarshal(voucherRows[id], &v) != nil {
					continue
				}
				if v.Expired {
					view.ExpiredVouchers++
					continue
				}
				view.ActiveVouchers = append(view.ActiveVouchers, guestVoucher{
					ID: v.ID, Name: v.Name, Code: v.Code, Expired: v.Expired,
					AuthorizedGuestCount: v.AuthorizedGuestCount, AuthorizedGuestLimit: v.AuthorizedGuestLimit,
				})
			}
			for _, id := range sortedKeys(clientRows) {
				var c struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					MAC    string `json:"macAddress"`
					IP     string `json:"ipAddress"`
					Access struct {
						Type       string `json:"type"`
						Authorized bool   `json:"authorized"`
					} `json:"access"`
				}
				if json.Unmarshal(clientRows[id], &c) != nil {
					continue
				}
				if c.Access.Type != "GUEST" {
					continue
				}
				view.ConnectedGuests = append(view.ConnectedGuests, guestClient{
					ID: c.ID, Name: c.Name, MAC: c.MAC, IP: c.IP, Authorized: c.Access.Authorized,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Site: %s\n", view.Site)
			fmt.Fprintf(w, "Active vouchers: %d (expired: %d)\n", len(view.ActiveVouchers), view.ExpiredVouchers)
			for _, v := range view.ActiveVouchers {
				fmt.Fprintf(w, "  %s  code=%s  used %d/%d\n", v.Name, v.Code, v.AuthorizedGuestCount, v.AuthorizedGuestLimit)
			}
			fmt.Fprintf(w, "Connected guest clients: %d\n", len(view.ConnectedGuests))
			for _, c := range view.ConnectedGuests {
				fmt.Fprintf(w, "  %s (%s) authorized=%v\n", c.Name, c.IP, c.Authorized)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	return cmd
}
