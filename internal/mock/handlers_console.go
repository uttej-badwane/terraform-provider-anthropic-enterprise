package mock

import (
	"net/http"
	"slices"
	"strings"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

var (
	writableOrgRoles   = []string{"billing", "claude_code_user", "developer", "managed", "user"}
	protectedOrgRoles  = []string{"admin", "owner", "primary_owner", "membership_admin"}
	workspaceRolesAll  = []string{"workspace_admin", "workspace_billing", "workspace_developer", "workspace_restricted_developer", "workspace_user"}
	workspaceRolesAdd  = []string{"workspace_admin", "workspace_developer", "workspace_restricted_developer", "workspace_user"}
	apiKeyStatusUpdate = []string{"active", "archived", "inactive"}
)

func (s *Server) getOrganization(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	writeJSON(w, client.Organization{ID: s.OrgID, Name: s.OrgName, Type: "organization"})
}

func (s *Server) getCompliance(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	writeJSON(w, s.store.compliance)
}

func (s *Server) updateCompliance(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		State any `json:"state"`
	}
	if !decode(w, r, &in) {
		return
	}
	typ := ""
	switch v := in.State.(type) {
	case string:
		typ = v
	case map[string]any:
		typ, _ = v["type"].(string)
	}
	if typ != "enabled" && typ != "disabled" {
		writeError(w, 400, "invalid_request_error", "state must be enabled or disabled")
		return
	}
	s.store.compliance.State.Type = typ
	writeJSON(w, s.store.compliance)
}

// --- users -----------------------------------------------------------------

func (s *Server) findUser(id string) *client.User {
	for _, u := range s.store.users {
		if u.ID == id {
			return u
		}
	}
	return nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	email := strings.ToLower(r.URL.Query().Get("email"))
	roles := r.URL.Query()["roles"]
	var out []*client.User
	for _, u := range s.store.users {
		if email != "" && strings.ToLower(u.Email) != email {
			continue
		}
		if len(roles) > 0 && !slices.Contains(roles, u.Role) {
			continue
		}
		out = append(out, u)
	}
	cursorList(w, r, out, func(u *client.User) string { return u.ID })
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	u := s.findUser(r.PathValue("id"))
	if u == nil {
		notFound(w, "user")
		return
	}
	writeJSON(w, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	u := s.findUser(r.PathValue("id"))
	if u == nil {
		notFound(w, "user")
		return
	}
	var in client.UserUpdate
	if !decode(w, r, &in) {
		return
	}
	if !slices.Contains(writableOrgRoles, in.Role) {
		writeError(w, 400, "invalid_request_error", "role must be one of "+strings.Join(writableOrgRoles, ", "))
		return
	}
	if slices.Contains(protectedOrgRoles, u.Role) {
		writeError(w, 400, "invalid_request_error", "admins and owners cannot be modified via the API")
		return
	}
	u.Role = in.Role
	writeJSON(w, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	id := r.PathValue("id")
	for i, u := range s.store.users {
		if u.ID == id {
			if slices.Contains(protectedOrgRoles, u.Role) {
				writeError(w, 400, "invalid_request_error", "admins and owners cannot be removed via the API")
				return
			}
			s.store.users = slices.Delete(s.store.users, i, i+1)
			s.store.members = slices.DeleteFunc(s.store.members, func(m *client.WorkspaceMember) bool { return m.UserID == id })
			deleted(w, id, "user_deleted")
			return
		}
	}
	notFound(w, "user")
}

// --- invites ---------------------------------------------------------------

func (s *Server) listInvites(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	q := r.URL.Query()
	email := strings.ToLower(q.Get("email"))
	var out []*client.Invite
	for _, inv := range s.store.invites {
		if inv.Status == "deleted" {
			continue
		}
		if email != "" && strings.ToLower(inv.Email) != email {
			continue
		}
		if roles := q["roles"]; len(roles) > 0 && !slices.Contains(roles, inv.Role) {
			continue
		}
		if st := q["statuses"]; len(st) > 0 && !slices.Contains(st, inv.Status) {
			continue
		}
		out = append(out, inv)
	}
	cursorList(w, r, out, func(i *client.Invite) string { return i.ID })
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	var in client.InviteCreate
	if !decode(w, r, &in) {
		return
	}
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		writeError(w, 400, "invalid_request_error", "email is required")
		return
	}
	if !slices.Contains(writableOrgRoles, in.Role) {
		writeError(w, 400, "invalid_request_error", "role must be one of "+strings.Join(writableOrgRoles, ", "))
		return
	}
	if len(in.RBACGroupIDs) > 0 && s.credential(r) != credEnterprise {
		writeError(w, 400, "invalid_request_error", "rbac_group_ids requires a Claude Enterprise organization")
		return
	}
	for _, gid := range in.RBACGroupIDs {
		if s.findGroup(gid) == nil {
			notFound(w, "rbac_group")
			return
		}
	}
	for _, inv := range s.store.invites {
		if inv.Status == "pending" && strings.EqualFold(inv.Email, in.Email) {
			writeError(w, 409, "conflict_error", "a pending invite for this email already exists")
			return
		}
	}
	for _, u := range s.store.users {
		if strings.EqualFold(u.Email, in.Email) {
			writeError(w, 409, "conflict_error", "user is already a member")
			return
		}
	}
	ids := in.RBACGroupIDs
	if ids == nil {
		ids = []string{}
	}
	inv := &client.Invite{ID: s.nextID("invite"), Email: in.Email, ExpiresAt: now(), InvitedAt: now(),
		RBACGroupIDs: ids, Role: in.Role, Status: "pending", Type: "invite"}
	s.store.invites = append(s.store.invites, inv)
	writeJSON(w, inv)
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	for _, inv := range s.store.invites {
		if inv.ID == r.PathValue("id") && inv.Status != "deleted" {
			writeJSON(w, inv)
			return
		}
	}
	notFound(w, "invite")
}

func (s *Server) deleteInvite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminOrEnterprise(w, r) {
		return
	}
	for _, inv := range s.store.invites {
		if inv.ID == r.PathValue("id") && inv.Status != "deleted" {
			if inv.Status != "pending" {
				writeError(w, 400, "invalid_request_error", "only pending invites can be deleted")
				return
			}
			inv.Status = "deleted"
			deleted(w, inv.ID, "invite_deleted")
			return
		}
	}
	notFound(w, "invite")
}

