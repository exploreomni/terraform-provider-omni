package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ContentRole is a role on a folder or document.
//
// These are distinct from model roles: they govern who can see and edit
// content, not who can query data.
const (
	ContentRoleNoAccess = "NO_ACCESS"
	ContentRoleViewer   = "VIEWER"
	ContentRoleExplorer = "EXPLORER"
	ContentRoleEditor   = "EDITOR"
	ContentRoleManager  = "MANAGER"
	ContentRoleOwner    = "OWNER"
)

// PermissionGrant is the body for granting or updating content permissions.
type PermissionGrant struct {
	Role         string   `json:"role,omitempty"`
	UserIDs      []string `json:"userIds,omitempty"`
	UserGroupIDs []string `json:"userGroupIds,omitempty"`
	AccessBoost  *bool    `json:"accessBoost,omitempty"`
}

// PermissionRevoke is the body for removing content permissions.
type PermissionRevoke struct {
	UserIDs      []string `json:"userIds,omitempty"`
	UserGroupIDs []string `json:"userGroupIds,omitempty"`
}

// Permit is one entry in a content permission list.
//
// The spec documents only that each permit carries a role, so the remaining
// fields are decoded loosely and the subject is resolved from whichever key the
// API actually uses. Raw keeps the original object so a shape change shows up
// as a diagnosable value rather than a silent mismatch.
type Permit struct {
	Role        string         `json:"role"`
	AccessBoost bool           `json:"accessBoost"`
	Raw         map[string]any `json:"-"`
}

// SubjectID returns the user or group this permit applies to, and whether it is
// a group. It checks the key names an API of this shape is likely to use.
func (p Permit) SubjectID() (id string, isGroup bool) {
	for _, key := range []string{"userGroupId", "groupId", "userGroupID"} {
		if v, ok := p.Raw[key].(string); ok && v != "" {
			return v, true
		}
	}
	for _, key := range []string{"userId", "membershipId", "userID", "id"} {
		if v, ok := p.Raw[key].(string); ok && v != "" {
			return v, false
		}
	}
	// Nested subject objects, e.g. {"userGroup": {"id": "..."}}.
	for _, key := range []string{"userGroup", "group"} {
		if obj, ok := p.Raw[key].(map[string]any); ok {
			if v, ok := obj["id"].(string); ok && v != "" {
				return v, true
			}
		}
	}
	if obj, ok := p.Raw["user"].(map[string]any); ok {
		if v, ok := obj["id"].(string); ok && v != "" {
			return v, false
		}
	}
	return "", false
}

type permitsResponse struct {
	Permits []json.RawMessage `json:"permits"`
}

// FolderOrgAccess is the organization-wide access setting on a folder.
type FolderOrgAccess struct {
	OrganizationRole        *string `json:"organizationRole"`
	OrganizationAccessBoost *bool   `json:"organizationAccessBoost,omitempty"`
}

// DocumentSettings is the organization-wide setting and ability set on a
// document.
type DocumentSettings struct {
	OrganizationRole            *string `json:"organizationRole,omitempty"`
	OrganizationAccessBoost     *bool   `json:"organizationAccessBoost,omitempty"`
	RequirePullRequestToPublish *bool   `json:"requirePullRequestToPublish,omitempty"`

	CanAnalyze             *bool `json:"canAnalyze,omitempty"`
	CanDownload            *bool `json:"canDownload,omitempty"`
	CanDrill               *bool `json:"canDrill,omitempty"`
	CanDuplicate           *bool `json:"canDuplicate,omitempty"`
	CanRequestAccess       *bool `json:"canRequestAccess,omitempty"`
	CanSaveSpreadsheets    *bool `json:"canSaveSpreadsheets,omitempty"`
	CanSchedule            *bool `json:"canSchedule,omitempty"`
	CanUpload              *bool `json:"canUpload,omitempty"`
	CanUseDashboardAI      *bool `json:"canUseDashboardAi,omitempty"`
	CanUseTimezoneOverride *bool `json:"canUseTimezoneOverride,omitempty"`
	CanViewWorkbook        *bool `json:"canViewWorkbook,omitempty"`
}

