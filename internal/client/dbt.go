package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DbtVariable is one variable on a dbt environment.
//
// Value is a pointer because the API returns null for a secret variable rather
// than omitting it, and because sending null on an update is how you keep the
// current value instead of clearing it.
//
// On update, an existing variable is identified by ID alone. The API rejects
// name alongside an id with "variables: Unrecognized key: name", so Name is
// omitted when empty and callers leave it empty for a variable that has an ID.
type DbtVariable struct {
	ID       string  `json:"id,omitempty"`
	Name     string  `json:"name,omitempty"`
	Value    *string `json:"value"`
	IsSecret bool    `json:"isSecret"`
}

// DbtConfiguration is the dbt project attached to a connection.
//
// GET returns 200 with only a message when dbt is not configured, rather than a
// 404, so Message is the signal that there is nothing here. Everything else is
// conditional on the auth method: publicKey and commitSigningPublicKey appear
// for ssh, githubAppInstallationId for github_app, and the token never comes
// back at all.
type DbtConfiguration struct {
	SupportsDbt          bool `json:"supportsDbt"`
	AutogenRelationships bool `json:"autogenRelationships"`
	EnableVirtualSchemas bool `json:"enableVirtualSchemas"`
	EnableSemanticLayer  bool `json:"enableSemanticLayer"`

	SSHURL          string  `json:"sshUrl"`
	Branch          string  `json:"branch"`
	AuthMethod      string  `json:"authMethod"`
	DbtVersion      *string `json:"dbtVersion"`
	ProjectRootPath *string `json:"projectRootPath"`

	CommitterName           string `json:"committerName"`
	CommitterEmail          string `json:"committerEmail"`
	GithubAppInstallationID string `json:"githubAppInstallationId"`

	PublicKey              string `json:"publicKey"`
	CommitSigningPublicKey string `json:"commitSigningPublicKey"`

	Message string `json:"message"`
}

// Configured reports whether the connection actually has dbt set up.
func (d *DbtConfiguration) Configured() bool {
	return d != nil && d.SSHURL != ""
}

// DbtConfigurationInput is the PUT body. The first four fields are required by
// the API and are sent unconditionally; the rest are pointers so an unset value
// is omitted rather than sent as a zero value that would overwrite.
type DbtConfigurationInput struct {
	AutogenRelationships bool   `json:"autogenRelationships"`
	Branch               string `json:"branch"`
	EnableVirtualSchemas bool   `json:"enableVirtualSchemas"`
	SSHURL               string `json:"sshUrl"`

	AuthMethod              *string `json:"authMethod,omitempty"`
	CommitterEmail          *string `json:"committerEmail,omitempty"`
	CommitterName           *string `json:"committerName,omitempty"`
	DbtVersion              *string `json:"dbtVersion,omitempty"`
	EnableSemanticLayer     *bool   `json:"enableSemanticLayer,omitempty"`
	GithubAppInstallationID *string `json:"githubAppInstallationId,omitempty"`
	ProjectRootPath         *string `json:"projectRootPath,omitempty"`
	RotateKeys              *bool   `json:"rotateKeys,omitempty"`
	Token                   *string `json:"token,omitempty"`
}

// DbtEnvironment is a target schema a dbt connection can route queries to.
type DbtEnvironment struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	IsDefaultEnvironment bool          `json:"isDefaultEnvironment"`
	IsDeferralEnabled    bool          `json:"isDeferralEnabled"`
	OwnerID              *string       `json:"ownerId"`
	TargetDatabase       *string       `json:"targetDatabase"`
	TargetName           *string       `json:"targetName"`
	TargetRole           *string       `json:"targetRole"`
	TargetSchema         string        `json:"targetSchema"`
	Variables            []DbtVariable `json:"variables"`
}

// DbtEnvironmentInput is the create and update body.
type DbtEnvironmentInput struct {
	Name              string        `json:"name"`
	TargetSchema      string        `json:"targetSchema"`
	IsDeferralEnabled *bool         `json:"isDeferralEnabled,omitempty"`
	OwnerID           *string       `json:"ownerId,omitempty"`
	TargetDatabase    *string       `json:"targetDatabase,omitempty"`
	TargetName        *string       `json:"targetName,omitempty"`
	TargetRole        *string       `json:"targetRole,omitempty"`
	Variables         []DbtVariable `json:"variables,omitempty"`
}

func connectionDbtPath(connectionID string) string {
	return "/v1/connections/" + url.PathEscape(connectionID) + "/dbt"
}

func dbtEnvironmentsPath(connectionID string) string {
	return connectionDbtPath(connectionID) + "/environments"
}

func dbtEnvironmentPath(connectionID, environmentID string) string {
	return dbtEnvironmentsPath(connectionID) + "/" + url.PathEscape(environmentID)
}

