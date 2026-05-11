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
	jar     http.CookieJar
	path    string
	baseURL *url.URL
	mu      sync.Mutex
}

func newPersistentJar(cookiePath, rawBaseURL string) (*persistentJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(rawBaseURL)
	if u == nil || u.Host == "" {
		u = &url.URL{Scheme: "http", Host: "127.0.0.1:3456"}
	}
	pj := &persistentJar{jar: jar, path: cookiePath, baseURL: u}
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
		pj.jar.SetCookies(pj.baseURL, cookies)
	}
}

func (pj *persistentJar) save() {
	cookies := pj.jar.Cookies(pj.baseURL)
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
