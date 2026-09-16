package mock

import "github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"

// Invites returns all invites (excluding deleted ones).
func (s *Server) Invites() []client.Invite {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]client.Invite, 0, len(s.store.invites))
	for _, inv := range s.store.invites {
		if inv.Status != "deleted" {
			out = append(out, *inv)
		}
	}
	return out
}
