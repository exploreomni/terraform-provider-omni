package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

var (
	_ resource.Resource                = &connectionResource{}
	_ resource.ResourceWithConfigure   = &connectionResource{}
	_ resource.ResourceWithImportState = &connectionResource{}
)

// supportedDialects mirrors the Dialect enum in the Omni OpenAPI spec.
var supportedDialects = []string{
	"athena", "bigquery", "clickhouse", "databricks", "databricks_lakebase",
	"exasol", "mariadb", "motherduck", "mssql", "mysql", "oracle", "postgres",
	"redshift", "sap_hana", "snowflake", "starrocks", "trino",
}

// NewConnectionResource returns the omni_connection resource.
func NewConnectionResource() resource.Resource { return &connectionResource{} }

type connectionResource struct {
	client *client.Client
}

type connectionResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Dialect  types.String `tfsdk:"dialect"`
	Name     types.String `tfsdk:"name"`
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	Database types.String `tfsdk:"database"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`

	BaseRole   types.String `tfsdk:"base_role"`
	Warehouse  types.String `tfsdk:"warehouse"`
	Region     types.String `tfsdk:"region"`
	PrivateKey types.String `tfsdk:"private_key"`

	IncludeSchemas       types.String `tfsdk:"include_schemas"`
	IncludeOtherCatalogs types.String `tfsdk:"include_other_catalogs"`
	OffloadedSchemas     types.String `tfsdk:"offloaded_schemas"`
	DefaultSchema        types.String `tfsdk:"default_schema"`
	ScratchSchema        types.String `tfsdk:"scratch_schema"`

	QueryTimeoutSeconds types.Int64  `tfsdk:"query_timeout_seconds"`
	MaxBillingBytes     types.String `tfsdk:"max_billing_bytes"`
	SystemTimezone      types.String `tfsdk:"system_timezone"`
	QueryTimezone       types.String `tfsdk:"query_timezone"`

	AllowsUserSpecificTimezones       types.Bool `tfsdk:"allows_user_specific_timezones"`
	AlwaysScopeViewNames              types.Bool `tfsdk:"always_scope_view_names"`
	TrustServerCertificate            types.Bool `tfsdk:"trust_server_certificate"`
	AcceptsLicense                    types.Bool `tfsdk:"accepts_license"`
	InferRelationshipsFromColumnNames types.Bool `tfsdk:"infer_relationships_from_column_names"`
	InferRelationshipsFromForeignKeys types.Bool `tfsdk:"infer_relationships_from_foreign_keys"`
	EnableDbSemanticLayerIntegration  types.Bool `tfsdk:"enable_db_semantic_layer_integration"`
	EnableDbSemanticLayerTopics       types.Bool `tfsdk:"enable_db_semantic_layer_topics"`
	UseMachineAuth                    types.Bool `tfsdk:"use_machine_auth"`

	AuthenticationType            types.String `tfsdk:"authentication_type"`
	AwsRoleArn                    types.String `tfsdk:"aws_role_arn"`
	HostOverride                  types.String `tfsdk:"host_override"`
	OauthClientID                 types.String `tfsdk:"oauth_client_id"`
	OauthClientSecret             types.String `tfsdk:"oauth_client_secret"`
	ExternalOauthAudience         types.String `tfsdk:"external_oauth_audience"`
	ExternalOauthAuthorizationURL types.String `tfsdk:"external_oauth_authorization_url"`
	ExternalOauthTokenURL         types.String `tfsdk:"external_oauth_token_url"`

	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (r *connectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (r *connectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The update endpoint only accepts credential rotation and base_role
	// changes, so everything else forces replacement.
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	replaceBool := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a database connection. Only `password`, `private_key`, `oauth_client_secret` " +
			"and `base_role` can be changed in place; every other change replaces the connection.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The connection's unique ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dialect": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The database dialect.",
				Validators:          []validator.String{stringvalidator.OneOf(supportedDialects...)},
				PlanModifiers:       replaceString,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A descriptive name for the connection.",
				PlanModifiers:       replaceString,
			},
			"host": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Hostname or IP of the database server. For Snowflake, the account " +
					"identifier only. Not used by MotherDuck or BigQuery.",
				PlanModifiers: replaceString,
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Port for the database connection. Not used by Snowflake, MotherDuck, BigQuery, Databricks, or Athena.",
				PlanModifiers:       replaceInt,
			},
			"database": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Default database or catalog. For BigQuery, the project ID. For Athena, the data catalog.",
				PlanModifiers:       replaceString,
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Username to authenticate with. For BigQuery, the service account client email.",
				PlanModifiers:       replaceString,
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Password to authenticate with, sent as `passwordUnencrypted`. For BigQuery, " +
					"the full service account JSON. Can be rotated in place. Never returned by the API, so " +
					"drift on this value is not detected.",
			},
			"base_role": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Default role for users on this connection. " + roleNameGuidance,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"warehouse": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake warehouse, or Databricks HTTP path.",
				PlanModifiers:       replaceString,
			},
			"region": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Required for BigQuery (e.g. `us`) and Athena (e.g. `us-east-1`).",
				PlanModifiers:       replaceString,
			},
			"private_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Snowflake keypair authentication RSA private key, PEM format, 2048 bits " +
					"or more. Can be rotated in place.",
			},
			"include_schemas": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comma-separated list of schemas to include. Empty includes all schemas.",
				PlanModifiers:       replaceString,
			},
			"include_other_catalogs": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comma-separated list of additional catalogs, for dialects that support multi-catalog queries.",
				PlanModifiers:       replaceString,
			},
			"offloaded_schemas": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comma-separated list of schemas to offload.",
				PlanModifiers:       replaceString,
			},
			"default_schema": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Default schema. Required for MSSQL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"scratch_schema": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Schema used for uploaded data tables.",
				PlanModifiers:       replaceString,
			},
			"query_timeout_seconds": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Query timeout in seconds. Maximum `3600`.",
				PlanModifiers:       replaceInt,
			},
			"max_billing_bytes": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "BigQuery only. Maximum bytes billable for a query.",
				PlanModifiers:       replaceString,
			},
			"system_timezone": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "System timezone. Defaults to `UTC`.",
				PlanModifiers:       replaceString,
			},
			"query_timezone": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Query timezone. Defaults to `NONE`.",
				PlanModifiers:       replaceString,
			},
			"allows_user_specific_timezones": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether users can set their own timezone.",
				PlanModifiers:       replaceBool,
			},
			"always_scope_view_names": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Prefix generated view names with schema and catalog.",
				PlanModifiers:       replaceBool,
			},
			"trust_server_certificate": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "MSSQL, Exasol, and ClickHouse only. Trust the server certificate.",
				PlanModifiers:       replaceBool,
			},
			"accepts_license": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Oracle only. Accept the license terms.",
				PlanModifiers:       replaceBool,
			},
			"infer_relationships_from_column_names": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Infer relationships from column naming conventions during schema refresh.",
				PlanModifiers:       replaceBool,
			},
			"infer_relationships_from_foreign_keys": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Postgres and Snowflake only. Infer relationships from foreign keys.",
				PlanModifiers:       replaceBool,
			},
			"enable_db_semantic_layer_integration": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Enable Snowflake semantic views or Databricks Unity Catalog integration.",
				PlanModifiers:       replaceBool,
			},
			"enable_db_semantic_layer_topics": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake and Databricks only. Enable database semantic layer topics.",
				PlanModifiers:       replaceBool,
			},
			"use_machine_auth": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Athena and Databricks only. Use machine authentication.",
				PlanModifiers:       replaceBool,
			},
			"authentication_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "BigQuery, MSSQL, Snowflake, Databricks, and Athena only. Authentication method.",
				PlanModifiers:       replaceString,
			},
			"aws_role_arn": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Athena only. AWS role ARN to assume.",
				PlanModifiers:       replaceString,
			},
			"host_override": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake only. Override for the host value.",
				PlanModifiers:       replaceString,
			},
			"oauth_client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake and Databricks only. OAuth client ID for native OAuth.",
				PlanModifiers:       replaceString,
			},
			"oauth_client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Snowflake and Databricks only. OAuth client secret, sent as `oauthClientSecretUnencrypted`.",
				PlanModifiers:       replaceString,
			},
			"external_oauth_audience": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake only. External OAuth audience.",
				PlanModifiers:       replaceString,
			},
			"external_oauth_authorization_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake only. External OAuth authorization URL.",
				PlanModifiers:       replaceString,
			},
			"external_oauth_token_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snowflake only. External OAuth token URL.",
				PlanModifiers:       replaceString,
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the connection was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "When the connection was last updated.",
			},
		},
	}
}