// --- workspaces ------------------------------------------------------------

func (s *Server) findWorkspace(id string) *client.Workspace {
	for _, ws := range s.store.workspaces {
		if ws.ID == id {
			return ws
		}
	}
	return nil
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	var out []*client.Workspace
	for _, ws := range s.store.workspaces {
		if ws.ArchivedAt != nil && !inc {
			continue
		}
		out = append(out, ws)
	}
	cursorList(w, r, out, func(x *client.Workspace) string { return x.ID })
}

func validTags(tags map[string]string) bool {
	for k := range tags {
		if strings.HasPrefix(strings.ToLower(k), "anthropic") || k == "" {
			return false
		}
	}
	return true
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in client.WorkspaceCreate
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 40 {
		writeError(w, 400, "invalid_request_error", "name must be between 1 and 40 characters")
		return
	}
	if !validTags(in.Tags) {
		writeError(w, 400, "invalid_request_error", "tag keys may not start with anthropic")
		return
	}
	live := 0
	for _, ws := range s.store.workspaces {
		if ws.ArchivedAt == nil {
			live++
		}
	}
	if live >= 100 {
		writeError(w, 400, "invalid_request_error", "workspace limit reached")
		return
	}
	ws := &client.Workspace{ID: s.nextID("wrkspc"), CompartmentID: "00000000-0000-4000-8000-" + s.nextID("")[1:13], CreatedAt: now(),
		DisplayColor: "#6C5BB9", Name: in.Name, Tags: map[string]string{}, Type: "workspace",
		DataResidency: &client.DataResidency{AllowedInferenceGeos: client.AllowedGeos{Unrestricted: true}, DefaultInferenceGeo: "global", WorkspaceGeo: "us"}}
	if in.DisplayColor != nil {
		ws.DisplayColor = *in.DisplayColor
	}
	if in.Tags != nil {
		ws.Tags = in.Tags
	}
	if in.ExternalKeyID != nil {
		ek := s.findExternalKey(*in.ExternalKeyID)
		if ek == nil {
			notFound(w, "external_key")
			return
		}
		ek.Attachment.Type = "attached"
		ws.ExternalKeyID = in.ExternalKeyID
	}
	if in.DataResidency != nil {
		applyResidency(ws.DataResidency, in.DataResidency, true)
	}
	s.store.workspaces = append(s.store.workspaces, ws)
	writeJSON(w, ws)
}

