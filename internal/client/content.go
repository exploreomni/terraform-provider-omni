package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Folder is a content folder.
type Folder struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Path           string       `json:"path"`
	Scope          string       `json:"scope"`
	OwnerID        string       `json:"ownerId"`
	BreadcrumbRoot bool         `json:"breadcrumbRoot"`
	Owner          *FolderOwner `json:"owner,omitempty"`
	URL            string       `json:"url,omitempty"`
}

// FolderOwner is the owner block returned by the list endpoint.
type FolderOwner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// FolderInput is the create body for POST /v1/folders.
type FolderInput struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId,omitempty"`
	Scope          *string `json:"scope,omitempty"`
	UserID         *string `json:"userId,omitempty"`
	BreadcrumbRoot *bool   `json:"breadcrumbRoot,omitempty"`
}

// FolderUpdate is the PATCH body for a folder. At least one of Name or Path
// must be set.
type FolderUpdate struct {
	Name                *string `json:"name,omitempty"`
	Path                *string `json:"path,omitempty"`
	ResolvePathConflict *bool   `json:"resolvePathConflict,omitempty"`
	BreadcrumbRoot      *bool   `json:"breadcrumbRoot,omitempty"`
}

type listFoldersResponse struct {
	Records  []Folder  `json:"records"`
	PageInfo *PageInfo `json:"pageInfo,omitempty"`
}

// PageInfo is the cursor pagination block used by list endpoints.
type PageInfo struct {
	HasNextPage  bool   `json:"hasNextPage"`
	NextCursor   string `json:"nextCursor"`
	PageSize     int    `json:"pageSize"`
	TotalRecords int    `json:"totalRecords"`
}