func (r *connectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResourceRequest(req, resp)
}

func (r *connectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	id, err := r.client.CreateConnection(ctx, connectionInputFromPlan(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Omni connection", err.Error())
		return
	}
	if id == "" {
		resp.Diagnostics.AddError(
			"Unable to create Omni connection",
			"The API reported success but did not return a connection ID.",
		)
		return
	}

	connection, err := r.client.GetConnection(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Connection created but could not be read back", err.Error())
		return
	}

	state := plan
	applyConnectionToState(&state, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	connection, err := r.client.GetConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Omni connection", err.Error())
		return
	}
	if connection.DeletedAt != "" {
		resp.State.RemoveResource(ctx)
		return
	}

	applyConnectionToState(&state, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	update := client.ConnectionUpdate{}
	if !plan.Password.Equal(state.Password) {
		update.PasswordUnencrypted = stringPtr(plan.Password)
	}
	if !plan.PrivateKey.Equal(state.PrivateKey) {
		update.PrivateKey = stringPtr(plan.PrivateKey)
	}
	if !plan.BaseRole.Equal(state.BaseRole) {
		update.BaseRole = stringPtr(plan.BaseRole)
	}

	if update.PasswordUnencrypted != nil || update.PrivateKey != nil || update.BaseRole != nil {
		if err := r.client.UpdateConnection(ctx, state.ID.ValueString(), update); err != nil {
			resp.Diagnostics.AddError("Unable to update Omni connection", err.Error())
			return
		}
	}

	connection, err := r.client.GetConnection(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Connection updated but could not be read back", err.Error())
		return
	}

	next := plan
	next.ID = state.ID
	applyConnectionToState(&next, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *connectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || notConfigured(r.client, &resp.Diagnostics) {
		return
	}

	if err := r.client.DeleteConnection(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Omni connection", err.Error())
	}
}

func (r *connectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, pathRoot("id"), req, resp)
}

func connectionInputFromPlan(plan connectionResourceModel) client.ConnectionInput {
	return client.ConnectionInput{
		Dialect:             plan.Dialect.ValueString(),
		Name:                plan.Name.ValueString(),
		PasswordUnencrypted: plan.Password.ValueString(),

		Host:       stringPtr(plan.Host),
		Port:       int64Ptr(plan.Port),
		Database:   stringPtr(plan.Database),
		Username:   stringPtr(plan.Username),
		BaseRole:   stringPtr(plan.BaseRole),
		Warehouse:  stringPtr(plan.Warehouse),
		Region:     stringPtr(plan.Region),
		PrivateKey: stringPtr(plan.PrivateKey),

		IncludeSchemas:       stringPtr(plan.IncludeSchemas),
		IncludeOtherCatalogs: stringPtr(plan.IncludeOtherCatalogs),
		OffloadedSchemas:     stringPtr(plan.OffloadedSchemas),
		DefaultSchema:        stringPtr(plan.DefaultSchema),
		ScratchSchema:        stringPtr(plan.ScratchSchema),

		QueryTimeoutSeconds: int64Ptr(plan.QueryTimeoutSeconds),
		MaxBillingBytes:     stringPtr(plan.MaxBillingBytes),
		SystemTimezone:      stringPtr(plan.SystemTimezone),
		QueryTimezone:       stringPtr(plan.QueryTimezone),

		AllowsUserSpecificTimezones:       boolPtr(plan.AllowsUserSpecificTimezones),
		AlwaysScopeViewNames:              boolPtr(plan.AlwaysScopeViewNames),
		TrustServerCertificate:            boolPtr(plan.TrustServerCertificate),
		AcceptsLicense:                    boolPtr(plan.AcceptsLicense),
		InferRelationshipsFromColumnNames: boolPtr(plan.InferRelationshipsFromColumnNames),
		InferRelationshipsFromForeignKeys: boolPtr(plan.InferRelationshipsFromForeignKeys),
		EnableDbSemanticLayerIntegration:  boolPtr(plan.EnableDbSemanticLayerIntegration),
		EnableDbSemanticLayerTopics:       boolPtr(plan.EnableDbSemanticLayerTopics),
		UseMachineAuth:                    boolPtr(plan.UseMachineAuth),

		AuthenticationType:            stringPtr(plan.AuthenticationType),
		AwsRoleArn:                    stringPtr(plan.AwsRoleArn),
		HostOverride:                  stringPtr(plan.HostOverride),
		OauthClientID:                 stringPtr(plan.OauthClientID),
		OauthClientSecretUnencrypted:  stringPtr(plan.OauthClientSecret),
		ExternalOauthAudience:         stringPtr(plan.ExternalOauthAudience),
		ExternalOauthAuthorizationURL: stringPtr(plan.ExternalOauthAuthorizationURL),
		ExternalOauthTokenURL:         stringPtr(plan.ExternalOauthTokenURL),
	}
}

// applyConnectionToState refreshes only the fields the API returns. Credentials
// and dialect-specific settings are not echoed back, so they stay as configured.
func applyConnectionToState(state *connectionResourceModel, connection *client.Connection) {
	state.ID = types.StringValue(connection.ID)
	if connection.Name != "" {
		state.Name = types.StringValue(connection.Name)
	}
	if connection.Dialect != "" {
		state.Dialect = types.StringValue(connection.Dialect)
	}
	if connection.Database != "" {
		state.Database = types.StringValue(connection.Database)
	}
	state.BaseRole = stringOrNull(connection.BaseRole)
	state.DefaultSchema = stringOrNull(connection.DefaultSchema)
	state.CreatedAt = stringOrNull(connection.CreatedAt)
	state.UpdatedAt = stringOrNull(connection.UpdatedAt)
}
