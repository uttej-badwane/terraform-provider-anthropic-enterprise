package mock

const base = "/v1/organizations"

func (s *Server) routes() {
	s.reportRoutes()
	s.analyticsRoutes()
	s.agentRoutes()
	s.modelRoutes()
	s.deploymentRunRoutes()
	s.handle("POST /v1/oauth/token", s.exchangeFederationToken)
	s.handle("GET "+base+"/me", s.getOrganization)
	s.handle("GET "+base+"/compliance_settings", s.getCompliance)
	s.handle("POST "+base+"/compliance_settings", s.updateCompliance)

	s.handle("GET "+base+"/users", s.listUsers)
	s.handle("GET "+base+"/users/{id}", s.getUser)
	s.handle("POST "+base+"/users/{id}", s.updateUser)
	s.handle("DELETE "+base+"/users/{id}", s.deleteUser)

	s.handle("GET "+base+"/invites", s.listInvites)
	s.handle("POST "+base+"/invites", s.createInvite)
	s.handle("GET "+base+"/invites/{id}", s.getInvite)
	s.handle("DELETE "+base+"/invites/{id}", s.deleteInvite)

	s.handle("GET "+base+"/workspaces", s.listWorkspaces)
	s.handle("POST "+base+"/workspaces", s.createWorkspace)
	s.handle("GET "+base+"/workspaces/{id}", s.getWorkspace)
	s.handle("POST "+base+"/workspaces/{id}", s.updateWorkspace)
	s.handle("POST "+base+"/workspaces/{id}/archive", s.archiveWorkspace)
	s.handle("GET "+base+"/workspaces/{id}/rate_limits", s.listWorkspaceRateLimits)

	s.handle("GET "+base+"/workspaces/{id}/members", s.listMembers)
	s.handle("POST "+base+"/workspaces/{id}/members", s.addMember)
	s.handle("GET "+base+"/workspaces/{id}/members/{uid}", s.getMember)
	s.handle("POST "+base+"/workspaces/{id}/members/{uid}", s.updateMember)
	s.handle("DELETE "+base+"/workspaces/{id}/members/{uid}", s.deleteMember)

	s.handle("GET "+base+"/workspaces/{id}/service_accounts", s.listWorkspaceSAs)
	s.handle("POST "+base+"/workspaces/{id}/service_accounts", s.addWorkspaceSA)
	s.handle("GET "+base+"/workspaces/{id}/service_accounts/{sid}", s.getWorkspaceSA)
	s.handle("POST "+base+"/workspaces/{id}/service_accounts/{sid}", s.updateWorkspaceSA)
	s.handle("DELETE "+base+"/workspaces/{id}/service_accounts/{sid}", s.deleteWorkspaceSA)

	s.handle("GET "+base+"/api_keys", s.listAPIKeys)
	s.handle("GET "+base+"/api_keys/{id}", s.getAPIKey)
	s.handle("POST "+base+"/api_keys/{id}", s.updateAPIKey)

	s.handle("GET "+base+"/rate_limits", s.listRateLimits)

	s.handle("GET "+base+"/service_accounts", s.listServiceAccounts)
	s.handle("POST "+base+"/service_accounts", s.createServiceAccount)
	s.handle("GET "+base+"/service_accounts/{id}", s.getServiceAccount)
	s.handle("POST "+base+"/service_accounts/{id}", s.updateServiceAccount)
	s.handle("POST "+base+"/service_accounts/{id}/archive", s.archiveServiceAccount)

	s.handle("GET "+base+"/federation_issuers", s.listIssuers)
	s.handle("POST "+base+"/federation_issuers", s.createIssuer)
	s.handle("GET "+base+"/federation_issuers/{id}", s.getIssuer)
	s.handle("POST "+base+"/federation_issuers/{id}", s.updateIssuer)
	s.handle("POST "+base+"/federation_issuers/{id}/archive", s.archiveIssuer)

	s.handle("GET "+base+"/federation_rules", s.listRules)
	s.handle("POST "+base+"/federation_rules", s.createRule)
	s.handle("GET "+base+"/federation_rules/{id}", s.getRule)
	s.handle("POST "+base+"/federation_rules/{id}", s.updateRule)
	s.handle("POST "+base+"/federation_rules/{id}/archive", s.archiveRule)
	s.handle("GET "+base+"/federation_rules/{id}/workspaces", s.listRuleWorkspaces)
	s.handle("POST "+base+"/federation_rules/{id}/workspaces", s.addRuleWorkspace)
	s.handle("DELETE "+base+"/federation_rules/{id}/workspaces/{wid}", s.deleteRuleWorkspace)

	s.handle("GET "+base+"/external_keys", s.listExternalKeys)
	s.handle("POST "+base+"/external_keys", s.createExternalKey)
	s.handle("GET "+base+"/external_keys/{id}", s.getExternalKey)
	s.handle("POST "+base+"/external_keys/{id}", s.updateExternalKey)
	s.handle("DELETE "+base+"/external_keys/{id}", s.deleteExternalKey)
	s.handle("POST "+base+"/external_keys/{id}/validate", s.validateExternalKey)

	s.handle("GET "+base+"/rbac_groups", s.listGroups)
	s.handle("POST "+base+"/rbac_groups", s.createGroup)
	s.handle("GET "+base+"/rbac_groups/{id}", s.getGroup)
	s.handle("POST "+base+"/rbac_groups/{id}", s.updateGroup)
	s.handle("DELETE "+base+"/rbac_groups/{id}", s.deleteGroup)
	s.handle("GET "+base+"/rbac_groups/{id}/members", s.listGroupMembers)
	s.handle("POST "+base+"/rbac_groups/{id}/members", s.addGroupMember)
	s.handle("DELETE "+base+"/rbac_groups/{id}/members/{uid}", s.deleteGroupMember)

	s.handle("GET "+base+"/rbac_roles", s.listRoles)
	s.handle("GET "+base+"/rbac_roles/{id}", s.getRole)
	s.handle("GET "+base+"/rbac_roles/{id}/permissions", s.listRolePermissions)

	s.handle("GET "+base+"/spend_limits/effective", s.listEffectiveSpendLimits)
	s.handle("POST "+base+"/spend_limits", s.upsertSpendLimit)
	s.handle("GET "+base+"/spend_limits/{id}", s.getSpendLimit)
	s.handle("DELETE "+base+"/spend_limits/{id}", s.deleteSpendLimit)
}
