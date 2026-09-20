package provider

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// The live acceptance tiers create objects named tf-acc-<random>. A run that
// fails partway, or is cancelled, leaves behind whatever it had already
// created. Several of these archive rather than delete, so a development
// organization accumulates permanent records with no way to tidy up.
//
// Sweep with:
//
//	go test ./internal/provider/ -sweep=all -timeout 30m
//
// Only names beginning with tf-acc- are touched. That prefix is the whole
// safety boundary: a sweeper that guessed would be pointed at whatever
// organization the credentials in the environment happen to reach.

const sweepPrefix = "tf-acc-"

// sweepable reports whether a name belongs to the acceptance tests. Anything
// else is left alone, including objects whose name merely contains the prefix.
func sweepable(name string) bool { return strings.HasPrefix(name, sweepPrefix) }

// sweeperClient builds a client from the environment. Sweeping is only
// meaningful against a real organization, so a missing credential is an error
// rather than a silent no-op.
func sweeperClient() (*client.Client, error) {
	if os.Getenv("ANTHROPIC_ADMIN_API_KEY") == "" && os.Getenv("ANTHROPIC_AUTH_TOKEN") == "" &&
		os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("sweeping needs real credentials: set ANTHROPIC_ADMIN_API_KEY, ANTHROPIC_AUTH_TOKEN or ANTHROPIC_API_KEY")
	}
	return client.New(client.Config{
		BaseURL:          os.Getenv("ANTHROPIC_BASE_URL"),
		AdminAPIKey:      os.Getenv("ANTHROPIC_ADMIN_API_KEY"),
		OAuthToken:       os.Getenv("ANTHROPIC_AUTH_TOKEN"),
		EnterpriseAPIKey: os.Getenv("ANTHROPIC_ENTERPRISE_API_KEY"),
		APIKey:           os.Getenv("ANTHROPIC_API_KEY"),
		WorkspaceID:      os.Getenv("ANTHROPIC_WORKSPACE_ID"),
	})
}

// swept logs one removal. A sweeper reports what it touched, because the point
// is to be able to see afterwards what was cleaned up.
func swept(kind, name, id string) { log.Printf("swept %s %s (%s)", kind, name, id) }

// sweepErr collects failures without stopping: one object refusing to go is no
// reason to leave the rest behind.
type sweepErr []error

func (s sweepErr) orNil() error {
	if len(s) == 0 {
		return nil
	}
	return fmt.Errorf("%d object(s) could not be swept: %v", len(s), []error(s))
}

