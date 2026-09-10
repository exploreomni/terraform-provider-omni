package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

const roleNameGuidance = "One of `VIEWER`, `QUERIER`, `QUERY_TOPICS` (Restricted Querier), `MODELER`, " +
	"`CONNECTION_ADMIN`, `NO_ACCESS`, or a custom role defined for your organization."

type modelRoleResourceModel struct {
	ID            types.String `tfsdk:"id"`
	SubjectID     types.String `tfsdk:"user_id"`
	ModelID       types.String `tfsdk:"model_id"`
	ConnectionID  types.String `tfsdk:"connection_id"`
	RoleName      types.String `tfsdk:"role_name"`
	RoleOnDestroy types.String `tfsdk:"role_on_destroy"`
}

type groupModelRoleResourceModel struct {
	ID            types.String `tfsdk:"id"`
	SubjectID     types.String `tfsdk:"user_group_id"`
	ModelID       types.String `tfsdk:"model_id"`
	ConnectionID  types.String `tfsdk:"connection_id"`
	RoleName      types.String `tfsdk:"role_name"`
	RoleOnDestroy types.String `tfsdk:"role_on_destroy"`
}

func modelRoleAttributes(subjectAttr, subjectDescription string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite ID in the form `<subject id>:<model or connection id>`.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		subjectAttr: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: subjectDescription,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"model_id": schema.StringAttribute{
			Optional: true,
			MarkdownDescription: "The model to grant the role on. Required for every role except " +
				"`CONNECTION_ADMIN` and custom roles based on it.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"connection_id": schema.StringAttribute{
			Optional: true,
			MarkdownDescription: "The connection the model belongs to. Required when `model_id` is omitted; " +
				"otherwise inferred from the model.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"role_name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The role to assign. " + roleNameGuidance,
		},
		"role_on_destroy": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString("NO_ACCESS"),
			MarkdownDescription: "The Omni API has no endpoint for removing a role assignment, so destroying " +
				"this resource downgrades the role to this value instead. Defaults to `NO_ACCESS`.\n\n" +
				"Be aware that `NO_ACCESS` does not necessarily revoke access. Omni resolves roles by " +
				"priority, and a direct assignment sits at priority 0 while the connection's base role sits " +
				"higher. If the connection grants `QUERIER` to everyone, a destroyed assignment leaves the " +
				"subject with `QUERIER`, not no access. Set the connection's `base_role` to `NO_ACCESS` if " +
				"you need destroy to actually remove access.",
		},
	}
}

func modelRoleID(subjectID, modelID, connectionID string) string {
	target := modelID
	if target == "" {
		target = connectionID
	}
	return subjectID + ":" + target
}

func splitModelRoleID(id string) (subjectID, targetID string, err error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected an ID in the form <subject id>:<model or connection id>, got %q", id)
	}
	return parts[0], parts[1], nil
}

// findModelRole locates the assignment that matches the tracked model or
// connection.
//
// The endpoint reports every role that applies, including ones the caller never
// assigned: the connection's base role shows up as "Connection Base Role", and
// group membership as "User Group Role". Only the entry whose from.type matches
// wantSource is a direct assignment, so base roles must not be mistaken for one.
// Without this filter, reading a role that was never assigned would appear to
// succeed, and a role that was assigned could be matched against the wrong row.
func findModelRole(roles []client.ModelRole, modelID, connectionID, wantSource string) *client.ModelRole {
	var fallback *client.ModelRole

	for i := range roles {
		role := roles[i]

		matchesTarget := (modelID != "" && role.ModelID == modelID) ||
			(modelID == "" && connectionID != "" && role.ConnectionID == connectionID)
		if !matchesTarget {
			continue
		}

		source := role.SourceType()
		lowered := strings.ToLower(source)

		// The connection default, not something Terraform assigned.
		if strings.Contains(lowered, "base role") {
			continue
		}

		// An exact match on the expected origin is the assignment we made.
		if strings.EqualFold(source, wantSource) {
			return &roles[i]
		}

		// A role a user inherits from a group is not their direct assignment,
		// so it must never satisfy a user role resource.
		if wantSource == sourceUserRole && strings.Contains(lowered, "group") {
			continue
		}

		// Anything else that is not a base role is a direct assignment on this
		// endpoint. Kept as a fallback because the exact from.type labels are
		// not documented and differ between the user and group endpoints.
		if fallback == nil {
			fallback = &roles[i]
		}
	}
	return fallback
}

const (
	// from.type values the model-roles endpoints report for direct assignments.
	sourceUserRole  = "User Role"
	sourceGroupRole = "User Group Role"
)

// ---------------------------------------------------------------------------
// omni_user_model_role
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                     = &userModelRoleResource{}
	_ resource.ResourceWithConfigure        = &userModelRoleResource{}
	_ resource.ResourceWithImportState      = &userModelRoleResource{}
	_ resource.ResourceWithConfigValidators = &userModelRoleResource{}
)

// NewUserModelRoleResource returns the omni_user_model_role resource.
func NewUserModelRoleResource() resource.Resource { return &userModelRoleResource{} }

type userModelRoleResource struct {
	client *client.Client
}

func (r *userModelRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_model_role"
}

