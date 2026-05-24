// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// Custom SRM transport layer for synology-router-pp-cli.
//
// Synology Router Manager (SRM) uses an RPC-style API where all operations
// POST to /webapi/entry.cgi (or /webapi/auth.cgi) with form-encoded parameters:
//   api=SYNO.<Namespace>.<Method>  (API namespace)
//   method=<operation>             (e.g. get, list, set)
//   version=<int>                  (API version)
//   ...plus operation-specific fields
//
// This transport intercepts outgoing requests to virtual REST paths
// (e.g. GET /webapi/devices) and rewrites them as actual SRM API calls
// (POST /webapi/entry.cgi with the correct form params).
//
// The route table below maps each virtual path to its SRM API equivalent.

package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// srmRoute describes how to translate a virtual REST path to an SRM API call.
type srmRoute struct {
	// Endpoint is the actual SRM CGI endpoint (e.g. /webapi/entry.cgi)
	Endpoint string
	// API is the SRM API namespace (e.g. SYNO.Core.Network.NSM.Device)
	API string
	// Method is the SRM operation (e.g. get, list, set)
	Method string
	// Version is the API version integer
	Version int
	// NoAuth skips adding the session cookie (for login/public endpoints)
	NoAuth bool
	// QueryParamsToForm lists which query params to add to the form body
	QueryParamsToForm []string
	// DefaultParams are always sent (overridable by query params)
	DefaultParams map[string]string
}

// srmRouteTable maps "HTTP_METHOD /path" to SRM route info.
// Virtual paths are relative to the webapi base (without /webapi prefix).
var srmRouteTable = map[string]srmRoute{
	// Authentication
	"POST /session/login": {
		Endpoint: "/webapi/auth.cgi",
		API:      "SYNO.API.Auth",
		Method:   "Login",
		Version:  2,
		NoAuth:   true,
	},
	"POST /session/logout": {
		Endpoint: "/webapi/auth.cgi",
		API:      "SYNO.API.Auth",
		Method:   "Logout",
		Version:  2,
	},

	// Device management
	"GET /devices": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.Network.NSM.Device",
		Method:            "get",
		Version:           4,
		QueryParamsToForm: []string{"conntype", "info"},
	},

	// Traffic
	"GET /traffic": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.NGFW.Traffic",
		Method:            "get",
		Version:           1,
		QueryParamsToForm: []string{"interval", "mode"},
	},

	// System utilization
	"GET /utilization": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.System.Utilization",
		Method:            "get",
		Version:           1,
		QueryParamsToForm: []string{"resource"},
	},

	// WAN status
	"GET /wan/status": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.Router.ConnectionStatus",
		Method:   "get",
		Version:  1,
	},

	// Wi-Fi
	"GET /wifi/settings": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Mesh.Network.WifiDevice",
		Method:            "get",
		Version:           1,
		QueryParamsToForm: []string{"band"},
	},
	"PUT /wifi/settings": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Mesh.Network.WifiDevice",
		Method:   "set",
		Version:  1,
	},

	// Wake-on-LAN
	"GET /wol/devices": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.Network.WOL",
		Method:            "get_devices",
		Version:           1,
		QueryParamsToForm: []string{"client_list"},
	},
	"POST /wol/devices": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.WOL",
		Method:   "add_device",
		Version:  1,
	},
	"POST /wol/wake": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.WOL",
		Method:   "wake",
		Version:  1,
	},

	// QoS
	"GET /qos/rules": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.NGFW.QoS.Rules",
		Method:   "get",
		Version:  1,
	},

	// Firewall / policy routes
	"GET /firewall/rules": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.Network.Router.PolicyRoute",
		Method:            "get",
		Version:           1,
		QueryParamsToForm: []string{"type"},
		DefaultParams:     map[string]string{"type": "ipv4"},
	},
	"PUT /firewall/rules": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.Router.PolicyRoute",
		Method:   "set",
		Version:  1,
	},

	// Smart WAN
	"GET /smartwan/gateways": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.Core.Network.SmartWAN.Gateway",
		Method:            "list",
		Version:           1,
		QueryParamsToForm: []string{"gatewaytype"},
		DefaultParams:     map[string]string{"gatewaytype": "ipv4"},
	},
	"GET /smartwan/config": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.SmartWAN.General",
		Method:   "get",
		Version:  1,
	},
	"PUT /smartwan/config": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.Network.SmartWAN.General",
		Method:   "set",
		Version:  1,
	},

	// DNS / DDNS
	"GET /dns/external-ip": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.DDNS.ExtIP",
		Method:   "list",
		Version:  1,
	},
	"GET /dns/ddns": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Core.DDNS.Record",
		Method:   "list",
		Version:  1,
	},

	// Mesh
	"GET /mesh/nodes": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Mesh.Node.List",
		Method:   "get",
		Version:  3,
	},
	"GET /mesh/info": {
		Endpoint: "/webapi/entry.cgi",
		API:      "SYNO.Mesh.System.Info",
		Method:   "get",
		Version:  1,
	},

	// Access control
	"GET /access-control/groups": {
		Endpoint:          "/webapi/entry.cgi",
		API:               "SYNO.SafeAccess.AccessControl.ConfigGroup",
		Method:            "get",
		Version:           1,
		QueryParamsToForm: []string{"additional"},
	},

	// System API discovery
	"GET /system/api-info": {
		Endpoint:          "/webapi/query.cgi",
		API:               "SYNO.API.Info",
		Method:            "query",
		Version:           1,
		QueryParamsToForm: []string{"query"},
	},
}

