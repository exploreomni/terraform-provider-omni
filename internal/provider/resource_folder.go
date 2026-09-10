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
	_ resource.Resource                = &folderResource{}
	_ resource.ResourceWithConfigure   = &folderResource{}
	_ resource.ResourceWithImportState = &folderResource{}
)

// NewFolderResource returns the omni_folder resource.
func NewFolderResource() resource.Resource { return &folderResource{} }

type folderResource struct {
	client *client.Client
}

type folderResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Path                types.String `tfsdk:"path"`
	ParentFolderID      types.String `tfsdk:"parent_folder_id"`
	Scope               types.String `tfsdk:"scope"`
	OwnerID             types.String `tfsdk:"owner_id"`
	BreadcrumbRoot      types.Bool   `tfsdk:"breadcrumb_root"`
	ResolvePathConflict types.Bool   `tfsdk:"resolve_path_conflict"`
	DeleteRecursively   types.Bool   `tfsdk:"delete_recursively"`
	URL                 types.String `tfsdk:"url"`
}

func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (r *folderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a content folder. Folders can be nested up to seven levels deep.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The folder's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the folder.",
			},
			"path": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "URL path segment for the folder. Lowercased by the API, and limited to " +
					"alphanumeric characters and dashes. Changing it updates every descendant folder path.\n\n" +
					"No `UseStateForUnknown` here on purpose: renaming a parent rewrites every descendant's " +
					"path server side, so a child's path cannot be assumed to survive an update unchanged.",
			},
			"parent_folder_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ID of the parent folder. Omit for a top-level folder.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scope": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stringvalidator.OneOf("organization", "restricted")},
				MarkdownDescription: "Folder scope. Defaults to `organization`, or the parent folder's scope when " +
					"nested. A child folder's scope must match its parent.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"owner_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "ID of the folder owner. Required for `restricted` scope when using an organization API key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"breadcrumb_root": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "When `true`, breadcrumbs for this folder and its descendants start here, " +
					"hiding ancestors and the scope crumb.",
			},
			"resolve_path_conflict": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, path conflicts on update are resolved by appending a numeric " +
					"suffix instead of returning a conflict error.",
			},
			"delete_recursively": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, destroying this folder also deletes its documents and " +
					"sub-folders (`force=true`). Defaults to `false`, which fails on a non-empty folder.",
			},
			"url": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "Direct link to the folder in the Omni UI. Derived from the path, so it " +
					"changes when an ancestor folder is renamed.",
			},
		},
	}
}

func (r *folderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan folderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	created, err := r.client.CreateFolder(ctx, client.FolderInput{
		Name:           plan.Name.ValueString(),
		ParentFolderID: stringPtr(plan.ParentFolderID),
		Scope:          stringPtr(plan.Scope),
		UserID:         stringPtr(plan.OwnerID),
		BreadcrumbRoot: boolPtr(plan.BreadcrumbRoot),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni folder", err.Error())
		return
	}

	// The create response does not accept a path, so honour an explicit path by
	// patching immediately after creation.
	if !plan.Path.IsNull() && !plan.Path.IsUnknown() && plan.Path.ValueString() != created.Path {
		updated, err := r.client.UpdateFolder(ctx, created.ID, client.FolderUpdate{
			Path:                stringPtr(plan.Path),
			ResolvePathConflict: boolPtr(plan.ResolvePathConflict),
		})
		if err != nil {
			resp.Diagnostics.AddError("Folder created but the path could not be set", err.Error())
			return
		}
		created.Path = updated.Path
	}

	// The create response omits url, which only the list endpoint returns. Read
	// the folder back so computed attributes are populated straight away. This
	// is best effort: a folder that was created fine should not fail the apply
	// just because the read-back did not find it.
	if refreshed, err := r.client.GetFolder(ctx, created.ID, created.Scope, created.OwnerID); err == nil {
		created = refreshed
	}

	state := plan
	applyFolderToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state folderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	folder, err := r.client.GetFolder(
		ctx,
		state.ID.ValueString(),
		state.Scope.ValueString(),
		state.OwnerID.ValueString(),
	)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni folder", err.Error())
		return
	}

	applyFolderToState(&state, folder)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state folderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	update := client.FolderUpdate{
		ResolvePathConflict: boolPtr(plan.ResolvePathConflict),
	}
	if !plan.Name.Equal(state.Name) {
		update.Name = stringPtr(plan.Name)
	}
	if !plan.Path.Equal(state.Path) && !plan.Path.IsUnknown() {
		update.Path = stringPtr(plan.Path)
	}
	if !plan.BreadcrumbRoot.Equal(state.BreadcrumbRoot) {
		update.BreadcrumbRoot = boolPtr(plan.BreadcrumbRoot)
	}

	next := plan
	next.ID = state.ID

	if update.Name != nil || update.Path != nil || update.BreadcrumbRoot != nil {
		if update.Name == nil && update.Path == nil {
			// The API requires at least one of name or path in the body.
			update.Name = stringPtr(plan.Name)
		}
		updated, err := r.client.UpdateFolder(ctx, state.ID.ValueString(), update)
		if err != nil {
			resp.Diagnostics.AddError("Unable to update Omni folder", err.Error())
			return
		}

		// The update response omits url, which only the list endpoint returns,
		// and a path change alters the url. Read the folder back so url tracks
		// the new path instead of reverting to null.
		if refreshed, err := r.client.GetFolder(ctx, state.ID.ValueString(), state.Scope.ValueString(), state.OwnerID.ValueString()); err == nil {
			updated = refreshed
		}

		applyFolderToState(&next, updated)
	}

	// Carry over any computed values Terraform left unknown in the plan.
	if next.Path.IsUnknown() {
		next.Path = state.Path
	}
	if next.URL.IsUnknown() {
		next.URL = state.URL
	}
	if next.Scope.IsUnknown() {
		next.Scope = state.Scope
	}
	if next.OwnerID.IsUnknown() {
		next.OwnerID = state.OwnerID
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state folderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.DeleteFolder(ctx, state.ID.ValueString(), state.DeleteRecursively.ValueBool())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni folder", err.Error())
	}
}

func (r *folderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
	resp.State.SetAttribute(ctx, pathRoot("scope"), "organization")
	resp.State.SetAttribute(ctx, pathRoot("resolve_path_conflict"), false)
	resp.State.SetAttribute(ctx, pathRoot("delete_recursively"), false)
}

func applyFolderToState(state *folderResourceModel, folder *client.Folder) {
	state.ID = types.StringValue(folder.ID)
	if folder.Name != "" {
		state.Name = types.StringValue(folder.Name)
	}
	// Responses vary by endpoint: create and update omit url, and update omits
	// scope. Only take values the response actually carried, so a partial
	// response cannot null out something Terraform already knows.
	if folder.Path != "" {
		state.Path = types.StringValue(folder.Path)
	} else if state.Path.IsUnknown() {
		state.Path = types.StringNull()
	}
	if folder.URL != "" {
		state.URL = types.StringValue(folder.URL)
	} else if state.URL.IsUnknown() {
		state.URL = types.StringNull()
	}
	if folder.Scope != "" {
		state.Scope = types.StringValue(folder.Scope)
	}

	ownerID := folder.OwnerID
	if ownerID == "" && folder.Owner != nil {
		ownerID = folder.Owner.ID
	}
	if ownerID != "" {
		state.OwnerID = types.StringValue(ownerID)
	} else if state.OwnerID.IsUnknown() {
		state.OwnerID = types.StringNull()
	}
}
