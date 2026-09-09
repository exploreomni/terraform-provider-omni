package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// UserAttributeSchema is the SCIM extension urn Omni uses for user attributes.
const UserAttributeSchema = "urn:omni:params:1.0:UserAttribute"

// User is the SCIM 2.0 user resource returned by /scim/v2/users.
type User struct {
	ID             string         `json:"id,omitempty"`
	UserName       string         `json:"userName,omitempty"`
	DisplayName    string         `json:"displayName,omitempty"`
	Active         *bool          `json:"active,omitempty"`
	Emails         []UserEmail    `json:"emails,omitempty"`
	Groups         []GroupRef     `json:"groups,omitempty"`
	Meta           *Meta          `json:"meta,omitempty"`
	Schemas        []string       `json:"schemas,omitempty"`
	UserAttributes map[string]any `json:"urn:omni:params:1.0:UserAttribute,omitempty"`
}

// UserEmail is a SCIM email entry.
type UserEmail struct {
	Primary bool   `json:"primary,omitempty"`
	Value   string `json:"value,omitempty"`
}

// GroupRef is a SCIM reference to a group.
type GroupRef struct {
	Value   string `json:"value,omitempty"`
	Display string `json:"display,omitempty"`
}

// Meta is the SCIM meta block.
type Meta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Created      string `json:"created,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

// UserInput is the create/replace body for a user.
type UserInput struct {
	UserName       string         `json:"userName"`
	DisplayName    string         `json:"displayName"`
	UserAttributes map[string]any `json:"urn:omni:params:1.0:UserAttribute,omitempty"`
}

// Group is the SCIM 2.0 group resource returned by /scim/v2/groups.
type Group struct {
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	Members     []GroupMembr `json:"members,omitempty"`
	Meta        *Meta        `json:"meta,omitempty"`
	Schemas     []string     `json:"schemas,omitempty"`
}

// GroupMembr is a member entry on a SCIM group.
type GroupMembr struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
}

// GroupInput is the create/replace body for a group.
type GroupInput struct {
	DisplayName string       `json:"displayName"`
	Members     []GroupMembr `json:"members"`
}

type scimListResponse[T any] struct {
	TotalResults int `json:"totalResults"`
	Resources    []T `json:"Resources"`
}

// CreateUser provisions a user. Requires an organization API key.
func (c *Client) CreateUser(ctx context.Context, in UserInput) (*User, error) {
	var out User
	if err := c.Post(ctx, "/scim/v2/users", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetUser retrieves a user by ID.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	var out User
	if err := c.Get(ctx, "/scim/v2/users/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReplaceUser applies a SCIM PUT, replacing the whole resource.
func (c *Client) ReplaceUser(ctx context.Context, id string, in UserInput) (*User, error) {
	var out User
	if err := c.Put(ctx, "/scim/v2/users/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteUser removes a user.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.Delete(ctx, "/scim/v2/users/"+url.PathEscape(id), nil)
}

// FindUserByUserName looks a user up by email address (SCIM userName).
func (c *Client) FindUserByUserName(ctx context.Context, userName string) (*User, error) {
	q := url.Values{}
	q.Set("filter", fmt.Sprintf("userName eq %q", userName))

	var out scimListResponse[User]
	if err := c.Get(ctx, "/scim/v2/users", q, &out); err != nil {
		return nil, err
	}
	for i := range out.Resources {
		if out.Resources[i].UserName == userName {
			return &out.Resources[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/scim/v2/users",
		Message:    fmt.Sprintf("no user found with userName %q", userName),
	}
}

// CreateGroup creates a user group. Requires an organization API key.
func (c *Client) CreateGroup(ctx context.Context, in GroupInput) (*Group, error) {
	var out Group
	if err := c.Post(ctx, "/scim/v2/groups", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGroup retrieves a user group by ID.
func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	var out Group
	if err := c.Get(ctx, "/scim/v2/groups/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReplaceGroup applies a SCIM PUT, replacing name and membership.
func (c *Client) ReplaceGroup(ctx context.Context, id string, in GroupInput) (*Group, error) {
	var out Group
	if err := c.Put(ctx, "/scim/v2/groups/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteGroup removes a user group.
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.Delete(ctx, "/scim/v2/groups/"+url.PathEscape(id), nil)
}

// FindGroupByDisplayName looks a group up by name.
func (c *Client) FindGroupByDisplayName(ctx context.Context, displayName string) (*Group, error) {
	q := url.Values{}
	q.Set("filter", fmt.Sprintf("displayName eq %q", displayName))

	var out scimListResponse[Group]
	if err := c.Get(ctx, "/scim/v2/groups", q, &out); err != nil {
		return nil, err
	}
	for i := range out.Resources {
		if out.Resources[i].DisplayName == displayName {
			return &out.Resources[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/scim/v2/groups",
		Message:    fmt.Sprintf("no user group found with displayName %q", displayName),
	}
}

// ModelRoleInput assigns a role on a model or connection.
type ModelRoleInput struct {
	ConnectionID string `json:"connectionId,omitempty"`
	ModelID      string `json:"modelId,omitempty"`
	RoleName     string `json:"roleName"`
}

// ModelRole is a resolved role assignment.
type ModelRole struct {
	UserID       string `json:"userId,omitempty"`
	UserGroupID  string `json:"userGroupId,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
	ModelID      string `json:"modelId,omitempty"`
	RoleName     string `json:"roleName,omitempty"`
	// Source distinguishes direct assignments from group-inherited ones on
	// endpoints that report it.
	Source string `json:"source,omitempty"`
}

// AssignUserModelRole assigns or updates a user's role on a model or connection.
func (c *Client) AssignUserModelRole(ctx context.Context, userID string, in ModelRoleInput) (*ModelRole, error) {
	var out ModelRole
	path := "/v1/users/" + url.PathEscape(userID) + "/model-roles"
	if err := c.Post(ctx, path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListUserModelRoles returns a user's role assignments, direct and inherited.
func (c *Client) ListUserModelRoles(ctx context.Context, userID string) ([]ModelRole, error) {
	path := "/v1/users/" + url.PathEscape(userID) + "/model-roles"
	return c.listModelRoles(ctx, path)
}

// AssignGroupModelRole assigns or updates a group's role on a model or connection.
func (c *Client) AssignGroupModelRole(ctx context.Context, groupID string, in ModelRoleInput) (*ModelRole, error) {
	var out ModelRole
	path := "/v1/user-groups/" + url.PathEscape(groupID) + "/model-roles"
	if err := c.Post(ctx, path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGroupModelRoles returns a group's role assignments.
func (c *Client) ListGroupModelRoles(ctx context.Context, groupID string) ([]ModelRole, error) {
	path := "/v1/user-groups/" + url.PathEscape(groupID) + "/model-roles"
	return c.listModelRoles(ctx, path)
}

// listModelRoles tolerates the response being a bare array or an object wrapping
// the array under one of a few keys.
func (c *Client) listModelRoles(ctx context.Context, path string) ([]ModelRole, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, path, nil, &raw); err != nil {
		return nil, err
	}

	var direct []ModelRole
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}

	var wrapped struct {
		ModelRoles []ModelRole `json:"modelRoles"`
		Records    []ModelRole `json:"records"`
		Roles      []ModelRole `json:"roles"`
		Data       []ModelRole `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decoding model roles from %s: %w", path, err)
	}
	for _, candidate := range [][]ModelRole{wrapped.ModelRoles, wrapped.Records, wrapped.Roles, wrapped.Data} {
		if candidate != nil {
			return candidate, nil
		}
	}
	return nil, nil
}