func applyResidency(dst *client.DataResidency, in *client.DataResidencyParams, create bool) {
	if in.AllowedInferenceGeos != nil {
		dst.AllowedInferenceGeos = *in.AllowedInferenceGeos
	}
	if in.DefaultInferenceGeo != nil {
		dst.DefaultInferenceGeo = *in.DefaultInferenceGeo
	}
	if create && in.WorkspaceGeo != nil {
		dst.WorkspaceGeo = *in.WorkspaceGeo
	}
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ws := s.findWorkspace(r.PathValue("id"))
	if ws == nil {
		notFound(w, "workspace")
		return
	}
	writeJSON(w, ws)
}

func (s *Server) updateWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ws := s.findWorkspace(r.PathValue("id"))
	if ws == nil {
		notFound(w, "workspace")
		return
	}
	if ws.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "workspace is archived")
		return
	}
	var in client.WorkspaceUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		if l := len(*in.Name); l < 1 || l > 40 {
			writeError(w, 400, "invalid_request_error", "name must be between 1 and 40 characters")
			return
		}
		ws.Name = *in.Name
	}
	if in.DisplayColor != nil {
		ws.DisplayColor = *in.DisplayColor
	}
	if in.ExternalKeyID != nil {
		if ws.ExternalKeyID != nil && *ws.ExternalKeyID != *in.ExternalKeyID {
			writeError(w, 400, "invalid_request_error", "external_key_id is write-once")
			return
		}
		ek := s.findExternalKey(*in.ExternalKeyID)
		if ek == nil {
			notFound(w, "external_key")
			return
		}
		ek.Attachment.Type = "attached"
		ws.ExternalKeyID = in.ExternalKeyID
	}
	if in.Tags != nil {
		for k, v := range in.Tags {
			if strings.HasPrefix(strings.ToLower(k), "anthropic") {
				writeError(w, 400, "invalid_request_error", "tag keys may not start with anthropic")
				return
			}
			if v == nil {
				delete(ws.Tags, k)
			} else {
				ws.Tags[k] = *v
			}
		}
	}
	if in.DataResidency != nil {
		applyResidency(ws.DataResidency, in.DataResidency, false)
	}
	writeJSON(w, ws)
}

func (s *Server) archiveWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ws := s.findWorkspace(r.PathValue("id"))
	if ws == nil {
		notFound(w, "workspace")
		return
	}
	if ws.ArchivedAt == nil {
		ws.ArchivedAt = ptr(now())
		for _, k := range s.store.apiKeys {
			if k.Scope.WorkspaceID != nil && *k.Scope.WorkspaceID == ws.ID {
				k.Status = "archived"
			}
		}
	}
	writeJSON(w, ws)
}

func (s *Server) listWorkspaceRateLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if s.findWorkspace(id) == nil && id != s.store.defaultWorkspace {
		notFound(w, "workspace")
		return
	}
	gt := r.URL.Query().Get("group_type")
	var out []client.WorkspaceRateLimit
	for _, rl := range s.store.wsRateLimits[id] {
		if gt == "" || rl.GroupType == gt {
			out = append(out, rl)
		}
	}
	if out == nil {
		out = []client.WorkspaceRateLimit{}
	}
	tokenList(w, r, out)
}

// --- workspace members -----------------------------------------------------

