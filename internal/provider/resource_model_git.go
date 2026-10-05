package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &modelGitResource{}
	_ resource.ResourceWithConfigure   = &modelGitResource{}
	_ resource.ResourceWithImportState = &modelGitResource{}
)

// NewModelGitResource returns the omni_model_git resource.
func NewModelGitResource() resource.Resource { return &modelGitResource{} }

type modelGitResource struct {
	client *client.Client
}

type modelGitResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ModelID              types.String `tfsdk:"model_id"`
	CloneURL             types.String `tfsdk:"clone_url"`
	AuthMethod           types.String `tfsdk:"auth_method"`
	BaseBranch           types.String `tfsdk:"base_branch"`
	BranchPerPullRequest types.Bool   `tfsdk:"branch_per_pull_request"`
	RequirePullRequest   types.String `tfsdk:"require_pull_request"`
	GitFollower          types.Bool   `tfsdk:"git_follower"`
	GitServiceProvider   types.String `tfsdk:"git_service_provider"`
	ModelPath            types.String `tfsdk:"model_path"`
	WebURL               types.String `tfsdk:"web_url"`
	SSHURL               types.String `tfsdk:"ssh_url"`

	DeployPrivateKey        types.String `tfsdk:"deploy_private_key"`
	DeployKeyPassphrase     types.String `tfsdk:"deploy_key_passphrase"`
	Token                   types.String `tfsdk:"token"`
	GithubAppInstallationID types.String `tfsdk:"github_app_installation_id"`

	CommitSigningCommitterName  types.String `tfsdk:"commit_signing_committer_name"`
	CommitSigningCommitterEmail types.String `tfsdk:"commit_signing_committer_email"`
}

func (r *modelGitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_git"
}

func (r *modelGitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Connects a model to a git repository.\n\n" +
			"This configures the connection itself: repository, branch, authentication and pull request " +
			"policy. The model content that git then versions is not managed here.\n\n" +
			"Credentials are never returned by the API, so Terraform cannot detect a deploy key or token " +
			"changed outside configuration. They are also stored in plain text in state, so treat state as " +
			"secret material.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tracks `model_id`: a model has at most one git configuration.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"model_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The model to connect.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"clone_url": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Clone URL of the repository. Use SSH (`git@github.com:org/repo.git`) " +
					"with `ssh` auth, HTTPS with `https_token` or `github_app`.",
			},
			"auth_method": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stringvalidator.OneOf("ssh", "https_token", "github_app")},
				MarkdownDescription: "`ssh` for a deploy key, the default. `https_token` for a deploy token " +
					"or PAT. `github_app` for a GitHub App installation, github.com only.",
			},
			"base_branch": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Target branch for Omni pull requests. Defaults to `main`.",
			},
			"branch_per_pull_request": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When `true`, every pull request creates a branch in Omni.",
			},
			"require_pull_request": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stringvalidator.OneOf("always", "users-only", "never")},
				MarkdownDescription: "Whether model changes must go through a pull request. `always` is the " +
					"setting to use where model content is reviewed.",
			},
			"git_follower": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "When `true`, the model follows the repository and cannot be edited in " +
					"Omni. Use this for an environment that mirrors another.",
			},
			"git_service_provider": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{stringvalidator.OneOf(
					"github", "gitlab", "azure_devops", "bitbucket", "bitbucket_datacenter", "auto")},
				MarkdownDescription: "Which git host. `auto` detects it from the clone URL.",
			},
			"model_path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Path within the repository where the model lives.",
			},
			"web_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Browser URL of the repository, used for links back to it.",
			},
			"ssh_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "SSH URL of the repository, where it differs from the clone URL.",
			},
			"deploy_private_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Deploy key private key, for `ssh` auth. Never returned by the API, so " +
					"drift on it cannot be detected.",
			},
			"deploy_key_passphrase": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Passphrase for the deploy key, if it has one.",
			},
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Deploy token or personal access token, for `https_token` auth.",
			},
			"github_app_installation_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "GitHub App installation ID, for `github_app` auth.",
			},
			"commit_signing_committer_name": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Display name written into signed commits. `github_app` auth only, and " +
					"must be set or cleared together with the committer email.",
			},
			"commit_signing_committer_email": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Verified email of the GitHub user owning the signing key. `github_app` " +
					"auth only, and must be set or cleared together with the committer name.",
			},
		},
	}
}

