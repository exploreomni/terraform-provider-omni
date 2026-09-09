// Package client is a thin, hand-written client for the Omni REST API.
//
// Base URL semantics follow https://docs.omni.co/api/base-url: the API lives at
// <your Omni URL>/api, e.g. https://blobsrus.omniapp.co/api. Callers pass either
// form; NewClient normalises it.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 60 * time.Second
	maxRetries     = 4
)

// Client talks to a single Omni instance.
type Client struct {
	baseURL   string
	token     string
	userAgent string
	http      *http.Client
}

// Option customises a Client.
type Option func(*Client)

// WithHTTPClient replaces the underlying *http.Client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.http.Timeout = d }
}

// NewClient builds a client for an Omni instance.
//
// baseURL may be "https://blobsrus.omniapp.co" or "https://blobsrus.omniapp.co/api";
// both resolve to the same API root.
func NewClient(baseURL, token string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("base_url is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("api_token is required")
	}

	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base_url %q: %w", baseURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("base_url must use https, got %q", u.Scheme)
	}
	if !strings.HasSuffix(u.Path, "/api") {
		u.Path = strings.TrimRight(u.Path, "/") + "/api"
	}

	c := &Client{
		baseURL:   u.String(),
		token:     token,
		userAgent: "terraform-provider-omni",
		http:      &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// BaseURL returns the resolved API root, e.g. https://blobsrus.omniapp.co/api.
func (c *Client) BaseURL() string { return c.baseURL }

// APIError is a non-2xx response from the Omni API.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Body
	}
	if len(msg) > 800 {
		msg = msg[:800] + "..."
	}
	return fmt.Sprintf("omni api: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, msg)
}

// IsNotFound reports whether err is a 404 from the Omni API. Resources use this
// to drop themselves from state instead of erroring.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}

// Request describes a single API call.
type Request struct {
	Method string
	Path   string     // e.g. "/v1/connections", relative to the API root
	Query  url.Values // optional
	Body   any        // marshalled as JSON when non-nil
	// Out, when non-nil, receives the unmarshalled JSON response body.
	Out any
}

// Do executes a request, retrying on 429 and 5xx with exponential backoff.
func (c *Client) Do(ctx context.Context, req Request) error {
	var payload []byte
	if req.Body != nil {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshalling request body: %w", err)
		}
		payload = b
	}

	endpoint := c.baseURL + req.Path
	if len(req.Query) > 0 {
		endpoint += "?" + req.Query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}

		httpReq, err := http.NewRequestWithContext(ctx, req.Method, endpoint, body)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("User-Agent", c.userAgent)
		if payload != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("%s %s: %w", req.Method, req.Path, err)
			continue
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("reading response body: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if req.Out == nil || len(bytes.TrimSpace(respBody)) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, req.Out); err != nil {
				return fmt.Errorf("decoding response from %s %s: %w", req.Method, req.Path, err)
			}
			return nil
		}

		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Method:     req.Method,
			Path:       req.Path,
			Message:    extractMessage(respBody),
			Body:       string(respBody),
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			apiErr.Message = retryAfterMessage(resp, apiErr.Message)
		}
		lastErr = apiErr

		if !retryable(resp.StatusCode) {
			return apiErr
		}
	}
	return lastErr
}

// Get is a convenience wrapper for GET requests.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.Do(ctx, Request{Method: http.MethodGet, Path: path, Query: query, Out: out})
}

// Post is a convenience wrapper for POST requests.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, Request{Method: http.MethodPost, Path: path, Body: body, Out: out})
}

// Patch is a convenience wrapper for PATCH requests.
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, Request{Method: http.MethodPatch, Path: path, Body: body, Out: out})
}

// Put is a convenience wrapper for PUT requests.
func (c *Client) Put(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, Request{Method: http.MethodPut, Path: path, Body: body, Out: out})
}

// Delete is a convenience wrapper for DELETE requests.
func (c *Client) Delete(ctx context.Context, path string, query url.Values) error {
	return c.Do(ctx, Request{Method: http.MethodDelete, Path: path, Query: query})
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func backoff(attempt int, lastErr error) time.Duration {
	// Omni rate limits at 60 requests/minute, so back off generously on 429.
	base := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	if apiErr, ok := lastErr.(*APIError); ok && apiErr.StatusCode == http.StatusTooManyRequests {
		if base < 5*time.Second {
			base = 5 * time.Second
		}
	}
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	return base
}

func retryAfterMessage(resp *http.Response, msg string) string {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if _, err := strconv.Atoi(v); err == nil {
			return strings.TrimSpace(msg + " (retry after " + v + "s)")
		}
	}
	return msg
}

// extractMessage pulls a human-readable message out of the several error shapes
// the Omni API uses: {"message":...}, {"detail":...}, {"error":...}.
func extractMessage(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	for _, key := range []string{"message", "detail", "error", "errors"} {
		if v, ok := payload[key]; ok {
			switch typed := v.(type) {
			case string:
				if typed != "" {
					return typed
				}
			default:
				if b, err := json.Marshal(typed); err == nil {
					return string(b)
				}
			}
		}
	}
	return strings.TrimSpace(string(body))
}
