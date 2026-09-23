package mock

import (
	"encoding/json"
	"net/http"
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