// SRMTransport is an http.RoundTripper that intercepts requests to virtual
// REST paths and rewrites them as actual SRM API form-encoded POST calls.
//
// The SRM API requires:
//   - POST to /webapi/entry.cgi (or /webapi/auth.cgi for auth)
//   - Content-Type: application/x-www-form-urlencoded
//   - Form fields: api=<namespace>, method=<op>, version=<n>, _sid=<sid>
//   - Cookie: id=<sid> (session ID from auth login)
//
// Nested transports: the inner transport is the original http.Transport
// (or surf transport) that handles TLS, HTTP/2, etc.
type SRMTransport struct {
	// Inner is the underlying transport for actual HTTP execution
	Inner http.RoundTripper
	// SID is the current session ID (set after successful login)
	SID string
	// BaseURL is the router base URL (used to strip prefix from paths)
	BaseURL string
}

// RoundTrip intercepts HTTP requests, rewrites them to SRM API format, and executes them.
func (t *SRMTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Extract the path relative to the base URL
	virtualPath := req.URL.Path
	if t.BaseURL != "" {
		basePath := extractPath(t.BaseURL)
		if basePath != "" && strings.HasPrefix(virtualPath, basePath) {
			virtualPath = virtualPath[len(basePath):]
		}
	}
	if !strings.HasPrefix(virtualPath, "/") {
		virtualPath = "/" + virtualPath
	}

	// Look up route in table
	routeKey := req.Method + " " + virtualPath
	route, found := srmRouteTable[routeKey]
	if !found {
		// Not an SRM virtual path — pass through unmodified
		return t.Inner.RoundTrip(req)
	}

	// Build the SRM form body
	form := url.Values{}
	form.Set("api", route.API)
	form.Set("method", route.Method)
	form.Set("version", fmt.Sprintf("%d", route.Version))

	// Add session ID if not a no-auth route
	if !route.NoAuth && t.SID != "" {
		form.Set("_sid", t.SID)
	}

	// Apply default params (query params override these below)
	for k, v := range route.DefaultParams {
		form.Set(k, v)
	}

	// Move query params to form body (for GET-style params that SRM expects in form)
	queryParams := req.URL.Query()
	for _, paramName := range route.QueryParamsToForm {
		if val := queryParams.Get(paramName); val != "" {
			form.Set(paramName, val)
		}
	}

	// For non-GET requests (POST/PUT), also include original query params
	if req.Method != http.MethodGet {
		for k, vals := range queryParams {
			if !isInList(k, route.QueryParamsToForm) {
				for _, v := range vals {
					form.Set(k, v)
				}
			}
		}
	}

	// Merge any existing form body (for POST/PUT with body)
	if req.Body != nil && req.Body != http.NoBody {
		existingBody, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err == nil && len(existingBody) > 0 {
			ct := req.Header.Get("Content-Type")
			if strings.Contains(ct, "application/x-www-form-urlencoded") {
				if existingForm, err := url.ParseQuery(string(existingBody)); err == nil {
					for k, vals := range existingForm {
						for _, v := range vals {
							form.Set(k, v)
						}
					}
				}
			} else if strings.Contains(ct, "application/json") {
				// JSON body: store as the "payload" field so the SRM API can see it
				// For SRM, JSON bodies are typically stringified and passed as specific fields
				form.Set("payload", string(existingBody))
			}
		}
	}

	// Build the actual SRM request
	// The endpoint path is relative to the server root (not the webapi base)
	srmPath := route.Endpoint
	srmURL := *req.URL
	srmURL.Path = srmPath
	srmURL.RawQuery = "" // All params go in the form body

	formEncoded := form.Encode()
	srmReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, srmURL.String(), strings.NewReader(formEncoded))
	if err != nil {
		return nil, fmt.Errorf("srm_transport: building request: %w", err)
	}

	// Copy headers from original request, then override for SRM
	for k, vals := range req.Header {
		for _, v := range vals {
			srmReq.Header.Add(k, v)
		}
	}
	srmReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Set session cookie if we have one
	if !route.NoAuth && t.SID != "" {
		srmReq.AddCookie(&http.Cookie{
			Name:  "id",
			Value: t.SID,
		})
	}

	return t.Inner.RoundTrip(srmReq)
}

