package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// One attribute value on one user, written with a SCIM PATCH that names only
// that key. Unlike ReplaceUser, which PUTs the whole resource, this leaves the
// user's other attributes, name and groups alone, so it works on a user that
// Terraform does not own - one who signed in on their own, or was invited from
// the UI.
//
// The path is the extension URN with the attribute's reference appended:
//
//	urn:omni:params:1.0:UserAttribute:location_id

func userAttributePath(name string) string {
	return UserAttributeSchema + ":" + name
}

// SetUserAttributeValue sets one attribute on one user. value is a string for
// a single-valued attribute and a []string for one that takes multiple values;
// Omni rejects the other shape with a 400.
func (c *Client) SetUserAttributeValue(ctx context.Context, userID, name string, value any) error {
	patch := scimPatch{
		Schemas: []string{scimPatchOpSchema},
		Operations: []scimOperation{{
			Op:    "replace",
			Path:  userAttributePath(name),
			Value: value,
		}},
	}
	return c.Patch(ctx, "/scim/v2/users/"+url.PathEscape(userID), patch, nil)
}

// ClearUserAttributeValue removes one attribute key from one user, leaving the
// user and every other attribute in place.
func (c *Client) ClearUserAttributeValue(ctx context.Context, userID, name string) error {
	patch := scimPatch{
		Schemas: []string{scimPatchOpSchema},
		Operations: []scimOperation{{
			Op:   "remove",
			Path: userAttributePath(name),
		}},
	}
	return c.Patch(ctx, "/scim/v2/users/"+url.PathEscape(userID), patch, nil)
}

// GetUserAttributeValues returns a user's attribute values as Omni reports
// them. Numbers are kept as json.Number rather than float64, so an ID past
// 2^53 comes back exactly as it was written.
func (c *Client) GetUserAttributeValues(ctx context.Context, userID string) (map[string]any, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, "/scim/v2/users/"+url.PathEscape(userID), nil, &raw); err != nil {
		return nil, err
	}

	var body struct {
		Attributes map[string]any `json:"urn:omni:params:1.0:UserAttribute"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding user %s: %w", userID, err)
	}
	if body.Attributes == nil {
		return map[string]any{}, nil
	}
	return body.Attributes, nil
}
