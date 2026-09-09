package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &modelResource{}
	_ resource.ResourceWithConfigure   = &modelResource{}
	_ resource.ResourceWithImportState = &modelResource{}
)

// NewModelResource returns the omni_model resource.
func NewModelResource() resource.Resource { return &modelResource{} }

type modelResource struct {
	client *client.Client
}

type modelResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	ModelKind    types.String `tfsdk:"model_kind"`
	ConnectionID types.String `tfsdk:"connection_id"`
	BaseModelID  types.String `tfsdk:"base_model_id"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *modelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a shared or shared extension model. Destroying the resource archives the " +
			"model, which is recoverable from the trash.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The model's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The model name. Must be unique among active models with the same kind and base model.",
			},
			"model_kind": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("SHARED_EXTENSION"),
				Validators: []validator.String{
					stringvalidator.OneOf("SCHEMA", "SHARED", "SHARED_EXTENSION"),
				},
				MarkdownDescription: "The kind of model to create. Defaults to `SHARED_EXTENSION`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"connection_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The connection the model is built on. Required unless `base_model_id` is set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"base_model_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The model this one extends, for `SHARED_EXTENSION` models.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the model was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the model was last updated.",
			},
		},
	}
}

func (r *modelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *modelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if plan.ConnectionID.IsNull() && plan.BaseModelID.IsNull() {
		resp.Diagnostics.AddError(
			"Missing model source",
			"Set connection_id, base_model_id, or both when creating a model.",
		)
		return
	}

	created, err := r.client.CreateModel(ctx, client.ModelInput{
		ConnectionID: plan.ConnectionID.ValueString(),
		BaseModelID:  plan.BaseModelID.ValueString(),
		ModelKind:    plan.ModelKind.ValueString(),
		ModelName:    plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni model", err.Error())
		return
	}

	state := plan
	applyModelToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	model, err := r.client.GetModel(ctx, state.ID.ValueString(), state.ModelKind.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni model", err.Error())
		return
	}
	if model.DeletedAt != "" {
		resp.State.RemoveResource(ctx)
		return
	}

	applyModelToState(&state, model)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state modelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	next := plan
	next.ID = state.ID

	if !plan.Name.Equal(state.Name) {
		renamed, err := r.client.RenameModel(ctx, state.ID.ValueString(), plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to rename Omni model", err.Error())
			return
		}
		applyModelToState(&next, renamed)
	}

	// Carry over any computed values Terraform left unknown in the plan.
	if next.CreatedAt.IsUnknown() {
		next.CreatedAt = state.CreatedAt
	}
	if next.UpdatedAt.IsUnknown() {
		next.UpdatedAt = state.UpdatedAt
	}
	if next.ConnectionID.IsUnknown() {
		next.ConnectionID = state.ConnectionID
	}
	if next.BaseModelID.IsUnknown() {
		next.BaseModelID = state.BaseModelID
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *modelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state modelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteModel(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to archive Omni model", err.Error())
	}
}

func (r *modelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

func applyModelToState(state *modelResourceModel, model *client.Model) {
	state.ID = types.StringValue(model.ID)
	if model.Name != "" {
		state.Name = types.StringValue(model.Name)
	}
	if model.ModelKind != "" {
		state.ModelKind = types.StringValue(model.ModelKind)
	}
	state.ConnectionID = stringOrNull(model.ConnectionID)
	state.BaseModelID = stringOrNull(model.BaseModelID)
	state.CreatedAt = stringOrNull(model.CreatedAt)
	state.UpdatedAt = stringOrNull(model.UpdatedAt)
}

// ---------------------------------------------------------------------------
// omni_model_yaml_file
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                = &modelYAMLFileResource{}
	_ resource.ResourceWithConfigure   = &modelYAMLFileResource{}
	_ resource.ResourceWithImportState = &modelYAMLFileResource{}
)

// NewModelYAMLFileResource returns the omni_model_yaml_file resource.
func NewModelYAMLFileResource() resource.Resource { return &modelYAMLFileResource{} }

type modelYAMLFileResource struct {
	client *client.Client
}

type modelYAMLFileResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ModelID       types.String `tfsdk:"model_id"`
	FileName      types.String `tfsdk:"file_name"`
	YAML          types.String `tfsdk:"yaml"`
	Mode          types.String `tfsdk:"mode"`
	BranchID      types.String `tfsdk:"branch_id"`
	CommitMessage types.String `tfsdk:"commit_message"`
	FullyResolved types.Bool   `tfsdk:"fully_resolved"`
	Checksum      types.String `tfsdk:"checksum"`
}

func (r *modelYAMLFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model_yaml_file"
}

func (r *modelYAMLFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single YAML file in a model: `model`, `relationships`, `<name>.topic`, " +
			"`<name>.view`, or `<name>.composite_topic`. Schema models and models in git follower mode cannot " +
			"be edited this way.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite ID in the form `<model id>:<file name>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"model_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The model that owns the file.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"file_name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "File name, for example `model`, `relationships`, `orders.topic`, or " +
					"`orders.view`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"yaml": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The YAML contents of the file.",
			},
			"mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("combined"),
				Validators: []validator.String{
					stringvalidator.OneOf("combined", "extension", "staged", "merged", "history"),
				},
				MarkdownDescription: "Write mode. Defaults to `combined`. Workbook models must use `combined` when a branch is set.",
			},
			"branch_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Branch to write to. Required when the model requires pull requests.",
			},
			"commit_message": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Commit message. Required for git-enabled models.",
			},
			"fully_resolved": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "When `true`, Omni resolves `extends` usage and writes changes into the " +
					"appropriate model files.",
			},
			"checksum": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Checksum of the file as last read, used for conflict detection.",
			},
		},
	}
}

