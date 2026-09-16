// Package client is a minimal HTTP client for the Anthropic Admin API
// (https://api.anthropic.com/v1/organizations/*).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return nil, fmt.Errorf("invalid base_url %q: %w", cfg.BaseURL, err)
	}
	if cfg.AdminAPIKey == "" && cfg.OAuthToken == "" && cfg.EnterpriseAPIKey == "" && cfg.ComplianceAPIKey == "" && cfg.AnalyticsAPIKey == "" && cfg.APIKey == "" {
		return nil, fmt.Errorf("no credentials configured: set admin_api_key, oauth_token, enterprise_api_key, compliance_api_key, analytics_api_key or api_key")
	}

	rc := retryablehttp.NewClient()
	rc.Logger = nil
	rc.RetryMax = defaultRetries
	if cfg.MaxRetries != nil {
		rc.RetryMax = *cfg.MaxRetries
	}
	rc.RetryWaitMin = 500 * time.Millisecond
	rc.RetryWaitMax = 20 * time.Second
	rc.CheckRetry = checkRetry
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	if cfg.HTTPClient != nil {
		rc.HTTPClient = cfg.HTTPClient
	} else {
		rc.HTTPClient.Timeout = defaultTimeout
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
	}
	return false
}

func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return true, nil //nolint:nilerr // transport errors are retried
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return true, nil
	}
	return false, nil
}

func (c *Client) authorize(req *http.Request, class CredentialClass) error {
	switch class {
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

	tflog.Debug(ctx, "anthropic admin api request", map[string]any{"method": method, "path": path})

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("%s %s: reading response: %w", method, path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(method, path, resp, raw)
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