func (r *modelGitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *modelGitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modelGitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	created, err := r.client.CreateModelGit(ctx, plan.ModelID.ValueString(), modelGitInputFrom(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to connect the Omni model to git", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(plan.ModelID.ValueString())
	applyModelGitToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelGitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modelGitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	git, err := r.client.GetModelGit(ctx, state.ModelID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the Omni model git configuration", err.Error())
		return
	}

	applyModelGitToState(&state, git)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelGitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state modelGitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	// Secrets are only sent when they changed, so an unchanged key is not
	// rewritten on every apply.
	in := modelGitInputFrom(plan)
	if plan.DeployPrivateKey.Equal(state.DeployPrivateKey) {
		in.DeployPrivateKey = nil
	}
	if plan.DeployKeyPassphrase.Equal(state.DeployKeyPassphrase) {
		in.DeployKeyPassphrase = nil
	}
	if plan.Token.Equal(state.Token) {
		in.Token = nil
	}

	updated, err := r.client.UpdateModelGit(ctx, state.ModelID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the Omni model git configuration", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	applyModelGitToState(&next, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *modelGitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state modelGitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteModelGit(ctx, state.ModelID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to disconnect the Omni model from git", err.Error())
	}
}

func (r *modelGitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("model_id"), req.ID)
}

func modelGitInputFrom(plan modelGitResourceModel) client.ModelGitInput {
	return client.ModelGitInput{
		CloneURL:             stringPtr(plan.CloneURL),
		SSHURL:               stringPtr(plan.SSHURL),
		WebURL:               stringPtr(plan.WebURL),
		AuthMethod:           stringPtr(plan.AuthMethod),
		BaseBranch:           stringPtr(plan.BaseBranch),
		BranchPerPullRequest: boolPtr(plan.BranchPerPullRequest),
		GitFollower:          boolPtr(plan.GitFollower),
		GitServiceProvider:   stringPtr(plan.GitServiceProvider),
		ModelPath:            stringPtr(plan.ModelPath),
		RequirePullRequest:   stringPtr(plan.RequirePullRequest),

		DeployPrivateKey:        stringPtr(plan.DeployPrivateKey),
		DeployKeyPassphrase:     stringPtr(plan.DeployKeyPassphrase),
		Token:                   stringPtr(plan.Token),
		GithubAppInstallationID: stringPtr(plan.GithubAppInstallationID),

		CommitSigningCommitterName:  stringPtr(plan.CommitSigningCommitterName),
		CommitSigningCommitterEmail: stringPtr(plan.CommitSigningCommitterEmail),
	}
}

// applyModelGitToState takes only what a response carried. Credentials are
// never returned, so they are left as configured.
func applyModelGitToState(state *modelGitResourceModel, g *client.ModelGit) {
	if g.CloneURL != "" {
		state.CloneURL = types.StringValue(g.CloneURL)
	}
	if g.AuthMethod != "" {
		state.AuthMethod = types.StringValue(g.AuthMethod)
	}
	if g.BaseBranch != "" {
		state.BaseBranch = types.StringValue(g.BaseBranch)
	}
	if g.RequirePullRequest != "" {
		state.RequirePullRequest = types.StringValue(g.RequirePullRequest)
	}
	if g.GitServiceProvider != "" {
		state.GitServiceProvider = types.StringValue(g.GitServiceProvider)
	}
	state.BranchPerPullRequest = types.BoolValue(g.BranchPerPullRequest)
	state.GitFollower = types.BoolValue(g.GitFollower)

	for target, value := range map[*types.String]string{
		&state.ModelPath: g.ModelPath,
		&state.WebURL:    g.WebURL,
		&state.SSHURL:    g.SSHURL,
	} {
		if value != "" {
			*target = types.StringValue(value)
		} else if target.IsUnknown() {
			*target = types.StringNull()
		}
	}
}
