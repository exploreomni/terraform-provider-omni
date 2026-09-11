package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
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
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	ModelKind            types.String `tfsdk:"model_kind"`
	ConnectionID         types.String `tfsdk:"connection_id"`
	BaseModelID          types.String `tfsdk:"base_model_id"`
	AllowAsWorkbookBase  types.Bool   `tfsdk:"allow_as_workbook_base"`
	UsesIsolatedBranches types.Bool   `tfsdk:"uses_isolated_branches"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func (r *modelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a shared or shared extension model. Destroying the resource archives the " +
			"model, which is recoverable from the trash.\n\n" +
			"Access grants are deliberately not managed here. They are model content, declared in model YAML " +
			"and versioned through git sync. Setting them from Terraform as well would mean two systems " +
			"writing one source of truth.",
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
			"allow_as_workbook_base": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether workbooks may be built on this model. Set at creation only.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"uses_isolated_branches": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "For `SHARED_EXTENSION` models, show branches on the extension model page " +
					"rather than the parent shared model. Set at creation only.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
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
		ConnectionID:         plan.ConnectionID.ValueString(),
		BaseModelID:          plan.BaseModelID.ValueString(),
		ModelKind:            plan.ModelKind.ValueString(),
		ModelName:            plan.Name.ValueString(),
		AllowAsWorkbookBase:  boolPtr(plan.AllowAsWorkbookBase),
		UsesIsolatedBranches: boolPtr(plan.UsesIsolatedBranches),
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

	// allow_as_workbook_base and uses_isolated_branches are never returned by
	// the API, so state keeps whatever was configured.
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

// applyModelToState takes only the fields a response actually carried. Write
// responses are partial across this API, so an absent field must never null out
// something Terraform already knows.
func applyModelToState(state *modelResourceModel, model *client.Model) {
	if model.ID != "" {
		state.ID = types.StringValue(model.ID)
	}
	if model.Name != "" {
		state.Name = types.StringValue(model.Name)
	}
	if model.ModelKind != "" {
		state.ModelKind = types.StringValue(model.ModelKind)
	}
	if model.ConnectionID != "" {
		state.ConnectionID = types.StringValue(model.ConnectionID)
	} else if state.ConnectionID.IsUnknown() {
		state.ConnectionID = types.StringNull()
	}
	if model.BaseModelID != "" {
		state.BaseModelID = types.StringValue(model.BaseModelID)
	} else if state.BaseModelID.IsUnknown() {
		state.BaseModelID = types.StringNull()
	}
	if model.CreatedAt != "" {
		state.CreatedAt = types.StringValue(model.CreatedAt)
	} else if state.CreatedAt.IsUnknown() {
		state.CreatedAt = types.StringNull()
	}
	if model.UpdatedAt != "" {
		state.UpdatedAt = types.StringValue(model.UpdatedAt)
	} else if state.UpdatedAt.IsUnknown() {
		state.UpdatedAt = types.StringNull()
	}
}