func init() {
	ctx := context.Background()

	// Federation rules reference an issuer, so they go first.
	resource.AddTestSweepers("anthropic_federation_rule", &resource.Sweeper{
		Name: "anthropic_federation_rule",
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			rules, err := c.ListFederationRules(ctx, client.FederationRuleListOptions{})
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, r := range rules {
				if !sweepable(r.Name) || r.ArchivedAt != nil {
					continue
				}
				if _, err := c.ArchiveFederationRule(ctx, r.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", r.Name, err))
					continue
				}
				swept("federation rule", r.Name, r.ID)
			}
			return errs.orNil()
		},
	})

	resource.AddTestSweepers("anthropic_federation_issuer", &resource.Sweeper{
		Name:         "anthropic_federation_issuer",
		Dependencies: []string{"anthropic_federation_rule"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			issuers, err := c.ListFederationIssuers(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, i := range issuers {
				if !sweepable(i.Name) {
					continue
				}
				if _, err := c.ArchiveFederationIssuer(ctx, i.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", i.Name, err))
					continue
				}
				swept("federation issuer", i.Name, i.ID)
			}
			return errs.orNil()
		},
	})

	// A service account may still be bound to a workspace or named by a rule,
	// so it follows both.
	resource.AddTestSweepers("anthropic_service_account", &resource.Sweeper{
		Name:         "anthropic_service_account",
		Dependencies: []string{"anthropic_federation_rule"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			accounts, err := c.ListServiceAccounts(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, a := range accounts {
				if !sweepable(a.Name) {
					continue
				}
				if _, err := c.ArchiveServiceAccount(ctx, a.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", a.Name, err))
					continue
				}
				swept("service account", a.Name, a.ID)
			}
			return errs.orNil()
		},
	})

	resource.AddTestSweepers("anthropic_workspace", &resource.Sweeper{
		Name:         "anthropic_workspace",
		Dependencies: []string{"anthropic_federation_rule", "anthropic_service_account"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			workspaces, err := c.ListWorkspaces(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, w := range workspaces {
				if !sweepable(w.Name) || w.ArchivedAt != nil {
					continue
				}
				if _, err := c.ArchiveWorkspace(ctx, w.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", w.Name, err))
					continue
				}
				swept("workspace", w.Name, w.ID)
			}
			return errs.orNil()
		},
	})

	// Managed Agents. Deployments pin an agent version, so they go first.
	resource.AddTestSweepers("anthropic_deployment", &resource.Sweeper{
		Name: "anthropic_deployment",
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			deployments, err := c.ListDeployments(ctx, client.DeploymentListOptions{})
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, d := range deployments {
				if !sweepable(d.Name) {
					continue
				}
				if _, err := c.ArchiveDeployment(ctx, d.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", d.Name, err))
					continue
				}
				swept("deployment", d.Name, d.ID)
			}
			return errs.orNil()
		},
	})

	resource.AddTestSweepers("anthropic_agent", &resource.Sweeper{
		Name:         "anthropic_agent",
		Dependencies: []string{"anthropic_deployment"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			agents, err := c.ListAgents(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, a := range agents {
				if !sweepable(a.Name) {
					continue
				}
				if _, err := c.ArchiveAgent(ctx, a.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", a.Name, err))
					continue
				}
				swept("agent", a.Name, a.ID)
			}
			return errs.orNil()
		},
	})

	resource.AddTestSweepers("anthropic_environment", &resource.Sweeper{
		Name:         "anthropic_environment",
		Dependencies: []string{"anthropic_deployment"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			environments, err := c.ListEnvironments(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, e := range environments {
				if !sweepable(e.Name) {
					continue
				}
				if err := c.DeleteEnvironment(ctx, e.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", e.Name, err))
					continue
				}
				swept("environment", e.Name, e.ID)
			}
			return errs.orNil()
		},
	})

	// Deleting a vault takes its credentials with it, so the credentials need
	// no sweeper of their own.
	resource.AddTestSweepers("anthropic_vault", &resource.Sweeper{
		Name:         "anthropic_vault",
		Dependencies: []string{"anthropic_deployment"},
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			vaults, err := c.ListVaults(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, v := range vaults {
				if !sweepable(v.DisplayName) {
					continue
				}
				if err := c.DeleteVault(ctx, v.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", v.DisplayName, err))
					continue
				}
				swept("vault", v.DisplayName, v.ID)
			}
			return errs.orNil()
		},
	})

	resource.AddTestSweepers("anthropic_memory_store", &resource.Sweeper{
		Name: "anthropic_memory_store",
		F: func(string) error {
			c, err := sweeperClient()
			if err != nil {
				return err
			}
			stores, err := c.ListMemoryStores(ctx, false)
			if err != nil {
				return err
			}
			var errs sweepErr
			for _, s := range stores {
				if !sweepable(s.Name) {
					continue
				}
				if err := c.DeleteMemoryStore(ctx, s.ID); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
					continue
				}
				swept("memory store", s.Name, s.ID)
			}
			return errs.orNil()
		},
	})
}

// The prefix check is the only thing standing between a sweeper and whatever
// organization the credentials in the environment reach, so it is worth
// testing on its own.
func TestSweepableOnlyMatchesTestObjects(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"tf-acc-12345":            true,
		"tf-acc-":                 true,
		"tf-acc-workspace-rename": true,

		"":                        false,
		"production":              false,
		"Production":              false,
		"TF-ACC-12345":            false, // case-sensitive on purpose
		"tf-acc":                  false, // no separator, so not one of ours
		"my-tf-acc-workspace":     false, // contains the prefix, does not start with it
		" tf-acc-12345":           false, // leading space is a different name
		"staging-tf-acc-rollback": false,
	}

	for name, want := range cases {
		if got := sweepable(name); got != want {
			t.Errorf("sweepable(%q) = %v, want %v", name, got, want)
		}
	}
}
