package client

import (
	"context"
	"net/url"
)

// Connection is a database connection as returned by the API. Credentials are
// never returned.
type Connection struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Dialect       string `json:"dialect"`
	Database      string `json:"database"`
	DefaultSchema string `json:"defaultSchema"`
	BaseRole      string `json:"baseRole"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	DeletedAt     string `json:"deletedAt"`

	UserAttributeNameForConnectionEnvironments   string `json:"userAttributeNameForConnectionEnvironments"`
	UserAttributeValuesForDefaultEnvironment     string `json:"userAttributeValuesForDefaultEnvironment"`
	BranchConnectionEnvironmentOverridesUserAttr bool   `json:"branchConnectionEnvironmentOverridesUserAttr"`
	EnvironmentConnectionSwitchesSchemaModel     bool   `json:"environmentConnectionSwitchesSchemaModel"`
}

// ConnectionInput is the create body for POST /v1/connections. Optional fields
// are pointers so that unset values are omitted rather than sent as zero values.
type ConnectionInput struct {
	Dialect             string `json:"dialect"`
	Name                string `json:"name"`
	PasswordUnencrypted string `json:"passwordUnencrypted,omitempty"`

	Host       *string `json:"host,omitempty"`
	Port       *int64  `json:"port,omitempty"`
	Database   *string `json:"database,omitempty"`
	Username   *string `json:"username,omitempty"`
	BaseRole   *string `json:"baseRole,omitempty"`
	Warehouse  *string `json:"warehouse,omitempty"`
	Region     *string `json:"region,omitempty"`
	PrivateKey *string `json:"privateKey,omitempty"`

	IncludeSchemas       *string `json:"includeSchemas,omitempty"`
	IncludeOtherCatalogs *string `json:"includeOtherCatalogs,omitempty"`
	OffloadedSchemas     *string `json:"offloadedSchemas,omitempty"`
	DefaultSchema        *string `json:"defaultSchema,omitempty"`
	ScratchSchema        *string `json:"scratchSchema,omitempty"`

	QueryTimeoutSeconds *int64  `json:"queryTimeoutSeconds,omitempty"`
	MaxBillingBytes     *string `json:"maxBillingBytes,omitempty"`
	SystemTimezone      *string `json:"systemTimezone,omitempty"`
	QueryTimezone       *string `json:"queryTimezone,omitempty"`

	AllowsUserSpecificTimezones       *bool `json:"allowsUserSpecificTimezones,omitempty"`
	AlwaysScopeViewNames              *bool `json:"alwaysScopeViewNames,omitempty"`
	TrustServerCertificate            *bool `json:"trustServerCertificate,omitempty"`
	AcceptsLicense                    *bool `json:"acceptsLicense,omitempty"`
	InferRelationshipsFromColumnNames *bool `json:"inferRelationshipsFromColumnNames,omitempty"`
	InferRelationshipsFromForeignKeys *bool `json:"inferRelationshipsFromForeignKeys,omitempty"`
	EnableDbSemanticLayerIntegration  *bool `json:"enableDbSemanticLayerIntegration,omitempty"`
	EnableDbSemanticLayerTopics       *bool `json:"enableDbSemanticLayerTopics,omitempty"`
	UseMachineAuth                    *bool `json:"useMachineAuth,omitempty"`

	AuthenticationType            *string `json:"authenticationType,omitempty"`
	AwsRoleArn                    *string `json:"awsRoleArn,omitempty"`
	HostOverride                  *string `json:"hostOverride,omitempty"`
	OauthClientID                 *string `json:"oauthClientId,omitempty"`
	OauthClientSecretUnencrypted  *string `json:"oauthClientSecretUnencrypted,omitempty"`
	ExternalOauthAudience         *string `json:"externalOauthAudience,omitempty"`
	ExternalOauthAuthorizationURL *string `json:"externalOauthAuthorizationUrl,omitempty"`
	ExternalOauthTokenURL         *string `json:"externalOauthTokenUrl,omitempty"`
}

// ConnectionUpdate is the PATCH body. The API only supports rotating credentials
// and changing the base role; every other field requires replacing the
// connection.
type ConnectionUpdate struct {
	PasswordUnencrypted *string `json:"passwordUnencrypted,omitempty"`
	PrivateKey          *string `json:"privateKey,omitempty"`
	BaseRole            *string `json:"baseRole,omitempty"`
}

type createConnectionResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data"`
}

type getConnectionResponse struct {
	Connection Connection `json:"connection"`
}

type listConnectionsResponse struct {
	Records []Connection `json:"records"`
	// Some deployments return a bare list under "connections".
	Connections []Connection `json:"connections"`
}

// CreateConnection creates a database connection and returns its ID.
func (c *Client) CreateConnection(ctx context.Context, in ConnectionInput) (string, error) {
	var out createConnectionResponse
	if err := c.Post(ctx, "/v1/connections", in, &out); err != nil {
		return "", err
	}
	return out.Data, nil
}

// GetConnection fetches a single connection by ID.
func (c *Client) GetConnection(ctx context.Context, id string) (*Connection, error) {
	var out getConnectionResponse
	if err := c.Get(ctx, "/v1/connections/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Connection, nil
}

// UpdateConnection rotates credentials and/or updates the base role.
func (c *Client) UpdateConnection(ctx context.Context, id string, in ConnectionUpdate) error {
	return c.Patch(ctx, "/v1/connections/"+url.PathEscape(id), in, nil)
}

// DeleteConnection deletes a connection.
func (c *Client) DeleteConnection(ctx context.Context, id string) error {
	return c.Delete(ctx, "/v1/connections/"+url.PathEscape(id), nil)
}

// ListConnections returns all connections visible to the caller.
func (c *Client) ListConnections(ctx context.Context) ([]Connection, error) {
	var out listConnectionsResponse
	if err := c.Get(ctx, "/v1/connections", nil, &out); err != nil {
		return nil, err
	}
	if len(out.Records) > 0 {
		return out.Records, nil
	}
	return out.Connections, nil
}
