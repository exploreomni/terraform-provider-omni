package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &colorPaletteResource{}
	_ resource.ResourceWithConfigure   = &colorPaletteResource{}
	_ resource.ResourceWithImportState = &colorPaletteResource{}
)

// NewColorPaletteResource returns the omni_color_palette resource.
func NewColorPaletteResource() resource.Resource { return &colorPaletteResource{} }

type colorPaletteResource struct {
	client *client.Client
}

type colorPaletteResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	Colors    types.List   `tfsdk:"colors"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *colorPaletteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_color_palette"
}

func (r *colorPaletteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a colour palette that visualisations can reference.\n\n" +
			"For embedding, a palette per tenant is how an embedded experience picks up the host " +
			"application's branding. Defining them in configuration keeps branding consistent across " +
			"instances rather than recreated by hand.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The palette's unique ID. Visualisations reference the palette by this value.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
				MarkdownDescription: "Palette name. Must be unique per type within the organization.",
			},
			"type": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{stringvalidator.OneOf("discrete", "continuous")},
				MarkdownDescription: "`discrete` palettes colour categories and are used in the order given. " +
					"`continuous` palettes define a gradient for numeric scales and interpolate between the " +
					"colours.",
			},
			"colors": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 50),
				},
				MarkdownDescription: "Ordered hex colours, for example `[\"#1f77b4\", \"#ff7f0e\"]`. Order " +
					"matters: discrete palettes assign them in sequence, continuous palettes interpolate " +
					"between adjacent values.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the palette was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *colorPaletteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *colorPaletteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan colorPaletteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	colors, diags := listToStrings(ctx, plan.Colors)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateColorPalette(ctx, client.ColorPaletteInput{
		Name:   plan.Name.ValueString(),
		Type:   plan.Type.ValueString(),
		Colors: colors,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni colour palette", err.Error())
		return
	}

	state := plan
	resp.Diagnostics.Append(applyPaletteToState(ctx, &state, created)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *colorPaletteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state colorPaletteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	palette, err := r.client.GetColorPalette(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni colour palette", err.Error())
		return
	}

	resp.Diagnostics.Append(applyPaletteToState(ctx, &state, palette)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *colorPaletteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state colorPaletteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	colors, diags := listToStrings(ctx, plan.Colors)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateColorPalette(ctx, state.ID.ValueString(), client.ColorPaletteInput{
		Name:   plan.Name.ValueString(),
		Type:   plan.Type.ValueString(),
		Colors: colors,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni colour palette", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	next.CreatedAt = state.CreatedAt
	resp.Diagnostics.Append(applyPaletteToState(ctx, &next, updated)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *colorPaletteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state colorPaletteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteColorPalette(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni colour palette", err.Error())
	}
}

func (r *colorPaletteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

// applyPaletteToState takes only what a response carried, following the rule
// the rest of this provider learned the hard way: writes return partial
// objects.
func applyPaletteToState(ctx context.Context, state *colorPaletteResourceModel, p *client.ColorPalette) diagnostics {
	var diags diagnostics

	if p.ID != "" {
		state.ID = types.StringValue(p.ID)
	}
	if p.Name != "" {
		state.Name = types.StringValue(p.Name)
	}
	if p.Type != "" {
		state.Type = types.StringValue(p.Type)
	}
	if p.CreatedAt != "" {
		state.CreatedAt = types.StringValue(p.CreatedAt)
	} else if state.CreatedAt.IsUnknown() {
		state.CreatedAt = types.StringNull()
	}

	if len(p.Colors) > 0 {
		value, d := types.ListValueFrom(ctx, types.StringType, p.Colors)
		diags.Append(d...)
		if !diags.HasError() {
			state.Colors = value
		}
	}
	return diags
}