// CreateFolder creates a folder.
func (c *Client) CreateFolder(ctx context.Context, in FolderInput) (*Folder, error) {
	var out Folder
	if err := c.Post(ctx, "/v1/folders", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFolder renames a folder and/or changes its path segment.
func (c *Client) UpdateFolder(ctx context.Context, id string, in FolderUpdate) (*Folder, error) {
	var out Folder
	if err := c.Patch(ctx, "/v1/folders/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteFolder deletes a folder. force recursively deletes contents.
func (c *Client) DeleteFolder(ctx context.Context, id string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	return c.Delete(ctx, "/v1/folders/"+url.PathEscape(id), q)
}

// ListFolders pages through /v1/folders for the given scope. ownerID is required
// by the API for restricted scope when using an organization API key.
func (c *Client) ListFolders(ctx context.Context, scope, ownerID string) ([]Folder, error) {
	var all []Folder
	cursor := ""

	for {
		q := url.Values{}
		q.Set("pageSize", "100")
		if scope != "" {
			q.Set("scope", scope)
		}
		if ownerID != "" {
			q.Set("ownerId", ownerID)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		var page listFoldersResponse
		if err := c.Get(ctx, "/v1/folders", q, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Records...)

		if page.PageInfo == nil || !page.PageInfo.HasNextPage || page.PageInfo.NextCursor == "" {
			return all, nil
		}
		cursor = page.PageInfo.NextCursor
	}
}

// GetFolder finds a folder by ID. The API has no get-by-id route for folders, so
// this pages the list endpoint and matches on ID.
//
// ownerID is only sent for restricted scope, where an organization API key is
// required to supply it. For organization scope an organization key returns
// every folder when ownerID is omitted, but filters to a single user's folders
// when it is present, which would hide the folder we are looking for.
func (c *Client) GetFolder(ctx context.Context, id, scope, ownerID string) (*Folder, error) {
	if scope != "restricted" {
		ownerID = ""
	}

	folders, err := c.ListFolders(ctx, scope, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range folders {
		if folders[i].ID == id {
			return &folders[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/v1/folders",
		Message:    fmt.Sprintf("no folder found with id %q in scope %q", id, scope),
	}
}

// Model is a shared, shared extension, branch, or schema model.
type Model struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ModelKind    string `json:"modelKind"`
	ConnectionID string `json:"connectionId"`
	BaseModelID  string `json:"baseModelId"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	DeletedAt    string `json:"deletedAt"`
}

// ModelInput is the create body for POST /v1/models.
//
// accessGrants is intentionally absent: access grants are model content,
// declared in model YAML and versioned through git sync.
type ModelInput struct {
	ConnectionID         string `json:"connectionId,omitempty"`
	BaseModelID          string `json:"baseModelId,omitempty"`
	ModelKind            string `json:"modelKind"`
	ModelName            string `json:"modelName,omitempty"`
	AllowAsWorkbookBase  *bool  `json:"allowAsWorkbookBase,omitempty"`
	UsesIsolatedBranches *bool  `json:"usesIsolatedBranches,omitempty"`
}

// UserAttribute is a user attribute definition.
//
// The API exposes these read-only. Definitions are created in the UI under
// Settings > User attributes, which means the schema that row-level security
// keys off cannot be managed as code.
type UserAttribute struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Label          string  `json:"label"`
	Description    *string `json:"description"`
	Type           string  `json:"type"`
	DefaultValue   *string `json:"default_value"`
	MultipleValues bool    `json:"multiple_values"`
	System         bool    `json:"system"`
}

type listUserAttributesResponse struct {
	Records []UserAttribute `json:"records"`
}

// ListUserAttributes returns every user attribute definition, including the
// built-in ones Omni populates itself.
func (c *Client) ListUserAttributes(ctx context.Context) ([]UserAttribute, error) {
	var out listUserAttributesResponse
	if err := c.Get(ctx, "/v1/user-attributes", nil, &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// FindUserAttribute looks up a definition by its reference name.
// FindUserAttributeByID looks a definition up by its ID rather than its name.
//
// An ID is the stable handle: a definition can be renamed in the UI, and a
// configuration that refers to it by name silently stops matching. Referring by
// ID means a rename is invisible to Terraform, which is what you want, and a
// deletion fails loudly, which is also what you want.
func (c *Client) FindUserAttributeByID(ctx context.Context, id string) (*UserAttribute, error) {
	attrs, err := c.ListUserAttributes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range attrs {
		if attrs[i].ID == id {
			return &attrs[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/v1/user-attributes",
		Message:    fmt.Sprintf("no user attribute with ID %q. Definitions are created in the UI under Settings > User attributes; the API has no create endpoint", id),
	}
}

func (c *Client) FindUserAttribute(ctx context.Context, name string) (*UserAttribute, error) {
	attrs, err := c.ListUserAttributes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range attrs {
		if attrs[i].Name == name {
			return &attrs[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/v1/user-attributes",
		Message:    fmt.Sprintf("no user attribute named %q. Definitions are created in the UI under Settings > User attributes; the API has no create endpoint", name),
	}
}

type modelResponse struct {
	Model Model `json:"model"`
}

type listModelsResponse struct {
	Records  []Model   `json:"records"`
	Models   []Model   `json:"models"`
	PageInfo *PageInfo `json:"pageInfo,omitempty"`
}

// CreateModel creates a schema, shared or shared extension model.
//
// A model built on a connection needs that connection's schema model to be
// refreshed, not merely to exist. The refresh is asynchronous: creating a
// SCHEMA model registers it and starts the introspection, and POST /v1/models
// answers a bare 404 for a shared model until that has produced something.
//
// So a 404 here is retried. It means "not ready yet" far more often than it
// means "wrong id", and when it really is a wrong id the budget expires and the
// error says so. 429 is retried for the same reason the schedule create does:
// the inner backoff in Do can run out while the outer loop can afford to wait.
//
// A SCHEMA model create is not subject to this and succeeds on the first
// attempt, so the budget only costs time in the case it exists for.
//
// 90 seconds: the refresh on a warehouse of this size settles well inside that.
// A cold or very large warehouse may need longer, and the error says what to
// check rather than leaving the cause to be guessed.
func (c *Client) CreateModel(ctx context.Context, in ModelInput) (*Model, error) {
	const (
		attempts = 10
		budget   = 90 * time.Second
	)

	deadline := time.Now().Add(budget)

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if time.Now().After(deadline) {
				break
			}
			// 3, 6, 9 ... capped at 20, so the budget goes on waiting rather
			// than on hammering the endpoint.
			wait := time.Duration(attempt*3) * time.Second
			if wait > 20*time.Second {
				wait = 20 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		var out modelResponse
		err := c.Post(ctx, "/v1/models", in, &out)
		if err == nil {
			return &out.Model, nil
		}
		lastErr = err
		if !IsNotFound(err) && !IsRateLimited(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf(
		"creating a %s model on connection %s still failed after waiting up to %s. "+
			"A model needs its connection's schema model to have been refreshed, not just "+
			"created, and the refresh is asynchronous. If the connection's schema is empty in "+
			"Omni, refresh it and apply again: %w",
		in.ModelKind, in.ConnectionID, budget, lastErr,
	)
}

// RefreshModel triggers a schema refresh on a model.
//
// This is what makes a newly created SCHEMA model useful: creating the model
// registers it, introspecting the warehouse is a separate step. The UI's
// "Build Schema" button does both.
//
// The call starts the refresh rather than waiting for it, so a success here
// means Omni accepted the request, not that the schema is populated yet.
func (c *Client) RefreshModel(ctx context.Context, id string) error {
	return c.Post(ctx, "/v1/models/"+url.PathEscape(id)+"/refresh", struct{}{}, nil)
}

// RenameModel renames a model. Workbook and query models cannot be renamed.
//
// The PATCH response shape is not dependable: it may be the model, may wrap it
// under "model", and may be empty. There is no get-by-id route for models, so
// when the response yields no ID the model is located through the list
// endpoint rather than returning a hollow object.
func (c *Client) RenameModel(ctx context.Context, id, name string) (*Model, error) {
	body := map[string]string{"name": name}

	var raw json.RawMessage
	if err := c.Patch(ctx, "/v1/models/"+url.PathEscape(id), body, &raw); err != nil {
		return nil, err
	}

	var flat Model
	if err := json.Unmarshal(raw, &flat); err == nil && flat.ID != "" {
		return &flat, nil
	}

	var wrapped modelResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Model.ID != "" {
		return &wrapped.Model, nil
	}

	refreshed, err := c.GetModel(ctx, id, "")
	if err != nil {
		return nil, fmt.Errorf("model renamed but could not be read back: %w", err)
	}
	return refreshed, nil
}

// DeleteModel archives a shared or shared extension model (soft delete).
func (c *Client) DeleteModel(ctx context.Context, id string) error {
	return c.Delete(ctx, "/v1/models/"+url.PathEscape(id), nil)
}

// ListModels pages through /v1/models, optionally filtered by model kind.
func (c *Client) ListModels(ctx context.Context, modelKind string) ([]Model, error) {
	var all []Model
	cursor := ""

	for {
		q := url.Values{}
		q.Set("pageSize", "100")
		if modelKind != "" {
			q.Set("modelKind", modelKind)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		var page listModelsResponse
		if err := c.Get(ctx, "/v1/models", q, &page); err != nil {
			return nil, err
		}
		records := page.Records
		if len(records) == 0 {
			records = page.Models
		}
		all = append(all, records...)

		if page.PageInfo == nil || !page.PageInfo.HasNextPage || page.PageInfo.NextCursor == "" {
			return all, nil
		}
		cursor = page.PageInfo.NextCursor
	}
}

// GetModel finds a model by ID across the given kind (empty means all kinds).
func (c *Client) GetModel(ctx context.Context, id, modelKind string) (*Model, error) {
	models, err := c.ListModels(ctx, modelKind)
	if err != nil {
		return nil, err
	}
	for i := range models {
		if models[i].ID == id {
			return &models[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/v1/models",
		Message:    fmt.Sprintf("no model found with id %q", id),
	}
}
