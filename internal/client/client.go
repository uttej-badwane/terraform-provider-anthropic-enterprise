// Package client is a minimal HTTP client for the Anthropic Admin API
// (https://api.anthropic.com/v1/organizations/*).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	DefaultBaseURL   = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
	defaultTimeout   = 60 * time.Second
	defaultRetries   = 4
	// maxResponseBytes bounds how much of a response body is read.
	maxResponseBytes = 16 << 20
	// errorBodySnippet bounds how much of a non-JSON error body is logged.
	errorBodySnippet = 512
)

// CredentialClass selects which credential a request must be sent with.
type CredentialClass int

const (
	// CredAdmin accepts an Admin API key; an org:admin OAuth token is used
	// instead when one is configured.
	CredAdmin CredentialClass = iota
	// CredOAuth requires an org:admin OAuth token (service accounts,
	// federation issuers/rules, workspace service-account memberships).
	CredOAuth
	// CredEnterprise requires a Claude Enterprise scoped API key
	// (RBAC groups, roles, spend limits on claude.ai organizations).
	CredEnterprise
	// CredCompliance requires a Compliance Access Key (or an Enterprise key
	// carrying compliance scopes) for the /v1/compliance directory endpoints.
	CredCompliance
	// CredAnalytics requires an Analytics API key (or an Enterprise key
	// carrying read:analytics) for /v1/organizations/analytics.
	CredAnalytics
	// CredAPIKey requires a regular workspace API key for the Managed Agents
	// control plane (/v1/agents, environments, vaults, deployments, memory
	// stores) and the Skills API. Admin keys are rejected there.
	CredAPIKey
	// CredNone carries no credential. The federated token exchange is the one
	// endpoint that needs this: the signed assertion in the body is what
	// authenticates the caller, and sending a key alongside it would only widen
	// what a compromised request could reach.
	CredNone
)

func (c CredentialClass) String() string {
	switch c {
	case CredAdmin:
		return "admin_api_key"
	case CredOAuth:
		return "oauth_token"
	case CredEnterprise:
		return "enterprise_api_key"
	case CredCompliance:
		return "compliance_api_key"
	case CredAnalytics:
		return "analytics_api_key"
	case CredAPIKey:
		return "api_key"
	case CredNone:
		return "none"
	}
	return "unknown"
}

// Config configures a Client.
type Config struct {
	BaseURL          string
	AdminAPIKey      string
	OAuthToken       string
	EnterpriseAPIKey string
	ComplianceAPIKey string
	AnalyticsAPIKey  string
	APIKey           string
	// WorkspaceID is sent as anthropic-workspace-id on CredAPIKey requests.
	WorkspaceID string
	UserAgent   string
	// HTTPClient is optional; tests inject one.
	HTTPClient *http.Client
	// MaxRetries overrides the default retry count; negative disables retries.
	MaxRetries *int
	// RequestTimeout overrides the per-request timeout. Ignored when
	// HTTPClient is supplied, since that client carries its own.
	RequestTimeout *time.Duration
}

// Client talks to the Admin API.
type Client struct {
	baseURL    string
	admin      string
	oauth      string
	enterprise string
	compliance string
	analytics  string
	apiKey     string
	workspace  string
	userAgent  string
	http       *retryablehttp.Client
}

// New returns a configured Client. At least one credential must be set.
func New(cfg Config) (*Client, error) {
	base, err := validateBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	// A client with no credential at all is valid: the federated token exchange
	// authenticates with the assertion in its body, so a configuration whose
	// only use of the provider is minting a token has nothing to supply here.
	// Every other endpoint still refuses to send an unauthenticated request —
	// authorize returns a MissingCredentialError naming the attribute that
	// endpoint needs, which is a better message than a list of all six.

	rc := retryablehttp.NewClient()
	rc.Logger = nil
	rc.RetryMax = defaultRetries
	if cfg.MaxRetries != nil {
		rc.RetryMax = *cfg.MaxRetries
	}
	rc.RetryWaitMin = 500 * time.Millisecond
	rc.RetryWaitMax = 20 * time.Second
	rc.CheckRetry = checkRetry
	rc.Backoff = clampedBackoff
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	if cfg.HTTPClient != nil {
		rc.HTTPClient = cfg.HTTPClient
	} else {
		rc.HTTPClient.Timeout = defaultTimeout
		if cfg.RequestTimeout != nil {
			rc.HTTPClient.Timeout = *cfg.RequestTimeout
		}
	}
	// The API never redirects. Following one would replay X-Api-Key on the
	// new host, because net/http strips only Authorization and Cookie when the
	// host changes. Surface the 3xx as an error instead.
	if rc.HTTPClient.CheckRedirect == nil {
		rc.HTTPClient.CheckRedirect = refuseRedirect
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = "terraform-provider-anthropic-enterprise"
	}

	return &Client{
		baseURL:    base,
		admin:      cfg.AdminAPIKey,
		oauth:      cfg.OAuthToken,
		enterprise: cfg.EnterpriseAPIKey,
		compliance: cfg.ComplianceAPIKey,
		analytics:  cfg.AnalyticsAPIKey,
		apiKey:     cfg.APIKey,
		workspace:  cfg.WorkspaceID,
		userAgent:  ua,
		http:       rc,
	}, nil
}