func (s *Server) liveWorkspace(w http.ResponseWriter, id string) *client.Workspace {
	ws := s.findWorkspace(id)
	if ws == nil {
		notFound(w, "workspace")
		return nil
	}
	if ws.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "workspace is archived")
		return nil
	}
	return ws
}

func (s *Server) findMember(wsID, userID string) (int, *client.WorkspaceMember) {
	for i, m := range s.store.members {
		if m.WorkspaceID == wsID && m.UserID == userID {
			return i, m
		}
	}
	return -1, nil
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.findWorkspace(r.PathValue("id")) == nil {
		notFound(w, "workspace")
		return
	}
	var out []*client.WorkspaceMember
	for _, m := range s.store.members {
		if m.WorkspaceID == r.PathValue("id") {
			out = append(out, m)
		}
	}
	cursorList(w, r, out, func(m *client.WorkspaceMember) string { return m.UserID })
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ws := s.liveWorkspace(w, r.PathValue("id"))
	if ws == nil {
		return
	}
	var in client.WorkspaceMemberAdd
	if !decode(w, r, &in) {
		return
	}
	if s.findUser(in.UserID) == nil {
		notFound(w, "user")
		return
	}
	if !slices.Contains(workspaceRolesAdd, in.WorkspaceRole) {
		writeError(w, 400, "invalid_request_error", "workspace_role must be one of "+strings.Join(workspaceRolesAdd, ", "))
		return
	}
	if _, m := s.findMember(ws.ID, in.UserID); m != nil {
		writeError(w, 409, "conflict_error", "user is already a member of this workspace")
		return
	}
	m := &client.WorkspaceMember{Type: "workspace_member", UserID: in.UserID, WorkspaceID: ws.ID, WorkspaceRole: in.WorkspaceRole}
	s.store.members = append(s.store.members, m)
	writeJSON(w, m)
}

func (s *Server) getMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	_, m := s.findMember(r.PathValue("id"), r.PathValue("uid"))
	if m == nil {
		notFound(w, "workspace_member")
		return
	}
	writeJSON(w, m)
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	_, m := s.findMember(r.PathValue("id"), r.PathValue("uid"))
	if m == nil {
		notFound(w, "workspace_member")
		return
	}
	var in client.WorkspaceRoleUpdate
	if !decode(w, r, &in) {
		return
	}
	if !slices.Contains(workspaceRolesAll, in.WorkspaceRole) {
		writeError(w, 400, "invalid_request_error", "invalid workspace_role")
		return
	}
	m.WorkspaceRole = in.WorkspaceRole
	writeJSON(w, m)
}

func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	i, m := s.findMember(r.PathValue("id"), r.PathValue("uid"))
	if m == nil {
		notFound(w, "workspace_member")
		return
	}
	s.store.members = slices.Delete(s.store.members, i, i+1)
	writeJSON(w, map[string]string{"type": "workspace_member_deleted", "user_id": m.UserID, "workspace_id": m.WorkspaceID})
}

// --- workspace service accounts --------------------------------------------

func (s *Server) findSAMember(wsID, saID string) (int, *client.ServiceAccountWorkspaceMember) {
	for i, m := range s.store.saMembers {
		if m.WorkspaceID == wsID && m.ServiceAccountID == saID {
			return i, m
		}
	}
	return -1, nil
}

