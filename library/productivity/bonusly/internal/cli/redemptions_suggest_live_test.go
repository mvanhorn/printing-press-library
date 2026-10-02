package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
)

func seedUnownedRedemptionHistory(t *testing.T) {
	t.Helper()
	db, err := store.OpenWithContext(t.Context(), defaultDBPath("bonusly-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.UpsertRedemptions([]byte(`{"id":"a-only","reward_name":"A private reward","state":"fulfilled","created_at":"2026-01-01"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureBonuslyBalanceHistory(t.Context(), db.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO balance_history (recorded_at, giving_balance, earning_balance, monthly_budget) VALUES ('2026-09-01', 999, 999, 999)`); err != nil {
		t.Fatal(err)
	}
}

func TestRedemptionsSuggestProfileSwitchIgnoresSharedHistoryAndCache(t *testing.T) {
	t.Setenv("BONUSLY_HOME", t.TempDir())
	seedUnownedRedemptionHistory(t)
	var accounts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		accounts = append(accounts, account+":"+r.URL.Path)
		switch r.URL.Path {
		case "/users/me":
			fmt.Fprintf(w, `{"result":{"id":%q,"earning_balance":20,"giving_balance":5}}`, account)
		case "/users/A/redemptions", "/users/B/redemptions":
			if r.URL.Path != "/users/"+account+"/redemptions" {
				t.Errorf("account %q queried another identity: %s", account, r.URL.Path)
			}
			if r.URL.Query().Get("cursor") == "" {
				fmt.Fprintf(w, `{"result":[{"reward_name":%q,"state":"fulfilled","created_at":"2026-02-01"}],"cursor":"next","meta":{"has_more":true}}`, account+" live reward")
				return
			}
			if r.URL.Query().Get("cursor") != "next" {
				t.Errorf("unexpected continuation cursor: %s", r.URL.RawQuery)
			}
			fmt.Fprintf(w, `{"result":[{"reward_name":%q,"state":"fulfilled","created_at":"2026-02-02"}],"meta":{"has_more":false}}`, account+" second reward")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("BONUSLY_BASE_URL", srv.URL)
	// Seed the shared /users/me response cache with A before selecting B.
	a := client.New(&config.Config{BaseURL: srv.URL, BonuslyApiToken: "A"}, time.Second, 0)
	if _, err := a.Get(t.Context(), "/users/me", nil); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"A", "B"} {
		t.Setenv("BONUSLY_API_TOKEN", account)
		cmd := newNovelRedemptionsSuggestCmd(&rootFlags{asJSON: true, clientProfileName: "profile-" + account, timeout: time.Second})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "A private reward") || !strings.Contains(out.String(), account+" live reward") || !strings.Contains(out.String(), account+" second reward") || strings.Contains(out.String(), "999") {
			t.Fatalf("selected account received unowned history or balance: %s", out.String())
		}
		if account == "B" && strings.Contains(out.String(), "A live reward") {
			t.Fatalf("profile B received A's history: %s", out.String())
		}
	}
	if !strings.Contains(strings.Join(accounts, ","), "B:/users/me") {
		t.Fatalf("B identity came from the shared cache instead of the network: %v", accounts)
	}
}

func TestRedemptionsSuggestAcceptsBareArrayHistory(t *testing.T) {
	t.Setenv("BONUSLY_HOME", t.TempDir())
	var historyRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/me":
			fmt.Fprint(w, `{"result":{"id":"B","earning_balance":20}}`)
		case "/users/B/redemptions":
			historyRequests++
			fmt.Fprint(w, `[{"reward_name":"B live reward","state":"fulfilled","created_at":"2026-02-01"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL, BonuslyApiToken: "B"}, time.Second, 0)
	account, history, err := fetchRedemptionSuggestionInputs(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(account) == 0 || len(history) != 1 || history[0].RewardName != "B live reward" || historyRequests != 1 || c.NoCache {
		t.Fatalf("bare-array redemption history did not load from the selected account: rows=%d requests=%d cache-disabled=%t", len(history), historyRequests, c.NoCache)
	}
}

func TestRedemptionsSuggestFailuresNeverFallBackToUnownedData(t *testing.T) {
	t.Setenv("BONUSLY_HOME", t.TempDir())
	seedUnownedRedemptionHistory(t)
	for _, failPath := range []string{"/users/me", "/users/B/redemptions"} {
		t.Run(failPath, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == failPath {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				fmt.Fprint(w, `{"result":{"id":"B","earning_balance":20}}`)
			}))
			defer srv.Close()
			c := client.New(&config.Config{BaseURL: srv.URL, BonuslyApiToken: "B"}, time.Second, 0)
			account, history, err := fetchRedemptionSuggestionInputs(t.Context(), c)
			if err == nil || account != nil || len(history) != 0 {
				t.Fatalf("failure must return no account/history, got account=%s history=%v err=%v", account, history, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1:1", BonuslyApiToken: "B"}, time.Second, 0)
	account, history, err := fetchRedemptionSuggestionInputs(ctx, c)
	if err == nil || account != nil || len(history) != 0 {
		t.Fatalf("network/cancellation failure must not fall back: %s %v %v", account, history, err)
	}
}
