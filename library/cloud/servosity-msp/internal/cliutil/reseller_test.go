// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestParseResellerURL(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"https://api.servosity.com/api/v1/resellers/2/", 2, false},
		{"https://api.servosity.com/api/v1/resellers/4421", 4421, false},
		{"/api/v1/resellers/12/", 12, false},
		{"https://example.com/resellers/2/companies/", 0, true},
		{"", 0, true},
		{"garbage", 0, true},
	}
	for _, c := range cases {
		got, err := parseResellerURL(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseResellerURL(%q): err = %v, wantErr = %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseResellerURL(%q): got %d want %d", c.in, got, c.want)
		}
	}
}

type fakeClient struct {
	body []byte
	err  error
}

func (f *fakeClient) Get(_ string, _ map[string]string) (json.RawMessage, error) {
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(f.body), nil
}

func TestResolveResellerID_EnvOverride(t *testing.T) {
	t.Setenv("SERVOSITY_MSP_RESELLER_ID", "99")
	got, err := ResolveResellerID(context.Background(), &fakeClient{})
	if err != nil {
		t.Fatalf("expected env override to win: %v", err)
	}
	if got != 99 {
		t.Errorf("got %d want 99", got)
	}
}

func TestResolveResellerID_EnvBadValue(t *testing.T) {
	t.Setenv("SERVOSITY_MSP_RESELLER_ID", "not-a-number")
	_, err := ResolveResellerID(context.Background(), &fakeClient{})
	if err == nil {
		t.Fatal("expected error on bad env value")
	}
}

func TestResolveResellerID_FromCompanies(t *testing.T) {
	os.Unsetenv("SERVOSITY_MSP_RESELLER_ID")
	body := []byte(`{"results":[{"id":1,"reseller":"https://api.servosity.com/api/v1/resellers/7/"},{"id":2,"reseller":"https://api.servosity.com/api/v1/resellers/7/"}]}`)
	got, err := ResolveResellerID(context.Background(), &fakeClient{body: body})
	if err != nil {
		t.Fatalf("ResolveResellerID: %v", err)
	}
	if got != 7 {
		t.Errorf("got %d want 7", got)
	}
}

func TestResolveResellerID_EmptyCompanies(t *testing.T) {
	os.Unsetenv("SERVOSITY_MSP_RESELLER_ID")
	body := []byte(`{"results":[]}`)
	_, err := ResolveResellerID(context.Background(), &fakeClient{body: body})
	if err == nil {
		t.Fatal("expected error on empty companies list")
	}
}

func TestResolveResellerID_ClientError(t *testing.T) {
	os.Unsetenv("SERVOSITY_MSP_RESELLER_ID")
	_, err := ResolveResellerID(context.Background(), &fakeClient{err: errors.New("network down")})
	if err == nil {
		t.Fatal("expected error from client failure")
	}
}
