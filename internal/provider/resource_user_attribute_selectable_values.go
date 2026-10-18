package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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
	_ resource.Resource                     = &selectableValuesResource{}
	_ resource.ResourceWithConfigure        = &selectableValuesResource{}
	_ resource.ResourceWithImportState      = &selectableValuesResource{}
	_ resource.ResourceWithConfigValidators = &selectableValuesResource{}
)

// NewUserAttributeSelectableValuesResource returns the
// omni_user_attribute_selectable_values resource.
func NewUserAttributeSelectableValuesResource() resource.Resource {
	return &selectableValuesResource{}
}

type selectableValuesResource struct {
	client *client.Client
}

type selectableValuesResourceModel struct {
	ID              types.String `tfsdk:"id"`
	UserAttributeID types.String `tfsdk:"user_attribute_id"`
	UserID          types.String `tfsdk:"user_id"`
	GroupID         types.String `tfsdk:"group_id"`
	Values          types.List   `tfsdk:"values"`
	AllowAnyValue   types.Bool   `tfsdk:"allow_any_value"`
}

func (r *selectableValuesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute_selectable_values"
}

func (r *selectableValuesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets the values a user or group may switch a user attribute to. Requires an " +
			"organization API key.\n\n" +
			"This is not the value a user is assigned. It is the set they can switch between, from the " +
			"File menu of a dashboard, workbook or model. A user's actual value still comes from a " +
			"direct assignment or the definition's default.\n\n" +
			"**Provisioning a value grants data access.** Access filters and shared extensions commonly " +
			"route on the same attribute, so letting someone switch to another tenant's value usually " +
			"lets them read that tenant's rows. List only what the subject should be able to see.\n\n" +
			"Group assignment of a *value* does not exist in Omni; this is the group-scoped control " +
			"that does.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Composite ID in the form `<attribute id>:user:<id>` or " +
					"`<attribute id>:group:<id>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_attribute_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The attribute being provisioned.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_id": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The user to provision for. **This is the membership ID, not the user " +
					"ID.** The two are different values and the API does not say which one it received, " +
					"so passing a user ID here provisions nothing and reports success. Exactly one of " +
					"`user_id` or `group_id`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"group_id": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The group to provision for. **Not `omni_user_group.id`.** This endpoint " +
					"wants the group's UUID primary key, while SCIM - and every other group-scoped endpoint " +
					"in the API, including model roles and folder permissions - uses an eight-character " +
					"`mini_uuid` as the group's id. The two are generated independently, so one cannot be " +
					"derived from the other, and no API returns the UUID. Passing a `mini_uuid` here fails " +
					"with `group_id: Invalid UUID`.\n\n" +
					"Until an endpoint exposes it, a group subject needs a UUID obtained some other way. " +
					"Exactly one of `user_id` or `group_id`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"values": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Values the subject may switch to, validated against the attribute's " +
					"type: a `Number` attribute rejects a value that is not numeric. Numbers are given as " +
					"strings to keep precision beyond 2^53 - 1.\n\n" +
					"At most 300 values of up to 512 characters. For a larger set, use `allow_any_value`.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(300),
					listvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 512)),
				},
			},
			"allow_any_value": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Whether the subject may switch to a value that is not listed. When " +
					"true, `values` are suggestions rather than a limit, and the access this grants is " +
					"bounded only by what the attribute means to the filters using it.",
			},
		},
	}
}

func (r *selectableValuesResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(pathExpr("user_id"), pathExpr("group_id")),
	}
}

func (r *selectableValuesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (m selectableValuesResourceModel) subject() client.SelectableValuesSubject {
	return client.SelectableValuesSubject{
		UserID:  m.UserID.ValueString(),
		GroupID: m.GroupID.ValueString(),
	}
}

func (m selectableValuesResourceModel) compositeID() string {
	if m.UserID.ValueString() != "" {
		return fmt.Sprintf("%s:user:%s", m.UserAttributeID.ValueString(), m.UserID.ValueString())
	}
	return fmt.Sprintf("%s:group:%s", m.UserAttributeID.ValueString(), m.GroupID.ValueString())
}

// write sends the desired state. Create and Update are the same call: the
// endpoint is a PUT that replaces the whole provisioning.
func (r *selectableValuesResource) write(ctx context.Context, plan selectableValuesResourceModel, diags *diagnostics) *client.SelectableValues {
	var values []string
	if !plan.Values.IsNull() && !plan.Values.IsUnknown() {
		diags.Append(plan.Values.ElementsAs(ctx, &values, false)...)
		if diags.HasError() {
			return nil
		}
	}

	out, err := r.client.SetSelectableValues(
		ctx,
		plan.UserAttributeID.ValueString(),
		plan.subject(),
		values,
		plan.AllowAnyValue.ValueBool(),
	)
	if err != nil {
		diags.AddError("Unable to set Omni selectable values", err.Error())
		return nil
	}
	return out
}

func applySelectableValuesToState(ctx context.Context, state *selectableValuesResourceModel, out *client.SelectableValues, diags *diagnostics) {
	state.AllowAnyValue = types.BoolValue(out.AllowAnyValue)

	// An absent provisioning and one holding no values both read back as an
	// empty list, so a configuration that set nothing keeps a null here rather
	// than flipping to [] and showing a diff on every plan.
	if len(out.Values) == 0 && state.Values.IsNull() {
		return
	}
	value, d := types.ListValueFrom(ctx, types.StringType, out.Values)
	diags.Append(d...)
	state.Values = value
}

func (r *selectableValuesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan selectableValuesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	out := r.write(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	state := plan
	state.ID = types.StringValue(plan.compositeID())
	applySelectableValuesToState(ctx, &state, out, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *selectableValuesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state selectableValuesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	out, err := r.client.GetSelectableValues(ctx, state.UserAttributeID.ValueString(), state.subject())
	if err != nil {
		if client.IsNotFound(err) {
			// The attribute itself is gone, so the provisioning cannot exist.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni selectable values", err.Error())
		return
	}

	applySelectableValuesToState(ctx, &state, out, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *selectableValuesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state selectableValuesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	out := r.write(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	next := plan
	next.ID = state.ID
	applySelectableValuesToState(ctx, &next, out, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

// Delete removes the provisioning by replacing it with nothing. There is no
// DELETE on this endpoint: an empty list with allow_any_value false is how the
// API expresses removal, and any switch the subject already made stops applying.
func (r *selectableValuesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state selectableValuesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	_, err := r.client.SetSelectableValues(ctx, state.UserAttributeID.ValueString(), state.subject(), []string{}, false)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to remove Omni selectable values", err.Error())
	}
}

func (r *selectableValuesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 3 || (parts[1] != "user" && parts[1] != "group") {
		resp.Diagnostics.AddError(
			"Unexpected import ID",
			fmt.Sprintf("Expected `<attribute id>:user:<membership id>` or `<attribute id>:group:<group id>`, got %q.", req.ID),
		)
		return
	}

	state := selectableValuesResourceModel{
		ID:              types.StringValue(req.ID),
		UserAttributeID: types.StringValue(parts[0]),
		UserID:          types.StringNull(),
		GroupID:         types.StringNull(),
		Values:          types.ListNull(types.StringType),
	}
	if parts[1] == "user" {
		state.UserID = types.StringValue(parts[2])
	} else {
		state.GroupID = types.StringValue(parts[2])
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
