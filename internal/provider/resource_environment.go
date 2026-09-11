package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// ---------------------------------------------------------------------------
// omni_connection_environment
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                = &connectionEnvironmentResource{}
	_ resource.ResourceWithConfigure   = &connectionEnvironmentResource{}
	_ resource.ResourceWithImportState = &connectionEnvironmentResource{}
)

// NewConnectionEnvironmentResource returns the omni_connection_environment resource.
func NewConnectionEnvironmentResource() resource.Resource { return &connectionEnvironmentResource{} }

type connectionEnvironmentResource struct {
	client *client.Client
}

type connectionEnvironmentResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	BaseConnectionID        types.String `tfsdk:"base_connection_id"`
	EnvironmentConnectionID types.String `tfsdk:"environment_connection_id"`
	UserAttributeValues     types.Set    `tfsdk:"user_attribute_values"`
}

func (r *connectionEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_environment"
}

func (r *connectionEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Routes queries to an alternate connection based on a user attribute value.\n\n" +
			"This is how physical tenant isolation works for embedding: a signed URL carries a user attribute " +
			"value, and Omni sends that session's queries to the connection whose environment lists the value. " +
			"Set the attribute name on the base connection first.\n\n" +
			"**This resource cannot detect drift.** The API offers no endpoint for reading connection " +
			"environments, only create, update and delete. Terraform keeps what it wrote and cannot tell you " +
			"if someone changed or removed the environment in the UI. Treat the configuration as the record, " +
			"and check the UI if you suspect a change.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The connection environment's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"base_connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The connection users query through, which routes to the environment.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"environment_connection_id": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The connection queries are routed to when a session carries one of " +
					"`user_attribute_values`. Usually the same warehouse with a different database or schema.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_attribute_values": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Values of the base connection's environment user attribute that route " +
					"here. For per-tenant isolation this is typically one tenant identifier per environment.",
			},
		},
	}
}

func (r *connectionEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *connectionEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	values, diags := setToStrings(ctx, plan.UserAttributeValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateConnectionEnvironment(
		ctx,
		plan.BaseConnectionID.ValueString(),
		plan.EnvironmentConnectionID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni connection environment", err.Error())
		return
	}

	// Create attaches the connection; the attribute values are a second call.
	if err := r.client.SetConnectionEnvironmentValues(ctx, created.ID, values); err != nil {
		resp.Diagnostics.AddError(
			"Connection environment created but its attribute values could not be set",
			fmt.Sprintf("The environment exists with ID %s and will be adopted on the next apply. %s", created.ID, err.Error()),
		)
		return
	}

	state := plan
	state.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read is deliberately a no-op: the API has no endpoint for reading connection
// environments. Returning state unchanged is the honest behaviour. Clearing it
// would propose spurious recreates, and inventing values would be worse.
func (r *connectionEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectionEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	values, diags := setToStrings(ctx, plan.UserAttributeValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetConnectionEnvironmentValues(ctx, state.ID.ValueString(), values); err != nil {
		resp.Diagnostics.AddError("Unable to update Omni connection environment", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *connectionEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteConnectionEnvironment(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni connection environment", err.Error())
	}
}

func (r *connectionEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"Import is not supported for omni_connection_environment",
		"The Omni API has no endpoint for reading a connection environment, so an imported resource could "+
			"not be populated. Recreate it through Terraform instead.",
	)
}

// ---------------------------------------------------------------------------
// omni_connection_schedule
// ---------------------------------------------------------------------------

var (
	_ resource.Resource                = &connectionScheduleResource{}
	_ resource.ResourceWithConfigure   = &connectionScheduleResource{}
	_ resource.ResourceWithImportState = &connectionScheduleResource{}
)

// NewConnectionScheduleResource returns the omni_connection_schedule resource.
func NewConnectionScheduleResource() resource.Resource { return &connectionScheduleResource{} }

type connectionScheduleResource struct {
	client *client.Client
}

type connectionScheduleResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ConnectionID types.String `tfsdk:"connection_id"`
	Schedule     types.String `tfsdk:"schedule"`
	Timezone     types.String `tfsdk:"timezone"`
	HardRefresh  types.Bool   `tfsdk:"hard_refresh"`
	Description  types.String `tfsdk:"description"`
}

func (r *connectionScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_schedule"
}

func (r *connectionScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Schedules a schema refresh on a connection.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The schedule's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The connection to refresh.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"schedule": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "An AWS EventBridge cron expression with **six** fields: " +
					"`minute hour day-of-month month day-of-week year`. Note this is not the five-field " +
					"Unix cron format. Daily at 02:00 is `0 2 * * ? *`.",
			},
			"timezone": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IANA timezone the schedule runs in, for example `Australia/Melbourne`.",
			},
			"hard_refresh": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, the refresh discards and rebuilds the schema model. When " +
					"`false`, the default, it merges newly generated views into the existing model. A hard " +
					"refresh can drop model customisations, so leave this off unless you mean it.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable description the API derives from the cron expression.",
			},
		},
	}
}

func (r *connectionScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *connectionScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	created, err := r.client.CreateConnectionSchedule(ctx, plan.ConnectionID.ValueString(), client.ConnectionScheduleInput{
		Schedule:    plan.Schedule.ValueString(),
		Timezone:    plan.Timezone.ValueString(),
		HardRefresh: boolPtr(plan.HardRefresh),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni connection schedule", err.Error())
		return
	}

	state := plan
	applyScheduleToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	found, err := r.client.GetConnectionSchedule(ctx, state.ConnectionID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni connection schedule", err.Error())
		return
	}

	applyScheduleToState(&state, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectionScheduleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	updated, err := r.client.UpdateConnectionSchedule(
		ctx,
		state.ConnectionID.ValueString(),
		state.ID.ValueString(),
		client.ConnectionScheduleInput{
			Schedule:    plan.Schedule.ValueString(),
			Timezone:    plan.Timezone.ValueString(),
			HardRefresh: boolPtr(plan.HardRefresh),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Omni connection schedule", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	applyScheduleToState(&next, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *connectionScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionScheduleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.DeleteConnectionSchedule(ctx, state.ConnectionID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni connection schedule", err.Error())
	}
}

func (r *connectionScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	connectionID, scheduleID, found := strings.Cut(req.ID, ":")
	if !found || connectionID == "" || scheduleID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected an ID in the form <connection id>:<schedule id>, got %q", req.ID),
		)
		return
	}
	resp.State.SetAttribute(ctx, pathRoot("id"), scheduleID)
	resp.State.SetAttribute(ctx, pathRoot("connection_id"), connectionID)
}

// applyScheduleToState takes only what a response carried, following the rule
// the rest of this provider learned the hard way: writes return partial objects.
func applyScheduleToState(state *connectionScheduleResourceModel, s *client.ConnectionSchedule) {
	if s.ScheduleID != "" {
		state.ID = types.StringValue(s.ScheduleID)
	}
	if s.ConnectionID != "" {
		state.ConnectionID = types.StringValue(s.ConnectionID)
	}
	if s.Schedule != "" {
		state.Schedule = types.StringValue(s.Schedule)
	}
	if s.Timezone != "" {
		state.Timezone = types.StringValue(s.Timezone)
	}
	state.HardRefresh = types.BoolValue(s.HardRefresh)
	if s.Description != "" {
		state.Description = types.StringValue(s.Description)
	} else if state.Description.IsUnknown() {
		state.Description = types.StringNull()
	}
}
