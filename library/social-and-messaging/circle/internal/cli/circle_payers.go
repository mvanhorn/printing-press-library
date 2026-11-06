// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"

	"github.com/spf13/cobra"
)

type circlePayment struct {
	MemberID       json.Number `json:"community_member_id"`
	Email          string      `json:"community_member_email"`
	Name           string      `json:"community_member_name"`
	PaywallID      json.Number `json:"paywall_id"`
	PaywallName    string      `json:"paywall_name"`
	CreatedAt      string      `json:"created_at"`
	Status         string      `json:"status"`
	Amount         int64       `json:"amount"`
	AmountRefunded int64       `json:"amount_refunded"`
}

type circlePayer struct {
	MemberID    string `json:"member_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	PaywallName string `json:"paywall_name"`
	FirstPaidAt string `json:"first_paid_at"`
	Source      string `json:"source"`
}

func distinctCirclePayers(subscriptions, charges []circlePayment, paywall string) []circlePayer {
	payers := make([]circlePayer, 0)
	byMember := make(map[string]int)
	for listIndex, records := range [][]circlePayment{subscriptions, charges} {
		source := "subscription"
		if listIndex == 1 {
			source = "charge"
		}
		for _, r := range records {
			if paywall != "" && r.PaywallID.String() != paywall {
				continue
			}
			status := strings.ToLower(strings.TrimSpace(r.Status))
			if source == "subscription" {
				if status != "active" && status != "trialing" && status != "trial" && status != "past_due" {
					continue
				}
			} else if (status != "paid" && status != "succeeded" && status != "complete" && status != "partial_refunded") || r.AmountRefunded >= r.Amount {
				continue
			}
			key := "id:" + r.MemberID.String()
			if r.MemberID == "" {
				if strings.TrimSpace(r.Email) == "" {
					continue
				}
				key = "email:" + strings.ToLower(strings.TrimSpace(r.Email))
			}
			p := circlePayer{r.MemberID.String(), r.Email, r.Name, r.PaywallName, r.CreatedAt, source}
			if index, ok := byMember[key]; ok {
				existing := &payers[index]
				earlier, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
				previous, prevErr := time.Parse(time.RFC3339Nano, existing.FirstPaidAt)
				if err == nil && (prevErr != nil || earlier.Before(previous)) {
					existing.FirstPaidAt = r.CreatedAt
					existing.Source = source
					existing.PaywallName = r.PaywallName
				}
				if existing.Email == "" {
					existing.Email = r.Email
				}
				if existing.Name == "" {
					existing.Name = r.Name
				}
			} else {
				byMember[key] = len(payers)
				payers = append(payers, p)
			}
		}
	}
	return payers
}

func loadCirclePayers(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, paywall string) ([]circlePayer, string, error) {
	resources := []string{"community-member-subscriptions", "community-member-charges"}
	lists := make([][]circlePayment, 2)
	if flags.dataSource != "live" {
		_, err := os.Stat(dbPath)
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "hint: local store does not exist. Run 'circle-pp-cli sync' to populate it.")
			if flags.dataSource == "local" {
				return nil, "local", fmt.Errorf("no synced payment data; run sync --resources community-member-subscriptions,community-member-charges")
			}
		} else if err != nil {
			return nil, "local", err
		} else {
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return nil, "local", err
			}
			defer func() { _ = db.Close() }()
			unsyncedHint := false
			for i, resource := range resources {
				if !unsyncedHint {
					unsyncedHint = hintIfUnsynced(cmd, db, resource)
				}
				hintIfStale(cmd, db, resource, flags.maxAge)
				lists[i], err = readCirclePayments(ctx, db, resource)
				if err != nil {
					return nil, "local", err
				}
			}
			if flags.dataSource == "local" || (len(lists[0]) > 0 && len(lists[1]) > 0) {
				if len(lists[0])+len(lists[1]) == 0 {
					return nil, "local", fmt.Errorf("no synced payment data; run sync --resources community-member-subscriptions,community-member-charges")
				}
				flags.agentSource = "local"
				return distinctCirclePayers(lists[0], lists[1], paywall), "local", nil
			}
		}
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, "live", err
	}
	for i, resource := range resources {
		lists[i], err = fetchCirclePayments(ctx, c, "/"+strings.ReplaceAll(resource, "-", "_"))
		if err != nil {
			return nil, "live", err
		}
	}
	flags.agentSource = "live"
	return distinctCirclePayers(lists[0], lists[1], paywall), "live", nil
}

func readCirclePayments(ctx context.Context, db *store.Store, resource string) ([]circlePayment, error) {
	rows, err := db.DB().QueryContext(ctx, "SELECT data FROM resources WHERE resource_type = ? ORDER BY id", resource)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	raw := make([][]byte, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		raw = append(raw, data)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil {
		return nil, rowErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	records := make([]circlePayment, 0, len(raw))
	for _, data := range raw {
		var record circlePayment
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode local %s: %w", resource, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func fetchCirclePayments(ctx context.Context, c *client.Client, path string) ([]circlePayment, error) {
	limit := 20
	if cliutil.IsDogfoodEnv() {
		limit = 1
	}
	records := make([]circlePayment, 0)
	for page := 1; page <= limit; page++ {
		data, err := c.Get(ctx, path, map[string]string{"page": strconv.Itoa(page), "per_page": "100"})
		if err != nil {
			return nil, err
		}
		var response struct {
			Success     *bool           `json:"success"`
			Message     string          `json:"message"`
			Records     []circlePayment `json:"records"`
			HasNextPage *bool           `json:"has_next_page"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		if response.Success != nil && !*response.Success {
			return nil, fmt.Errorf("%s: %s", path, response.Message)
		}
		if response.HasNextPage == nil || response.Records == nil {
			return nil, fmt.Errorf("%s: missing records or has_next_page in response", path)
		}
		records = append(records, response.Records...)
		if !*response.HasNextPage {
			return records, nil
		}
	}
	return nil, fmt.Errorf("%s: incomplete payer data after %d pages; sync the full collection and use --data-source local", path, limit)
}
