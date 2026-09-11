package client

import (
	"context"
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
// The real shape, confirmed against a live instance, is not what the spec
// implies. The role is nested under "direct", the subject is a flat "id" with a
// "type" discriminator, and for groups that id is the group's full UUID rather
// than the miniUuid the SCIM API returns. Matching therefore goes by name for
// groups and by id for users.
type Permit struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	IsEmbed     bool        `json:"isEmbed"`
	Direct      *PermitRole `json:"direct"`
}

// PermitRole is the role block nested inside a permit.
type PermitRole struct {
	Role        string `json:"role"`
	AccessBoost bool   `json:"accessBoost"`
	IsOwner     bool   `json:"isOwner"`
}

// Role returns the granted role, or an empty string when the permit carries no
// direct grant (inherited access, for instance).
func (p Permit) Role() string {
	if p.Direct == nil {
		return ""
	}
	return p.Direct.Role
}

// IsGroup reports whether this permit applies to a user group.
func (p Permit) IsGroup() bool { return p.Type == "userGroup" }

type permitsResponse struct {
	Permits []Permit `json:"permits"`
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

// listPermits decodes a permits array.
func (c *Client) listPermits(ctx context.Context, path string) ([]Permit, error) {
	var wrapper permitsResponse
	if err := c.Get(ctx, path, nil, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Permits, nil
}
