package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &dbtEnvironmentResource{}
	_ resource.ResourceWithConfigure   = &dbtEnvironmentResource{}
	_ resource.ResourceWithImportState = &dbtEnvironmentResource{}
)

// dbtVariableNameValidators enforces the API's naming rule for dbt variables.
// The create endpoint rejects any other name with a 400, and the reference
// documentation does not state the rule, so it is checked at plan time.
var dbtVariableNameValidators = []validator.Map{
	mapvalidator.KeysAre(stringvalidator.RegexMatches(
		regexp.MustCompile(`^DBT_[A-Za-z0-9_]*$`),
		"must start with DBT_ and contain only letters, digits and underscores, which is what the "+
			"Omni API accepts for a dbt variable name",
	)),
}

// NewDbtEnvironmentResource returns the omni_dbt_environment resource.
func NewDbtEnvironmentResource() resource.Resource { return &dbtEnvironmentResource{} }

type dbtEnvironmentResource struct {
	client *client.Client
}

type dbtEnvironmentResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ConnectionID types.String `tfsdk:"connection_id"`

	Name         types.String `tfsdk:"name"`
	TargetSchema types.String `tfsdk:"target_schema"`

	TargetDatabase types.String `tfsdk:"target_database"`
	TargetName     types.String `tfsdk:"target_name"`
	TargetRole     types.String `tfsdk:"target_role"`
	OwnerID        types.String `tfsdk:"owner_id"`

	IsDeferralEnabled    types.Bool `tfsdk:"is_deferral_enabled"`
	IsDefaultEnvironment types.Bool `tfsdk:"is_default_environment"`

	Variables       types.Map `tfsdk:"variables"`
	SecretVariables types.Map `tfsdk:"secret_variables"`
}

func (r *dbtEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dbt_environment"
}

func (r *dbtEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A dbt environment on a connection, routing queries to a different target " +
			"schema. Requires `omni_dbt_configuration` on the same connection, with " +
			"`enable_virtual_schemas` set.\n\n" +
			"The first environment created on a configuration becomes its default, reported by " +
			"`is_default_environment`. Omni does not allow a default environment to be deleted on " +
			"its own: destroying one removes it from state with a warning, and the environment " +
			"itself is deleted when the `omni_dbt_configuration` is destroyed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The environment's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"connection_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The connection whose dbt project this environment belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the environment.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"target_schema": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Schema queries in this environment run against.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"target_database": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Overrides the connection's database for this environment.",
			},
			"target_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "dbt target name, matching a target in the project's `profiles.yml`.",
			},
			"target_role": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Overrides the warehouse role queries in this environment run as.",
			},
			"owner_id": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Makes this a personal environment belonging to one user. Leave unset " +
					"for an environment shared across the organization.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"is_deferral_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Defer to the production environment for models this environment has " +
					"not built. The API forces this to `false` on the default environment.",
			},
			"is_default_environment": schema.BoolAttribute{
				Computed: true,
				MarkdownDescription: "Whether this is the connection's default, production environment. " +
					"Omni decides this; it cannot be set here.",
			},
			"variables": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators:  dbtVariableNameValidators,
				MarkdownDescription: "Non-secret dbt variables, keyed by variable name. Names must start with `DBT_`. Values are " +
					"readable from the API, so drift on them is detected.",
			},
			"secret_variables": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Validators:  dbtVariableNameValidators,
				MarkdownDescription: "Secret dbt variables, keyed by variable name. Names must start with `DBT_`. The API returns `null` " +
					"for a secret value, so drift on these values is not detected: only adding or removing " +
					"a key is. Omni stores these separately from `variables`, and a name must not appear " +
					"in both.",
			},
		},
	}
}

