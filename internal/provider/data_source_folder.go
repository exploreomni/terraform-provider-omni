package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ datasource.DataSource                     = &folderDataSource{}
	_ datasource.DataSourceWithConfigure        = &folderDataSource{}
	_ datasource.DataSourceWithConfigValidators = &folderDataSource{}
)

// NewFolderDataSource returns the omni_folder data source.
//
// This exists so a configuration can grant access to a folder someone else
// made, which is the usual shape: shared content lives in a hub folder created
// by a person, and Terraform grants groups access to it rather than creating
// folders of its own.
func NewFolderDataSource() datasource.DataSource { return &folderDataSource{} }

type folderDataSource struct {
	client *client.Client
}

type folderDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Path    types.String `tfsdk:"path"`
	Name    types.String `tfsdk:"name"`
	Scope   types.String `tfsdk:"scope"`
	URL     types.String `tfsdk:"url"`
	OwnerID types.String `tfsdk:"owner_id"`
}

func (d *folderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (d *folderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Looks up an existing folder by path or ID.\n\n" +
			"Use this to grant access to a folder Terraform does not manage, such as a shared hub folder " +
			"that content is published into. Referencing it here fails at plan time if it is missing, " +
			"rather than applying cleanly and granting nothing.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The folder's unique ID. Give this or `path`.",
			},
			"path": dschema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Full path, for example `reporting/finance`. Give this or `id`. " +
					"Paths are lowercased by the API.",
			},
			"name": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name.",
			},
			"scope": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "`organization` or `restricted`.",
			},
			"url": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Direct link to the folder in the Omni UI.",
			},
			"owner_id": dschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The folder's owner.",
			},
		},
	}
}

func (d *folderDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(pathExpr("id"), pathExpr("path")),
	}
}

func (d *folderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSourceRequest(req, resp)
}

func (d *folderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config folderDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || notConfigured(d.client, &resp.Diagnostics) {
		return
	}

	// Folders have no get-by-id route, so both lookups page the list.
	folders, err := d.client.ListFolders(ctx, "", "")
	if err != nil {
		resp.Diagnostics.AddError("Unable to list Omni folders", err.Error())
		return
	}

	wantID := config.ID.ValueString()
	wantPath := config.Path.ValueString()

	var found *client.Folder
	for i := range folders {
		if wantID != "" && folders[i].ID == wantID {
			found = &folders[i]
			break
		}
		if wantPath != "" && folders[i].Path == wantPath {
			found = &folders[i]
			break
		}
	}

	if found == nil {
		target := wantID
		if target == "" {
			target = wantPath
		}
		resp.Diagnostics.AddError(
			"Omni folder not found",
			fmt.Sprintf("No folder matching %q. Folder paths are lowercased by the API, so check the case.", target),
		)
		return
	}

	state := folderDataSourceModel{
		ID:      types.StringValue(found.ID),
		Path:    stringOrNull(found.Path),
		Name:    stringOrNull(found.Name),
		Scope:   stringOrNull(found.Scope),
		URL:     stringOrNull(found.URL),
		OwnerID: stringOrNull(found.OwnerID),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
