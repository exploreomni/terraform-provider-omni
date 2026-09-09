package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &userGroupResource{}
	_ resource.ResourceWithConfigure   = &userGroupResource{}
	_ resource.ResourceWithImportState = &userGroupResource{}
)

// NewUserGroupResource returns the omni_user_group resource.
func NewUserGroupResource() resource.Resource { return &userGroupResource{} }

type userGroupResource struct {
	client *client.Client
}

type userGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	MemberIDs   types.Set    `tfsdk:"member_ids"`
}

func (r *userGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (r *userGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Omni user group and its membership. Requires an organization API key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The group's unique ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The group name. Maximum 64 characters.",
			},
			"member_ids": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "User IDs that belong to the group. Membership is authoritative: users " +
					"removed from this set are removed from the group.",
			},
		},
	}
}

func (r *userGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *userGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	members, diags := membersFromSet(ctx, plan.MemberIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateGroup(ctx, client.GroupInput{
		DisplayName: plan.DisplayName.ValueString(),
		Members:     members,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni user group", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(created.ID)
	state.DisplayName = types.StringValue(created.DisplayName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	group, err := r.client.GetGroup(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni user group", err.Error())
		return
	}

	state.DisplayName = types.StringValue(group.DisplayName)
	if !state.MemberIDs.IsNull() {
		ids := make([]string, 0, len(group.Members))
		for _, m := range group.Members {
			ids = append(ids, m.Value)
		}
		value, diags := types.SetValueFrom(ctx, types.StringType, ids)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.MemberIDs = value
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	members, diags := membersFromSet(ctx, plan.MemberIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.ReplaceGroup(ctx, state.ID.ValueString(), client.GroupInput{
		DisplayName: plan.DisplayName.ValueString(),
		Members:     members,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni user group", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	next.DisplayName = types.StringValue(updated.DisplayName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *userGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteGroup(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni user group", err.Error())
	}
}

func (r *userGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

func membersFromSet(ctx context.Context, set types.Set) ([]client.GroupMembr, diagnostics) {
	ids, diags := setToStrings(ctx, set)
	if diags.HasError() {
		return nil, diags
	}

	members := make([]client.GroupMembr, 0, len(ids))
	for _, id := range ids {
		members = append(members, client.GroupMembr{Value: id})
	}
	return members, diags
}
