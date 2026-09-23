package mock

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Deployment run fixtures follow a payload captured from a live run rather
// than the reference. A run carries no status field and no start or end
// timestamps; it has either a session_id or an error, which is what the
// has_error filter selects on.
var mockDeploymentRuns = []json.RawMessage{
	json.RawMessage(`{
  "type": "deployment_run",
  "id": "drun_01Succeeded0000000000000",
  "deployment_id": "depl_01Example00000000000000",
  "session_id": "sesn_01Example00000000000000",
  "created_at": "2026-09-23T01:58:04.943481Z",
  "error": null,
  "agent": {"id": "agent_01Example0000000000000", "type": "agent", "version": 1},
  "trigger_context": {"type": "schedule", "scheduled_at": "2026-09-23T01:58:00Z"}
}`),
	json.RawMessage(`{
  "type": "deployment_run",
  "id": "drun_01Failed00000000000000000",
  "deployment_id": "depl_01Example00000000000000",
  "session_id": null,
  "created_at": "2026-09-23T01:57:04.000000Z",
  "error": {"type": "environment_error", "message": "environment unavailable"},
  "agent": {"id": "agent_01Example0000000000000", "type": "agent", "version": 1},
  "trigger_context": {"type": "manual"}
}`),
}

func runHasError(m json.RawMessage) bool {
	var probe struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(m, &probe); err != nil {
		return false
	}
	return len(probe.Error) > 0 && string(probe.Error) != "null"
}

func runTriggerType(m json.RawMessage) string {
	var probe struct {
		TriggerContext struct {
			Type string `json:"type"`
		} `json:"trigger_context"`
	}
	_ = json.Unmarshal(m, &probe)
	return probe.TriggerContext.Type
}

// validDeploymentID mirrors the shape the API accepts: the depl_ prefix
// followed by 24 base62 characters. Checked because the endpoint rejects a
// malformed id rather than treating it as a filter that matches nothing.
//
// The length is 24 because that is what live ids measure. A 22-character id
// was rejected by the real endpoint with "Invalid deployment ID", which is how
// the length was pinned rather than assumed.
func validDeploymentID(id string) bool {
	const want = 24
	rest, ok := strings.CutPrefix(id, "depl_")
	if !ok || len(rest) != want {
		return false
	}
	for _, c := range rest {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		default:
			return false
		}
	}
	return true
}

func (s *Server) deploymentRunRoutes() {
	s.handle("GET /v1/deployment_runs", s.listDeploymentRuns)
	s.handle("GET /v1/deployment_runs/{id}", s.getDeploymentRun)
}

func (s *Server) listDeploymentRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	q := r.URL.Query()

	// The real endpoint distinguishes the two cases, and only one of them is
	// an empty list: a well-formed id matching nothing returns 200 with no
	// rows, while a malformed one is a 400. A mock that returned empty for
	// both would let a typo look like "no runs yet".
	if id := q.Get("deployment_id"); id != "" && !validDeploymentID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "Invalid deployment ID.")
		return
	}

	out := make([]json.RawMessage, 0, len(mockDeploymentRuns))
	for _, m := range mockDeploymentRuns {
		// A deployment_id that matches nothing returns an empty list rather
		// than a 404, which is what the real endpoint does.
		if id := q.Get("deployment_id"); id != "" {
			var probe struct {
				DeploymentID string `json:"deployment_id"`
			}
			if err := json.Unmarshal(m, &probe); err != nil || probe.DeploymentID != id {
				continue
			}
		}
		if t := q.Get("trigger_type"); t != "" && runTriggerType(m) != t {
			continue
		}
		if he := q.Get("has_error"); he != "" {
			want, err := strconv.ParseBool(he)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request_error", "has_error must be a boolean")
				return
			}
			if runHasError(m) != want {
				continue
			}
		}
		out = append(out, m)
	}
	tokenList(w, r, out)
}

func (s *Server) getDeploymentRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	id := r.PathValue("id")
	for _, m := range mockDeploymentRuns {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m, &probe); err == nil && probe.ID == id {
			writeJSON(w, m)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found_error", "deployment run not found")
}
