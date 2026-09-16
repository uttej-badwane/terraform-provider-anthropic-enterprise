package mock

import (
	"net/http"
	"slices"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// --- RBAC groups ------------------------------------------------------------

func (s *Server) findGroup(id string) *client.RBACGroup {
	for _, g := range s.store.groups {
		if g.ID == id {
			return g
		}
	}
	return nil
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	out := []*client.RBACGroup{}
	out = append(out, s.store.groups...)
	tokenList(w, r, out)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	var in client.RBACGroupWrite
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 255 {
		writeError(w, 400, "invalid_request_error", "name must be between 1 and 255 characters")
		return
	}
	g := &client.RBACGroup{ID: s.nextID("rbac_group"), CreatedAt: now(), UpdatedAt: now(), Name: in.Name, Roles: []string{}, SourceType: "direct", Type: "rbac_group"}
	s.store.groups = append(s.store.groups, g)
	writeJSON(w, g)
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "rbac_group")
		return
	}
	writeJSON(w, g)
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "rbac_group")
		return
	}
	if g.SourceType == "scim" {
		writeError(w, 400, "invalid_request_error", "SCIM-managed groups cannot be modified via the API")
		return
	}
	var in client.RBACGroupWrite
	if !decode(w, r, &in) {
		return
	}
	if in.Name != "" {
		if len(in.Name) > 255 {
			writeError(w, 400, "invalid_request_error", "name must be at most 255 characters")
			return
		}
		g.Name = in.Name
	}
	g.UpdatedAt = now()
	writeJSON(w, g)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for i, g := range s.store.groups {
		if g.ID == r.PathValue("id") {
			if g.SourceType == "scim" {
				writeError(w, 400, "invalid_request_error", "SCIM-managed groups cannot be deleted via the API")
				return
			}
			s.store.groups = slices.Delete(s.store.groups, i, i+1)
			s.store.groupMembers = slices.DeleteFunc(s.store.groupMembers, func(m *client.RBACGroupMember) bool { return m.GroupID == g.ID })
			deleted(w, g.ID, "rbac_group_deleted")
			return
		}
	}
	notFound(w, "rbac_group")
}

func (s *Server) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "rbac_group")
		return
	}
	out := []*client.RBACGroupMember{}
	for _, m := range s.store.groupMembers {
		if m.GroupID == g.ID {
			out = append(out, m)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "rbac_group")
		return
	}
	if g.SourceType == "scim" {
		writeError(w, 400, "invalid_request_error", "SCIM-managed group membership cannot be modified via the API")
		return
	}
	var in client.RBACGroupMemberAdd
	if !decode(w, r, &in) {
		return
	}
	u := s.findUser(in.UserID)
	if u == nil {
		notFound(w, "user")
		return
	}
	for _, m := range s.store.groupMembers {
		if m.GroupID == g.ID && m.UserID == u.ID {
			writeError(w, 409, "conflict_error", "user is already a member of this group")
			return
		}
	}
	m := &client.RBACGroupMember{CreatedAt: now(), Email: u.Email, GroupID: g.ID, Type: "rbac_group_member", UserID: u.ID}
	s.store.groupMembers = append(s.store.groupMembers, m)
	writeJSON(w, m)
}

func (s *Server) deleteGroupMember(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "rbac_group")
		return
	}
	if g.SourceType == "scim" {
		writeError(w, 400, "invalid_request_error", "SCIM-managed group membership cannot be modified via the API")
		return
	}
	uid := r.PathValue("uid")
	before := len(s.store.groupMembers)
	s.store.groupMembers = slices.DeleteFunc(s.store.groupMembers, func(m *client.RBACGroupMember) bool { return m.GroupID == g.ID && m.UserID == uid })
	if len(s.store.groupMembers) == before {
		notFound(w, "rbac_group_member")
		return
	}
	writeJSON(w, map[string]string{"group_id": g.ID, "type": "rbac_group_member_deleted", "user_id": uid})
}

// --- RBAC roles -------------------------------------------------------------

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	out := []*client.RBACRole{}
	out = append(out, s.store.roles...)
	tokenList(w, r, out)
}

