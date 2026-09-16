package client

import (
	"context"
	"net/url"
)

const orgPath = "/v1/organizations"

// GetOrganization returns the organization the credential belongs to.
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	var out Organization
	if err := c.get(ctx, c.memberCred(), orgPath+"/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetComplianceSettings reads the compliance API toggle.
func (c *Client) GetComplianceSettings(ctx context.Context) (*ComplianceSettings, error) {
	var out ComplianceSettings
	if err := c.get(ctx, CredAdmin, orgPath+"/compliance_settings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateComplianceSettings sets the compliance API toggle.
func (c *Client) UpdateComplianceSettings(ctx context.Context, in ComplianceSettingsUpdate) (*ComplianceSettings, error) {
	var out ComplianceSettings
	if err := c.post(ctx, CredAdmin, orgPath+"/compliance_settings", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- users -----------------------------------------------------------------

// UserListOptions filters ListUsers.
type UserListOptions struct {
	Email string
	Roles []string
}

// ListUsers returns all members matching opts. It uses the Admin credential
// unless only an Enterprise key is configured.
func (c *Client) ListUsers(ctx context.Context, opts UserListOptions) ([]User, error) {
	q := url.Values{}
	if opts.Email != "" {
		q.Set("email", opts.Email)
	}
	for _, r := range opts.Roles {
		q.Add("roles", r)
	}
	return listCursor[User](ctx, c, c.memberCred(), orgPath+"/users", q)
}

// GetUser returns one member.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	var out User
	if err := c.get(ctx, c.memberCred(), orgPath+"/users/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateUser changes a member's role.
func (c *Client) UpdateUser(ctx context.Context, id string, in UserUpdate) (*User, error) {
	var out User
	if err := c.post(ctx, c.memberCred(), orgPath+"/users/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveUser removes a member from the organization.
func (c *Client) RemoveUser(ctx context.Context, id string) error {
	return c.delete(ctx, c.memberCred(), orgPath+"/users/"+url.PathEscape(id))
}

// --- invites ---------------------------------------------------------------

// InviteListOptions filters ListInvites.
type InviteListOptions struct {
	Email    string
	Roles    []string
	Statuses []string
}

// ListInvites returns invitations.
func (c *Client) ListInvites(ctx context.Context, opts InviteListOptions) ([]Invite, error) {
	q := url.Values{}
	if opts.Email != "" {
		q.Set("email", opts.Email)
	}
	for _, r := range opts.Roles {
		q.Add("roles", r)
	}
	for _, s := range opts.Statuses {
		q.Add("statuses", s)
	}
	return listCursor[Invite](ctx, c, c.memberCred(), orgPath+"/invites", q)
}

// CreateInvite sends an invitation.
func (c *Client) CreateInvite(ctx context.Context, in InviteCreate) (*Invite, error) {
	var out Invite
	if err := c.create(ctx, c.memberCred(), orgPath+"/invites", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetInvite returns one invitation.
func (c *Client) GetInvite(ctx context.Context, id string) (*Invite, error) {
	var out Invite
	if err := c.get(ctx, c.memberCred(), orgPath+"/invites/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteInvite revokes a pending invitation.
func (c *Client) DeleteInvite(ctx context.Context, id string) error {
	return c.delete(ctx, c.memberCred(), orgPath+"/invites/"+url.PathEscape(id))
}

// memberCred picks the credential for users/invites, which exist on both the
// Console and Claude Enterprise surfaces.
func (c *Client) memberCred() CredentialClass {
	if c.HasAdmin() {
		return CredAdmin
	}
	return CredEnterprise
}

// MemberCred exposes memberCred for resources that need to check availability.
func (c *Client) MemberCred() CredentialClass { return c.memberCred() }