func (r *userModelRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a user's role on a model or connection.",
		Attributes:          modelRoleAttributes("user_id", "The ID of the user to assign the role to."),
	}
}

func (r *userModelRoleResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(pathExpr("model_id"), pathExpr("connection_id")),
	}
}

func (r *userModelRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *userModelRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan modelRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	assigned, err := r.client.AssignUserModelRole(ctx, plan.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: plan.ConnectionID.ValueString(),
		ModelID:      plan.ModelID.ValueString(),
		RoleName:     plan.RoleName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to assign Omni model role", err.Error())
		return
	}

	state := plan
	if assigned.ConnectionID != "" {
		state.ConnectionID = types.StringValue(assigned.ConnectionID)
	}
	state.ID = types.StringValue(modelRoleID(
		plan.SubjectID.ValueString(),
		plan.ModelID.ValueString(),
		state.ConnectionID.ValueString(),
	))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userModelRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state modelRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	roles, err := r.client.ListUserModelRoles(ctx, state.SubjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni model roles", err.Error())
		return
	}

	match := findModelRole(roles, state.ModelID.ValueString(), state.ConnectionID.ValueString(), sourceUserRole)
	if match == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.RoleName = types.StringValue(match.RoleName)
	if match.ConnectionID != "" {
		state.ConnectionID = types.StringValue(match.ConnectionID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userModelRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state modelRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if _, err := r.client.AssignUserModelRole(ctx, plan.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: plan.ConnectionID.ValueString(),
		ModelID:      plan.ModelID.ValueString(),
		RoleName:     plan.RoleName.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to update Omni model role", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *userModelRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state modelRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if _, err := r.client.AssignUserModelRole(ctx, state.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: state.ConnectionID.ValueString(),
		ModelID:      state.ModelID.ValueString(),
		RoleName:     state.RoleOnDestroy.ValueString(),
	}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to downgrade Omni model role on destroy", err.Error())
	}
}

func (r *userModelRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	userID, targetID, err := splitModelRoleID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("user_id"), userID)
	resp.State.SetAttribute(ctx, pathRoot("model_id"), targetID)
	resp.State.SetAttribute(ctx, pathRoot("role_on_destroy"), "NO_ACCESS")
}

// ---------------------------------------------------------------------------
// omni_user_group_model_role
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                     = &userGroupModelRoleResource{}
	_ resource.ResourceWithConfigure        = &userGroupModelRoleResource{}
	_ resource.ResourceWithImportState      = &userGroupModelRoleResource{}
	_ resource.ResourceWithConfigValidators = &userGroupModelRoleResource{}
)

// NewUserGroupModelRoleResource returns the omni_user_group_model_role resource.
func NewUserGroupModelRoleResource() resource.Resource { return &userGroupModelRoleResource{} }

type userGroupModelRoleResource struct {
	client *client.Client
}

func (r *userGroupModelRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group_model_role"
}

func (r *userGroupModelRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns a user group's role on a model or connection. Every member of the group " +
			"inherits the role.",
		Attributes: modelRoleAttributes("user_group_id", "The ID of the user group to assign the role to."),
	}
}

func (r *userGroupModelRoleResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(pathExpr("model_id"), pathExpr("connection_id")),
	}
}

func (r *userGroupModelRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *userGroupModelRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModelRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	assigned, err := r.client.AssignGroupModelRole(ctx, plan.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: plan.ConnectionID.ValueString(),
		ModelID:      plan.ModelID.ValueString(),
		RoleName:     plan.RoleName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to assign Omni model role to user group", err.Error())
		return
	}

	state := plan
	if assigned.ConnectionID != "" {
		state.ConnectionID = types.StringValue(assigned.ConnectionID)
	}
	state.ID = types.StringValue(modelRoleID(
		plan.SubjectID.ValueString(),
		plan.ModelID.ValueString(),
		state.ConnectionID.ValueString(),
	))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userGroupModelRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModelRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	roles, err := r.client.ListGroupModelRoles(ctx, state.SubjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni user group model roles", err.Error())
		return
	}

	match := findModelRole(roles, state.ModelID.ValueString(), state.ConnectionID.ValueString(), sourceGroupRole)
	if match == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.RoleName = types.StringValue(match.RoleName)
	if match.ConnectionID != "" {
		state.ConnectionID = types.StringValue(match.ConnectionID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userGroupModelRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupModelRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if _, err := r.client.AssignGroupModelRole(ctx, plan.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: plan.ConnectionID.ValueString(),
		ModelID:      plan.ModelID.ValueString(),
		RoleName:     plan.RoleName.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to update Omni user group model role", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *userGroupModelRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModelRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if _, err := r.client.AssignGroupModelRole(ctx, state.SubjectID.ValueString(), client.ModelRoleInput{
		ConnectionID: state.ConnectionID.ValueString(),
		ModelID:      state.ModelID.ValueString(),
		RoleName:     state.RoleOnDestroy.ValueString(),
	}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to downgrade Omni user group model role on destroy", err.Error())
	}
}

func (r *userGroupModelRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	groupID, targetID, err := splitModelRoleID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("user_group_id"), groupID)
	resp.State.SetAttribute(ctx, pathRoot("model_id"), targetID)
	resp.State.SetAttribute(ctx, pathRoot("role_on_destroy"), "NO_ACCESS")
}
