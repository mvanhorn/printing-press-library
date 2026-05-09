package client

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type persistentJar struct {
	jar  http.CookieJar
	path string
	mu   sync.Mutex
}

func newPersistentJar(cookiePath string) (*persistentJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	pj := &persistentJar{jar: jar, path: cookiePath}
	pj.load()
	return pj, nil
}

func (pj *persistentJar) Cookies(u *url.URL) []*http.Cookie {
	return pj.jar.Cookies(u)
}

func (pj *persistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	pj.mu.Lock()
	defer pj.mu.Unlock()
	pj.jar.SetCookies(u, cookies)
	pj.save()
}

func (pj *persistentJar) load() {
	data, err := os.ReadFile(pj.path)
	if err != nil {
		return
	}
	var entries []struct {
		Domain   string `json:"domain"`
		Path     string `json:"path"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		Secure   bool   `json:"secure"`
		HttpOnly bool   `json:"http_only"`
	}
	if json.Unmarshal(data, &entries) != nil {
		return
	}
	u := &url.URL{Scheme: "http", Host: "127.0.0.1:3456"}
	var cookies []*http.Cookie
	for _, e := range entries {
		cookies = append(cookies, &http.Cookie{
			Name:     e.Name,
			Value:    e.Value,
			Path:     e.Path,
			Domain:   e.Domain,
			Secure:   e.Secure,
			HttpOnly: e.HttpOnly,
		})
	}
	if len(cookies) > 0 {
		pj.jar.SetCookies(u, cookies)
	}
}

func (pj *persistentJar) save() {
	u := &url.URL{Scheme: "http", Host: "127.0.0.1:3456"}
	cookies := pj.jar.Cookies(u)
	var entries []struct {
		Domain   string `json:"domain"`
		Path     string `json:"path"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		Secure   bool   `json:"secure"`
		HttpOnly bool   `json:"http_only"`
	}
	for _, c := range cookies {
		entries = append(entries, struct {
			Domain   string `json:"domain"`
			Path     string `json:"path"`
			Name     string `json:"name"`
			Value    string `json:"value"`
			Secure   bool   `json:"secure"`
			HttpOnly bool   `json:"http_only"`
		}{Domain: c.Domain, Path: c.Path, Name: c.Name, Value: c.Value, Secure: c.Secure, HttpOnly: c.HttpOnly})
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.MkdirAll(filepath.Dir(pj.path), 0755)
	os.WriteFile(pj.path, data, 0600)
}
