package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

const dropboxAccountMismatch = "this index/journal belongs to another Dropbox account: stop and ask the user to switch back to the original account's token, or have the user run `dropbox-pp-cli index --rebind` in a terminal (old undo history stays tied to the old account)"

func checkJournalAccount(batch store.DropboxJournalBatch, accountID string, force bool) (string, error) {
	if batch.LegacyAccount {
		if !force {
			return "", usageErr(fmt.Errorf("legacy journal has no account binding; use --force to accept the risk"))
		}
		return "legacy journal has no account binding; --force accepted the risk", nil
	}
	if batch.AccountID != accountID {
		return "", usageErr(fmt.Errorf("%s", dropboxAccountMismatch))
	}
	return "", nil
}

type accountInfo struct {
	AccountID       string
	AccountType     string
	RootNamespaceID string
	HomeNamespaceID string
}

func fetchDropboxAccount(ctx context.Context, c *client.Client) (accountInfo, error) {
	raw, _, err := c.PostQueryWithParams(ctx, "/users/get_current_account", nil, struct{}{})
	if err != nil {
		return accountInfo{}, err
	}
	var response struct {
		AccountID   string `json:"account_id"`
		AccountType struct {
			Tag string `json:".tag"`
		} `json:"account_type"`
		RootInfo struct {
			RootNamespaceID string `json:"root_namespace_id"`
			HomeNamespaceID string `json:"home_namespace_id"`
		} `json:"root_info"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return accountInfo{}, err
	}
	return accountInfo{AccountID: response.AccountID, AccountType: response.AccountType.Tag, RootNamespaceID: response.RootInfo.RootNamespaceID, HomeNamespaceID: response.RootInfo.HomeNamespaceID}, nil
}

func verifiedDropboxAccount(ctx context.Context, c *client.Client, db *store.Store) (accountInfo, error) {
	info, err := fetchDropboxAccount(ctx, c)
	if err != nil {
		return accountInfo{}, err
	}
	bound, _, err := db.GetDropboxMeta(ctx, "account_id")
	if err != nil {
		return accountInfo{}, err
	}
	if bound != info.AccountID {
		return accountInfo{}, usageErr(fmt.Errorf("%s", dropboxAccountMismatch))
	}
	return info, nil
}

func bindDropboxIndexAccount(ctx context.Context, db *store.Store, info accountInfo, rebind bool) error {
	bound, _, err := db.GetDropboxMeta(ctx, "account_id")
	if err != nil {
		return err
	}
	if rebind {
		if err := db.RebindDropboxIndex(ctx); err != nil {
			return err
		}
	} else if bound != "" && bound != info.AccountID {
		return usageErr(fmt.Errorf("%s", dropboxAccountMismatch))
	}
	if bound == "" && info.AccountID != "" {
		var indexed int
		if err := db.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dbx_files LIMIT 1)`).Scan(&indexed); err != nil {
			return err
		}
		if indexed != 0 {
			storedRoot, _, err := db.GetDropboxMeta(ctx, "root_namespace_id")
			if err != nil {
				return err
			}
			if !rebind && (storedRoot == "" || storedRoot != info.RootNamespaceID) {
				return usageErr(fmt.Errorf("%s", dropboxAccountMismatch))
			}
		}
	}
	for _, item := range [][2]string{{"account_id", info.AccountID}, {"account_type", info.AccountType}, {"root_namespace_id", info.RootNamespaceID}, {"home_namespace_id", info.HomeNamespaceID}, {"fetched_at", time.Now().UTC().Format(time.RFC3339)}} {
		if err := db.SetDropboxMeta(ctx, item[0], item[1]); err != nil {
			return err
		}
	}
	return nil
}

func pathRootHeaders(info accountInfo) map[string]string {
	if info.RootNamespaceID == "" || info.RootNamespaceID == info.HomeNamespaceID {
		return nil
	}
	b, _ := json.Marshal(map[string]string{".tag": "root", "root": info.RootNamespaceID})
	return map[string]string{"Dropbox-API-Path-Root": string(b)}
}
