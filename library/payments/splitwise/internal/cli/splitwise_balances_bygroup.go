// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"math"
	"sort"
)

type groupBalanceRow struct {
	GroupID      int     `json:"group_id"`
	GroupName    string  `json:"group_name"`
	CurrencyCode string  `json:"currency_code"`
	Amount       float64 `json:"amount"`
}

func groupBalances(groups []Group, currentUserID int) []groupBalanceRow {
	rows := make([]groupBalanceRow, 0)
	for _, g := range groups {
		var me *GroupMember
		for i := range g.Members {
			if g.Members[i].ID == currentUserID {
				me = &g.Members[i]
				break
			}
		}
		if me == nil {
			continue
		}
		for _, b := range me.Balance {
			amt := parseAmount(b.Amount)
			if amt == 0 {
				continue
			}
			rows = append(rows, groupBalanceRow{
				GroupID:      g.ID,
				GroupName:    g.Name,
				CurrencyCode: b.CurrencyCode,
				Amount:       round2(amt),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		ai := math.Abs(rows[i].Amount)
		aj := math.Abs(rows[j].Amount)
		if ai != aj {
			return ai > aj
		}
		if rows[i].GroupName != rows[j].GroupName {
			return rows[i].GroupName < rows[j].GroupName
		}
		return rows[i].CurrencyCode < rows[j].CurrencyCode
	})
	return rows
}
