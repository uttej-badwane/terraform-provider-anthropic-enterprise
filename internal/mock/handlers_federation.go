package mock

import (
	"net/http"
	"slices"
	"strings"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// --- service accounts -------------------------------------------------------

func (s *Server) findServiceAccount(id string) *client.ServiceAccount {
	for _, sa := range s.store.serviceAccounts {
		if sa.ID == id {
			return sa
		}
	}
	return nil
}

func (s *Server) listServiceAccounts(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.ServiceAccount{}
	for _, sa := range s.store.serviceAccounts {
		if sa.ArchivedAt != nil && !inc {
			continue
		}
		out = append(out, sa)
	}
	tokenList(w, r, out)
}

func (s *Server) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	var in client.ServiceAccountCreate
	if !decode(w, r, &in) {
		return
	}
	if !slugRe.MatchString(in.Name) {
		writeError(w, 400, "invalid_request_error", "name must contain only lowercase letters, digits and hyphens")
		return
	}
	for _, sa := range s.store.serviceAccounts {
		if sa.Name == in.Name && sa.ArchivedAt == nil {
			writeError(w, 409, "conflict_error", "a service account with this name already exists")
			return
		}
	}
	role := "developer"
	if in.OrganizationRole != nil {
		role = *in.OrganizationRole
	}
	if role != "developer" && role != "admin" {
		writeError(w, 400, "invalid_request_error", "organization_role must be developer or admin")
		return
	}
	actor := s.store.users[1].ID
	sa := &client.ServiceAccount{ID: s.nextID("svac"), CreatedAt: now(), UpdatedAt: now(), CreatedByActorID: &actor, UpdatedByActorID: &actor,
		Description: in.Description, Name: in.Name, OrganizationRole: role, Type: "service_account"}
	s.store.serviceAccounts = append(s.store.serviceAccounts, sa)
	// implicit default-workspace membership
	s.store.saMembers = append(s.store.saMembers, &client.ServiceAccountWorkspaceMember{Implicit: ptr(true), ServiceAccountID: sa.ID,
		Type: "service_account_workspace_member", WorkspaceID: s.store.defaultWorkspace, WorkspaceRole: "workspace_user"})
	writeJSON(w, sa)
}

func (s *Server) getServiceAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	sa := s.findServiceAccount(r.PathValue("id"))
	if sa == nil {
		notFound(w, "service_account")
		return
	}
	writeJSON(w, sa)
}

func (s *Server) updateServiceAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	sa := s.findServiceAccount(r.PathValue("id"))
	if sa == nil {
		notFound(w, "service_account")
		return
	}
	if sa.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "service account is archived")
		return
	}
	var in client.ServiceAccountUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Description.Set {
		if in.Description.Null {
			sa.Description = ptr("")
		} else {
			sa.Description = ptr(in.Description.Value)
		}
	}
	if in.OrganizationRole != nil {
		if *in.OrganizationRole != "developer" && *in.OrganizationRole != "admin" {
			writeError(w, 400, "invalid_request_error", "organization_role must be developer or admin")
			return
		}
		sa.OrganizationRole = *in.OrganizationRole
	}
	sa.UpdatedAt = now()
	writeJSON(w, sa)
}

func (s *Server) archiveServiceAccount(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	sa := s.findServiceAccount(r.PathValue("id"))
	if sa == nil {
		notFound(w, "service_account")
		return
	}
	for _, rule := range s.store.rules {
		if rule.ArchivedAt == nil && rule.Target.ServiceAccountID == sa.ID {
			writeError(w, 400, "invalid_request_error", "service account is the target of a live federation rule")
			return
		}
	}
	if sa.ArchivedAt == nil {
		sa.ArchivedAt = ptr(now())
		sa.ArchivedByActor = ptr(s.store.users[1].ID)
		s.store.saMembers = slices.DeleteFunc(s.store.saMembers, func(m *client.ServiceAccountWorkspaceMember) bool { return m.ServiceAccountID == sa.ID })
	}
	writeJSON(w, sa)
}

