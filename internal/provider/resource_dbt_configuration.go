package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &dbtConfigurationResource{}
	_ resource.ResourceWithConfigure   = &dbtConfigurationResource{}
	_ resource.ResourceWithImportState = &dbtConfigurationResource{}
)

// NewDbtConfigurationResource returns the omni_dbt_configuration resource.
func NewDbtConfigurationResource() resource.Resource { return &dbtConfigurationResource{} }

type dbtConfigurationResource struct {
	client *client.Client
}

type dbtConfigurationResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ConnectionID types.String `tfsdk:"connection_id"`

	SSHURL types.String `tfsdk:"ssh_url"`
	Branch types.String `tfsdk:"branch"`

	AuthMethod              types.String `tfsdk:"auth_method"`
	Token                   types.String `tfsdk:"token"`
	GithubAppInstallationID types.String `tfsdk:"github_app_installation_id"`
	RotateKeys              types.Bool   `tfsdk:"rotate_keys"`

	AutogenRelationships types.Bool `tfsdk:"autogen_relationships"`
	EnableVirtualSchemas types.Bool `tfsdk:"enable_virtual_schemas"`
	EnableSemanticLayer  types.Bool `tfsdk:"enable_semantic_layer"`

	DbtVersion      types.String `tfsdk:"dbt_version"`
	ProjectRootPath types.String `tfsdk:"project_root_path"`
	CommitterName   types.String `tfsdk:"committer_name"`
	CommitterEmail  types.String `tfsdk:"committer_email"`

	SupportsDbt            types.Bool   `tfsdk:"supports_dbt"`
	PublicKey              types.String `tfsdk:"public_key"`
	CommitSigningPublicKey types.String `tfsdk:"commit_signing_public_key"`
}

func (r *dbtConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbt_configuration"
}

func (r *dbtConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a dbt project to a connection. There is at most one of these per " +
			"connection: the API exposes a single `PUT`, with no separate create, so applying this " +
			"resource to a connection that already has dbt configured replaces that configuration " +
			"rather than failing.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Same as `connection_id`. The configuration has no identifier of its " +
					"own because it is a property of the connection.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The connection the dbt project is attached to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_url": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Git URL of the dbt project. Despite the name the API uses, this " +
					"carries an HTTPS URL when `auth_method` is `https_token`.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"branch": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Branch Omni reads the dbt project from.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"auth_method": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "How Omni authenticates to the repository: `ssh`, `https_token` or " +
					"`github_app`. `https_token` requires `token`, and `github_app` requires " +
					"`github_app_installation_id`.",
				Validators: []validator.String{stringvalidator.OneOf("ssh", "https_token", "github_app")},
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Deploy token or personal access token, required when `auth_method` is " +
					"`https_token`. Never returned by the API, so drift on this value is not detected.",
				Validators: []validator.String{stringvalidator.LengthAtMost(1000)},
			},
			"github_app_installation_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Numeric GitHub App installation ID, required when `auth_method` is " +
					"`github_app`.",
			},
			"rotate_keys": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Set to `true` to have Omni generate a new SSH key pair on the next " +
					"apply. Only meaningful for `ssh`. This is an instruction rather than a setting: the " +
					"API does not return it, so Terraform cannot tell whether a rotation has happened, " +
					"and leaving it `true` rotates on every apply.",
			},
			"autogen_relationships": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Generate Omni relationships from the dbt project's declared " +
					"relationships.",
			},
			"enable_virtual_schemas": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Allow dbt environments to route queries to a different target schema.",
			},
			"enable_semantic_layer": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Import the dbt semantic layer alongside the models.",
			},
			"dbt_version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "dbt version Omni parses the project with. `Auto` tracks the version " +
					"declared in the project.",
				Validators: []validator.String{stringvalidator.OneOf("Auto", "1.10", "1.11")},
			},
			"project_root_path": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Path to the dbt project within the repository, for a monorepo. " +
					"Leave unset when `dbt_project.yml` is at the repository root.",
			},
			"committer_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Name Omni commits as when it writes back to the repository.",
			},
			"committer_email": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Email Omni commits as when it writes back to the repository.",
			},
			"supports_dbt": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the connection's dialect supports dbt at all.",
			},
			"public_key": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Deploy key to add to the repository, returned when `auth_method` is " +
					"`ssh`. Add this to the repository before the first schema refresh, or Omni cannot " +
					"clone it.",
			},
			"commit_signing_public_key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Public key Omni signs its commits with, returned when `auth_method` is `ssh`.",
			},
		},
	}
}