// GetDbtConfiguration reads the dbt configuration on a connection.
//
// A connection without dbt returns 200 and a message rather than a 404, so
// callers check Configured rather than relying on IsNotFound.
func (c *Client) GetDbtConfiguration(ctx context.Context, connectionID string) (*DbtConfiguration, error) {
	var out DbtConfiguration
	if err := c.Get(ctx, connectionDbtPath(connectionID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetDbtConfiguration creates or replaces the dbt configuration on a
// connection. The endpoint is a PUT with no separate create, so the same call
// serves both.
func (c *Client) SetDbtConfiguration(ctx context.Context, connectionID string, in DbtConfigurationInput) (*DbtConfiguration, error) {
	var out DbtConfiguration
	if err := c.Put(ctx, connectionDbtPath(connectionID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDbtConfiguration removes the dbt configuration from a connection.
func (c *Client) DeleteDbtConfiguration(ctx context.Context, connectionID string) error {
	return c.Delete(ctx, connectionDbtPath(connectionID), nil)
}

// ListDbtEnvironments returns every dbt environment on a connection.
//
// The reference documents a bare array. In practice the endpoint returns a
// paginated object, {pageInfo, records}, twenty to a page. Both shapes are
// accepted, and pages are followed by cursor: reading only the first page
// would make an environment beyond the twentieth look deleted, and Terraform
// would drop it from state.
func (c *Client) ListDbtEnvironments(ctx context.Context, connectionID string) ([]DbtEnvironment, error) {
	var all []DbtEnvironment
	cursor := ""

	for {
		var query url.Values
		if cursor != "" {
			query = url.Values{}
			query.Set("cursor", cursor)
		}

		var raw json.RawMessage
		if err := c.Get(ctx, dbtEnvironmentsPath(connectionID), query, &raw); err != nil {
			return nil, err
		}

		var bare []DbtEnvironment
		if err := json.Unmarshal(raw, &bare); err == nil {
			return append(all, bare...), nil
		}

		var page struct {
			Records      []DbtEnvironment `json:"records"`
			Environments []DbtEnvironment `json:"environments"`
			PageInfo     struct {
				HasNextPage bool    `json:"hasNextPage"`
				NextCursor  *string `json:"nextCursor"`
			} `json:"pageInfo"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Records...)
		all = append(all, page.Environments...)

		if !page.PageInfo.HasNextPage || page.PageInfo.NextCursor == nil || *page.PageInfo.NextCursor == "" {
			return all, nil
		}
		cursor = *page.PageInfo.NextCursor
	}
}

// IsDefaultDbtEnvironmentRefusal reports whether err is the API refusing to
// delete a configuration's default environment.
//
// The first environment created on a dbt configuration becomes its default,
// and the default cannot be deleted on its own: the API answers 400 "Cannot
// delete the repo's default environment". It is removed with the
// configuration, which deletes every environment beneath it.
func IsDefaultDbtEnvironmentRefusal(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(apiErr.Message+" "+apiErr.Body), "default environment")
}

// GetDbtEnvironment finds one environment by ID.
//
// There is no get-by-id route for dbt environments, so this pages the list and
// matches. Unlike the folders case this is not a fallback: the list is the only
// read available.
func (c *Client) GetDbtEnvironment(ctx context.Context, connectionID, environmentID string) (*DbtEnvironment, error) {
	environments, err := c.ListDbtEnvironments(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	for i := range environments {
		if environments[i].ID == environmentID {
			return &environments[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       dbtEnvironmentsPath(connectionID),
		Message:    fmt.Sprintf("no dbt environment found with id %q on connection %s", environmentID, connectionID),
	}
}

// CreateDbtEnvironment adds an environment to a connection's dbt configuration.
func (c *Client) CreateDbtEnvironment(ctx context.Context, connectionID string, in DbtEnvironmentInput) (*DbtEnvironment, error) {
	var out DbtEnvironment
	if err := c.Post(ctx, dbtEnvironmentsPath(connectionID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDbtEnvironment replaces an environment's settings.
//
// A variable that already exists must carry its ID or the update creates a
// duplicate instead of changing it. A variable whose Value is nil keeps the
// value it already has, which is the only way to update a secret variable
// without knowing its value.
func (c *Client) UpdateDbtEnvironment(ctx context.Context, connectionID, environmentID string, in DbtEnvironmentInput) (*DbtEnvironment, error) {
	var out DbtEnvironment
	if err := c.Put(ctx, dbtEnvironmentPath(connectionID, environmentID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteDbtEnvironment removes an environment.
func (c *Client) DeleteDbtEnvironment(ctx context.Context, connectionID, environmentID string) error {
	return c.Delete(ctx, dbtEnvironmentPath(connectionID, environmentID), nil)
}
