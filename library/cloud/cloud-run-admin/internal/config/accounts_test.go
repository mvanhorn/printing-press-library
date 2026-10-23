package config

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccounts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"CLOUD_RUN_ADMIN_ACCOUNT", "CLOUD_RUN_ADMIN_CONFIG", "CLOUD_RUN_ADMIN_OAUTH2C"} {
		t.Setenv(k, "")
	}
	legacy, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.SaveTokens("id", "secret", "legacy", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	legacy, err = Load("")
	if err != nil || legacy.AuthHeader() != "Bearer legacy" || legacy.Account != "default" {
		t.Fatal("legacy", err)
	}
	for _, name := range []string{"bestself", "littlemight"} {
		if err := ImportAccount(name, []byte("access_token='"+name+"'\nrefresh_token='refresh-"+name+"'")); err != nil {
			t.Fatal(err)
		}
		c, err := LoadAccount("", name)
		if err != nil || c.AuthHeader() != "Bearer "+name {
			t.Fatal("switch", err)
		}
		if err := os.Chmod(c.Path, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := c.SaveTokens("id", "secret", name, "refresh-"+name, time.Now()); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(c.Path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("permissions", err)
		}
	}
	names, err := ListAccounts()
	if err != nil || strings.Join(names, ",") != "bestself,default,littlemight" {
		t.Fatal(names, err)
	}
	if err := SetDefaultAccount("bestself"); err != nil {
		t.Fatal(err)
	}
	c, err := Load("")
	if err != nil || c.Account != "bestself" {
		t.Fatal("default", err)
	}
	t.Setenv("CLOUD_RUN_ADMIN_ACCOUNT", "littlemight")
	c, err = Load("")
	if err != nil || c.Account != "littlemight" {
		t.Fatal("env", err)
	}
	t.Setenv("CLOUD_RUN_ADMIN_OAUTH2C", "ambient-token")
	c, err = LoadAccount("", "bestself")
	if err != nil || c.AuthHeader() != "Bearer bestself" {
		t.Fatal("flag/credential isolation", err)
	}
	if _, err := LoadAccount(c.Path, "bestself"); err == nil {
		t.Fatal("ambiguous selectors")
	}
	if err := RemoveAccount("bestself"); err == nil {
		t.Fatal("removed default")
	}
	if err := RemoveAccount("littlemight"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../bad", "..", "/tmp/bad", "a/b", "A", "", "a\\b", "default-account", "config.toml"} {
		if _, err := AccountPath(name); err == nil {
			t.Fatal("invalid name accepted")
		}
	}
	if DataPath(WithAccount(context.Background(), "bestself")) == DataPath(WithAccount(context.Background(), "littlemight")) {
		t.Fatal("shared DB")
	}
	if err := ImportAccount("bad", []byte("access_token=never-print-this")); err == nil || strings.Contains(err.Error(), "never-print-this") {
		t.Fatal("parse redaction")
	}
}

func TestImportAndDefaultEdgeCases(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"CLOUD_RUN_ADMIN_ACCOUNT", "CLOUD_RUN_ADMIN_CONFIG", "CLOUD_RUN_ADMIN_OAUTH2C"} {
		t.Setenv(k, "")
	}
	if err := ImportAccount("refresh-only", []byte("client_id='id'\nrefresh_token='refresh'")); err == nil {
		t.Fatal("accepted unusable refresh-only account")
	}
	if err := SetDefaultAccount("default"); err != nil {
		t.Fatal("cannot restore environment-only default", err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- ImportAccount("concurrent", []byte("access_token='synthetic'")) }()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent import overwrote credentials")
	}
}
