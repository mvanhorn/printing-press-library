package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLoopbackRejectsWrongState(t *testing.T) {
	server, err := StartLoopback(context.Background(), LoopbackOptions{State: "good", Timeout: time.Second})
	if err != nil {
		if isBindDenied(err) {
			t.Skipf("local listener unavailable: %v", err)
		}
		t.Fatal(err)
	}
	defer server.Close()
	resp, err := http.Get(server.RedirectURI + "?state=bad&code=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "State mismatch") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	select {
	case got := <-server.Results:
		t.Fatalf("unexpected result: %+v", got)
	default:
	}
}

func TestLoopbackAcceptsValidStateAndCloses(t *testing.T) {
	server, err := StartLoopback(context.Background(), LoopbackOptions{State: "good", Timeout: time.Second})
	if err != nil {
		if isBindDenied(err) {
			t.Skipf("local listener unavailable: %v", err)
		}
		t.Fatal(err)
	}
	defer server.Close()
	resp, err := http.Get(server.RedirectURI + "?state=good&code=abc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	select {
	case got := <-server.Results:
		if got.Code != "abc" {
			t.Fatalf("code=%q", got.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for callback")
	}
}

func isBindDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "operation not permitted") || strings.Contains(msg, "permission denied")
}

func TestLoopbackBindFailsAfterThreeAttempts(t *testing.T) {
	attempts := 0
	_, err := StartLoopback(context.Background(), LoopbackOptions{
		State: "good",
		Listen: func(network, address string) (net.Listener, error) {
			attempts++
			if address != "127.0.0.1:0" {
				t.Fatalf("listener tried non-loopback address %q", address)
			}
			return nil, errors.New("bind failed")
		},
	})
	var bindErr *BindError
	if !errors.As(err, &bindErr) || bindErr.Attempts != 3 || attempts != 3 {
		t.Fatalf("expected 3-attempt bind error, got attempts=%d err=%v", attempts, err)
	}
}