// --- federation issuers -----------------------------------------------------

func (s *Server) findIssuer(id string) *client.FederationIssuer {
	for _, is := range s.store.issuers {
		if is.ID == id {
			return is
		}
	}
	return nil
}

func (s *Server) listIssuers(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.FederationIssuer{}
	for _, is := range s.store.issuers {
		if is.ArchivedAt != nil && !inc {
			continue
		}
		out = append(out, is)
	}
	tokenList(w, r, out)
}

func validJWKS(w http.ResponseWriter, j *client.JWKS) bool {
	switch j.Type {
	case "discovery":
		return true
	case "explicit_url":
		if j.URL == nil || *j.URL == "" {
			writeError(w, 400, "invalid_request_error", "jwks.url is required for explicit_url")
			return false
		}
		return true
	case "inline":
		if len(j.Keys) == 0 {
			writeError(w, 400, "invalid_request_error", "jwks.keys is required for inline")
			return false
		}
		return true
	}
	writeError(w, 400, "invalid_request_error", "jwks.type must be discovery, explicit_url or inline")
	return false
}

func (s *Server) createIssuer(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	var in client.FederationIssuerCreate
	if !decode(w, r, &in) {
		return
	}
	if !slugRe.MatchString(in.Name) {
		writeError(w, 400, "invalid_request_error", "name must contain only lowercase letters, digits and hyphens")
		return
	}
	if !strings.HasPrefix(in.IssuerURL, "https://") {
		writeError(w, 400, "invalid_request_error", "issuer_url must be https")
		return
	}
	for _, is := range s.store.issuers {
		if is.Name == in.Name && is.ArchivedAt == nil {
			writeError(w, 409, "conflict_error", "a federation issuer with this name already exists")
			return
		}
	}
	jwks := client.JWKS{Type: "discovery"}
	if in.JWKS != nil {
		if !validJWKS(w, in.JWKS) {
			return
		}
		jwks = *in.JWKS
	}
	is := &client.FederationIssuer{ID: s.nextID("fdis"), CheckJTI: true, CreatedAt: now(), UpdatedAt: now(), CreatedByActorID: ptr(s.store.users[1].ID),
		IssuerURL: in.IssuerURL, JWKS: jwks, MaxJWTLifetimeSeconds: 3600, Name: in.Name, Type: "federation_issuer",
		PollStatus: &client.PollStatus{ConsecutiveFailures: 0, LastFetchedAt: ptr(now()), NextPollAt: ptr(now())}}
	if in.CheckJTI != nil {
		is.CheckJTI = *in.CheckJTI
	}
	if in.MaxJWTLifetimeSeconds != nil {
		if *in.MaxJWTLifetimeSeconds < 1 || *in.MaxJWTLifetimeSeconds > 176400 {
			writeError(w, 400, "invalid_request_error", "max_jwt_lifetime_seconds must be between 1 and 176400")
			return
		}
		is.MaxJWTLifetimeSeconds = *in.MaxJWTLifetimeSeconds
	}
	s.store.issuers = append(s.store.issuers, is)
	writeJSON(w, is)
}

func (s *Server) getIssuer(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	is := s.findIssuer(r.PathValue("id"))
	if is == nil {
		notFound(w, "federation_issuer")
		return
	}
	writeJSON(w, is)
}

func (s *Server) updateIssuer(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	is := s.findIssuer(r.PathValue("id"))
	if is == nil {
		notFound(w, "federation_issuer")
		return
	}
	if is.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "federation issuer is archived")
		return
	}
	var in client.FederationIssuerUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		if !slugRe.MatchString(*in.Name) {
			writeError(w, 400, "invalid_request_error", "invalid name")
			return
		}
		is.Name = *in.Name
	}
	if in.IssuerURL != nil {
		is.IssuerURL = *in.IssuerURL
	}
	if in.CheckJTI != nil {
		is.CheckJTI = *in.CheckJTI
	}
	if in.JWKS != nil {
		if !validJWKS(w, in.JWKS) {
			return
		}
		is.JWKS = *in.JWKS
	}
	if in.MaxJWTLifetimeSeconds != nil {
		is.MaxJWTLifetimeSeconds = *in.MaxJWTLifetimeSeconds
	}
	if in.JWKSPollingDisabled != nil {
		if *in.JWKSPollingDisabled {
			writeError(w, 400, "invalid_request_error", "jwks_polling_disabled may only be set to false")
			return
		}
		is.JWKSPollingDisabledAt = nil
	}
	is.UpdatedAt = now()
	writeJSON(w, is)
}