func (s *Server) listWorkspaceSAs(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	if s.findWorkspace(r.PathValue("id")) == nil {
		notFound(w, "workspace")
		return
	}
	out := []*client.ServiceAccountWorkspaceMember{}
	for _, m := range s.store.saMembers {
		if m.WorkspaceID == r.PathValue("id") {
			out = append(out, m)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) addWorkspaceSA(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	ws := s.liveWorkspace(w, r.PathValue("id"))
	if ws == nil {
		return
	}
	var in client.ServiceAccountWorkspaceMemberAdd
	if !decode(w, r, &in) {
		return
	}
	sa := s.findServiceAccount(in.ServiceAccountID)
	if sa == nil || sa.ArchivedAt != nil {
		notFound(w, "service_account")
		return
	}
	if !slices.Contains(workspaceRolesAdd, in.WorkspaceRole) {
		writeError(w, 400, "invalid_request_error", "workspace_role must be one of "+strings.Join(workspaceRolesAdd, ", "))
		return
	}
	if _, m := s.findSAMember(ws.ID, sa.ID); m != nil {
		writeError(w, 409, "conflict_error", "service account is already a member of this workspace")
		return
	}
	m := &client.ServiceAccountWorkspaceMember{CreatedByActorID: ptr(s.store.users[1].ID), Implicit: ptr(false),
		ServiceAccountID: sa.ID, Type: "service_account_workspace_member", WorkspaceID: ws.ID, WorkspaceRole: in.WorkspaceRole}
	s.store.saMembers = append(s.store.saMembers, m)
	writeJSON(w, m)
}

func (s *Server) getWorkspaceSA(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	_, m := s.findSAMember(r.PathValue("id"), r.PathValue("sid"))
	if m == nil {
		notFound(w, "service_account_workspace_member")
		return
	}
	writeJSON(w, m)
}

func (s *Server) updateWorkspaceSA(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	_, m := s.findSAMember(r.PathValue("id"), r.PathValue("sid"))
	if m == nil {
		notFound(w, "service_account_workspace_member")
		return
	}
	var in client.WorkspaceRoleUpdate
	if !decode(w, r, &in) {
		return
	}
	if !slices.Contains(workspaceRolesAdd, in.WorkspaceRole) {
		writeError(w, 400, "invalid_request_error", "invalid workspace_role")
		return
	}
	m.WorkspaceRole = in.WorkspaceRole
	writeJSON(w, m)
}

func (s *Server) deleteWorkspaceSA(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	i, m := s.findSAMember(r.PathValue("id"), r.PathValue("sid"))
	if m != nil {
		s.store.saMembers = slices.Delete(s.store.saMembers, i, i+1)
	}
	writeJSON(w, map[string]string{"type": "service_account_workspace_member_deleted", "service_account_id": r.PathValue("sid"), "workspace_id": r.PathValue("id")})
}

// --- api keys --------------------------------------------------------------

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	var out []*client.APIKey
	for _, k := range s.store.apiKeys {
		if st := q.Get("status"); st != "" && k.Status != st {
			continue
		}
		if ws := q.Get("workspace_id"); ws != "" && (k.Scope.WorkspaceID == nil || *k.Scope.WorkspaceID != ws) {
			continue
		}
		if cb := q.Get("created_by_user_id"); cb != "" && (k.CreatedBy == nil || k.CreatedBy.ID != cb) {
			continue
		}
		out = append(out, k)
	}
	cursorList(w, r, out, func(k *client.APIKey) string { return k.ID })
}

func (s *Server) findAPIKey(id string) *client.APIKey {
	for _, k := range s.store.apiKeys {
		if k.ID == id {
			return k
		}
	}
	return nil
}

func (s *Server) getAPIKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	k := s.findAPIKey(r.PathValue("id"))
	if k == nil {
		notFound(w, "api_key")
		return
	}
	writeJSON(w, k)
}

func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	k := s.findAPIKey(r.PathValue("id"))
	if k == nil {
		notFound(w, "api_key")
		return
	}
	var in client.APIKeyUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Status != nil {
		if !slices.Contains(apiKeyStatusUpdate, *in.Status) {
			writeError(w, 400, "invalid_request_error", "status must be active, archived or inactive")
			return
		}
		k.Status = *in.Status
	}
	if in.Name != nil {
		k.Name = *in.Name
	}
	writeJSON(w, k)
}

// --- rate limits -----------------------------------------------------------

func (s *Server) listRateLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	out := []client.RateLimit{}
	for _, rl := range s.store.rateLimits {
		if gt := q.Get("group_type"); gt != "" && rl.GroupType != gt {
			continue
		}
		if m := q.Get("model"); m != "" && !slices.Contains(rl.Models, m) {
			continue
		}
		out = append(out, rl)
	}
	if m := q.Get("model"); m != "" && len(out) == 0 {
		notFound(w, "model")
		return
	}
	tokenList(w, r, out)
}
