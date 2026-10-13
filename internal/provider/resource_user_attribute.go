package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                     = &userAttributeResource{}
	_ resource.ResourceWithConfigure        = &userAttributeResource{}
	_ resource.ResourceWithImportState      = &userAttributeResource{}
	_ resource.ResourceWithConfigValidators = &userAttributeResource{}
)

// NewUserAttributeResource returns the omni_user_attribute resource.
func NewUserAttributeResource() resource.Resource { return &userAttributeResource{} }

type userAttributeResource struct {
	client *client.Client
}

type userAttributeResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Label          types.String `tfsdk:"label"`
	Description    types.String `tfsdk:"description"`
	Type           types.String `tfsdk:"type"`
	DefaultValue   types.String `tfsdk:"default_value"`
	DefaultValues  types.List   `tfsdk:"default_values"`
	MultipleValues types.Bool   `tfsdk:"multiple_values"`
	System         types.Bool   `tfsdk:"system"`
}

func (r *userAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute"
}

func (r *userAttributeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom user attribute definition. Requires an organization API key.\n\n" +
			"An attribute is what an access filter keys off, so this is usually the first thing a " +
			"row-level security setup needs and the last thing to remove.\n\n" +
			"**Destroying this removes the value every user holds for the attribute.** It also breaks " +
			"model SQL and embed SSO URLs that reference it by name, and any connection selecting an " +
			"environment by its value falls back to the default environment. None of that is reported " +
			"by the destroy, which succeeds.\n\n" +
			"Omni's own attributes (`omni_user_id`, `omni_user_email` and the rest) are system defined " +
			"and cannot be managed here. Use the `omni_user_attribute` data source to reference one.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The definition's unique ID. Reference attributes by this rather than by name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Reference name used in model SQL and in embed SSO URL parameters. Must " +
					"start with a letter, hold only letters, numbers and underscores, and must not start " +
					"with `omni_`.\n\n" +
					"Renaming is allowed and happens in place, but every reference to the old name breaks: " +
					"model SQL, embed URLs and anything else naming it. Terraform cannot see those, so a " +
					"rename here is a change to make deliberately.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"label": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Display name shown in the Omni UI. Unique among custom attributes. Must " +
					"not start with `Omni`, and must not be one of the reserved connection labels: `Host`, " +
					"`Password`, `Database`, `Port`, `Schema`, `Connection Name`.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "What the attribute is for. An empty string is stored as null.",
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "`String` or `Number`. Immutable: changing it replaces the attribute, " +
					"which destroys every user's value for it.",
				Validators:    []validator.String{stringvalidator.OneOf("String", "Number")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"multiple_values": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Whether the attribute holds more than one value per user.\n\n" +
					"`false` to `true` is an update. `true` to `false` is not: the API refuses it, so " +
					"Terraform replaces the attribute instead, which destroys every user's value. The plan " +
					"shows that replacement rather than failing during apply.",
				PlanModifiers: []planmodifier.Bool{narrowingRequiresReplace{}},
			},
			"default_value": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Value used when a user has none set, for a single-valued attribute. " +
					"Conflicts with `default_values`.\n\n" +
					"A `Number` attribute takes its default here as a string, which is how Omni stores " +
					"numbers so that values beyond 2^53 - 1 keep their precision.",
			},
			"default_values": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Values used when a user has none set, for an attribute with " +
					"`multiple_values = true`. Conflicts with `default_value`.",
			},
			"system": schema.BoolAttribute{
				Computed: true,
				MarkdownDescription: "Always false here. System attributes are Omni's own and cannot be " +
					"created or managed.",
			},
		},
	}
}

func (r *userAttributeResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		// Both would be ambiguous: the API takes one default_value whose shape
		// follows multiple_values, not two separate fields.
		resourcevalidator.Conflicting(pathExpr("default_value"), pathExpr("default_values")),
	}
}

