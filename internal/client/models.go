package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// Model is one entry from the models endpoint.
//
// Capabilities is deliberately kept as raw JSON. The payload is not a flat
// map of name to boolean: `context_management` and `effort` nest further
// sub-capabilities under their own `supported` flag, and the set of both
// capabilities and sub-capabilities grows as models ship. Decoding into a
// fixed Go shape would drop whatever arrived that the shape did not
// anticipate, silently.
type Model struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	DisplayName    string          `json:"display_name"`
	CreatedAt      string          `json:"created_at"`
	MaxInputTokens int64           `json:"max_input_tokens"`
	MaxTokens      int64           `json:"max_tokens"`
	Capabilities   json.RawMessage `json:"capabilities"`
}

// SupportedCapabilities reports the top-level `supported` flag of each
// capability, which is the question configurations actually ask. Anything
// nested below stays available through the raw payload.
//
// A capability whose value is not an object, or which carries no `supported`
// key, is omitted rather than guessed at.
func (m Model) SupportedCapabilities() (map[string]bool, error) {
	if len(m.Capabilities) == 0 {
		return map[string]bool{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(m.Capabilities, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(raw))
	for name, v := range raw {
		var probe struct {
			Supported *bool `json:"supported"`
		}
		if err := json.Unmarshal(v, &probe); err != nil || probe.Supported == nil {
			continue
		}
		out[name] = *probe.Supported
	}
	return out, nil
}

// ListModels returns the models the configured credential can see.
//
// This is an inference-scoped endpoint, so it takes CredAPIKey rather than an
// admin credential: an org:admin OAuth token is rejected with "does not meet
// scope requirement any_of(user:inference, workspace:inference, ...)".
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	return listToken[Model](ctx, c, CredAPIKey, "/v1/models", nil)
}

// GetModel returns one model by id.
func (c *Client) GetModel(ctx context.Context, id string) (*Model, error) {
	var out Model
	if err := c.get(ctx, CredAPIKey, "/v1/models/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
