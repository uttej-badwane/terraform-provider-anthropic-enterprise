package client

import (
	"context"
	"net/url"
)

func workspacePath(id string) string { return orgPath + "/workspaces/" + url.PathEscape(id) }

// ListWorkspaces returns workspaces; archived ones only when includeArchived.
func (c *Client) ListWorkspaces(ctx context.Context, includeArchived bool) ([]Workspace, error) {
	q := url.Values{}
	if includeArchived {
		q.Set("include_archived", "true")
	}
	return listCursor[Workspace](ctx, c, CredAdmin, orgPath+"/workspaces", q)
}

// CreateWorkspace creates a workspace.
func (c *Client) CreateWorkspace(ctx context.Context, in WorkspaceCreate) (*Workspace, error) {
	var out Workspace
	if err := c.post(ctx, CredAdmin, orgPath+"/workspaces", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkspace returns one workspace (archived included).
func (c *Client) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var out Workspace
	if err := c.get(ctx, CredAdmin, workspacePath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWorkspace updates mutable fields.
func (c *Client) UpdateWorkspace(ctx context.Context, id string, in WorkspaceUpdate) (*Workspace, error) {
	var out Workspace
	if err := c.post(ctx, CredAdmin, workspacePath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveWorkspace archives a workspace (irreversible).
func (c *Client) ArchiveWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var out Workspace
	if err := c.post(ctx, CredAdmin, workspacePath(id)+"/archive", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- workspace members -----------------------------------------------------

// ListWorkspaceMembers lists members of a workspace.
func (c *Client) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	return listCursor[WorkspaceMember](ctx, c, CredAdmin, workspacePath(workspaceID)+"/members", nil)
}

// AddWorkspaceMember adds a user to a workspace.
func (c *Client) AddWorkspaceMember(ctx context.Context, workspaceID string, in WorkspaceMemberAdd) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.post(ctx, CredAdmin, workspacePath(workspaceID)+"/members", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkspaceMember returns one membership.
func (c *Client) GetWorkspaceMember(ctx context.Context, workspaceID, userID string) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.get(ctx, CredAdmin, workspacePath(workspaceID)+"/members/"+url.PathEscape(userID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWorkspaceMember changes a member's workspace role.
func (c *Client) UpdateWorkspaceMember(ctx context.Context, workspaceID, userID string, in WorkspaceRoleUpdate) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.post(ctx, CredAdmin, workspacePath(workspaceID)+"/members/"+url.PathEscape(userID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveWorkspaceMember removes a user from a workspace.
func (c *Client) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	return c.delete(ctx, CredAdmin, workspacePath(workspaceID)+"/members/"+url.PathEscape(userID))
}

// --- workspace service accounts (OAuth only) --------------------------------

// ListWorkspaceServiceAccounts lists service-account memberships of a workspace.
func (c *Client) ListWorkspaceServiceAccounts(ctx context.Context, workspaceID string) ([]ServiceAccountWorkspaceMember, error) {
	return listToken[ServiceAccountWorkspaceMember](ctx, c, CredOAuth, workspacePath(workspaceID)+"/service_accounts", nil)
}

// AddWorkspaceServiceAccount grants a service account a role in a workspace.
func (c *Client) AddWorkspaceServiceAccount(ctx context.Context, workspaceID string, in ServiceAccountWorkspaceMemberAdd) (*ServiceAccountWorkspaceMember, error) {
	var out ServiceAccountWorkspaceMember
	if err := c.post(ctx, CredOAuth, workspacePath(workspaceID)+"/service_accounts", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkspaceServiceAccount returns one service-account membership.
func (c *Client) GetWorkspaceServiceAccount(ctx context.Context, workspaceID, serviceAccountID string) (*ServiceAccountWorkspaceMember, error) {
	var out ServiceAccountWorkspaceMember
	if err := c.get(ctx, CredOAuth, workspacePath(workspaceID)+"/service_accounts/"+url.PathEscape(serviceAccountID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWorkspaceServiceAccount changes a service account's workspace role.
func (c *Client) UpdateWorkspaceServiceAccount(ctx context.Context, workspaceID, serviceAccountID string, in WorkspaceRoleUpdate) (*ServiceAccountWorkspaceMember, error) {
	var out ServiceAccountWorkspaceMember
	if err := c.post(ctx, CredOAuth, workspacePath(workspaceID)+"/service_accounts/"+url.PathEscape(serviceAccountID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveWorkspaceServiceAccount removes a service account from a workspace.
func (c *Client) RemoveWorkspaceServiceAccount(ctx context.Context, workspaceID, serviceAccountID string) error {
	return c.delete(ctx, CredOAuth, workspacePath(workspaceID)+"/service_accounts/"+url.PathEscape(serviceAccountID))
}

// --- rate limits -----------------------------------------------------------

// RateLimitListOptions filters ListRateLimits.
type RateLimitListOptions struct {
	GroupType string
	Model     string
}

// ListRateLimits returns organization rate limit groups.
func (c *Client) ListRateLimits(ctx context.Context, opts RateLimitListOptions) ([]RateLimit, error) {
	q := url.Values{}
	if opts.GroupType != "" {
		q.Set("group_type", opts.GroupType)
	}
	if opts.Model != "" {
		q.Set("model", opts.Model)
	}
	return listToken[RateLimit](ctx, c, CredAdmin, orgPath+"/rate_limits", q)
}

// ListWorkspaceRateLimits returns a workspace's rate limit overrides.
func (c *Client) ListWorkspaceRateLimits(ctx context.Context, workspaceID, groupType string) ([]WorkspaceRateLimit, error) {
	q := url.Values{}
	if groupType != "" {
		q.Set("group_type", groupType)
	}
	return listToken[WorkspaceRateLimit](ctx, c, CredAdmin, workspacePath(workspaceID)+"/rate_limits", q)
}

// --- api keys --------------------------------------------------------------

// APIKeyListOptions filters ListAPIKeys.
type APIKeyListOptions struct {
	Status          string
	WorkspaceID     string
	CreatedByUserID string
}

// ListAPIKeys returns API key records (never the secrets).
func (c *Client) ListAPIKeys(ctx context.Context, opts APIKeyListOptions) ([]APIKey, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.WorkspaceID != "" {
		q.Set("workspace_id", opts.WorkspaceID)
	}
	if opts.CreatedByUserID != "" {
		q.Set("created_by_user_id", opts.CreatedByUserID)
	}
	return listCursor[APIKey](ctx, c, CredAdmin, orgPath+"/api_keys", q)
}

// GetAPIKey returns one API key record.
func (c *Client) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var out APIKey
	if err := c.get(ctx, CredAdmin, orgPath+"/api_keys/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAPIKey renames or changes the status of a key.
func (c *Client) UpdateAPIKey(ctx context.Context, id string, in APIKeyUpdate) (*APIKey, error) {
	var out APIKey
	if err := c.post(ctx, CredAdmin, orgPath+"/api_keys/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindWorkspaceMember locates a membership through the list endpoint. The
// per-member GET is served from a cache that updates do not invalidate for
// tens of seconds, so reads that must be fresh go through the list instead.
// A missing membership is reported as a 404 APIError.
func (c *Client) FindWorkspaceMember(ctx context.Context, workspaceID, userID string) (*WorkspaceMember, error) {
	members, err := c.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].UserID == userID {
			return &members[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Type: "not_found_error", Message: "workspace member not found", Method: "GET", Path: workspacePath(workspaceID) + "/members"}
}

// FindWorkspaceServiceAccount is the list-based counterpart of
// GetWorkspaceServiceAccount, for the same caching reason.
func (c *Client) FindWorkspaceServiceAccount(ctx context.Context, workspaceID, serviceAccountID string) (*ServiceAccountWorkspaceMember, error) {
	members, err := c.ListWorkspaceServiceAccounts(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].ServiceAccountID == serviceAccountID {
			return &members[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Type: "not_found_error", Message: "workspace service account membership not found", Method: "GET", Path: workspacePath(workspaceID) + "/service_accounts"}
}
