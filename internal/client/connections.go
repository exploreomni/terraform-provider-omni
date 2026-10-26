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

	// Set when the connection is the base of a set of dynamic environments.
	// The read returns these flat, while the write takes them nested under
	// environmentUserAttribute. Both spellings are documented; only the nested
	// one is accepted on a PATCH.
	UserAttributeNameForConnectionEnvironments string   `json:"userAttributeNameForConnectionEnvironments"`
	UserAttributeValuesForDefaultEnvironment   []string `json:"userAttributeValuesForDefaultEnvironment"`
}

// EnvironmentUserAttribute names the user attribute whose value routes a query
// to a connection environment, together with the values that resolve to the
// base connection rather than to an environment.
type EnvironmentUserAttribute struct {
	AttributeName string   `json:"attributeName"`
	DefaultValues []string `json:"defaultValues"`
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
	WifAudience                   *string `json:"wifAudience,omitempty"`
	WifServiceAccountEmail        *string `json:"wifServiceAccountEmail,omitempty"`
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

// SetConnectionEnvironmentUserAttribute sets or clears the environment user
// attribute on a connection. Passing nil clears it.
//
// This is a separate call rather than a field on ConnectionUpdate because
// clearing requires sending an explicit null, which an omitempty field cannot
// express: a nil pointer is omitted, and an omitted field leaves the current
// value alone. The body is built as a map so that nil marshals to null.
//
// POST /v1/connections has no field for this, so setting it on a new connection
// means calling this after the create.
func (c *Client) SetConnectionEnvironmentUserAttribute(ctx context.Context, id string, in *EnvironmentUserAttribute) error {
	body := map[string]any{"environmentUserAttribute": in}
	return c.Patch(ctx, "/v1/connections/"+url.PathEscape(id), body, nil)
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
