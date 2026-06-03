// Copyright 2026 giacaglia. Licensed under Apache-2.0. See LICENSE.
// Extensions to the generated client — adds context-accepting variants and
// WithParams variants used by the mcp/tools.go and mcp/code_orch.go surfaces.
// The client does not thread context through its HTTP stack; these wrappers
// accept ctx for API compatibility and ignore it.

package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// BinaryResponseHeader is a sentinel request header that signals the response
// body is binary (e.g. a function body download). The MCP surface sets this
// header so callers can base64-encode the raw bytes instead of interpreting
// them as JSON text.
const BinaryResponseHeader = "X-PP-Binary-Response"

// --- Context-accepting wrappers for Get ---

// Get accepts an optional context (ignored) plus the standard path+params args.
// Overloads the non-context Get to satisfy generated mcp surface callers that
// pass ctx as the first argument.
//
// Disambiguation: the generated client.go defines Get(path, params).
// We shadow it here with the context form.
func (c *Client) GetCtx(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	return c.GetWithHeaders(path, params, nil)
}

// GetWithHeadersCtx is the context-accepting variant of GetWithHeaders.
func (c *Client) GetWithHeadersCtx(_ context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	return c.GetWithHeaders(path, params, headers)
}

// --- WithParams variants (query params forwarded for mutating methods) ---

// PostWithParams sends a POST with both query params and a JSON body.
func (c *Client) PostWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, body, nil)
}

// PostWithParamsAndHeaders sends a POST with query params, a JSON body, and
// per-request header overrides.
func (c *Client) PostWithParamsAndHeaders(_ context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, body, headers)
}

// PostQueryWithParams is PostWithParams for read-only (idempotent) POST endpoints.
// Semantically equivalent; the distinction exists so the MCP surface can annotate
// these tools as read-only without changing the wire behaviour.
func (c *Client) PostQueryWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, body, nil)
}

// PostQueryWithParamsAndHeaders is PostWithParamsAndHeaders for read-only POST endpoints.
func (c *Client) PostQueryWithParamsAndHeaders(_ context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, body, headers)
}

// PostMultipartWithParams sends a multipart POST with query params.
func (c *Client) PostMultipartWithParams(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, nil)
}

// PostMultipartWithParamsAndHeaders sends a multipart POST with query params and headers.
func (c *Client) PostMultipartWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, headers)
}

// PostFormWithParams sends a form-encoded POST with query params.
func (c *Client) PostFormWithParams(_ context.Context, path string, params map[string]string, fields url.Values) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, formRequestBody{Fields: fields}, nil)
}

// PostFormWithParamsAndHeaders sends a form-encoded POST with query params and headers.
func (c *Client) PostFormWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields url.Values, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("POST", path, params, formRequestBody{Fields: fields}, headers)
}

// PutWithParams sends a PUT with both query params and a JSON body.
func (c *Client) PutWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, body, nil)
}

// PutWithParamsAndHeaders sends a PUT with query params, body, and header overrides.
func (c *Client) PutWithParamsAndHeaders(_ context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, body, headers)
}

// PutMultipartWithParams sends a multipart PUT with query params.
func (c *Client) PutMultipartWithParams(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, nil)
}

// PutMultipartWithParamsAndHeaders sends a multipart PUT with query params and headers.
func (c *Client) PutMultipartWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, headers)
}

// PutFormWithParams sends a form-encoded PUT with query params.
func (c *Client) PutFormWithParams(_ context.Context, path string, params map[string]string, fields url.Values) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, formRequestBody{Fields: fields}, nil)
}

// PutFormWithParamsAndHeaders sends a form-encoded PUT with query params and headers.
func (c *Client) PutFormWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields url.Values, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PUT", path, params, formRequestBody{Fields: fields}, headers)
}

// PatchWithParams sends a PATCH with both query params and a JSON body.
func (c *Client) PatchWithParams(_ context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, body, nil)
}

// PatchWithParamsAndHeaders sends a PATCH with query params, body, and headers.
func (c *Client) PatchWithParamsAndHeaders(_ context.Context, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, body, headers)
}

// PatchMultipartWithParams sends a multipart PATCH with query params.
func (c *Client) PatchMultipartWithParams(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, nil)
}

// PatchMultipartWithParamsAndHeaders sends a multipart PATCH with query params and headers.
func (c *Client) PatchMultipartWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields map[string]string, fileFields map[string]string, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, multipartRequestBody{Fields: fields, FileFields: fileFields}, headers)
}

// PatchFormWithParams sends a form-encoded PATCH with query params.
func (c *Client) PatchFormWithParams(_ context.Context, path string, params map[string]string, fields url.Values) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, formRequestBody{Fields: fields}, nil)
}

// PatchFormWithParamsAndHeaders sends a form-encoded PATCH with query params and headers.
func (c *Client) PatchFormWithParamsAndHeaders(_ context.Context, path string, params map[string]string, fields url.Values, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("PATCH", path, params, formRequestBody{Fields: fields}, headers)
}

// DeleteWithParams sends a DELETE with query params forwarded as URL parameters.
func (c *Client) DeleteWithParams(_ context.Context, path string, params map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("DELETE", path, params, nil, nil)
}

// DeleteWithParamsAndHeaders sends a DELETE with query params and header overrides.
func (c *Client) DeleteWithParamsAndHeaders(_ context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, int, error) {
	return c.doWithParams("DELETE", path, params, nil, headers)
}

// doWithParams is a thin shim around do that passes params through.
// The generated do() already accepts params as the third argument, but the
// existing public convenience methods (Post, Put, etc.) hard-code nil there.
// WithParams variants call do() directly to pass the caller-supplied params.
func (c *Client) doWithParams(method, path string, params map[string]string, body any, headers map[string]string) (json.RawMessage, int, error) {
	return c.do(method, path, params, body, headers)
}
