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

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &labelResource{}
	_ resource.ResourceWithConfigure   = &labelResource{}
	_ resource.ResourceWithImportState = &labelResource{}
)

// NewLabelResource returns the omni_label resource.
func NewLabelResource() resource.Resource { return &labelResource{} }

type labelResource struct {
	client *client.Client
}

type labelResourceModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Color       types.String  `tfsdk:"color"`
	Description types.String  `tfsdk:"description"`
	Homepage    types.Bool    `tfsdk:"homepage"`
	Verified    types.Bool    `tfsdk:"verified"`
	UsageCount  types.Float64 `tfsdk:"usage_count"`
}

func (r *labelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *labelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a content label.\n\n" +
			"This manages the label itself, not its use. Applying a label to a document or folder is part of " +
			"authoring that content and is deliberately left to the UI. What belongs in configuration is the " +
			"set of labels an instance has, so the taxonomy is the same in every environment and additions " +
			"get reviewed.\n\n" +
			"Deleting a label does not delete the documents carrying it; they simply lose the label.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The label's name. Labels are addressed by name rather than an opaque ID, " +
					"so this tracks `name`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(2, 25)},
				MarkdownDescription: "Label name, 2 to 25 characters. Renaming updates the label in place.",
			},
			"color": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(9)},
				MarkdownDescription: "Hex colour, for example `#0366d6`.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(500)},
				MarkdownDescription: "What the label means and when to use it. Up to 500 characters.",
			},
			"homepage": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Show the label on the homepage. Requires admin permissions, so a " +
					"non-admin token gets a 403 when setting this.",
			},
			"verified": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Mark the label as verified. Requires admin permissions.",
			},
			"usage_count": schema.Float64Attribute{
				Computed: true,
				MarkdownDescription: "How many documents carry this label. Changes as people use it, so " +
					"expect it to move between plans.",
			},
		},
	}
}

func (r *labelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *labelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan labelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	created, err := r.client.CreateLabel(ctx, labelInputFrom(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni label", err.Error())
		return
	}

	state := plan
	applyLabelToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *labelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	label, err := r.client.GetLabel(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni label", err.Error())
		return
	}

	applyLabelToState(&state, label)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *labelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state labelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	// The path carries the current name; the body carries the new one.
	updated, err := r.client.UpdateLabel(ctx, state.ID.ValueString(), labelInputFrom(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni label", err.Error())
		return
	}

	next := plan
	applyLabelToState(&next, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *labelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state labelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteLabel(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni label", err.Error())
	}
}

func (r *labelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("name"), req.ID)
}

func labelInputFrom(plan labelResourceModel) client.LabelInput {
	return client.LabelInput{
		Name:        plan.Name.ValueString(),
		Color:       stringPtr(plan.Color),
		Description: stringPtr(plan.Description),
		Homepage:    boolPtr(plan.Homepage),
		Verified:    boolPtr(plan.Verified),
	}
}

// applyLabelToState takes only what a response carried, following the rule the
// rest of this provider learned the hard way: writes return partial objects.
func applyLabelToState(state *labelResourceModel, label *client.Label) {
	if label.Name != "" {
		state.ID = types.StringValue(label.Name)
		state.Name = types.StringValue(label.Name)
	}
	if label.Color != nil && *label.Color != "" {
		state.Color = types.StringValue(*label.Color)
	}
	if label.Description != nil && *label.Description != "" {
		state.Description = types.StringValue(*label.Description)
	}
	state.Homepage = types.BoolValue(label.Homepage)
	state.Verified = types.BoolValue(label.Verified)
	state.UsageCount = types.Float64Value(label.UsageCount)
}