// extractPath extracts the path portion from a full URL string.
func extractPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

// stripWebAPIBase returns the server root (scheme+host+port) from a full URL path.
// e.g. for URL "https://192.168.1.254:8001/webapi/devices" with base "https://192.168.1.254:8001/webapi"
// returns "https://192.168.1.254:8001"
func stripWebAPIBase(fullPath, baseURL string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// isInList checks if s is in the list.
func isInList(s string, list []string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// NewSRMClient creates a Client with the SRM transport installed.
// Call this instead of client.New when you need the actual SRM API transport.
// The sid parameter is the session ID from a previous auth login (may be empty
// if first login has not happened yet).
func NewSRMClient(c *Client, sid string) {
	if c == nil || c.HTTPClient == nil {
		return
	}
	inner := c.HTTPClient.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	// Don't double-wrap
	if _, ok := inner.(*SRMTransport); ok {
		return
	}
	c.HTTPClient.Transport = &SRMTransport{
		Inner:   inner,
		SID:     sid,
		BaseURL: c.BaseURL,
	}
}

// SRMTransportFrom returns the *SRMTransport from a Client's HTTP transport,
// or nil if not installed.
func SRMTransportFrom(c *Client) *SRMTransport {
	if c == nil || c.HTTPClient == nil {
		return nil
	}
	t, _ := c.HTTPClient.Transport.(*SRMTransport)
	return t
}

// WrapWithSRMAuth creates a new client with the SRM session ID installed.
// Used to set up authenticated calls after a successful login.
func WrapWithSRMAuth(c *Client, sid string) {
	if t := SRMTransportFrom(c); t != nil {
		t.SID = sid
	} else {
		NewSRMClient(c, sid)
	}
}

// SRMSessionFromConfig reads the SRM session ID from the client config.
// The session ID is stored as SynologyRouterCookieAuth in the config.
func SRMSessionFromConfig(c *Client) string {
	if c == nil || c.Config == nil {
		return ""
	}
	return c.Config.SynologyRouterCookieAuth
}

// formBody builds a form-encoded body string from the given fields.
// Convenience helper for SRM API calls in hand-written command code.
func FormBody(fields map[string]string) string {
	form := url.Values{}
	for k, v := range fields {
		if v != "" {
			form.Set(k, v)
		}
	}
	return form.Encode()
}

// PostSRMForm posts directly to an SRM API endpoint with the given form fields.
// This bypasses the virtual path routing and calls the SRM API directly.
// Used for operations that don't fit the standard routing (e.g. WoL wake with MAC arg).
func (c *Client) PostSRMForm(apiName, method string, version int, extraFields map[string]string) ([]byte, error) {
	form := url.Values{}
	form.Set("api", apiName)
	form.Set("method", method)
	form.Set("version", fmt.Sprintf("%d", version))

	sid := SRMSessionFromConfig(c)
	if sid != "" {
		form.Set("_sid", sid)
	}

	for k, v := range extraFields {
		if v != "" {
			form.Set(k, v)
		}
	}

	// Build the request URL directly to entry.cgi
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if idx := strings.LastIndex(baseURL, "/webapi"); idx >= 0 {
		baseURL = baseURL[:idx]
	}
	endpoint := baseURL + "/webapi/entry.cgi"

	formEncoded := form.Encode()
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(formEncoded))
	if err != nil {
		return nil, fmt.Errorf("creating SRM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SRM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading SRM response: %w", err)
	}
	return body, nil
}
