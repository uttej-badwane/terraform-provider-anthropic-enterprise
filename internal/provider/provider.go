package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	RequestTimeout   types.String `tfsdk:"request_timeout"`
	MaxRetries       types.Int64  `tfsdk:"max_retries"`
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
				MarkdownDescription: "API base URL. Defaults to `https://api.anthropic.com`. Must use `https`; `http` is accepted only for " +
					"loopback addresses such as the bundled mock server. Credentials embedded in the URL are rejected. " +
					"Can also be set with `ANTHROPIC_BASE_URL`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^https?://[^@/]+(/.*)?$`), "must be an absolute http(s) URL without embedded credentials"),
				},
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
			"request_timeout": schema.StringAttribute{
				MarkdownDescription: "How long to wait for a single API request, as a Go duration such as `90s` or `2m`. " +
					"Defaults to `60s`. Applies per attempt, so a run that retries can take longer than this in total. " +
					"Can also be set with `ANTHROPIC_REQUEST_TIMEOUT`.",
				Optional: true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "How many times to retry a request the API could not serve, such as a 429 or a 5xx. " +
					"Defaults to `4`. Set `0` to attempt each request once and fail fast, which is often what a CI run wants. " +
					"Can also be set with `ANTHROPIC_MAX_RETRIES`.",
				Optional:   true,
				Validators: []validator.Int64{int64validator.AtLeast(0)},
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

// credentialMismatchSummary heads the diagnostic raised when a credential
// belongs to a different class than the attribute holding it.
const credentialMismatchSummary = "Credential of the wrong class" //nolint:gosec // diagnostic title, not a credential

// credentialMismatch describes one credential attribute and a key prefix that
// certainly belongs to a different class.
type credentialMismatch struct {
	attribute string
	env       string
	value     string
	set       bool
	rejects   string
	detail    string
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
		"request_timeout":    cfg.RequestTimeout,
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

	requestTimeout, maxRetries := tuningOrEnv(cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if admin == "" && oauth == "" && enterprise == "" && compliance == "" && analytics == "" && apiKey == "" {
		resp.Diagnostics.AddError("Missing credentials",
			"Set at least one of admin_api_key (ANTHROPIC_ADMIN_API_KEY), oauth_token (ANTHROPIC_AUTH_TOKEN), "+
				"enterprise_api_key (ANTHROPIC_ENTERPRISE_API_KEY), compliance_api_key (ANTHROPIC_COMPLIANCE_API_KEY), "+
				"analytics_api_key (ANTHROPIC_ANALYTICS_API_KEY) or api_key (ANTHROPIC_API_KEY).")
		return
	}

	// Each credential class has its own key prefix, and an API handed a key of
	// the wrong class answers with a bare 401 that never mentions which
	// attribute is at fault. Catch the two unambiguous swaps here so the error
	// names the attribute and what it expects. Only a prefix that certainly
	// belongs to another class is rejected: refusing a key that would in fact
	// have worked is worse than letting the API answer.
	for _, m := range []credentialMismatch{
		{
			attribute: "api_key",
			env:       "ANTHROPIC_API_KEY",
			value:     apiKey,
			set:       !cfg.APIKey.IsNull(),
			rejects:   "sk-ant-admin",
			detail: "`api_key` expects a workspace API key (`sk-ant-api03-...`), but an Admin key was supplied. " +
				"The Managed Agents and Skills APIs reject Admin keys. Use `admin_api_key` for Console-organization " +
				"resources, and `api_key` for agents, environments, vaults, deployments, memory stores and skills.",
		},
		{
			attribute: "admin_api_key",
			env:       "ANTHROPIC_ADMIN_API_KEY",
			value:     admin,
			set:       !cfg.AdminAPIKey.IsNull(),
			rejects:   "sk-ant-api0",
			detail: "`admin_api_key` expects an Admin API key (`sk-ant-admin01-...`), but a regular API key was " +
				"supplied. Console-organization endpoints reject it. Use `api_key` for the Managed Agents and Skills " +
				"APIs, `enterprise_api_key` for Claude Enterprise, and `admin_api_key` for the Console organization.",
		},
	} {
		if m.value == "" || !strings.HasPrefix(m.value, m.rejects) {
			continue
		}
		detail := m.detail
		if !m.set {
			detail += fmt.Sprintf(" This value came from %s.", m.env)
		}
		if m.set {
			resp.Diagnostics.AddAttributeError(path.Root(m.attribute), credentialMismatchSummary, detail)
		} else {
			resp.Diagnostics.AddError(credentialMismatchSummary, detail)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Redact the credential values themselves from anything logged with
	// this context, wherever they appear. Each request context is masked
	// again by the client, since resource contexts do not derive from here.
	for _, s := range []string{admin, oauth, enterprise, compliance, analytics, apiKey} {
		if s != "" {
			ctx = tflog.MaskAllFieldValuesStrings(ctx, s)
			ctx = tflog.MaskMessageStrings(ctx, s)
		}
	}

	c, err := client.New(client.Config{
		BaseURL:          baseURL,
		AdminAPIKey:      admin,
		OAuthToken:       oauth,
		EnterpriseAPIKey: enterprise,
		ComplianceAPIKey: compliance,
		AnalyticsAPIKey:  analytics,
		APIKey:           apiKey,
		WorkspaceID:      workspaceID,
		RequestTimeout:   requestTimeout,
		MaxRetries:       maxRetries,
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
	resp.EphemeralResourceData = c
	resp.ResourceData = c
}

func (p *AnthropicProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources
}

// EphemeralResources returns values that are fetched for a run and never
// written to state or the plan.
func (p *AnthropicProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return ephemeralResources
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

var ephemeralResources []func() ephemeral.EphemeralResource

func registerResource(f func() resource.Resource) { resources = append(resources, f) }
func registerEphemeralResource(f func() ephemeral.EphemeralResource) {
	ephemeralResources = append(ephemeralResources, f)
}
func registerDataSource(f func() datasource.DataSource) { dataSources = append(dataSources, f) }

// tuningOrEnv resolves request_timeout and max_retries from configuration or
// the environment. Both are optional; a nil result means "use the client's
// default". A value that cannot be parsed is an error rather than a silent
// fallback, because silently ignoring a timeout someone set is worse than
// refusing to start.
func tuningOrEnv(cfg providerModel, diags *diag.Diagnostics) (*time.Duration, *int) {
	var timeout *time.Duration
	if raw := stringOrEnv(cfg.RequestTimeout, "ANTHROPIC_REQUEST_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		switch {
		case err != nil:
			diags.AddAttributeError(path.Root("request_timeout"), "Invalid request timeout",
				fmt.Sprintf("%q is not a duration. Use a Go duration such as \"90s\" or \"2m\".", raw))
		case d <= 0:
			diags.AddAttributeError(path.Root("request_timeout"), "Invalid request timeout",
				fmt.Sprintf("%q must be positive.", raw))
		default:
			timeout = &d
		}
	}

	var retries *int
	raw := ""
	if !cfg.MaxRetries.IsNull() {
		raw = strconv.FormatInt(cfg.MaxRetries.ValueInt64(), 10)
	} else {
		raw = os.Getenv("ANTHROPIC_MAX_RETRIES")
	}
	if raw != "" {
		n, err := strconv.Atoi(raw)
		switch {
		case err != nil:
			diags.AddAttributeError(path.Root("max_retries"), "Invalid retry count",
				fmt.Sprintf("%q is not a whole number.", raw))
		case n < 0:
			diags.AddAttributeError(path.Root("max_retries"), "Invalid retry count",
				fmt.Sprintf("%d is negative. Use 0 to attempt each request once.", n))
		default:
			retries = &n
		}
	}
	return timeout, retries
}

func stringOrEnv(v types.String, env string) string {
	if !v.IsNull() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(env)
}