func (s *Server) archiveIssuer(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	is := s.findIssuer(r.PathValue("id"))
	if is == nil {
		notFound(w, "federation_issuer")
		return
	}
	for _, rule := range s.store.rules {
		if rule.ArchivedAt == nil && rule.IssuerID == is.ID {
			writeError(w, 400, "invalid_request_error", "federation issuer is referenced by a live federation rule")
			return
		}
	}
	if is.ArchivedAt == nil {
		is.ArchivedAt = ptr(now())
	}
	writeJSON(w, is)
}

// --- federation rules -------------------------------------------------------

var ruleScopes = []string{"workspace:developer", "workspace:inference"}

func (s *Server) findRule(id string) *client.FederationRule {
	for _, rl := range s.store.rules {
		if rl.ID == id {
			return rl
		}
	}
	return nil
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	q := r.URL.Query()
	inc := q.Get("include_archived") == "true"
	out := []*client.FederationRule{}
	for _, rl := range s.store.rules {
		if rl.ArchivedAt != nil && !inc {
			continue
		}
		if iss := q.Get("issuer_id"); iss != "" && rl.IssuerID != iss {
			continue
		}
		out = append(out, rl)
	}
	tokenList(w, r, out)
}

func validMatch(w http.ResponseWriter, m client.RuleMatch) bool {
	hasPrefix := m.SubjectPrefix != nil && strings.Trim(*m.SubjectPrefix, "*") != ""
	if !hasPrefix && len(m.Claims) == 0 && (m.Condition == nil || *m.Condition == "") {
		writeError(w, 400, "invalid_request_error", "match requires at least one of subject_prefix, claims or condition")
		return false
	}
	return true
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	var in client.FederationRuleCreate
	if !decode(w, r, &in) {
		return
	}
	if !slugRe.MatchString(in.Name) {
		writeError(w, 400, "invalid_request_error", "name must contain only lowercase letters, digits and hyphens")
		return
	}
	for _, rl := range s.store.rules {
		if rl.Name == in.Name && rl.ArchivedAt == nil {
			writeError(w, 409, "conflict_error", "a federation rule with this name already exists")
			return
		}
	}
	is := s.findIssuer(in.IssuerID)
	if is == nil || is.ArchivedAt != nil {
		notFound(w, "federation_issuer")
		return
	}
	if in.Target.Type != "service_account" {
		writeError(w, 400, "invalid_request_error", "target.type must be service_account")
		return
	}
	sa := s.findServiceAccount(in.Target.ServiceAccountID)
	if sa == nil || sa.ArchivedAt != nil {
		notFound(w, "service_account")
		return
	}
	if !slices.Contains(ruleScopes, in.OAuthScope) {
		writeError(w, 400, "invalid_request_error", "oauth_scope must be workspace:developer or workspace:inference")
		return
	}
	if !validMatch(w, in.Match) {
		return
	}
	if len(in.Attributes) > 0 {
		writeError(w, 400, "invalid_request_error", "attributes are not yet supported")
		return
	}
	all := in.AppliesToAllWorkspaces != nil && *in.AppliesToAllWorkspaces
	if !all && (in.WorkspaceID == nil || *in.WorkspaceID == "") {
		writeError(w, 400, "invalid_request_error", "workspace_id is required unless applies_to_all_workspaces is true")
		return
	}
	lifetime := int64(3600)
	if in.TokenLifetimeSeconds != nil {
		if *in.TokenLifetimeSeconds < 60 || *in.TokenLifetimeSeconds > 86400 {
			writeError(w, 400, "invalid_request_error", "token_lifetime_seconds must be between 60 and 86400")
			return
		}
		lifetime = *in.TokenLifetimeSeconds
	}
	rl := &client.FederationRule{ID: s.nextID("fdrl"), AppliesToAllWorkspaces: all, CreatedAt: now(), UpdatedAt: now(),
		CreatedByActorID: ptr(s.store.users[1].ID), Description: in.Description, IssuerID: is.ID, IssuerName: &is.Name,
		Match: in.Match, Name: in.Name, OAuthScope: in.OAuthScope,
		Target:               client.RuleTarget{Type: "service_account", ServiceAccountID: sa.ID, ServiceAccountName: &sa.Name},
		TokenLifetimeSeconds: lifetime, Type: "federation_rule", WorkspaceIDs: []string{}}
	if !all {
		if s.findWorkspace(*in.WorkspaceID) == nil {
			notFound(w, "workspace")
			return
		}
		rl.WorkspaceID = in.WorkspaceID
		rl.WorkspaceIDs = []string{*in.WorkspaceID}
		s.store.ruleWorkspaces = append(s.store.ruleWorkspaces, &client.FederationRuleWorkspace{CreatedAt: now(), FederationRuleID: rl.ID,
			Type: "federation_rule_workspace", WorkspaceID: *in.WorkspaceID, WorkspaceName: &s.findWorkspace(*in.WorkspaceID).Name})
	}
	s.store.rules = append(s.store.rules, rl)
	writeJSON(w, withoutReadTimeNames(rl))
}