func (r *dbtConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *dbtConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dbtConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	configuration, err := r.client.SetDbtConfiguration(ctx, plan.ConnectionID.ValueString(), dbtConfigurationInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure dbt on the Omni connection", err.Error())
		return
	}

	state := plan
	applyDbtConfigurationToState(&state, configuration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dbtConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dbtConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	configuration, err := r.client.GetDbtConfiguration(ctx, state.ConnectionID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the dbt configuration on the Omni connection", err.Error())
		return
	}

	// A connection with no dbt configuration answers 200 with a message rather
	// than 404, so an absent configuration has to be detected from the body.
	if !configuration.Configured() {
		resp.State.RemoveResource(ctx)
		return
	}

	applyDbtConfigurationToState(&state, configuration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dbtConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dbtConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	configuration, err := r.client.SetDbtConfiguration(ctx, state.ConnectionID.ValueString(), dbtConfigurationInput(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the dbt configuration on the Omni connection", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	next.ConnectionID = state.ConnectionID
	applyDbtConfigurationToState(&next, configuration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *dbtConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dbtConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.DeleteDbtConfiguration(ctx, state.ConnectionID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to remove the dbt configuration from the Omni connection", err.Error())
	}
}

// ImportState takes the connection ID, since the configuration has no ID of its
// own. The token is not importable: the API never returns it, so a configuration
// using https_token has to have it supplied in config after import.
func (r *dbtConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("connection_id"), req.ID)
}

func dbtConfigurationInput(plan dbtConfigurationResourceModel) client.DbtConfigurationInput {
	return client.DbtConfigurationInput{
		AutogenRelationships: plan.AutogenRelationships.ValueBool(),
		Branch:               plan.Branch.ValueString(),
		EnableVirtualSchemas: plan.EnableVirtualSchemas.ValueBool(),
		SSHURL:               plan.SSHURL.ValueString(),

		AuthMethod:              stringPtr(plan.AuthMethod),
		CommitterEmail:          stringPtr(plan.CommitterEmail),
		CommitterName:           stringPtr(plan.CommitterName),
		DbtVersion:              stringPtr(plan.DbtVersion),
		EnableSemanticLayer:     boolPtr(plan.EnableSemanticLayer),
		GithubAppInstallationID: stringPtr(plan.GithubAppInstallationID),
		ProjectRootPath:         stringPtr(plan.ProjectRootPath),
		RotateKeys:              boolPtr(plan.RotateKeys),
		Token:                   stringPtr(plan.Token),
	}
}

// applyDbtConfigurationToState takes only what the response carried. The PUT
// response omits whatever the chosen auth method does not use, and never
// includes the token, so an unconditional copy would blank fields the config
// set.
func applyDbtConfigurationToState(state *dbtConfigurationResourceModel, d *client.DbtConfiguration) {
	state.ID = state.ConnectionID

	if d.SSHURL != "" {
		state.SSHURL = types.StringValue(d.SSHURL)
	}
	if d.Branch != "" {
		state.Branch = types.StringValue(d.Branch)
	}
	// These are Optional+Computed because the API supplies them when the
	// configuration does not: dbtVersion always comes back, as "Auto" by
	// default, and the rest depend on the auth method. keepKnown takes what
	// the response carried, keeps a configured value the response omitted,
	// and resolves an unconfigured, unreturned value to null rather than
	// leaving it unknown after apply.
	state.AuthMethod = keepKnown(state.AuthMethod, d.AuthMethod)
	state.GithubAppInstallationID = keepKnown(state.GithubAppInstallationID, d.GithubAppInstallationID)
	state.CommitterName = keepKnown(state.CommitterName, d.CommitterName)
	state.CommitterEmail = keepKnown(state.CommitterEmail, d.CommitterEmail)
	dbtVersion := ""
	if d.DbtVersion != nil {
		dbtVersion = *d.DbtVersion
	}
	state.DbtVersion = keepKnown(state.DbtVersion, dbtVersion)
	if d.ProjectRootPath != nil && *d.ProjectRootPath != "" {
		state.ProjectRootPath = types.StringValue(*d.ProjectRootPath)
	}

	state.AutogenRelationships = types.BoolValue(d.AutogenRelationships)
	state.EnableVirtualSchemas = types.BoolValue(d.EnableVirtualSchemas)
	state.EnableSemanticLayer = types.BoolValue(d.EnableSemanticLayer)
	state.SupportsDbt = types.BoolValue(d.SupportsDbt)

	state.PublicKey = stringOrNull(d.PublicKey)
	state.CommitSigningPublicKey = stringOrNull(d.CommitSigningPublicKey)
}