func (r *userAttributeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

// input converts the plan into a client input, reporting the config errors the
// API would otherwise return as a 400 during apply.
func (r *userAttributeResource) input(ctx context.Context, plan userAttributeResourceModel, diags *diagnostics) client.UserAttributeInput {
	in := client.UserAttributeInput{
		Name:           plan.Name.ValueString(),
		Label:          plan.Label.ValueString(),
		Type:           plan.Type.ValueString(),
		MultipleValues: plan.MultipleValues.ValueBool(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		d := plan.Description.ValueString()
		in.Description = &d
	}

	switch {
	case !plan.DefaultValues.IsNull() && !plan.DefaultValues.IsUnknown():
		var values []string
		diags.Append(plan.DefaultValues.ElementsAs(ctx, &values, false)...)
		if !in.MultipleValues {
			diags.AddError(
				"default_values needs multiple_values",
				"default_values sets more than one default, which only an attribute with "+
					"multiple_values = true can hold. Set multiple_values = true, or use default_value.",
			)
		}
		if len(values) == 0 {
			diags.AddError(
				"default_values is empty",
				"An empty list is not the same as no default and the API rejects it. Omit "+
					"default_values entirely for an attribute with no default.",
			)
		}
		in.Default = client.AttributeDefault{Values: values, Set: true}

	case !plan.DefaultValue.IsNull() && !plan.DefaultValue.IsUnknown():
		if in.MultipleValues {
			diags.AddError(
				"default_value cannot be used with multiple_values",
				"This attribute holds more than one value, so its default is a list. Use "+
					"default_values instead.",
			)
		}
		in.Default = client.AttributeDefault{Values: []string{plan.DefaultValue.ValueString()}, Set: true}
	}

	return in
}

// apply writes an API response back to state, keeping each default in the
// attribute the configuration used so a plan does not show a permanent diff.
func applyUserAttributeToState(ctx context.Context, state *userAttributeResourceModel, attr *client.UserAttribute, diags *diagnostics) {
	state.ID = types.StringValue(attr.ID)
	state.Name = types.StringValue(attr.Name)
	state.Label = types.StringValue(attr.Label)
	state.Description = stringOrNull(ptrString(attr.Description))
	state.Type = types.StringValue(attr.Type)
	state.MultipleValues = types.BoolValue(attr.MultipleValues)
	state.System = types.BoolValue(attr.System)

	if attr.MultipleValues {
		state.DefaultValue = types.StringNull()
		state.DefaultValues = defaultList(ctx, attr.DefaultValue, diags)
		return
	}
	state.DefaultValues = types.ListNull(types.StringType)
	state.DefaultValue = singleDefault(attr.DefaultValue)
}

func (r *userAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	in := r.input(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateUserAttribute(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni user attribute", err.Error())
		return
	}

	state := plan
	applyUserAttributeToState(ctx, &state, created, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	// There is no get-by-id for attributes, so this filters the list endpoint.
	attr, err := r.client.FindUserAttributeByID(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni user attribute", err.Error())
		return
	}

	applyUserAttributeToState(ctx, &state, attr, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userAttributeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	in := r.input(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateUserAttribute(ctx, state.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni user attribute", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	applyUserAttributeToState(ctx, &next, updated, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *userAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAttributeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteUserAttribute(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni user attribute", err.Error())
	}
}

func (r *userAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

// narrowingRequiresReplace makes multiple_values force replacement only when it
// goes from true to false.
//
// The API allows widening a single-valued attribute to multiple and refuses the
// reverse. A plain RequiresReplace would destroy the attribute on a widening
// that the API would have done in place, and no plan modifier at all would let
// the plan promise an update that fails at apply. This reports the replacement
// in the plan, where it can be seen before it happens.
type narrowingRequiresReplace struct{}

func (narrowingRequiresReplace) Description(_ context.Context) string {
	return "Replaces the attribute when multiple_values goes from true to false, which the API refuses to do in place."
}

func (m narrowingRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (narrowingRequiresReplace) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return // creating or destroying, nothing to compare
	}
	if req.StateValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if req.StateValue.ValueBool() && !req.PlanValue.ValueBool() {
		resp.RequiresReplace = true
		resp.Diagnostics.Append(diag.NewAttributeWarningDiagnostic(
			req.Path,
			"Narrowing multiple_values replaces the attribute",
			"Omni cannot turn a multi-valued attribute back into a single-valued one, so Terraform "+
				"will destroy and recreate it. That removes the value every user holds for this "+
				"attribute, and breaks model SQL and embed URLs referencing it until the new one exists.",
		))
	}
}
