package client_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// The API reference marks two fields deprecated and says each "always has the
// same value" as its replacement:
//
//	roles     -> role_ids       (RBAC groups)
//	group_id  -> rbac_group_id  (RBAC group members)
//
// The provider used to read the deprecated ones. The cases that matter are the
// ones where a deprecated field is gone, because that is the day the old code
// would have read back empty values and produced a diff, or a replacement, for
// every group and membership in state.
func TestRBACGroupPrefersRoleIDs(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want []string // nil means "null", distinct from an empty list
	}{
		"deprecated field removed": {
			body: `{"id":"g","role_ids":["rbac_role_a","rbac_role_b"]}`,
			want: []string{"rbac_role_a", "rbac_role_b"},
		},
		"only the deprecated field, from before role_ids existed": {
			body: `{"id":"g","roles":["rbac_role_a"]}`,
			want: []string{"rbac_role_a"},
		},
		"both present, as the API sends today": {
			body: `{"id":"g","role_ids":["rbac_role_a"],"roles":["rbac_role_a"]}`,
			want: []string{"rbac_role_a"},
		},
		// An explicit empty role_ids is a real answer, a group with no roles,
		// and must not be overridden by whatever roles happens to hold.
		"empty role_ids is not overridden": {
			body: `{"id":"g","role_ids":[],"roles":["rbac_role_stale"]}`,
			want: []string{},
		},
		// null is how the API says role data is temporarily unavailable.
		"both null stays null": {
			body: `{"id":"g","role_ids":null,"roles":null}`,
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var g client.RBACGroup
			if err := json.Unmarshal([]byte(tc.body), &g); err != nil {
				t.Fatalf("decode: %v", err)
			}
			got := g.EffectiveRoleIDs()
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("EffectiveRoleIDs() = %#v, want %#v (null and empty must stay distinct)", got, tc.want)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("EffectiveRoleIDs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRBACGroupMemberPrefersRBACGroupID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want string
	}{
		"deprecated field removed":  {`{"user_id":"u","rbac_group_id":"rbac_group_a"}`, "rbac_group_a"},
		"only the deprecated field": {`{"user_id":"u","group_id":"rbac_group_a"}`, "rbac_group_a"},
		"both present":              {`{"user_id":"u","rbac_group_id":"rbac_group_a","group_id":"rbac_group_a"}`, "rbac_group_a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var m client.RBACGroupMember
			if err := json.Unmarshal([]byte(tc.body), &m); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := m.EffectiveGroupID(); got != tc.want {
				t.Errorf("EffectiveGroupID() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Rate limits: group_type is deprecated in favour of group.type ("group_type
// is still returned and always equals group.type").
func TestRateLimitPrefersGroupType(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want string
	}{
		"deprecated field removed":  {`{"id":"rl","group":{"type":"model_group","id":"rlg_1"}}`, "model_group"},
		"only the deprecated field": {`{"id":"rl","group_type":"batch"}`, "batch"},
		"both present":              {`{"id":"rl","group":{"type":"batch"},"group_type":"batch"}`, "batch"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var rl client.RateLimit
			var wrl client.WorkspaceRateLimit
			if err := json.Unmarshal([]byte(tc.body), &rl); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.body), &wrl); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := rl.EffectiveGroupType(); got != tc.want {
				t.Errorf("RateLimit.EffectiveGroupType() = %q, want %q", got, tc.want)
			}
			if got := wrl.EffectiveGroupType(); got != tc.want {
				t.Errorf("WorkspaceRateLimit.EffectiveGroupType() = %q, want %q", got, tc.want)
			}
		})
	}
}