func (r *dbtEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *dbtEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dbtEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	variables, diags := dbtVariablesFromPlan(ctx, plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateDbtEnvironment(ctx, plan.ConnectionID.ValueString(), client.DbtEnvironmentInput{
		Name:              plan.Name.ValueString(),
		TargetSchema:      plan.TargetSchema.ValueString(),
		IsDeferralEnabled: boolPtr(plan.IsDeferralEnabled),
		OwnerID:           stringPtr(plan.OwnerID),
		TargetDatabase:    stringPtr(plan.TargetDatabase),
		TargetName:        stringPtr(plan.TargetName),
		TargetRole:        stringPtr(plan.TargetRole),
		Variables:         variables,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the Omni dbt environment", err.Error())
		return
	}

	state := plan
	applyDbtEnvironmentToState(&state, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dbtEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dbtEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	found, err := r.client.GetDbtEnvironment(ctx, state.ConnectionID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the Omni dbt environment", err.Error())
		return
	}

	applyDbtEnvironmentToState(&state, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dbtEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dbtEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	// A variable that already exists must carry its ID or the update adds a
	// second variable with the same name instead of changing the first. The IDs
	// are not held in state, so they are resolved from the API by name.
	current, err := r.client.GetDbtEnvironment(ctx, state.ConnectionID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read the Omni dbt environment before updating it", err.Error())
		return
	}

	variables, diags := dbtVariablesFromPlan(ctx, plan, current.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateDbtEnvironment(
		ctx,
		state.ConnectionID.ValueString(),
		state.ID.ValueString(),
		client.DbtEnvironmentInput{
			Name:              plan.Name.ValueString(),
			TargetSchema:      plan.TargetSchema.ValueString(),
			IsDeferralEnabled: boolPtr(plan.IsDeferralEnabled),
			OwnerID:           stringPtr(plan.OwnerID),
			TargetDatabase:    stringPtr(plan.TargetDatabase),
			TargetName:        stringPtr(plan.TargetName),
			TargetRole:        stringPtr(plan.TargetRole),
			Variables:         variables,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the Omni dbt environment", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	next.ConnectionID = state.ConnectionID
	applyDbtEnvironmentToState(&next, updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *dbtEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dbtEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	err := r.client.DeleteDbtEnvironment(ctx, state.ConnectionID.ValueString(), state.ID.ValueString())
	switch {
	case err == nil, client.IsNotFound(err):
		return
	case client.IsDefaultDbtEnvironmentRefusal(err):
		// The first environment on a configuration becomes its default, and the
		// API will not delete a default on its own. It goes when the
		// configuration does. Failing here would block exactly that: Terraform
		// destroys the environment before the configuration it depends on, so
		// an error would stop the delete that removes it. Dropping it from state
		// with a warning lets the configuration delete finish the job.
		resp.Diagnostics.AddWarning(
			"dbt environment left in place as the configuration's default",
			fmt.Sprintf("Environment %q is the default environment of its dbt configuration, which "+
				"Omni does not allow to be deleted on its own. It has been removed from Terraform "+
				"state and will be deleted when the omni_dbt_configuration on this connection is "+
				"destroyed. If that configuration stays, so does this environment.",
				state.Name.ValueString()),
		)
	default:
		resp.Diagnostics.AddError("Unable to delete the Omni dbt environment", err.Error())
	}
}

func (r *dbtEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	connectionID, environmentID, found := strings.Cut(req.ID, ":")
	if !found || connectionID == "" || environmentID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected an ID in the form <connection id>:<environment id>, got %q", req.ID),
		)
		return
	}
	resp.State.SetAttribute(ctx, pathRoot("id"), environmentID)
	resp.State.SetAttribute(ctx, pathRoot("connection_id"), connectionID)
}

// dbtVariablesFromPlan flattens the two maps into the single list the API takes.
//
// existing carries the IDs of variables already on the environment, matched by
// name, so an update changes a variable rather than duplicating it. On create
// it is nil.
func dbtVariablesFromPlan(ctx context.Context, plan dbtEnvironmentResourceModel, existing []client.DbtVariable) ([]client.DbtVariable, diag.Diagnostics) {
	var diags diag.Diagnostics

	plainValues := map[string]string{}
	if !plan.Variables.IsNull() && !plan.Variables.IsUnknown() {
		diags.Append(plan.Variables.ElementsAs(ctx, &plainValues, false)...)
	}

	secretValues := map[string]string{}
	if !plan.SecretVariables.IsNull() && !plan.SecretVariables.IsUnknown() {
		diags.Append(plan.SecretVariables.ElementsAs(ctx, &secretValues, false)...)
	}
	if diags.HasError() {
		return nil, diags
	}

	for name := range secretValues {
		if _, clash := plainValues[name]; clash {
			diags.AddError(
				"Duplicate dbt variable",
				fmt.Sprintf("The variable %q is set in both variables and secret_variables. Omni stores a "+
					"variable as either secret or not, so it can only appear in one of them.", name),
			)
		}
	}
	if diags.HasError() {
		return nil, diags
	}

	ids := map[string]string{}
	for _, variable := range existing {
		ids[variable.Name] = variable.ID
	}

	// A variable already on the environment is sent by id with no name: the
	// update endpoint identifies it by id and rejects a name beside one. A new
	// variable has no id yet and is sent by name.
	variable := func(name, value string, secret bool) client.DbtVariable {
		if id := ids[name]; id != "" {
			return client.DbtVariable{ID: id, Value: &value, IsSecret: secret}
		}
		return client.DbtVariable{Name: name, Value: &value, IsSecret: secret}
	}

	out := make([]client.DbtVariable, 0, len(plainValues)+len(secretValues))
	for name, value := range plainValues {
		out = append(out, variable(name, value, false))
	}
	for name, value := range secretValues {
		out = append(out, variable(name, value, true))
	}
	return out, diags
}

// applyDbtEnvironmentToState takes only what the response carried.
//
// Variable values are deliberately not read back. A secret variable returns a
// null value, so copying the response over state would blank what the config
// set and show a permanent diff. The non-secret map is refreshed, since those
// values do come back.
func applyDbtEnvironmentToState(state *dbtEnvironmentResourceModel, e *client.DbtEnvironment) {
	if e.ID != "" {
		state.ID = types.StringValue(e.ID)
	}
	if e.Name != "" {
		state.Name = types.StringValue(e.Name)
	}
	if e.TargetSchema != "" {
		state.TargetSchema = types.StringValue(e.TargetSchema)
	}
	if e.TargetDatabase != nil && *e.TargetDatabase != "" {
		state.TargetDatabase = types.StringValue(*e.TargetDatabase)
	}
	if e.TargetName != nil && *e.TargetName != "" {
		state.TargetName = types.StringValue(*e.TargetName)
	}
	if e.TargetRole != nil && *e.TargetRole != "" {
		state.TargetRole = types.StringValue(*e.TargetRole)
	}
	if e.OwnerID != nil && *e.OwnerID != "" {
		state.OwnerID = types.StringValue(*e.OwnerID)
	}

	state.IsDeferralEnabled = types.BoolValue(e.IsDeferralEnabled)
	state.IsDefaultEnvironment = types.BoolValue(e.IsDefaultEnvironment)
}
