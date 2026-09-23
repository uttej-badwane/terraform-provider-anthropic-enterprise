package client

import (
	"context"
	"encoding/json"
)

// CountTokensMessage is one turn in a token count request. Content is sent as a
// plain string, which the endpoint accepts alongside the block form.
type CountTokensMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CountTokensRequest is the body of a token count request.
//
// Tools is passed through as raw JSON. Tool definitions carry an arbitrary JSON
// Schema, which a fixed Go type could only approximate.
type CountTokensRequest struct {
	Model    string               `json:"model"`
	System   string               `json:"system,omitempty"`
	Messages []CountTokensMessage `json:"messages"`
	Tools    json.RawMessage      `json:"tools,omitempty"`
}

// CountTokens returns how many input tokens a request would use, without
// sending it to the model.
//
// Inference-scoped, like the models endpoints, so it takes CredAPIKey. The
// response carries input_tokens and nothing else.
func (c *Client) CountTokens(ctx context.Context, in CountTokensRequest) (int64, error) {
	var out struct {
		InputTokens int64 `json:"input_tokens"`
	}
	if err := c.post(ctx, CredAPIKey, "/v1/messages/count_tokens", in, &out); err != nil {
		return 0, err
	}
	return out.InputTokens, nil
}
