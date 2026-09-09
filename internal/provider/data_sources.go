package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// ---------------------------------------------------------------------------
// omni_user
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &userDataSource{}
	_ datasource.DataSourceWithConfigure = &userDataSource{}
)

// NewUserDataSource returns the omni_user data source.
func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct {
	client *client.Client
}

type userDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	UserName    types.String `tfsdk:"user_name"`
	DisplayName types.String `tfsdk:"display_name"`
	Active      types.Bool   `tfsdk:"active"`
	GroupIDs    types.Set    `tfsdk:"group_ids"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up an Omni user by ID or email address.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The user's unique ID. One of `id` or `user_name` is required.",
			},
			"user_name": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The user's email address. One of `id` or `user_name` is required.",
			},
			"display_name": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The user's display name.",
			},
			"active": dschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the user is active.",
			},
			"group_ids": dschema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the groups the user belongs to.",
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	var (
		user *client.User
		err  error
	)
	switch {
	case !config.ID.IsNull():
		user, err = d.client.GetUser(ctx, config.ID.ValueString())
	case !config.UserName.IsNull():
		user, err = d.client.FindUserByUserName(ctx, config.UserName.ValueString())
	default:
		resp.Diagnostics.AddError("Missing lookup key", "Set either id or user_name.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up Omni user", err.Error())
		return
	}

	groupIDs := make([]string, 0, len(user.Groups))
	for _, g := range user.Groups {
		groupIDs = append(groupIDs, g.Value)
	}
	groups, diags := types.SetValueFrom(ctx, types.StringType, groupIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := userDataSourceModel{
		ID:          types.StringValue(user.ID),
		UserName:    types.StringValue(user.UserName),
		DisplayName: types.StringValue(user.DisplayName),
		Active:      types.BoolValue(user.Active == nil || *user.Active),
		GroupIDs:    groups,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// omni_user_group
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &userGroupDataSource{}
	_ datasource.DataSourceWithConfigure = &userGroupDataSource{}
)

// NewUserGroupDataSource returns the omni_user_group data source.
func NewUserGroupDataSource() datasource.DataSource { return &userGroupDataSource{} }

type userGroupDataSource struct {
	client *client.Client
}

type userGroupDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	MemberIDs   types.Set    `tfsdk:"member_ids"`
}

func (d *userGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

func (d *userGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up an Omni user group by ID or name.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The group's unique ID. One of `id` or `display_name` is required.",
			},
			"display_name": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The group name. One of `id` or `display_name` is required.",
			},
			"member_ids": dschema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the users in the group.",
			},
		},
	}
}

func (d *userGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *userGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	var (
		group *client.Group
		err   error
	)
	switch {
	case !config.ID.IsNull():
		group, err = d.client.GetGroup(ctx, config.ID.ValueString())
	case !config.DisplayName.IsNull():
		group, err = d.client.FindGroupByDisplayName(ctx, config.DisplayName.ValueString())
	default:
		resp.Diagnostics.AddError("Missing lookup key", "Set either id or display_name.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up Omni user group", err.Error())
		return
	}

	memberIDs := make([]string, 0, len(group.Members))
	for _, m := range group.Members {
		memberIDs = append(memberIDs, m.Value)
	}
	members, diags := types.SetValueFrom(ctx, types.StringType, memberIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := userGroupDataSourceModel{
		ID:          types.StringValue(group.ID),
		DisplayName: types.StringValue(group.DisplayName),
		MemberIDs:   members,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// omni_connection
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &connectionDataSource{}
	_ datasource.DataSourceWithConfigure = &connectionDataSource{}
)

// NewConnectionDataSource returns the omni_connection data source.
func NewConnectionDataSource() datasource.DataSource { return &connectionDataSource{} }

type connectionDataSource struct {
	client *client.Client
}

type connectionDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Dialect       types.String `tfsdk:"dialect"`
	Database      types.String `tfsdk:"database"`
	DefaultSchema types.String `tfsdk:"default_schema"`
	BaseRole      types.String `tfsdk:"base_role"`
}

func (d *connectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (d *connectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up a database connection by ID or name.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The connection's unique ID. One of `id` or `name` is required.",
			},
			"name": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The connection name. One of `id` or `name` is required.",
			},
			"dialect": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The database dialect.",
			},
			"database": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The default database or catalog.",
			},
			"default_schema": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The default schema.",
			},
			"base_role": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The default role for users on this connection.",
			},
		},
	}
}

func (d *connectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *connectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config connectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	var connection *client.Connection

	switch {
	case !config.ID.IsNull():
		found, err := d.client.GetConnection(ctx, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to look up Omni connection", err.Error())
			return
		}
		connection = found
	case !config.Name.IsNull():
		connections, err := d.client.ListConnections(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Omni connections", err.Error())
			return
		}
		name := config.Name.ValueString()
		for i := range connections {
			if connections[i].Name == name {
				connection = &connections[i]
				break
			}
		}
		if connection == nil {
			resp.Diagnostics.AddError(
				"Connection not found",
				fmt.Sprintf("No connection named %q is visible to this API token.", name),
			)
			return
		}
	default:
		resp.Diagnostics.AddError("Missing lookup key", "Set either id or name.")
		return
	}

	state := connectionDataSourceModel{
		ID:            types.StringValue(connection.ID),
		Name:          types.StringValue(connection.Name),
		Dialect:       stringOrNull(connection.Dialect),
		Database:      stringOrNull(connection.Database),
		DefaultSchema: stringOrNull(connection.DefaultSchema),
		BaseRole:      stringOrNull(connection.BaseRole),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// omni_model
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &modelDataSource{}
	_ datasource.DataSourceWithConfigure = &modelDataSource{}
)

// NewModelDataSource returns the omni_model data source.
func NewModelDataSource() datasource.DataSource { return &modelDataSource{} }

type modelDataSource struct {
	client *client.Client
}

type modelDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	ModelKind    types.String `tfsdk:"model_kind"`
	ConnectionID types.String `tfsdk:"connection_id"`
	BaseModelID  types.String `tfsdk:"base_model_id"`
}

func (d *modelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (d *modelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up a model by ID or name.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The model's unique ID. One of `id` or `name` is required.",
			},
			"name": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The model name. One of `id` or `name` is required.",
			},
			"model_kind": dschema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Restrict the lookup to a model kind, for example `SHARED`, " +
					"`SHARED_EXTENSION`, `SCHEMA`, or `BRANCH`.",
			},
			"connection_id": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The connection the model is built on.",
			},
			"base_model_id": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The model this one extends, if any.",
			},
		},
	}
}

func (d *modelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *modelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config modelDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	models, err := d.client.ListModels(ctx, config.ModelKind.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Omni models", err.Error())
		return
	}

	var match *client.Model
	switch {
	case !config.ID.IsNull():
		id := config.ID.ValueString()
		for i := range models {
			if models[i].ID == id {
				match = &models[i]
				break
			}
		}
	case !config.Name.IsNull():
		name := config.Name.ValueString()
		for i := range models {
			if models[i].Name == name {
				match = &models[i]
				break
			}
		}
	default:
		resp.Diagnostics.AddError("Missing lookup key", "Set either id or name.")
		return
	}

	if match == nil {
		resp.Diagnostics.AddError(
			"Model not found",
			"No model matched the given id or name for this API token.",
		)
		return
	}

	state := modelDataSourceModel{
		ID:           types.StringValue(match.ID),
		Name:         stringOrNull(match.Name),
		ModelKind:    stringOrNull(match.ModelKind),
		ConnectionID: stringOrNull(match.ConnectionID),
		BaseModelID:  stringOrNull(match.BaseModelID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
