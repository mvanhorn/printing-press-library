package client

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const oauthAuthorizeEndpoint = "https://partner-identity.myq-cloud.com/connect/authorize"

type oauth struct {
	jar               *cookiejar.Jar
	challenge         string
	verifier          string
	verificationToken string
	debug             bool
}

func newOAuth(debug bool) (*oauth, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	challenge, verifier := pkceChallenge()
	return &oauth{
		jar:       jar,
		challenge: challenge,
		verifier:  verifier,
		debug:     debug,
	}, nil
}

func (o *oauth) authorize(ctx context.Context) (*url.URL, error) {
	u, err := url.Parse(oauthAuthorizeEndpoint)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("client_id", "IOS_CGI_MYQ")
	params.Set("code_challenge", o.challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("redirect_uri", "com.myqops://ios")
	params.Set("response_type", "code")
	params.Set("scope", "MyQ_Residential offline_access")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Jar: o.jar}
	resp, err := doRequest(client, req)
	if err != nil {
		return nil, err
	}
	defer drain(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received unexpected HTTP status code %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	tok := verificationToken(doc)
	if tok == "" {
		return nil, fmt.Errorf("unable to extract verification token from login page")
	}
	o.verificationToken = tok
	return resp.Request.URL, nil
}

func (o *oauth) login(ctx context.Context, u *url.URL, email, password string) (*url.URL, error) {
	params := url.Values{}
	params.Set("Email", email)
	params.Set("Password", password)
	params.Set("__RequestVerificationToken", o.verificationToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Jar: o.jar}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := doRequest(client, req)
	if err != nil {
		return nil, err
	}
	defer drain(resp.Body)

	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("received unexpected HTTP status code %d", resp.StatusCode)
	}
	return resp.Location()
}

func (o *oauth) callback(ctx context.Context, u *url.URL) (*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Jar: o.jar}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := doRequest(client, req)
	if err != nil {
		return nil, err
	}
	defer drain(resp.Body)

	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("received unexpected HTTP status code %d", resp.StatusCode)
	}
	return resp.Location()
}

func (o *oauth) token(ctx context.Context, u *url.URL) (string, error) {
	params := url.Values{}
	params.Set("client_id", "IOS_CGI_MYQ")
	params.Set("client_secret", "VUQ0RFhuS3lQV3EyNUJTdw==")
	params.Set("code", u.Query().Get("code"))
	params.Set("code_verifier", o.verifier)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", "com.myqops://ios")
	params.Set("scope", u.Query().Get("scope"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://partner-identity.myq-cloud.com/connect/token", strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Jar: o.jar}
	resp, err := doRequest(client, req)
	if err != nil {
		return "", err
	}
	defer drain(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received unexpected HTTP status code %d", resp.StatusCode)
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}
	return tokenResponse.AccessToken, nil
}

func pkceChallenge() (challenge, verifier string) {
	enc := base64.URLEncoding.WithPadding(base64.NoPadding)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	verifier = enc.EncodeToString(b)
	h := sha256.New()
	_, _ = h.Write([]byte(verifier))
	challenge = enc.EncodeToString(h.Sum(nil))
	return challenge, verifier
}

func verificationToken(doc *html.Node) string {
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "input" {
			var name, value string
			for _, a := range n.Attr {
				switch a.Key {
				case "name":
					name = a.Val
				case "value":
					value = a.Val
				}
			}
			if name == "__RequestVerificationToken" && value != "" {
				return value
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := walk(c); v != "" {
				return v
			}
		}
		return ""
	}
	return walk(doc)
}
