package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
func (c *Client) GetFolder(ctx context.Context, id, scope, ownerID string) (*Folder, error) {
	// ownerID is only sent for restricted scope. For organization scope an
	// organization API key returns every folder when ownerID is omitted, but
	// filters to a single user's folders when it is present, which would hide
	// the folder we are looking for.
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

// AccessGrant is an access grant declared when creating a model.
type AccessGrant struct {
	Name            string `json:"name"`
	AccessBoostable *bool  `json:"accessBoostable,omitempty"`
}

// ModelInput is the create body for POST /v1/models.
type ModelInput struct {
	ConnectionID string        `json:"connectionId,omitempty"`
	BaseModelID  string        `json:"baseModelId,omitempty"`
	ModelKind    string        `json:"modelKind"`
	ModelName    string        `json:"modelName"`
	AccessGrants []AccessGrant `json:"accessGrants,omitempty"`
}

type modelResponse struct {
	Model Model `json:"model"`
}

type listModelsResponse struct {
	Records  []Model   `json:"records"`
	Models   []Model   `json:"models"`
	PageInfo *PageInfo `json:"pageInfo,omitempty"`
}

// CreateModel creates a shared or shared extension model.
func (c *Client) CreateModel(ctx context.Context, in ModelInput) (*Model, error) {
	var out modelResponse
	if err := c.Post(ctx, "/v1/models", in, &out); err != nil {
		return nil, err
	}
	return &out.Model, nil
}

// RenameModel renames a model. Workbook and query models cannot be renamed.
func (c *Client) RenameModel(ctx context.Context, id, name string) (*Model, error) {
	body := map[string]string{"name": name}

	var out Model
	if err := c.Patch(ctx, "/v1/models/"+url.PathEscape(id), body, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		// Some responses wrap the model.
		var wrapped modelResponse
		if err := c.Get(ctx, "/v1/models/"+url.PathEscape(id), nil, &wrapped); err == nil && wrapped.Model.ID != "" {
			return &wrapped.Model, nil
		}
	}
	return &out, nil
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

// YAMLFileInput is the body for POST /v1/models/{modelId}/yaml.
type YAMLFileInput struct {
	FileName         string  `json:"fileName"`
	YAML             string  `json:"yaml"`
	Mode             string  `json:"mode"`
	BranchID         *string `json:"branchId,omitempty"`
	CommitMessage    *string `json:"commitMessage,omitempty"`
	PreviousChecksum *string `json:"previousChecksum,omitempty"`
	FullyResolved    *bool   `json:"fullyResolved,omitempty"`
}

// YAMLFile is a single model YAML file.
type YAMLFile struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
	Checksum string `json:"checksum"`
}

type getModelYAMLResponse struct {
	Files []YAMLFile `json:"files"`
}

// PutModelYAML creates or overwrites a model YAML file.
func (c *Client) PutModelYAML(ctx context.Context, modelID string, in YAMLFileInput) error {
	return c.Post(ctx, "/v1/models/"+url.PathEscape(modelID)+"/yaml", in, nil)
}

// GetModelYAMLFile retrieves a single YAML file from a model, with its checksum.
func (c *Client) GetModelYAMLFile(ctx context.Context, modelID, fileName, mode, branchID string) (*YAMLFile, error) {
	q := url.Values{}
	q.Set("includeChecksums", strconv.FormatBool(true))
	if mode != "" {
		q.Set("mode", mode)
	}
	if branchID != "" {
		q.Set("branchId", branchID)
	}

	var out getModelYAMLResponse
	if err := c.Get(ctx, "/v1/models/"+url.PathEscape(modelID)+"/yaml", q, &out); err != nil {
		return nil, err
	}
	for i := range out.Files {
		if out.Files[i].FileName == fileName {
			return &out.Files[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     http.MethodGet,
		Path:       "/v1/models/" + modelID + "/yaml",
		Message:    fmt.Sprintf("no YAML file named %q in model %q", fileName, modelID),
	}
}
