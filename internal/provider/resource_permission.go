package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// contentRoles are the roles the content permission endpoints accept.
var contentRoles = []string{"NO_ACCESS", "VIEWER", "EXPLORER", "EDITOR", "MANAGER", "OWNER"}

const contentRoleGuidance = "One of `NO_ACCESS`, `VIEWER`, `EXPLORER`, `EDITOR`, `MANAGER` or `OWNER`. " +
	"`OWNER` can only be granted to users, never to groups. These are content roles, which decide who can " +
	"see and edit a folder or document. They are separate from model roles, which decide who can query data."

type permissionResourceModel struct {
	ID           types.String `tfsdk:"id"`
	TargetID     types.String `tfsdk:"folder_id"`
	Role         types.String `tfsdk:"role"`
	UserIDs      types.Set    `tfsdk:"user_ids"`
	UserGroupIDs types.Set    `tfsdk:"user_group_ids"`
	AccessBoost  types.Bool   `tfsdk:"access_boost"`
}

type documentPermissionResourceModel struct {
	ID           types.String `tfsdk:"id"`
	TargetID     types.String `tfsdk:"document_id"`
	Role         types.String `tfsdk:"role"`
	UserIDs      types.Set    `tfsdk:"user_ids"`
	UserGroupIDs types.Set    `tfsdk:"user_group_ids"`
	AccessBoost  types.Bool   `tfsdk:"access_boost"`
}

func permissionAttributes(targetAttr, targetDescription string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite ID in the form `<target id>:<role>`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		targetAttr: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: targetDescription,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"role": schema.StringAttribute{
			Required:            true,
			Validators:          []validator.String{stringvalidator.OneOf(contentRoles...)},
			MarkdownDescription: "The content role to grant. " + contentRoleGuidance,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"user_ids": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
			MarkdownDescription: "User IDs granted this role. Authoritative for this resource: a user removed " +
				"from the set has the grant revoked.",
		},
		"user_group_ids": schema.SetAttribute{
			Optional:    true,
			ElementType: types.StringType,
			MarkdownDescription: "User group IDs granted this role. Authoritative for this resource. For embed, " +
				"this is usually what you want: the `groups` claim in a signed URL resolves to these groups.",
		},
		"access_boost": schema.BoolAttribute{
			Optional: true,
			MarkdownDescription: "Whether to grant access boost alongside the role. Rejected with 403 when the " +
				"organization has AccessBoost turned off.",
		},
	}
}

// subjectsFromSets flattens the configured users and groups.
func subjectsFromSets(ctx context.Context, users, groups types.Set) (userIDs, groupIDs []string, diags diagnostics) {
	u, d := setToStrings(ctx, users)
	diags.Append(d...)
	g, d2 := setToStrings(ctx, groups)
	diags.Append(d2...)
	return u, g, diags
}

// diffSubjects returns the subjects added and removed between two lists.
func diffSubjects(before, after []string) (added, removed []string) {
	in := func(list []string, v string) bool {
		for _, item := range list {
			if item == v {
				return true
			}
		}
		return false
	}
	for _, v := range after {
		if !in(before, v) {
			added = append(added, v)
		}
	}
	for _, v := range before {
		if !in(after, v) {
			removed = append(removed, v)
		}
	}
	return added, removed
}

// refreshFromPermits narrows the tracked subjects to those the API still
// reports holding this role. Subjects it no longer lists have lost the grant.
func refreshFromPermits(permits []client.Permit, role string, trackedUsers, trackedGroups []string) (users, groups []string) {
	held := map[string]bool{}
	for _, p := range permits {
		if !strings.EqualFold(p.Role, role) {
			continue
		}
		if id, _ := p.SubjectID(); id != "" {
			held[id] = true
		}
	}
	for _, id := range trackedUsers {
		if held[id] {
			users = append(users, id)
		}
	}
	for _, id := range trackedGroups {
		if held[id] {
			groups = append(groups, id)
		}
	}
	return users, groups
}

func permissionID(targetID, role string) string { return targetID + ":" + role }

func splitPermissionID(id string) (targetID, role string, err error) {
	targetID, role, found := strings.Cut(id, ":")
	if !found || targetID == "" || role == "" {
		return "", "", fmt.Errorf("expected an ID in the form <target id>:<role>, got %q", id)
	}
	return targetID, role, nil
}

