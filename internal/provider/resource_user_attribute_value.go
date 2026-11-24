package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                     = &userAttributeValueResource{}
	_ resource.ResourceWithConfigure        = &userAttributeValueResource{}
	_ resource.ResourceWithImportState      = &userAttributeValueResource{}
	_ resource.ResourceWithConfigValidators = &userAttributeValueResource{}
)

// NewUserAttributeValueResource returns the omni_user_attribute_value resource.
func NewUserAttributeValueResource() resource.Resource { return &userAttributeValueResource{} }

type userAttributeValueResource struct {
	client *client.Client
}

type userAttributeValueResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Name   types.String `tfsdk:"name"`
	Value  types.String `tfsdk:"value"`
	Values types.Set    `tfsdk:"values"`
}

func (r *userAttributeValueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute_value"
}

func (r *userAttributeValueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One attribute value on one existing user, and nothing else about that user.\n\n" +
			"Use this for people Terraform does not own: users who sign in on their own, were invited from " +
			"the UI, or are provisioned by something else. The user is looked up, not created, so destroying " +
			"this resource clears the one key and leaves the person in place. Their other attributes, name " +
			"and groups are never touched.\n\n" +
			"Written with a SCIM `PATCH` naming only this key, so it works on any user. It is also the way to " +
			"set an attribute that takes multiple values, which `omni_user.attributes` cannot express.\n\n" +
			"Do not set the same key on the same user here and in `omni_user.attributes`. The two would " +
			"overwrite each other, and `omni_user` replaces the whole user on every update.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`<user_id>/<name>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The user's id, from `data.omni_user.<name>.id` for someone Terraform does " +
					"not own or `omni_user.<name>.id` for someone it does.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"name": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The attribute's **Reference** value from **Settings > User attributes**, " +
					"for example `location_id`. Use `omni_user_attribute.<name>.name` to make the attribute " +
					"definition a dependency.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"value": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The value, for an attribute that takes a single value. Exactly one of " +
					"`value` or `values`. Numbers are given as strings.",
			},
			"values": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "The values, for an attribute created with `multiple_values = true`. " +
					"Exactly one of `value` or `values`. Order does not matter. To clear every value, destroy " +
					"the resource rather than setting an empty set.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
		},
	}
}

func (r *userAttributeValueResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(pathExpr("value"), pathExpr("values")),
	}
}

func (r *userAttributeValueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

// payload is what goes to Omni: a string for value, a sorted []string for
// values. Omni checks the shape against the attribute definition.
func (m userAttributeValueResourceModel) payload(ctx context.Context, diags *diagnostics) any {
	if !m.Values.IsNull() && !m.Values.IsUnknown() {
		values, d := setToStrings(ctx, m.Values)
		diags.Append(d...)
		sort.Strings(values)
		return values
	}
	return m.Value.ValueString()
}

func (r *userAttributeValueResource) write(ctx context.Context, plan *userAttributeValueResourceModel, diags *diagnostics) {
	value := plan.payload(ctx, diags)
	if diags.HasError() {
		return
	}
	if err := r.client.SetUserAttributeValue(ctx, plan.UserID.ValueString(), plan.Name.ValueString(), value); err != nil {
		diags.AddError("Unable to set Omni user attribute value", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.UserID.ValueString() + "/" + plan.Name.ValueString())
}

func (r *userAttributeValueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAttributeValueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userAttributeValueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAttributeValueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	attrs, err := r.client.GetUserAttributeValues(ctx, state.UserID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			// The person was removed. Nothing left to own.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni user", err.Error())
		return
	}

	raw, ok := attrs[state.Name.ValueString()]
	if !ok || raw == nil {
		// Cleared outside Terraform. Dropping it from state makes the next plan
		// set it again, which is the drift correction an admin would expect.
		resp.State.RemoveResource(ctx)
		return
	}

	list, isList := attributeValueStrings(raw)

	// Track the value in the shape the configuration uses. After an import
	// neither is set yet, so take the shape Omni reports.
	useValues := !state.Values.IsNull() || (state.Value.IsNull() && isList)
	if useValues {
		set, d := types.SetValueFrom(ctx, types.StringType, list)
		resp.Diagnostics.Append(d...)
		state.Values = set
		state.Value = types.StringNull()
	} else {
		// A single-valued configuration on a key that now holds several shows
		// them joined, so the plan reports the drift instead of hiding it.
		state.Value = types.StringValue(strings.Join(list, ","))
		state.Values = types.SetNull(types.StringType)
	}
	state.ID = types.StringValue(state.UserID.ValueString() + "/" + state.Name.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userAttributeValueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userAttributeValueResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the key. The user stays.
func (r *userAttributeValueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAttributeValueResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.ClearUserAttributeValue(ctx, state.UserID.ValueString(), state.Name.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to clear Omni user attribute value", err.Error())
	}
}

func (r *userAttributeValueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	userID, name, ok := strings.Cut(req.ID, "/")
	if !ok || userID == "" || name == "" {
		resp.Diagnostics.AddError(
			"Unexpected import ID",
			fmt.Sprintf("Expected `<user id>/<attribute name>`, got %q.", req.ID),
		)
		return
	}

	state := userAttributeValueResourceModel{
		ID:     types.StringValue(req.ID),
		UserID: types.StringValue(userID),
		Name:   types.StringValue(name),
		Value:  types.StringNull(),
		Values: types.SetNull(types.StringType),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// attributeValueStrings turns an attribute value as Omni reports it into
// strings, and says whether Omni reported a list. Numbers arrive as
// json.Number, so they keep every digit.
func attributeValueStrings(raw any) ([]string, bool) {
	if items, ok := raw.([]any); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, scalarString(item))
		}
		return out, true
	}
	return []string{scalarString(raw)}, false
}

func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", t)
	}
}
