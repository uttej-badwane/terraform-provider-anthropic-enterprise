package client

import (
	"context"
	"net/url"
	"strconv"
)

const listPageSize = 100

// cursorPage is the envelope used by users, invites, workspaces, workspace
// members and api_keys: has_more with after_id/before_id cursors.
type cursorPage[T any] struct {
	Data    []T     `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstID *string `json:"first_id"`
	LastID  *string `json:"last_id"`
}

func listCursor[T any](ctx context.Context, c *Client, class CredentialClass, path string, q url.Values) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", strconv.Itoa(listPageSize))
	var all []T
	for {
		var page cursorPage[T]
		if err := c.get(ctx, class, path, q, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if !page.HasMore || page.LastID == nil || *page.LastID == "" {
			return all, nil
		}
		q.Set("after_id", *page.LastID)
	}
}

// tokenPage is the envelope used by service accounts, federation, external
// keys, RBAC and spend limits: an opaque next_page token, null at the end.
type tokenPage[T any] struct {
	Data     []T     `json:"data"`
	HasMore  *bool   `json:"has_more,omitempty"`
	NextPage *string `json:"next_page"`
}

func listToken[T any](ctx context.Context, c *Client, class CredentialClass, path string, q url.Values, opts ...reqOption) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", strconv.Itoa(listPageSize))
	var all []T
	for {
		var page tokenPage[T]
		if err := c.get(ctx, class, path, q, &page, opts...); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if page.NextPage == nil || *page.NextPage == "" {
			return all, nil
		}
		q.Set("page", *page.NextPage)
	}
}