// ---------------------------------------------------------------------------
// omni_folder_permission
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                     = &folderPermissionResource{}
	_ resource.ResourceWithConfigure        = &folderPermissionResource{}
	_ resource.ResourceWithImportState      = &folderPermissionResource{}
	_ resource.ResourceWithConfigValidators = &folderPermissionResource{}
)

// NewFolderPermissionResource returns the omni_folder_permission resource.
func NewFolderPermissionResource() resource.Resource { return &folderPermissionResource{} }

type folderPermissionResource struct {
	client *client.Client
}

func (r *folderPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder_permission"
}

func (r *folderPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants a content role on a folder to users or groups.\n\n" +
			"One resource per role. To give two groups different roles on the same folder, declare two " +
			"resources. Changing `role` replaces the resource, because the role is part of its identity.",
		Attributes: permissionAttributes("folder_id", "The folder to grant access to."),
	}
}

func (r *folderPermissionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(pathExpr("user_ids"), pathExpr("user_group_ids")),
	}
}

func (r *folderPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *folderPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan permissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	users, groups, diags := subjectsFromSets(ctx, plan.UserIDs, plan.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.GrantFolderPermission(ctx, plan.TargetID.ValueString(), client.PermissionGrant{
		Role:         plan.Role.ValueString(),
		UserIDs:      users,
		UserGroupIDs: groups,
		AccessBoost:  boolPtr(plan.AccessBoost),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to grant Omni folder permission", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(permissionID(plan.TargetID.ValueString(), plan.Role.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state permissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	permits, err := r.client.ListFolderPermissions(ctx, state.TargetID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni folder permissions", err.Error())
		return
	}

	trackedUsers, trackedGroups, diags := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, groups := refreshFromPermits(permits, state.Role.ValueString(), trackedUsers, trackedGroups)
	if len(users) == 0 && len(groups) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(applySubjectsToState(ctx, &state.UserIDs, users)...)
	resp.Diagnostics.Append(applySubjectsToState(ctx, &state.UserGroupIDs, groups)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state permissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	wasUsers, wasGroups, d1 := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	nowUsers, nowGroups, d2 := subjectsFromSets(ctx, plan.UserIDs, plan.UserGroupIDs)
	resp.Diagnostics.Append(d1...)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	addedUsers, removedUsers := diffSubjects(wasUsers, nowUsers)
	addedGroups, removedGroups := diffSubjects(wasGroups, nowGroups)
	folderID := state.TargetID.ValueString()

	if len(removedUsers) > 0 || len(removedGroups) > 0 {
		if err := r.client.RevokeFolderPermission(ctx, folderID, client.PermissionRevoke{
			UserIDs:      removedUsers,
			UserGroupIDs: removedGroups,
		}); err != nil {
			resp.Diagnostics.AddError("Unable to revoke Omni folder permission", err.Error())
			return
		}
	}
	if len(addedUsers) > 0 || len(addedGroups) > 0 {
		if err := r.client.GrantFolderPermission(ctx, folderID, client.PermissionGrant{
			Role:         plan.Role.ValueString(),
			UserIDs:      addedUsers,
			UserGroupIDs: addedGroups,
			AccessBoost:  boolPtr(plan.AccessBoost),
		}); err != nil {
			resp.Diagnostics.AddError("Unable to grant Omni folder permission", err.Error())
			return
		}
	}
	// access_boost can change without the subject list changing.
	if !plan.AccessBoost.Equal(state.AccessBoost) && (len(nowUsers) > 0 || len(nowGroups) > 0) {
		if err := r.client.UpdateFolderPermission(ctx, folderID, client.PermissionGrant{
			Role:         plan.Role.ValueString(),
			UserIDs:      nowUsers,
			UserGroupIDs: nowGroups,
			AccessBoost:  boolPtr(plan.AccessBoost),
		}); err != nil {
			resp.Diagnostics.AddError("Unable to update Omni folder permission", err.Error())
			return
		}
	}

	next := plan
	next.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *folderPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state permissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	users, groups, diags := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RevokeFolderPermission(ctx, state.TargetID.ValueString(), client.PermissionRevoke{
		UserIDs:      users,
		UserGroupIDs: groups,
	}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to revoke Omni folder permission", err.Error())
	}
}

func (r *folderPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	folderID, role, err := splitPermissionID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("folder_id"), folderID)
	resp.State.SetAttribute(ctx, pathRoot("role"), role)
}

// ---------------------------------------------------------------------------
// omni_document_permission
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                     = &documentPermissionResource{}
	_ resource.ResourceWithConfigure        = &documentPermissionResource{}
	_ resource.ResourceWithImportState      = &documentPermissionResource{}
	_ resource.ResourceWithConfigValidators = &documentPermissionResource{}
)