func (r *modelYAMLFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *modelYAMLFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modelYAMLFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.PutModelYAML(ctx, plan.ModelID.ValueString(), client.YAMLFileInput{
		FileName:      plan.FileName.ValueString(),
		YAML:          plan.YAML.ValueString(),
		Mode:          plan.Mode.ValueString(),
		BranchID:      stringPtr(plan.BranchID),
		CommitMessage: stringPtr(plan.CommitMessage),
		FullyResolved: boolPtr(plan.FullyResolved),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to write Omni model YAML", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(yamlFileID(plan.ModelID.ValueString(), plan.FileName.ValueString()))
	state.Checksum = r.readChecksum(ctx, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelYAMLFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modelYAMLFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	file, err := r.client.GetModelYAMLFile(
		ctx,
		state.ModelID.ValueString(),
		state.FileName.ValueString(),
		state.Mode.ValueString(),
		state.BranchID.ValueString(),
	)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni model YAML", err.Error())
		return
	}

	if file.Content != "" {
		state.YAML = types.StringValue(file.Content)
	}
	state.Checksum = stringOrNull(file.Checksum)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelYAMLFileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state modelYAMLFileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	input := client.YAMLFileInput{
		FileName:      plan.FileName.ValueString(),
		YAML:          plan.YAML.ValueString(),
		Mode:          plan.Mode.ValueString(),
		BranchID:      stringPtr(plan.BranchID),
		CommitMessage: stringPtr(plan.CommitMessage),
		FullyResolved: boolPtr(plan.FullyResolved),
	}
	if !state.Checksum.IsNull() {
		input.PreviousChecksum = stringPtr(state.Checksum)
	}

	if err := r.client.PutModelYAML(ctx, plan.ModelID.ValueString(), input); err != nil {
		resp.Diagnostics.AddError("Unable to update Omni model YAML", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	next.Checksum = r.readChecksum(ctx, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

// Delete clears the file. With mode "extension" an empty body removes the file
// from the model; in other modes the file is emptied and ignored.
func (r *modelYAMLFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state modelYAMLFileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.PutModelYAML(ctx, state.ModelID.ValueString(), client.YAMLFileInput{
		FileName:      state.FileName.ValueString(),
		YAML:          "",
		Mode:          state.Mode.ValueString(),
		BranchID:      stringPtr(state.BranchID),
		CommitMessage: stringPtr(state.CommitMessage),
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to clear Omni model YAML", err.Error())
	}
}

func (r *modelYAMLFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	modelID, fileName, found := strings.Cut(req.ID, ":")
	if !found || modelID == "" || fileName == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected an ID in the form <model id>:<file name>, got %q", req.ID),
		)
		return
	}

	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("model_id"), modelID)
	resp.State.SetAttribute(ctx, pathRoot("file_name"), fileName)
	resp.State.SetAttribute(ctx, pathRoot("mode"), "combined")
}

func (r *modelYAMLFileResource) readChecksum(ctx context.Context, plan modelYAMLFileResourceModel) types.String {
	file, err := r.client.GetModelYAMLFile(
		ctx,
		plan.ModelID.ValueString(),
		plan.FileName.ValueString(),
		plan.Mode.ValueString(),
		plan.BranchID.ValueString(),
	)
	if err != nil || file.Checksum == "" {
		return types.StringNull()
	}
	return types.StringValue(file.Checksum)
}

func yamlFileID(modelID, fileName string) string {
	return modelID + ":" + fileName
}
