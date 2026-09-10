package provider

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// Ensure the provider satisfies the framework interfaces.
var _ provider.Provider = &omniProvider{}

type omniProvider struct {
	version string
}

// New returns a provider factory for the given version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &omniProvider{version: version}
	}
}

type omniProviderModel struct {
	BaseURL        types.String `tfsdk:"base_url"`
	APIToken       types.String `tfsdk:"api_token"`
	TimeoutSeconds types.Int64  `tfsdk:"timeout_seconds"`
}

func (p *omniProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "omni"
	resp.Version = p.version
}

func (p *omniProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Omni Analytics resources through the Omni REST API.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Your Omni instance URL, for example `https://blobsrus.omniapp.co`. " +
					"A trailing `/api` is added automatically. Falls back to the `OMNI_BASE_URL` environment variable.",
			},
			"api_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "An Omni API token. Organization API keys are required for user and user group " +
					"management; personal access tokens work for most other resources. Falls back to the " +
					"`OMNI_API_TOKEN` environment variable, then `OMNI_API_KEY`.",
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Per-request timeout in seconds. Defaults to `60`, or the value of the " +
					"`OMNI_TIMEOUT_SECONDS` environment variable.",
			},
		},
	}
}

func (p *omniProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config omniProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("base_url"),
			"Unknown Omni base URL",
			"base_url cannot be determined at plan time. Set it to a static value or use the OMNI_BASE_URL environment variable.",
		)
	}
	if config.APIToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("api_token"),
			"Unknown Omni API token",
			"api_token cannot be determined at plan time. Set it to a static value or use the OMNI_API_TOKEN environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := firstNonEmpty(config.BaseURL.ValueString(), os.Getenv("OMNI_BASE_URL"))
	token := firstNonEmpty(config.APIToken.ValueString(), os.Getenv("OMNI_API_TOKEN"), os.Getenv("OMNI_API_KEY"))

	if baseURL == "" {
		resp.Diagnostics.AddAttributeError(
			pathRoot("base_url"),
			"Missing Omni base URL",
			"Set the base_url provider attribute or the OMNI_BASE_URL environment variable.",
		)
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			pathRoot("api_token"),
			"Missing Omni API token",
			"Set the api_token provider attribute or the OMNI_API_TOKEN environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	timeout := 60 * time.Second
	if !config.TimeoutSeconds.IsNull() {
		timeout = time.Duration(config.TimeoutSeconds.ValueInt64()) * time.Second
	} else if env := os.Getenv("OMNI_TIMEOUT_SECONDS"); env != "" {
		if secs, err := strconv.Atoi(env); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}

	c, err := client.NewClient(
		baseURL,
		token,
		client.WithUserAgent("terraform-provider-omni/"+p.version),
		client.WithTimeout(timeout),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure the Omni API client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *omniProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserResource,
		NewUserGroupResource,
		NewUserModelRoleResource,
		NewUserGroupModelRoleResource,
		NewConnectionResource,
		NewFolderResource,
		NewModelResource,
	}
}

func (p *omniProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewUserDataSource,
		NewUserGroupDataSource,
		NewConnectionDataSource,
		NewModelDataSource,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