// withoutReadTimeNames mirrors the live API, whose create and update
// responses omit issuer_name and target.service_account_name.
func withoutReadTimeNames(rl *client.FederationRule) client.FederationRule {
	cp := *rl
	cp.IssuerName = nil
	cp.Target = client.RuleTarget{Type: rl.Target.Type, ServiceAccountID: rl.Target.ServiceAccountID}
	return cp
}

func (s *Server) getRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	writeJSON(w, rl)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	if rl.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "federation rule is archived")
		return
	}
	var in client.FederationRuleUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		if !slugRe.MatchString(*in.Name) {
			writeError(w, 400, "invalid_request_error", "invalid name")
			return
		}
		rl.Name = *in.Name
	}
	if in.Description.Set {
		if in.Description.Null {
			rl.Description = nil
		} else {
			rl.Description = ptr(in.Description.Value)
		}
	}
	if in.Match != nil {
		if !validMatch(w, *in.Match) {
			return
		}
		rl.Match = *in.Match
	}
	if in.OAuthScope != nil {
		if !slices.Contains(ruleScopes, *in.OAuthScope) {
			writeError(w, 400, "invalid_request_error", "invalid oauth_scope")
			return
		}
		rl.OAuthScope = *in.OAuthScope
	}
	if in.Target != nil {
		sa := s.findServiceAccount(in.Target.ServiceAccountID)
		if sa == nil || sa.ArchivedAt != nil {
			notFound(w, "service_account")
			return
		}
		rl.Target = client.RuleTarget{Type: "service_account", ServiceAccountID: sa.ID, ServiceAccountName: &sa.Name}
	}
	if in.TokenLifetimeSeconds != nil {
		rl.TokenLifetimeSeconds = *in.TokenLifetimeSeconds
	}
	if in.AppliesToAllWorkspaces != nil {
		rl.AppliesToAllWorkspaces = *in.AppliesToAllWorkspaces
	}
	if in.WorkspaceID != nil {
		if len(rl.WorkspaceIDs) > 1 {
			writeError(w, 400, "invalid_request_error", "workspace_id cannot be used when more than one workspace is enabled")
			return
		}
		if s.findWorkspace(*in.WorkspaceID) == nil {
			notFound(w, "workspace")
			return
		}
		s.store.ruleWorkspaces = slices.DeleteFunc(s.store.ruleWorkspaces, func(x *client.FederationRuleWorkspace) bool { return x.FederationRuleID == rl.ID })
		s.store.ruleWorkspaces = append(s.store.ruleWorkspaces, &client.FederationRuleWorkspace{CreatedAt: now(), FederationRuleID: rl.ID,
			Type: "federation_rule_workspace", WorkspaceID: *in.WorkspaceID, WorkspaceName: &s.findWorkspace(*in.WorkspaceID).Name})
		rl.WorkspaceID = in.WorkspaceID
		rl.WorkspaceIDs = []string{*in.WorkspaceID}
	}
	rl.UpdatedAt = now()
	writeJSON(w, withoutReadTimeNames(rl))
}