// NewDocumentPermissionResource returns the omni_document_permission resource.
func NewDocumentPermissionResource() resource.Resource { return &documentPermissionResource{} }

type documentPermissionResource struct {
	client *client.Client
}

func (r *documentPermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_document_permission"
}

func (r *documentPermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants a content role on a document to users or groups.\n\n" +
			"The documents API has no endpoint for revoking a permission, so destroying this resource sets " +
			"the role to `NO_ACCESS` rather than removing the entry.",
		Attributes: permissionAttributes("document_id", "The document identifier to grant access to."),
	}
}

func (r *documentPermissionResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(pathExpr("user_ids"), pathExpr("user_group_ids")),
	}
}

func (r *documentPermissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *documentPermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan documentPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	users, groups, diags := subjectsFromSets(ctx, plan.UserIDs, plan.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.GrantDocumentPermission(ctx, plan.TargetID.ValueString(), client.PermissionGrant{
		Role:         plan.Role.ValueString(),
		UserIDs:      users,
		UserGroupIDs: groups,
		AccessBoost:  boolPtr(plan.AccessBoost),
	}); err != nil {
		resp.Diagnostics.AddError("Unable to grant Omni document permission", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(permissionID(plan.TargetID.ValueString(), plan.Role.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *documentPermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state documentPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	permits, err := r.client.ListDocumentPermissions(ctx, state.TargetID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni document permissions", err.Error())
		return
	}

	trackedUsers, trackedGroups, diags := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, groups := refreshFromPermits(permits, state.Role.ValueString(), trackedUsers, trackedGroups)
	if len(users) == 0 && len(groups) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(applySubjectsToState(ctx, &state.UserIDs, users)...)
	resp.Diagnostics.Append(applySubjectsToState(ctx, &state.UserGroupIDs, groups)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *documentPermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state documentPermissionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	wasUsers, wasGroups, d1 := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	nowUsers, nowGroups, d2 := subjectsFromSets(ctx, plan.UserIDs, plan.UserGroupIDs)
	resp.Diagnostics.Append(d1...)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	addedUsers, removedUsers := diffSubjects(wasUsers, nowUsers)
	addedGroups, removedGroups := diffSubjects(wasGroups, nowGroups)
	docID := state.TargetID.ValueString()

	if len(removedUsers) > 0 || len(removedGroups) > 0 {
		if err := r.client.RevokeDocumentPermission(ctx, docID, client.PermissionRevoke{
			UserIDs:      removedUsers,
			UserGroupIDs: removedGroups,
		}); err != nil {
			resp.Diagnostics.AddError("Unable to revoke Omni document permission", err.Error())
			return
		}
	}
	if len(addedUsers) > 0 || len(addedGroups) > 0 {
		if err := r.client.GrantDocumentPermission(ctx, docID, client.PermissionGrant{
			Role:         plan.Role.ValueString(),
			UserIDs:      addedUsers,
			UserGroupIDs: addedGroups,
			AccessBoost:  boolPtr(plan.AccessBoost),
		}); err != nil {
			resp.Diagnostics.AddError("Unable to grant Omni document permission", err.Error())
			return
		}
	}

	next := plan
	next.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *documentPermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state documentPermissionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	users, groups, diags := subjectsFromSets(ctx, state.UserIDs, state.UserGroupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RevokeDocumentPermission(ctx, state.TargetID.ValueString(), client.PermissionRevoke{
		UserIDs:      users,
		UserGroupIDs: groups,
	}); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to revoke Omni document permission", err.Error())
	}
}

func (r *documentPermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	docID, role, err := splitPermissionID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.State.SetAttribute(ctx, pathRoot("id"), req.ID)
	resp.State.SetAttribute(ctx, pathRoot("document_id"), docID)
	resp.State.SetAttribute(ctx, pathRoot("role"), role)
}
