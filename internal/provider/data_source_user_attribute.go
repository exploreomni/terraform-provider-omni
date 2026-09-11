package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// User attribute definitions are read-only in the API. These data sources exist
// so a configuration can reference an attribute by name and fail clearly when
// the target instance does not have it, rather than silently producing a
// deployment whose row-level security does not work.

// ---------------------------------------------------------------------------
// omni_user_attribute
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &userAttributeDataSource{}
	_ datasource.DataSourceWithConfigure = &userAttributeDataSource{}
)

// NewUserAttributeDataSource returns the omni_user_attribute data source.
func NewUserAttributeDataSource() datasource.DataSource { return &userAttributeDataSource{} }

type userAttributeDataSource struct {
	client *client.Client
}

type userAttributeDataSourceModel struct {
	Name           types.String `tfsdk:"name"`
	ID             types.String `tfsdk:"id"`
	Label          types.String `tfsdk:"label"`
	Description    types.String `tfsdk:"description"`
	Type           types.String `tfsdk:"type"`
	DefaultValue   types.String `tfsdk:"default_value"`
	MultipleValues types.Bool   `tfsdk:"multiple_values"`
	System         types.Bool   `tfsdk:"system"`
}

func (d *userAttributeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute"
}

func (d *userAttributeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up a user attribute definition by its reference name.\n\n" +
			"Attribute definitions cannot be created through the API, only read. Make them in the UI under " +
			"**Settings > User attributes**, then reference them here so a configuration fails loudly on an " +
			"instance that is missing one, instead of applying cleanly with row-level security that does " +
			"nothing.",
		Attributes: map[string]dschema.Attribute{
			"name": dschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The attribute's **Reference** value, for example `tenant_id`.",
			},
			"id": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The attribute's unique ID.",
			},
			"label": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display label shown in the UI.",
			},
			"description": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the attribute and its purpose.",
			},
			"type": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`String` or `Number`. Number attributes are stored as strings for precision.",
			},
			"default_value": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Value used when a user has none set.",
			},
			"multiple_values": dschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the attribute accepts more than one value per user.",
			},
			"system": dschema.BoolAttribute{
				Computed: true,
				MarkdownDescription: "Whether Omni defines and populates this attribute. Do not set a system " +
					"attribute through `omni_user`: the platform owns the value and will not return it, so " +
					"every plan would propose setting it again.",
			},
		},
	}
}

func (d *userAttributeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *userAttributeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userAttributeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	attr, err := d.client.FindUserAttribute(ctx, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up Omni user attribute", err.Error())
		return
	}

	state := userAttributeDataSourceModel{
		Name:           types.StringValue(attr.Name),
		ID:             stringOrNull(attr.ID),
		Label:          stringOrNull(attr.Label),
		Description:    stringOrNull(ptrString(attr.Description)),
		Type:           stringOrNull(attr.Type),
		DefaultValue:   stringOrNull(ptrString(attr.DefaultValue)),
		MultipleValues: types.BoolValue(attr.MultipleValues),
		System:         types.BoolValue(attr.System),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---------------------------------------------------------------------------
// omni_user_attributes
// ---------------------------------------------------------------------------

var (
	_ datasource.DataSource              = &userAttributesDataSource{}
	_ datasource.DataSourceWithConfigure = &userAttributesDataSource{}
)

// NewUserAttributesDataSource returns the omni_user_attributes data source.
func NewUserAttributesDataSource() datasource.DataSource { return &userAttributesDataSource{} }

type userAttributesDataSource struct {
	client *client.Client
}

type userAttributesDataSourceModel struct {
	IncludeSystem types.Bool `tfsdk:"include_system"`
	Names         types.List `tfsdk:"names"`
	Attributes    types.Map  `tfsdk:"attributes"`
}

func (d *userAttributesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attributes"
}

func (d *userAttributesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Lists every user attribute definition on the instance.\n\n" +
			"Useful for checking that a target instance has the attributes a configuration expects before " +
			"applying it, since definitions cannot be created through the API.",
		Attributes: map[string]dschema.Attribute{
			"include_system": dschema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Whether to include Omni's built-in attributes. Defaults to `false`, " +
					"since those are populated by the platform and are not usable for tenant scoping.",
			},
			"names": dschema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Reference names of the matching attributes.",
			},
			"attributes": dschema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Reference name to display label, for the matching attributes.",
			},
		},
	}
}

func (d *userAttributesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *userAttributesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userAttributesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	attrs, err := d.client.ListUserAttributes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Omni user attributes", err.Error())
		return
	}

	includeSystem := config.IncludeSystem.ValueBool()
	names := []string{}
	labels := map[string]string{}
	for _, a := range attrs {
		if a.System && !includeSystem {
			continue
		}
		names = append(names, a.Name)
		labels[a.Name] = a.Label
	}

	nameList, diags := types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	labelMap, diags := types.MapValueFrom(ctx, types.StringType, labels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := config
	state.Names = nameList
	state.Attributes = labelMap
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
