package mock

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Model fixtures mirror the live payload, including the part that shapes the
// provider schema: capabilities are not a flat map of name to boolean.
// context_management carries sub-capabilities under its own supported flag, so
// a fixture that used only {"supported": true} everywhere would let a flat
// decode pass and hide the nesting the data source has to preserve.
var mockModels = []json.RawMessage{
	json.RawMessage(`{
  "type": "model",
  "id": "claude-opus-5",
  "display_name": "Claude Opus 5",
  "created_at": "2026-07-24T00:00:00Z",
  "max_input_tokens": 1000000,
  "max_tokens": 128000,
  "capabilities": {
    "batch": {"supported": true},
    "citations": {"supported": true},
    "code_execution": {"supported": true},
    "image_input": {"supported": true},
    "thinking": {"supported": true},
    "context_management": {
      "supported": true,
      "clear_tool_uses_20250919": {"supported": true},
      "compact_20260112": {"supported": false}
    },
    "effort": {
      "supported": false,
      "low": {"supported": false},
      "max": {"supported": false}
    }
  }
}`),
	json.RawMessage(`{
  "type": "model",
  "id": "claude-haiku-4-5-20251001",
  "display_name": "Claude Haiku 4.5",
  "created_at": "2025-10-01T00:00:00Z",
  "max_input_tokens": 200000,
  "max_tokens": 64000,
  "capabilities": {
    "batch": {"supported": true},
    "citations": {"supported": false},
    "code_execution": {"supported": false},
    "image_input": {"supported": true},
    "thinking": {"supported": false},
    "context_management": {"supported": false},
    "effort": {"supported": false}
  }
}`),
}

// requireInference mirrors the real endpoint's credential rule: the models API
// is inference-scoped, so an admin key or an org:admin OAuth token is refused.
// Without this the data source could be wired to the wrong credential class
// and every test would still pass.
func (s *Server) requireInference(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "" {
		writeError(w, http.StatusForbidden, "permission_error",
			"OAuth token does not meet scope requirement any_of(user:inference, workspace:inference)")
		return false
	}
	switch r.Header.Get("X-Api-Key") {
	case AgentsKey:
		return true
	case AdminKey, EnterpriseKey, ComplianceKey, AnalyticsKey:
		writeError(w, http.StatusUnauthorized, "authentication_error", "this endpoint requires a workspace API key")
		return false
	default:
		writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return false
	}
}

func (s *Server) modelRoutes() {
	s.handle("GET /v1/models", s.listModels)
	s.handle("GET /v1/models/{id}", s.getModel)
	s.handle("POST /v1/messages/count_tokens", s.countTokens)
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	if !s.requireInference(w, r) {
		return
	}
	tokenList(w, r, mockModels)
}

func (s *Server) getModel(w http.ResponseWriter, r *http.Request) {
	if !s.requireInference(w, r) {
		return
	}
	id := r.PathValue("id")
	for _, m := range mockModels {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m, &probe); err == nil && probe.ID == id {
			writeJSON(w, m)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found_error", "model not found")
}

// countTokens mirrors /v1/messages/count_tokens closely enough to exercise the
// data source: it refuses the inputs the live endpoint refuses, and returns a
// deterministic count that grows with the system prompt, the messages and the
// tools, so tests can assert that each one is actually sent.
//
// The arithmetic is not the real tokenizer and is not meant to be.
func (s *Server) countTokens(w http.ResponseWriter, r *http.Request) {
	if !s.requireInference(w, r) {
		return
	}
	var in struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body")
		return
	}
	if !mockModelExists(in.Model) {
		writeError(w, http.StatusNotFound, "not_found_error", "model: "+in.Model)
		return
	}
	if len(in.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "messages: at least one message is required")
		return
	}
	n := int64(4)
	for i, m := range in.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			writeError(w, http.StatusBadRequest, "invalid_request_error",
				"messages."+strconv.Itoa(i)+": use the top-level 'system' parameter for the initial system prompt")
			return
		}
		n += int64(len(m.Content)/4) + 3
	}
	if in.System != "" {
		n += int64(len(in.System)/4) + 5
	}
	if len(in.Tools) > 0 && string(in.Tools) != "null" {
		n += 500 + int64(len(in.Tools)/4)
	}
	writeJSON(w, map[string]int64{"input_tokens": n})
}

func mockModelExists(id string) bool {
	for _, m := range mockModels {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m, &probe); err == nil && probe.ID == id {
			return true
		}
	}
	return false
}
