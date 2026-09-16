package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

var _ provider.Provider = &AnthropicProvider{}

// AnthropicProvider manages an Anthropic organization through the Admin API.
type AnthropicProvider struct {
	version string
}

type providerModel struct {
	BaseURL          types.String `tfsdk:"base_url"`
	AdminAPIKey      types.String `tfsdk:"admin_api_key"`
	OAuthToken       types.String `tfsdk:"oauth_token"`
	EnterpriseAPIKey types.String `tfsdk:"enterprise_api_key"`
	ComplianceAPIKey types.String `tfsdk:"compliance_api_key"`
	AnalyticsAPIKey  types.String `tfsdk:"analytics_api_key"`
	APIKey           types.String `tfsdk:"api_key"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
}

// New returns a provider factory for the given version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AnthropicProvider{version: version}
	}
}

func (p *AnthropicProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anthropic"
	resp.Version = p.version
}

func (p *AnthropicProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage an Anthropic organization (workspaces, members, invites, API keys, " +
			"service accounts, workload identity federation, customer-managed encryption keys) and a Claude " +
			"Enterprise organization (users, invites, RBAC groups, spend limits) through the Admin API.\n\n" +
			"Three credential classes exist. Each resource documents which one it needs.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				MarkdownDescription: "API base URL. Defaults to `https://api.anthropic.com`. Can also be set with `ANTHROPIC_BASE_URL`.",
				Optional:            true,
			},
			"admin_api_key": schema.StringAttribute{
				MarkdownDescription: "Admin API key (`sk-ant-admin...`) created in the Claude Console. Reaches Console-organization " +
					"endpoints: organization, users, invites, workspaces, workspace members, API keys, external keys, rate limits. " +
					"Can also be set with `ANTHROPIC_ADMIN_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
			"oauth_token": schema.StringAttribute{
				MarkdownDescription: "An `org:admin` OAuth bearer token. Required for service accounts, federation issuers, " +
					"federation rules and workspace service-account memberships; when set it is also used in place of " +
					"`admin_api_key`. Can also be set with `ANTHROPIC_AUTH_TOKEN`.",
				Optional:  true,
				Sensitive: true,
			},
			"enterprise_api_key": schema.StringAttribute{
				MarkdownDescription: "Claude Enterprise scoped API key created by a claude.ai organization primary owner. " +
					"Required for RBAC groups, RBAC roles and spend limits. Also used for the Compliance directory and " +
					"analytics data sources when the dedicated keys below are not set and this key carries the scopes. " +
					"Can also be set with `ANTHROPIC_ENTERPRISE_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
			"compliance_api_key": schema.StringAttribute{
				MarkdownDescription: "Compliance Access Key created in claude.ai (Data and privacy). Used by the `anthropic_compliance_*` " +
					"directory data sources. Falls back to `enterprise_api_key`. Can also be set with `ANTHROPIC_COMPLIANCE_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Regular workspace API key (`sk-ant-api03-...`) for the Managed Agents control plane and the Skills API " +
					"(`anthropic_agent`, `anthropic_environment`, `anthropic_vault*`, `anthropic_deployment`, `anthropic_memory_store`, " +
					"`anthropic_skill`). Admin keys are rejected by those endpoints. Can also be set with `ANTHROPIC_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace to scope Managed Agents and Skills calls to (`wrkspc_...`), sent as the `anthropic-workspace-id` " +
					"header. Required when `api_key` can access more than one workspace. Can also be set with `ANTHROPIC_WORKSPACE_ID`.",
				Optional: true,
			},
			"analytics_api_key": schema.StringAttribute{
				MarkdownDescription: "Claude Enterprise Analytics API key (`read:analytics`). Used by the `anthropic_analytics_*` data " +
					"sources. Falls back to `enterprise_api_key`. Can also be set with `ANTHROPIC_ANALYTICS_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *AnthropicProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for name, v := range map[string]types.String{
		"base_url":           cfg.BaseURL,
		"admin_api_key":      cfg.AdminAPIKey,
		"oauth_token":        cfg.OAuthToken,
		"enterprise_api_key": cfg.EnterpriseAPIKey,
		"compliance_api_key": cfg.ComplianceAPIKey,
		"analytics_api_key":  cfg.AnalyticsAPIKey,
		"api_key":            cfg.APIKey,
		"workspace_id":       cfg.WorkspaceID,
	} {
		if v.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unknown provider configuration value",
				fmt.Sprintf("The provider cannot be configured while %q is unknown. Set a static value or resolve the dependency first.", name))
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := stringOrEnv(cfg.BaseURL, "ANTHROPIC_BASE_URL")
	admin := stringOrEnv(cfg.AdminAPIKey, "ANTHROPIC_ADMIN_API_KEY")
	oauth := stringOrEnv(cfg.OAuthToken, "ANTHROPIC_AUTH_TOKEN")
	enterprise := stringOrEnv(cfg.EnterpriseAPIKey, "ANTHROPIC_ENTERPRISE_API_KEY")
	compliance := stringOrEnv(cfg.ComplianceAPIKey, "ANTHROPIC_COMPLIANCE_API_KEY")
	analytics := stringOrEnv(cfg.AnalyticsAPIKey, "ANTHROPIC_ANALYTICS_API_KEY")
	apiKey := stringOrEnv(cfg.APIKey, "ANTHROPIC_API_KEY")
	workspaceID := stringOrEnv(cfg.WorkspaceID, "ANTHROPIC_WORKSPACE_ID")

	if admin == "" && oauth == "" && enterprise == "" && compliance == "" && analytics == "" && apiKey == "" {
		resp.Diagnostics.AddError("Missing credentials",
			"Set at least one of admin_api_key (ANTHROPIC_ADMIN_API_KEY), oauth_token (ANTHROPIC_AUTH_TOKEN), "+
				"enterprise_api_key (ANTHROPIC_ENTERPRISE_API_KEY), compliance_api_key (ANTHROPIC_COMPLIANCE_API_KEY), "+
				"analytics_api_key (ANTHROPIC_ANALYTICS_API_KEY) or api_key (ANTHROPIC_API_KEY).")
		return
	}

	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "admin_api_key", "oauth_token", "enterprise_api_key", "compliance_api_key", "analytics_api_key", "api_key")

	c, err := client.New(client.Config{
		BaseURL:          baseURL,
		AdminAPIKey:      admin,
		OAuthToken:       oauth,
		EnterpriseAPIKey: enterprise,
		ComplianceAPIKey: compliance,
		AnalyticsAPIKey:  analytics,
		APIKey:           apiKey,
		WorkspaceID:      workspaceID,
		UserAgent:        fmt.Sprintf("terraform-provider-anthropic-enterprise/%s (terraform %s)", p.version, req.TerraformVersion),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create API client", err.Error())
		return
	}

	tflog.Info(ctx, "configured anthropic provider", map[string]any{
		"base_url":       c.BaseURL(),
		"has_admin":      c.HasAdmin(),
		"has_oauth":      c.HasOAuth(),
		"has_enterprise": c.HasEnterprise(),
		"has_compliance": c.HasCompliance(),
		"has_analytics":  c.HasAnalytics(),
		"has_api_key":    c.HasAPIKey(),
	})

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *AnthropicProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources
}

func (p *AnthropicProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return dataSources
}

// resources and dataSources are appended to by init() functions in each
// *_resource.go / *_data_source.go file so a new file registers itself.
var (
	resources   []func() resource.Resource
	dataSources []func() datasource.DataSource
)

func registerResource(f func() resource.Resource)       { resources = append(resources, f) }
func registerDataSource(f func() datasource.DataSource) { dataSources = append(dataSources, f) }

func stringOrEnv(v types.String, env string) string {
	if !v.IsNull() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(env)
}