// BaseURL returns the configured API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// HasAdmin reports whether Console-org endpoints can be called.
func (c *Client) HasAdmin() bool { return c.admin != "" || c.oauth != "" }

// HasOAuth reports whether OAuth-only endpoints can be called.
func (c *Client) HasOAuth() bool { return c.oauth != "" }

// HasEnterprise reports whether Claude Enterprise endpoints can be called.
func (c *Client) HasEnterprise() bool { return c.enterprise != "" }

// HasCompliance reports whether Compliance directory endpoints can be called.
// An Enterprise key is accepted as a fallback because scoped keys may carry
// compliance scopes.
func (c *Client) HasCompliance() bool { return c.compliance != "" || c.enterprise != "" }

// HasAnalytics reports whether Enterprise analytics endpoints can be called.
func (c *Client) HasAnalytics() bool { return c.analytics != "" || c.enterprise != "" }

// HasAPIKey reports whether Managed Agents / Skills endpoints can be called.
func (c *Client) HasAPIKey() bool { return c.apiKey != "" }

// Has reports whether the given credential class is satisfiable.
func (c *Client) Has(class CredentialClass) bool {
	switch class {
	case CredAdmin:
		return c.HasAdmin()
	case CredOAuth:
		return c.HasOAuth()
	case CredEnterprise:
		return c.HasEnterprise()
	case CredCompliance:
		return c.HasCompliance()
	case CredAnalytics:
		return c.HasAnalytics()
	case CredAPIKey:
		return c.HasAPIKey()
	case CredNone:
		return true
	}
	return false
}

// validateBaseURL normalises the API base URL and rejects one that would send
// a credential somewhere it should not go: a non-HTTPS scheme (except to the
// loopback interface, where the bundled mock server listens) or embedded
// userinfo. Error text never contains the raw value.
func validateBaseURL(raw string) (string, error) {
	base := strings.TrimRight(raw, "/")
	if base == "" {
		return DefaultBaseURL, nil
	}
	u, err := url.ParseRequestURI(base)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid base_url: expected an absolute URL such as %s", DefaultBaseURL)
	}
	if u.User != nil {
		return "", fmt.Errorf("invalid base_url %s: credentials in the URL are not supported; use the provider credential attributes", u.Redacted())
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return "", fmt.Errorf("invalid base_url %s: http is only accepted for loopback addresses; use https", u.Redacted())
		}
	default:
		return "", fmt.Errorf("invalid base_url %s: scheme must be https", u.Redacted())
	}
	return base, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func refuseRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// clampedBackoff honours Retry-After like the default policy but never sleeps
// longer than RetryWaitMax, so a hostile or misconfigured server cannot park
// the provider indefinitely.
func clampedBackoff(minWait, maxWait time.Duration, attempt int, resp *http.Response) time.Duration {
	d := retryablehttp.DefaultBackoff(minWait, maxWait, attempt, resp)
	if d > maxWait {
		return maxWait
	}
	return d
}

// ctxKeyCreate marks a request that creates an object. Such a request is not
// replayed after a 5xx or a mid-flight transport error: the server may have
// committed the write before failing, and a replay would create a duplicate
// that Terraform never learns about.
type ctxKeyCreate struct{}

func isCreate(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyCreate{}).(bool)
	return v
}

func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	create := isCreate(ctx)
	if err != nil {
		if create && !failedBeforeSend(err) {
			return false, nil //nolint:nilerr // the write may have landed; surface the error
		}
		return true, nil //nolint:nilerr // transport errors are retried
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}
	if resp.StatusCode >= 500 {
		return !create, nil
	}
	return false, nil
}

