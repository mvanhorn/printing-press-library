package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/config"
)

func TestIcpStaffLoginStoresOnlyServerSession(t *testing.T) {
	var sawLogin bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a/examplegym/":
			if r.Method != http.MethodPost {
				t.Fatalf("login method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("stafflogin") != "1" || r.Form.Get("uname") != "staff-user" || r.Form.Get("passwd") != "temporary-secret" {
				t.Fatalf("unexpected login form: %#v", r.Form)
			}
			sawLogin = true
			http.SetCookie(w, &http.Cookie{Name: "office_session", Value: "server-token", Path: "/", HttpOnly: true})
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
		case "/api/v1/user/permissions":
			if !sawLogin || !strings.Contains(r.Header.Get("Cookie"), "office_session=server-token") {
				t.Fatalf("verification did not replay server cookie: %q", r.Header.Get("Cookie"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"canViewFamilies":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previous := icpStaffOfficeBase
	icpStaffOfficeBase = server.URL
	t.Cleanup(func() { icpStaffOfficeBase = previous })

	session, err := icpStaffLogin(context.Background(), "examplegym", "staff-user", "temporary-secret")
	if err != nil {
		t.Fatal(err)
	}
	if session.Cookie != "office_session=server-token" {
		t.Fatalf("cookie = %q", session.Cookie)
	}
	raw, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "temporary-secret") {
		t.Fatal("persisted session contains the password")
	}
}

func TestIcpStaffLoginRejectsUnverifiedSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/a/") {
			http.SetCookie(w, &http.Cookie{Name: "office_session", Value: "rejected", Path: "/", HttpOnly: true})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	previous := icpStaffOfficeBase
	icpStaffOfficeBase = server.URL
	t.Cleanup(func() { icpStaffOfficeBase = previous })

	if _, err := icpStaffLogin(context.Background(), "examplegym", "staff-user", "wrong"); err == nil {
		t.Fatal("expected an authentication error")
	}
}

func TestIcpStaffInputValidation(t *testing.T) {
	for _, account := range []string{"../escape", "bad/account", "with space"} {
		if _, err := icpStaffAccount(account); err == nil {
			t.Fatalf("account %q should be rejected", account)
		}
	}
	if account, err := icpStaffAccount("Example-Gym_2"); err != nil || account != "example-gym_2" {
		t.Fatalf("normalized account = %q, %v", account, err)
	}
	if err := icpStaffValidateDate("--date", "08/12/2026"); err == nil {
		t.Fatal("expected malformed date to be rejected")
	}
	if err := icpStaffValidateListFlags(icpStaffListFlags{page: 0, limit: 100}); err == nil {
		t.Fatal("expected page 0 to be rejected")
	}
	if err := icpStaffValidateListFlags(icpStaffListFlags{page: 1, limit: 501}); err == nil {
		t.Fatal("expected oversized page to be rejected")
	}
}

func TestAdminSurfaceContainsOnlyReadCommands(t *testing.T) {
	flags := &rootFlags{}
	admin := newIclassproAdminCmd(flags)
	want := map[string]bool{
		"attendance": true, "capabilities": true, "class-search": true, "dashboard": true, "enrollments": true,
		"families": true, "reports": true, "students": true, "transactions": true,
	}
	for _, cmd := range admin.Commands() {
		if !want[cmd.Name()] {
			t.Fatalf("unexpected admin command %q", cmd.Name())
		}
		if cmd.Annotations["mcp:read-only"] != "true" {
			t.Fatalf("admin command %q is not annotated read-only", cmd.Name())
		}
		if cmd.Name() != "capabilities" && cmd.Annotations["pp:requires-tier"] != "staff" {
			t.Fatalf("admin command %q is not isolated to the staff credential tier", cmd.Name())
		}
		if cmd.Name() == "capabilities" && cmd.Annotations["pp:requires-tier"] != "" {
			t.Fatal("admin capabilities must remain runnable without a staff credential")
		}
		delete(want, cmd.Name())
	}
	if len(want) != 0 {
		t.Fatalf("missing admin commands: %#v", want)
	}
}

func TestIcpStaffDashboardWidgetsFindsNestedSavedWidgets(t *testing.T) {
	value := map[string]any{
		"data": map[string]any{
			"dashboard": map[string]any{
				"widgets": []any{
					map[string]any{"id": float64(1), "type": "enrollment"},
					map[string]any{"id": float64(2), "type": "revenue"},
				},
			},
		},
	}
	widgets := icpStaffDashboardWidgets(value)
	if len(widgets) != 2 || widgets[0]["type"] != "enrollment" || widgets[1]["type"] != "revenue" {
		t.Fatalf("unexpected widgets: %#v", widgets)
	}
}

func TestIcpStaffMergedFiltersPreservesDefaultsAndOverridesPaging(t *testing.T) {
	raw := json.RawMessage(`{"data":{"filters":{"locations":{"1":true},"page":9,"pageSize":10}}}`)
	defaults := icpStaffDefaultFilters(raw)
	merged := icpStaffMergedFilters(defaults, icpStaffListFlags{query: "smith", page: 2, limit: 25})
	if merged["page"] != 2 || merged["pageSize"] != 25 || merged["searchString"] != "smith" {
		t.Fatalf("missing CLI overrides: %#v", merged)
	}
	if _, ok := merged["locations"]; !ok {
		t.Fatalf("server defaults were not preserved: %#v", merged)
	}
}

func TestIcpStaffReadRequestsUseOnlyGETAndPostQuery(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		if !strings.Contains(r.Header.Get("Cookie"), "office_session=synthetic") {
			t.Fatalf("missing staff session cookie")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	icpSessionOnce = sync.Once{}
	icpSessionOnce.Do(func() {
		icpSessionCache = icpSessionFile{
			Sessions: map[string]icpSession{},
			StaffSessions: map[string]icpStaffSession{
				"examplegym": {Cookie: "office_session=synthetic"},
			},
		}
	})
	t.Cleanup(func() {
		icpSessionOnce = sync.Once{}
		icpSessionCache = icpSessionFile{}
	})

	c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
	c.BaseURL = server.URL
	c.NoCache = true
	if _, err := icpStaffGet(context.Background(), c, "examplegym", "/get-family-filter-data", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := icpStaffPostQuery(context.Background(), c, "examplegym", "/get-family-list/", map[string]any{"filters": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /get-family-filter-data", "POST /get-family-list/"}
	if len(methods) != len(want) || methods[0] != want[0] || methods[1] != want[1] {
		t.Fatalf("request methods = %#v, want %#v", methods, want)
	}
}