func (s *Server) archiveRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	if rl.ArchivedAt == nil {
		rl.ArchivedAt = ptr(now())
	}
	writeJSON(w, rl)
}

func (s *Server) listRuleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	out := []*client.FederationRuleWorkspace{}
	for _, x := range s.store.ruleWorkspaces {
		if x.FederationRuleID == rl.ID {
			out = append(out, x)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) addRuleWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	var in struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	ws := s.findWorkspace(in.WorkspaceID)
	if ws == nil {
		notFound(w, "workspace")
		return
	}
	for _, x := range s.store.ruleWorkspaces {
		if x.FederationRuleID == rl.ID && x.WorkspaceID == ws.ID {
			writeJSON(w, &client.FederationRuleWorkspace{CreatedAt: x.CreatedAt, FederationRuleID: rl.ID, Type: "federation_rule_workspace", WorkspaceID: ws.ID})
			return
		}
	}
	x := &client.FederationRuleWorkspace{CreatedAt: now(), CreatedByActorID: ptr(s.store.users[1].ID), FederationRuleID: rl.ID,
		Type: "federation_rule_workspace", WorkspaceID: ws.ID, WorkspaceName: &ws.Name}
	s.store.ruleWorkspaces = append(s.store.ruleWorkspaces, x)
	rl.WorkspaceIDs = append(rl.WorkspaceIDs, ws.ID)
	if len(rl.WorkspaceIDs) > 1 {
		rl.WorkspaceID = nil
	}
	// add response has workspace_name null
	writeJSON(w, &client.FederationRuleWorkspace{CreatedAt: x.CreatedAt, CreatedByActorID: x.CreatedByActorID, FederationRuleID: rl.ID, Type: x.Type, WorkspaceID: ws.ID})
}

func (s *Server) deleteRuleWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuth(w, r) {
		return
	}
	rl := s.findRule(r.PathValue("id"))
	if rl == nil {
		notFound(w, "federation_rule")
		return
	}
	wid := r.PathValue("wid")
	s.store.ruleWorkspaces = slices.DeleteFunc(s.store.ruleWorkspaces, func(x *client.FederationRuleWorkspace) bool {
		return x.FederationRuleID == rl.ID && x.WorkspaceID == wid
	})
	rl.WorkspaceIDs = slices.DeleteFunc(rl.WorkspaceIDs, func(id string) bool { return id == wid })
	if rl.WorkspaceID != nil && *rl.WorkspaceID == wid {
		rl.WorkspaceID = nil
	}
	writeJSON(w, map[string]string{"federation_rule_id": rl.ID, "type": "federation_rule_workspace_deleted", "workspace_id": wid})
}

// --- external keys ----------------------------------------------------------

func (s *Server) findExternalKey(id string) *client.ExternalKey {
	for _, k := range s.store.externalKeys {
		if k.ID == id {
			return k
		}
	}
	return nil
}

func (s *Server) listExternalKeys(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	out := []*client.ExternalKey{}
	out = append(out, s.store.externalKeys...)
	tokenList(w, r, out)
}