// failedBeforeSend reports whether a transport error happened before any
// request bytes reached the server, in which case a replay cannot duplicate
// a write.
func failedBeforeSend(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

func (c *Client) authorize(req *http.Request, class CredentialClass) error {
	switch class {
	case CredNone:
		// Deliberately unauthenticated; the request body carries the assertion.
		return nil
	case CredAdmin:
		if c.oauth != "" {
			req.Header.Set("Authorization", "Bearer "+c.oauth)
			return nil
		}
		if c.admin != "" {
			req.Header.Set("X-Api-Key", c.admin)
			return nil
		}
	case CredOAuth:
		if c.oauth != "" {
			req.Header.Set("Authorization", "Bearer "+c.oauth)
			return nil
		}
	case CredEnterprise:
		if c.enterprise != "" {
			req.Header.Set("X-Api-Key", c.enterprise)
			return nil
		}
	case CredCompliance:
		if k := firstNonEmpty(c.compliance, c.enterprise); k != "" {
			req.Header.Set("X-Api-Key", k)
			return nil
		}
	case CredAnalytics:
		if k := firstNonEmpty(c.analytics, c.enterprise); k != "" {
			req.Header.Set("X-Api-Key", k)
			return nil
		}
	case CredAPIKey:
		if c.apiKey != "" {
			req.Header.Set("X-Api-Key", c.apiKey)
			if c.workspace != "" {
				req.Header.Set("Anthropic-Workspace-Id", c.workspace)
			}
			return nil
		}
	}
	return &MissingCredentialError{Class: class}
}

// reqOption mutates an outgoing request (extra headers such as anthropic-beta).
type reqOption func(*http.Request)

// withBeta adds an anthropic-beta header value.
func withBeta(value string) reqOption {
	return func(r *http.Request) { r.Header.Add("Anthropic-Beta", value) }
}

// rawBody carries a pre-encoded request body with its content type.
type rawBody struct {
	contentType string
	data        []byte
}

// do performs a JSON request. body may be nil, a JSON-encodable value, or a
// rawBody (multipart); out may be nil.
func (c *Client) do(ctx context.Context, class CredentialClass, method, path string, query url.Values, body, out any, opts ...reqOption) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var payload io.Reader
	contentType := "application/json"
	if body != nil {
		switch b := body.(type) {
		case rawBody:
			payload = bytes.NewReader(b.data)
			contentType = b.contentType
		default:
			enc, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("encoding request body: %w", err)
			}
			payload = bytes.NewReader(enc)
		}
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, u, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Anthropic-Version", anthropicVersion)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if err := c.authorize(req.Request, class); err != nil {
		return err
	}
	for _, o := range opts {
		o(req.Request)
	}

	ctx = c.maskSecrets(ctx)
	tflog.Debug(ctx, "anthropic admin api request", map[string]any{"method": method, "path": path})

	resp, err := c.http.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("%s %s: reading response: %w", method, path, err)
	}
	if len(raw) > maxResponseBytes {
		return fmt.Errorf("%s %s: response body exceeds %d bytes", method, path, maxResponseBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := newAPIError(method, path, resp, raw)
		if apiErr.Type == "" && len(raw) > 0 {
			// Not the API's error envelope, so it came from something in
			// between. Keep it out of diagnostics; a debug log is enough.
			snippet := raw
			if len(snippet) > errorBodySnippet {
				snippet = snippet[:errorBodySnippet]
			}
			tflog.Debug(ctx, "non-JSON error response body", map[string]any{"status": resp.StatusCode, "body": string(snippet)})
		}
		return apiErr
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s %s: decoding response: %w", method, path, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, class CredentialClass, path string, q url.Values, out any, opts ...reqOption) error {
	return c.do(ctx, class, http.MethodGet, path, q, nil, out, opts...)
}

func (c *Client) post(ctx context.Context, class CredentialClass, path string, body, out any, opts ...reqOption) error {
	return c.do(ctx, class, http.MethodPost, path, nil, body, out, opts...)
}

// create is post for requests that make a new object. See ctxKeyCreate.
func (c *Client) create(ctx context.Context, class CredentialClass, path string, body, out any, opts ...reqOption) error {
	return c.post(context.WithValue(ctx, ctxKeyCreate{}, true), class, path, body, out, opts...)
}

// maskSecrets redacts every configured credential from log output produced
// with the returned context, wherever the value appears.
func (c *Client) maskSecrets(ctx context.Context) context.Context {
	var secrets []string
	for _, s := range []string{c.admin, c.oauth, c.enterprise, c.compliance, c.analytics, c.apiKey} {
		if s != "" {
			secrets = append(secrets, s)
		}
	}
	ctx = tflog.MaskAllFieldValuesStrings(ctx, secrets...)
	return tflog.MaskMessageStrings(ctx, secrets...)
}

func (c *Client) delete(ctx context.Context, class CredentialClass, path string, opts ...reqOption) error {
	return c.do(ctx, class, http.MethodDelete, path, nil, nil, nil, opts...)
}

// FilePart is one file in a multipart upload.
type FilePart struct {
	Field    string // form field name, e.g. "files"
	Filename string // relative path inside the upload
	Data     []byte
}

// postMultipart sends form fields and files as multipart/form-data.
func (c *Client) postMultipart(ctx context.Context, class CredentialClass, path string, fields map[string]string, files []FilePart, out any, opts ...reqOption) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	for _, f := range files {
		part, err := mw.CreateFormFile(f.Field, f.Filename)
		if err != nil {
			return err
		}
		if _, err := part.Write(f.Data); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	// Every multipart endpoint creates a skill or a skill version.
	ctx = context.WithValue(ctx, ctxKeyCreate{}, true)
	return c.do(ctx, class, http.MethodPost, path, nil, rawBody{contentType: mw.FormDataContentType(), data: buf.Bytes()}, out, opts...)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
