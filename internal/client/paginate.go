package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

const (
	listPageSize = 100
	// maxListPages bounds a listing at 100k objects. A server that keeps
	// answering has_more with the same cursor would otherwise loop forever.
	maxListPages = 1000
)

var errCursorStuck = fmt.Errorf("pagination cursor did not advance; the server returned the same page twice")

// cloneValues copies q so pagination never mutates the caller's map.
func cloneValues(q url.Values) url.Values {
	out := make(url.Values, len(q)+2)
	for k, v := range q {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// cursorPage is the envelope used by users, invites, workspaces, workspace
// members and api_keys: has_more with after_id/before_id cursors.
type cursorPage[T any] struct {
	Data    []T     `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstID *string `json:"first_id"`
	LastID  *string `json:"last_id"`
}

func listCursor[T any](ctx context.Context, c *Client, class CredentialClass, path string, q url.Values) ([]T, error) {
	q = cloneValues(q)
	q.Set("limit", strconv.Itoa(listPageSize))
	var all []T
	prev := ""
	for n := 0; n < maxListPages; n++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var page cursorPage[T]
		if err := c.get(ctx, class, path, q, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if !page.HasMore || page.LastID == nil || *page.LastID == "" {
			return all, nil
		}
		if *page.LastID == prev {
			return nil, fmt.Errorf("GET %s: %w", path, errCursorStuck)
		}
		prev = *page.LastID
		q.Set("after_id", prev)
	}
	return nil, fmt.Errorf("GET %s: more than %d pages", path, maxListPages)
}

// tokenPage is the envelope used by service accounts, federation, external
// keys, RBAC and spend limits: an opaque next_page token, null at the end.
type tokenPage[T any] struct {
	Data     []T     `json:"data"`
	HasMore  *bool   `json:"has_more,omitempty"`
	NextPage *string `json:"next_page"`
}

func listToken[T any](ctx context.Context, c *Client, class CredentialClass, path string, q url.Values, opts ...reqOption) ([]T, error) {
	q = cloneValues(q)
	q.Set("limit", strconv.Itoa(listPageSize))
	var all []T
	prev := ""
	for n := 0; n < maxListPages; n++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var page tokenPage[T]
		if err := c.get(ctx, class, path, q, &page, opts...); err != nil {
			return nil, err
		}
		all = append(all, page.Data...)
		if page.NextPage == nil || *page.NextPage == "" {
			return all, nil
		}
		if *page.NextPage == prev {
			return nil, fmt.Errorf("GET %s: %w", path, errCursorStuck)
		}
		prev = *page.NextPage
		q.Set("page", prev)
	}
	return nil, fmt.Errorf("GET %s: more than %d pages", path, maxListPages)
}