func validProviderConfig(w http.ResponseWriter, pc client.ProviderConfig) bool {
	empty := func(p *string) bool { return p == nil || *p == "" }
	switch pc.Type {
	case "aws":
		if empty(pc.KMSARN) || !strings.HasPrefix(*pc.KMSARN, "arn:aws:kms:") {
			writeError(w, 400, "invalid_request_error", "provider_config.kms_arn is required for aws")
			return false
		}
	case "gcp":
		if empty(pc.KeyName) {
			writeError(w, 400, "invalid_request_error", "provider_config.key_name is required for gcp")
			return false
		}
	case "azure":
		if empty(pc.KeyName) || empty(pc.TenantID) || empty(pc.VaultURI) {
			writeError(w, 400, "invalid_request_error", "provider_config.key_name, tenant_id and vault_uri are required for azure")
			return false
		}
	default:
		writeError(w, 400, "invalid_request_error", "provider_config.type must be aws, gcp or azure")
		return false
	}
	return true
}

func (s *Server) createExternalKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in client.ExternalKeyCreate
	if !decode(w, r, &in) {
		return
	}
	if !validProviderConfig(w, in.ProviderConfig) {
		return
	}
	geo := "us"
	if in.Geo != nil {
		if *in.Geo != "us" {
			writeError(w, 400, "invalid_request_error", "geo must be us")
			return
		}
		geo = *in.Geo
	}
	pc := in.ProviderConfig
	if pc.Type == "aws" && pc.Region == nil {
		parts := strings.Split(*pc.KMSARN, ":")
		if len(parts) > 3 {
			pc.Region = ptr(parts[3])
		}
	}
	k := &client.ExternalKey{ID: s.nextID("ekey"), Attachment: client.Attachment{Type: "unattached"}, CreatedAt: now(), UpdatedAt: now(),
		DisplayName: in.DisplayName, Geo: geo, ProviderConfig: pc, Type: "external_key"}
	s.store.externalKeys = append(s.store.externalKeys, k)
	writeJSON(w, k)
}

func (s *Server) getExternalKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	k := s.findExternalKey(r.PathValue("id"))
	if k == nil {
		notFound(w, "external_key")
		return
	}
	writeJSON(w, k)
}

func (s *Server) updateExternalKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	k := s.findExternalKey(r.PathValue("id"))
	if k == nil {
		notFound(w, "external_key")
		return
	}
	var in client.ExternalKeyUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.DisplayName.Set {
		if in.DisplayName.Null {
			k.DisplayName = nil
		} else {
			k.DisplayName = ptr(in.DisplayName.Value)
		}
	}
	if in.Geo != nil && *in.Geo != "us" {
		writeError(w, 400, "invalid_request_error", "geo must be us")
		return
	}
	if in.ProviderConfig != nil {
		if k.Attachment.Type == "attached" {
			writeError(w, 400, "invalid_request_error", "provider_config cannot be changed while the key is attached")
			return
		}
		if !validProviderConfig(w, *in.ProviderConfig) {
			return
		}
		k.ProviderConfig = *in.ProviderConfig
	}
	k.UpdatedAt = now()
	writeJSON(w, k)
}

func (s *Server) deleteExternalKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	for i, k := range s.store.externalKeys {
		if k.ID == r.PathValue("id") {
			if k.Attachment.Type == "attached" {
				writeError(w, 400, "invalid_request_error", "external key is attached to a workspace")
				return
			}
			s.store.externalKeys = slices.Delete(s.store.externalKeys, i, i+1)
			deleted(w, k.ID, "external_key_deleted")
			return
		}
	}
	notFound(w, "external_key")
}

func (s *Server) validateExternalKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	k := s.findExternalKey(r.PathValue("id"))
	if k == nil {
		notFound(w, "external_key")
		return
	}
	out := client.ExternalKeyValidation{Status: "success", Type: "external_key_validation"}
	if k.DisplayName != nil && strings.Contains(*k.DisplayName, "invalid") {
		out.Status = "failure"
		out.Error = ptr("KMS key is not accessible")
	}
	writeJSON(w, out)
}
