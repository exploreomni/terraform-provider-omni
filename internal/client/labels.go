package client

import (
	"context"
	"net/url"
)

// Label is a content label. Labels are addressed by name rather than an opaque
// ID, so renaming one is a PUT against the old name carrying the new one.
type Label struct {
	Name        string  `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Homepage    bool    `json:"homepage"`
	Verified    bool    `json:"verified"`
	UsageCount  float64 `json:"usage_count"`
}

// LabelInput is the create and update body.
//
// Pointer fields distinguish "not set" from "set to empty": the API treats null
// as clearing a value, which is how a description is removed.
type LabelInput struct {
	Name        string  `json:"name"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Homepage    *bool   `json:"homepage,omitempty"`
	Verified    *bool   `json:"verified,omitempty"`
}

type listLabelsResponse struct {
	Labels []Label `json:"labels"`
}

func labelPath(name string) string {
	return "/v1/labels/" + url.PathEscape(name)
}

// CreateLabel creates a label. homepage and verified require admin permissions.
func (c *Client) CreateLabel(ctx context.Context, in LabelInput) (*Label, error) {
	var out Label
	if err := c.Post(ctx, "/v1/labels", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLabel fetches a label by name.
func (c *Client) GetLabel(ctx context.Context, name string) (*Label, error) {
	var out Label
	if err := c.Get(ctx, labelPath(name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateLabel updates the label currently called name. Passing a different Name
// in the body renames it.
func (c *Client) UpdateLabel(ctx context.Context, name string, in LabelInput) (*Label, error) {
	var out Label
	if err := c.Put(ctx, labelPath(name), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteLabel removes a label. Documents carrying it keep existing; they just
// lose the label.
func (c *Client) DeleteLabel(ctx context.Context, name string) error {
	return c.Delete(ctx, labelPath(name), nil)
}

// ListLabels returns every label on the instance.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var out listLabelsResponse
	if err := c.Get(ctx, "/v1/labels", nil, &out); err != nil {
		return nil, err
	}
	return out.Labels, nil
}
