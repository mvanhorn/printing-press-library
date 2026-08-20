package anylist

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEnsureClientIdentifierPersistsGeneratedIdentifier(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg := &config.Config{Path: configPath}

	if err := EnsureClientIdentifier(cfg); err != nil {
		t.Fatalf("EnsureClientIdentifier returned error: %v", err)
	}
	if cfg.ClientIdentifier == "" {
		t.Fatal("ClientIdentifier was not set")
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if reloaded.ClientIdentifier != cfg.ClientIdentifier {
		t.Fatalf("persisted ClientIdentifier = %q, want %q", reloaded.ClientIdentifier, cfg.ClientIdentifier)
	}
}

func TestProductLookupRefreshesAfterUnauthorized(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	}
	client := New(cfg)
	lookupCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/product-lookup/049000028904":
			lookupCount++
			if lookupCount == 1 {
				return response(http.StatusUnauthorized, []byte("expired"))
			}
			if got := req.Header.Get("Authorization"); got != "Bearer refreshed-token" {
				t.Fatalf("retry Authorization = %q, want refreshed token", got)
			}
			body, err := proto.Marshal(&pb.PBProductLookupResponse{
				ListItem: &pb.ListItem{ProductUpc: "049000028904", Name: "Coca-Cola"},
			})
			if err != nil {
				t.Fatalf("marshal lookup response: %v", err)
			}
			return response(http.StatusOK, body)
		case "/auth/token/refresh":
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	result, err := client.ProductLookup(t.Context(), "049000028904")
	if err != nil {
		t.Fatalf("ProductLookup returned error: %v", err)
	}
	if result.GetListItem().GetName() != "Coca-Cola" {
		t.Fatalf("lookup name = %q, want Coca-Cola", result.GetListItem().GetName())
	}
	if lookupCount != 2 {
		t.Fatalf("lookup requests = %d, want 2", lookupCount)
	}
	if cfg.AccessToken != "refreshed-token" {
		t.Fatalf("refreshed access token = %q, want refreshed-token", cfg.AccessToken)
	}
}
