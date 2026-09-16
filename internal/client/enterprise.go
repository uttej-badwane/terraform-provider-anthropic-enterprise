package client

import (
	"context"
	"net/url"
)

func rbacGroupPath(id string) string { return orgPath + "/rbac_groups/" + url.PathEscape(id) }

// --- RBAC groups (Enterprise) ----------------------------------------------

// ListRBACGroups lists groups.
func (c *Client) ListRBACGroups(ctx context.Context) ([]RBACGroup, error) {
	return listToken[RBACGroup](ctx, c, CredEnterprise, orgPath+"/rbac_groups", nil)
}

// CreateRBACGroup creates a group.
func (c *Client) CreateRBACGroup(ctx context.Context, in RBACGroupWrite) (*RBACGroup, error) {
	var out RBACGroup
	if err := c.post(ctx, CredEnterprise, orgPath+"/rbac_groups", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRBACGroup returns one group.
func (c *Client) GetRBACGroup(ctx context.Context, id string) (*RBACGroup, error) {
	var out RBACGroup
	if err := c.get(ctx, CredEnterprise, rbacGroupPath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRBACGroup renames a group.
func (c *Client) UpdateRBACGroup(ctx context.Context, id string, in RBACGroupWrite) (*RBACGroup, error) {
	var out RBACGroup
	if err := c.post(ctx, CredEnterprise, rbacGroupPath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRBACGroup deletes a group.
func (c *Client) DeleteRBACGroup(ctx context.Context, id string) error {
	return c.delete(ctx, CredEnterprise, rbacGroupPath(id))
}

// ListRBACGroupMembers lists a group's members.
func (c *Client) ListRBACGroupMembers(ctx context.Context, groupID string) ([]RBACGroupMember, error) {
	return listToken[RBACGroupMember](ctx, c, CredEnterprise, rbacGroupPath(groupID)+"/members", nil)
}

// AddRBACGroupMember adds a user to a group.
func (c *Client) AddRBACGroupMember(ctx context.Context, groupID, userID string) (*RBACGroupMember, error) {
	var out RBACGroupMember
	if err := c.post(ctx, CredEnterprise, rbacGroupPath(groupID)+"/members", RBACGroupMemberAdd{UserID: userID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveRBACGroupMember removes a user from a group.
func (c *Client) RemoveRBACGroupMember(ctx context.Context, groupID, userID string) error {
	return c.delete(ctx, CredEnterprise, rbacGroupPath(groupID)+"/members/"+url.PathEscape(userID))
}

// --- RBAC roles (Enterprise, read-only) ------------------------------------

// ListRBACRoles lists roles.
func (c *Client) ListRBACRoles(ctx context.Context) ([]RBACRole, error) {
	return listToken[RBACRole](ctx, c, CredEnterprise, orgPath+"/rbac_roles", nil)
}

// GetRBACRole returns one role.
func (c *Client) GetRBACRole(ctx context.Context, id string) (*RBACRole, error) {
	var out RBACRole
	if err := c.get(ctx, CredEnterprise, orgPath+"/rbac_roles/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRBACRolePermissions lists a role's permissions.
func (c *Client) ListRBACRolePermissions(ctx context.Context, roleID string) ([]RBACRolePermission, error) {
	return listToken[RBACRolePermission](ctx, c, CredEnterprise, orgPath+"/rbac_roles/"+url.PathEscape(roleID)+"/permissions", nil)
}

// --- spend limits (Enterprise) ---------------------------------------------

// SpendLimitListOptions filters ListEffectiveSpendLimits.
type SpendLimitListOptions struct {
	Periods []string
	UserIDs []string
}

// ListEffectiveSpendLimits returns the effective per-user limits.
func (c *Client) ListEffectiveSpendLimits(ctx context.Context, opts SpendLimitListOptions) ([]SpendSummary, error) {
	q := url.Values{}
	for _, p := range opts.Periods {
		q.Add("period[]", p)
	}
	for _, u := range opts.UserIDs {
		q.Add("user_ids[]", u)
	}
	return listToken[SpendSummary](ctx, c, CredEnterprise, orgPath+"/spend_limits/effective", q)
}

// GetSpendLimit returns one spend limit row.
func (c *Client) GetSpendLimit(ctx context.Context, id string) (*SpendLimit, error) {
	var out SpendLimit
	if err := c.get(ctx, CredEnterprise, orgPath+"/spend_limits/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpsertSpendLimit creates or replaces a per-user spend limit.
func (c *Client) UpsertSpendLimit(ctx context.Context, in SpendLimitCreate) (*SpendLimit, error) {
	var out SpendLimit
	if err := c.post(ctx, CredEnterprise, orgPath+"/spend_limits", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSpendLimit removes a per-user spend limit override.
func (c *Client) DeleteSpendLimit(ctx context.Context, id string) error {
	return c.delete(ctx, CredEnterprise, orgPath+"/spend_limits/"+url.PathEscape(id))
}
