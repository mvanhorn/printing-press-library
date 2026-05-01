package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn, fail
	Detail string `json:"detail,omitempty"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Diagnose config, session, and API reachability",
		Long: `Run a full health check: config file, local database, Chrome cookie auth,
and a live CurrentUserFields ping against Instacart. Useful as a first-run
sanity check and as an agent-friendly "is this thing working" probe.

Exit codes:
  0  all checks passed
  3  auth missing or rejected
  7  transient network / API error`,
		Example: "  instacart doctor\n  instacart doctor --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			var results []checkResult

			// 1. Config file
			if path, err := config.Path(); err == nil {
				results = append(results, checkResult{Name: "config", Status: "ok", Detail: path})
			} else {
				results = append(results, checkResult{Name: "config", Status: "warn", Detail: err.Error()})
			}

			// 2. SQLite store
			results = append(results, checkResult{Name: "store", Status: "ok", Detail: app.Store.Path()})

			// 3. Persisted op hashes seeded?
			if n, err := app.Store.CountOps(); err == nil {
				if n == 0 {
					results = append(results, checkResult{Name: "ops", Status: "warn", Detail: "no persisted query hashes known -- run `instacart capture` or `instacart ops seed`"})
				} else {
					results = append(results, checkResult{Name: "ops", Status: "ok", Detail: fmt.Sprintf("%d operations cached", n)})
				}
			}

			// 4. History store state. Instacart has no clean GraphQL op for
			// order history (see docs/solutions/best-practices/
			// instacart-orders-no-clean-graphql-op.md), so the only working
			// backfill path is the Chrome-MCP-driven `/pp-instacart backfill`
			// skill flow which dumps + imports JSONL.
			orderCount, _ := app.Store.CountOrders()
			itemCount, _, _ := app.Store.CountPurchasedItems()
			if orderCount == 0 && itemCount == 0 {
				results = append(results, checkResult{
					Name:   "history",
					Status: "warn",
					Detail: backfillHint(),
				})
			} else {
				results = append(results, checkResult{
					Name:   "history",
					Status: "ok",
					Detail: fmt.Sprintf("%d orders, %d purchased items", orderCount, itemCount),
				})
			}

			// 5. Auth session
			sess, err := auth.LoadSession()
			if err != nil {
				results = append(results, checkResult{Name: "session", Status: "fail", Detail: err.Error()})
				return finishDoctor(cmd, results, app.JSON, coded(ExitAuth, "no session: run `instacart auth login`"))
			}
			results = append(results, checkResult{Name: "session", Status: "ok", Detail: fmt.Sprintf("%d cookies from %s", len(sess.Cookies), sess.Source)})
			app.Session = sess

			// 6. Live ping - CurrentUserFields
			pingCtx, cancel := context.WithTimeout(app.Ctx, 10*time.Second)
			defer cancel()
			ok, detail := liveUserPing(pingCtx, app.Session, app.Cfg, app.Store)
			if ok {
				results = append(results, checkResult{Name: "api", Status: "ok", Detail: detail})
			} else {
				results = append(results, checkResult{Name: "api", Status: "fail", Detail: detail})
				return finishDoctor(cmd, results, app.JSON, coded(ExitTransient, "api: %s", detail))
			}

			return finishDoctor(cmd, results, app.JSON, nil)
		},
	}
}

func liveUserPing(ctx context.Context, sess *auth.Session, cfg *config.Config, st *store.Store) (bool, string) {
	client := gql.NewClient(sess, cfg, st)
	// Seed CurrentUserFields in the store if not already there so we can use it.
	if _, err := st.LookupOp("CurrentUserFields"); err != nil {
		_ = st.UpsertOp(store.Op{
			OperationName: "CurrentUserFields",
			Sha256Hash:    instacart.DefaultOps["CurrentUserFields"].Hash,
		})
	}
	resp, err := client.Query(ctx, "CurrentUserFields", map[string]any{})
	if err != nil {
		return false, err.Error()
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Data instacart.CurrentUserFieldsResponse `json:"data"`
	}
	// resp.RawBody holds the full response body
	if len(resp.RawBody) > 0 {
		if err := json.Unmarshal(resp.RawBody, &parsed); err == nil && parsed.Data.CurrentUser != nil {
			u := parsed.Data.CurrentUser
			name := u.FirstName
			if u.LastName != "" {
				name += " " + u.LastName
			}
			return true, fmt.Sprintf("logged in as %s (%s)", name, u.Email)
		}
	}
	return true, "HTTP 200 (no user fields in response, session may be guest)"
}

func finishDoctor(cmd *cobra.Command, results []checkResult, asJSON bool, outer error) error {
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"checks": results, "ok": outer == nil})
	} else {
		for _, r := range results {
			glyph := "ok "
			switch r.Status {
			case "warn":
				glyph = "warn"
			case "fail":
				glyph = "fail"
			}
			line := fmt.Sprintf("  [%s] %s", glyph, r.Name)
			if r.Detail != "" {
				line += ": " + r.Detail
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}
	return outer
}
