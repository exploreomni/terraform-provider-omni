package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

// NewUserResource returns the omni_user resource.
func NewUserResource() resource.Resource { return &userResource{} }

type userResource struct {
	client *client.Client
}

type userResourceModel struct {
	ID          types.String `tfsdk:"id"`
	UserName    types.String `tfsdk:"user_name"`
	DisplayName types.String `tfsdk:"display_name"`
	Attributes  types.Map    `tfsdk:"attributes"`
	Active      types.Bool   `tfsdk:"active"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provisions an Omni user through the SCIM API. Requires an organization API key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The user's unique ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user's email address (SCIM `userName`).",
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user's display name.",
			},
			"attributes": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "User attributes as key/value pairs. Keys are the **Reference** values from " +
					"**Settings > User attributes**. Only the keys declared here are tracked; attributes set " +
					"elsewhere are left alone.",
			},
			"active": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the user is active.",
			},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	attrs, diags := mapToAny(ctx, plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateUser(ctx, client.UserInput{
		UserName:       plan.UserName.ValueString(),
		DisplayName:    plan.DisplayName.ValueString(),
		UserAttributes: attrs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni user", err.Error())
		return
	}

	state := plan
	applyUserToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	user, err := r.client.GetUser(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni user", err.Error())
		return
	}

	refreshUserAttributes(ctx, &state, user, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	applyUserToState(&state, user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	attrs, diags := mapToAny(ctx, plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.ReplaceUser(ctx, state.ID.ValueString(), client.UserInput{
		UserName:       plan.UserName.ValueString(),
		DisplayName:    plan.DisplayName.ValueString(),
		UserAttributes: attrs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni user", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	applyUserToState(&next, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteUser(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni user", err.Error())
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

func applyUserToState(state *userResourceModel, user *client.User) {
	state.ID = types.StringValue(user.ID)
	if user.UserName != "" {
		state.UserName = types.StringValue(user.UserName)
	}
	if user.DisplayName != "" {
		state.DisplayName = types.StringValue(user.DisplayName)
	}
	if user.Active != nil {
		state.Active = types.BoolValue(*user.Active)
	} else {
		state.Active = types.BoolValue(true)
	}
}

// refreshUserAttributes updates only the attribute keys Terraform already
// tracks, so attributes managed outside Terraform never show up as drift.
func refreshUserAttributes(ctx context.Context, state *userResourceModel, user *client.User, diags *diagnostics) {
	if state.Attributes.IsNull() {
		return
	}

	tracked := map[string]string{}
	d := state.Attributes.ElementsAs(ctx, &tracked, false)
	diags.Append(d...)
	if diags.HasError() {
		return
	}

	next := map[string]string{}
	for key := range tracked {
		if raw, ok := user.UserAttributes[key]; ok {
			next[key] = fmt.Sprintf("%v", raw)
		}
	}

	value, d := types.MapValueFrom(ctx, types.StringType, next)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	state.Attributes = value
}