func (s *Server) getRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for _, role := range s.store.roles {
		if role.ID == r.PathValue("id") {
			writeJSON(w, role)
			return
		}
	}
	notFound(w, "rbac_role")
}

func (s *Server) listRolePermissions(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for _, role := range s.store.roles {
		if role.ID == r.PathValue("id") {
			tokenList(w, r, []client.RBACRolePermission{
				{Action: "chat", Resource: map[string]any{"type": "organization", "organization_id": s.OrgID}, Type: "rbac_role_permission"},
				{Action: "use", Resource: map[string]any{"type": "all_connectors"}, Type: "rbac_role_permission"},
			})
			return
		}
	}
	notFound(w, "rbac_role")
}

// --- spend limits -----------------------------------------------------------

var spendPeriods = []string{"daily", "monthly", "weekly"}

func (s *Server) listEffectiveSpendLimits(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	q := r.URL.Query()
	periods := q["period[]"]
	if len(periods) == 0 {
		periods = []string{"monthly"}
	}
	userIDs := q["user_ids[]"]
	out := []client.SpendSummary{}
	for _, u := range s.store.users {
		if len(userIDs) > 0 && !slices.Contains(userIDs, u.ID) {
			continue
		}
		for _, p := range periods {
			row := client.SpendSummary{
				Actor:             client.SpendActor{Deleted: false, EmailAddress: ptr(u.Email), Name: ptr(u.Name), Type: "user_actor", UserID: u.ID},
				Amount:            ptr("10000"),
				Currency:          "USD",
				Period:            p,
				PeriodToDateSpend: "0",
				Scope:             client.SpendScope{Type: "user", UserID: ptr(u.ID)},
				Source:            client.SpendScope{Type: "seat_tier", SeatTier: ptr("enterprise_standard")},
				SpendLimitID:      "spl_seat_tier_default",
			}
			for _, sl := range s.store.spendLimits {
				if sl.Scope.UserID != nil && *sl.Scope.UserID == u.ID && sl.Period == p {
					row.Amount = sl.Amount
					row.Source = sl.Scope
					row.SpendLimitID = sl.ID
				}
			}
			out = append(out, row)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) upsertSpendLimit(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	var in client.SpendLimitCreate
	if !decode(w, r, &in) {
		return
	}
	if in.Scope.Type != "user" || in.Scope.UserID == nil {
		writeError(w, 400, "invalid_request_error", "scope must be {type: user, user_id}")
		return
	}
	if s.findUser(*in.Scope.UserID) == nil {
		notFound(w, "user")
		return
	}
	period := "monthly"
	if in.Period != nil {
		period = *in.Period
	}
	if !slices.Contains(spendPeriods, period) {
		writeError(w, 400, "invalid_request_error", "period must be daily, weekly or monthly")
		return
	}
	if in.Amount != nil {
		for _, ch := range *in.Amount {
			if ch < '0' || ch > '9' {
				writeError(w, 400, "invalid_request_error", "amount must be a non-negative integer string in cents")
				return
			}
		}
	}
	for _, sl := range s.store.spendLimits {
		if *sl.Scope.UserID == *in.Scope.UserID && sl.Period == period {
			sl.Amount = in.Amount
			sl.UpdatedAt = now()
			writeJSON(w, sl)
			return
		}
	}
	sl := &client.SpendLimit{ID: s.nextID("spl"), Amount: in.Amount, CreatedAt: now(), UpdatedAt: now(), Currency: "USD", Period: period,
		Scope: client.SpendScope{Type: "user", UserID: in.Scope.UserID}, Type: "spend_limit"}
	s.store.spendLimits = append(s.store.spendLimits, sl)
	writeJSON(w, sl)
}

func (s *Server) getSpendLimit(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for _, sl := range s.store.spendLimits {
		if sl.ID == r.PathValue("id") {
			writeJSON(w, sl)
			return
		}
	}
	notFound(w, "spend_limit")
}

func (s *Server) deleteSpendLimit(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for i, sl := range s.store.spendLimits {
		if sl.ID == r.PathValue("id") {
			s.store.spendLimits = slices.Delete(s.store.spendLimits, i, i+1)
			deleted(w, sl.ID, "spend_limit_deleted")
			return
		}
	}
	notFound(w, "spend_limit")
}
