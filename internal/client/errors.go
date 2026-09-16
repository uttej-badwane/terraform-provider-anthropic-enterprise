package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the Admin API.
type APIError struct {
	StatusCode int
	Type       string
	Message    string
	RequestID  string
	Method     string
	Path       string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	s := fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.StatusCode, msg)
	if e.Type != "" {
		s += " (" + e.Type + ")"
	}
	if e.RequestID != "" {
		s += " request-id=" + e.RequestID
	}
	return s
}

type errorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func newAPIError(method, path string, resp *http.Response, raw []byte) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		Method:     method,
		Path:       path,
		RequestID:  resp.Header.Get("request-id"),
	}
	var env errorEnvelope
	if json.Unmarshal(raw, &env) == nil && env.Error.Message != "" {
		e.Type = env.Error.Type
		e.Message = env.Error.Message
	} else if s := strings.TrimSpace(string(raw)); s != "" && len(s) < 512 {
		e.Message = s
	}
	return e
}

// MissingCredentialError is returned when an endpoint's credential class is
// not configured on the provider.
type MissingCredentialError struct {
	Class CredentialClass
}

func (e *MissingCredentialError) Error() string {
	switch e.Class {
	case CredOAuth:
		return "this endpoint requires an org:admin OAuth token: set the provider attribute oauth_token or the ANTHROPIC_AUTH_TOKEN environment variable"
	case CredEnterprise:
		return "this endpoint requires a Claude Enterprise scoped API key: set the provider attribute enterprise_api_key or the ANTHROPIC_ENTERPRISE_API_KEY environment variable"
	case CredCompliance:
		return "this endpoint requires a Compliance Access Key: set the provider attribute compliance_api_key (ANTHROPIC_COMPLIANCE_API_KEY) or an enterprise_api_key that carries compliance scopes"
	case CredAPIKey:
		return "this endpoint requires a regular workspace API key: set the provider attribute api_key or the ANTHROPIC_API_KEY environment variable (Admin keys are not accepted by the Managed Agents and Skills APIs)"
	case CredAnalytics:
		return "this endpoint requires an Analytics API key: set the provider attribute analytics_api_key (ANTHROPIC_ANALYTICS_API_KEY) or an enterprise_api_key that carries read:analytics"
	default:
		return "this endpoint requires an Admin API key or org:admin OAuth token: set the provider attribute admin_api_key (ANTHROPIC_ADMIN_API_KEY) or oauth_token (ANTHROPIC_AUTH_TOKEN)"
	}
}

func hasStatus(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == code
}

// IsNotFound reports whether err is an HTTP 404.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsConflict reports whether err is an HTTP 409.
func IsConflict(err error) bool { return hasStatus(err, http.StatusConflict) }

// IsUnauthorized reports whether err is an HTTP 401.
func IsUnauthorized(err error) bool { return hasStatus(err, http.StatusUnauthorized) }

// IsForbidden reports whether err is an HTTP 403.
func IsForbidden(err error) bool { return hasStatus(err, http.StatusForbidden) }

// IsBadRequest reports whether err is an HTTP 400.
func IsBadRequest(err error) bool { return hasStatus(err, http.StatusBadRequest) }

// IsMissingCredential reports whether err is a MissingCredentialError.
func IsMissingCredential(err error) bool {
	var mc *MissingCredentialError
	return errors.As(err, &mc)
}