func folderPermissionsPath(folderID string) string {
	return "/v1/folders/" + url.PathEscape(folderID) + "/permissions"
}

func documentPermissionsPath(identifier string) string {
	return "/v1/documents/" + url.PathEscape(identifier) + "/permissions"
}

// GrantFolderPermission adds a content role for the given users and groups.
func (c *Client) GrantFolderPermission(ctx context.Context, folderID string, in PermissionGrant) error {
	return c.Post(ctx, folderPermissionsPath(folderID), in, nil)
}

// UpdateFolderPermission changes the role held by the given users and groups.
func (c *Client) UpdateFolderPermission(ctx context.Context, folderID string, in PermissionGrant) error {
	return c.Patch(ctx, folderPermissionsPath(folderID), in, nil)
}

// RevokeFolderPermission removes the given users and groups from a folder.
func (c *Client) RevokeFolderPermission(ctx context.Context, folderID string, in PermissionRevoke) error {
	return c.Do(ctx, Request{
		Method: http.MethodDelete,
		Path:   folderPermissionsPath(folderID),
		Body:   in,
	})
}

// ListFolderPermissions returns every permit on a folder.
func (c *Client) ListFolderPermissions(ctx context.Context, folderID string) ([]Permit, error) {
	return c.listPermits(ctx, folderPermissionsPath(folderID))
}

// SetFolderOrgAccess sets the organization-wide role on a folder.
func (c *Client) SetFolderOrgAccess(ctx context.Context, folderID string, in FolderOrgAccess) error {
	return c.Put(ctx, folderPermissionsPath(folderID), in, nil)
}

// GrantDocumentPermission adds a content role on a document.
func (c *Client) GrantDocumentPermission(ctx context.Context, identifier string, in PermissionGrant) error {
	return c.Post(ctx, documentPermissionsPath(identifier), in, nil)
}

// UpdateDocumentPermission changes the role held on a document.
func (c *Client) UpdateDocumentPermission(ctx context.Context, identifier string, in PermissionGrant) error {
	return c.Patch(ctx, documentPermissionsPath(identifier), in, nil)
}

// RevokeDocumentPermission removes users and groups from a document.
//
// The documents API has no DELETE on permissions, so revoking is done by
// setting the role to NO_ACCESS.
func (c *Client) RevokeDocumentPermission(ctx context.Context, identifier string, in PermissionRevoke) error {
	return c.Patch(ctx, documentPermissionsPath(identifier), PermissionGrant{
		Role:         ContentRoleNoAccess,
		UserIDs:      in.UserIDs,
		UserGroupIDs: in.UserGroupIDs,
	}, nil)
}

// ListDocumentPermissions returns every permit on a document.
func (c *Client) ListDocumentPermissions(ctx context.Context, identifier string) ([]Permit, error) {
	return c.listPermits(ctx, documentPermissionsPath(identifier))
}

// SetDocumentSettings applies the organization-wide role and ability flags.
func (c *Client) SetDocumentSettings(ctx context.Context, identifier string, in DocumentSettings) error {
	return c.Put(ctx, documentPermissionsPath(identifier), in, nil)
}

// listPermits decodes a permits array, keeping each raw object so the subject
// can be resolved whatever key the API uses for it.
func (c *Client) listPermits(ctx context.Context, path string) ([]Permit, error) {
	var wrapper permitsResponse
	if err := c.Get(ctx, path, nil, &wrapper); err != nil {
		return nil, err
	}

	permits := make([]Permit, 0, len(wrapper.Permits))
	for _, raw := range wrapper.Permits {
		var p Permit
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("decoding permit from %s: %w", path, err)
		}
		if err := json.Unmarshal(raw, &p.Raw); err != nil {
			return nil, fmt.Errorf("decoding permit object from %s: %w", path, err)
		}
		permits = append(permits, p)
	}
	return permits, nil
}
