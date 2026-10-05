package client

import (
	"context"
	"net/url"
)

// ColorPalette is a named set of colours visualisations reference.
//
// For embedding, a palette per tenant is how an embedded experience picks up
// the host application's branding.
type ColorPalette struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Colors    []string `json:"colors"`
	CreatedAt string   `json:"created_at"`
}

// ColorPaletteInput is the create and update body.
type ColorPaletteInput struct {
	// Name is unique per type within the organization.
	Name string `json:"name"`
	// Type is "discrete" or "continuous". Discrete palettes are used in order;
	// continuous palettes interpolate between the colours.
	Type string `json:"type"`
	// Colors are ordered hex values, 1 to 50 of them.
	Colors []string `json:"colors"`
}

type listColorPalettesResponse struct {
	ColorPalettes []ColorPalette `json:"color_palettes"`
}

func colorPalettePath(id string) string {
	return "/v1/color-palettes/" + url.PathEscape(id)
}

// CreateColorPalette creates a palette.
func (c *Client) CreateColorPalette(ctx context.Context, in ColorPaletteInput) (*ColorPalette, error) {
	var out ColorPalette
	if err := c.Post(ctx, "/v1/color-palettes", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetColorPalette fetches a palette by ID.
func (c *Client) GetColorPalette(ctx context.Context, id string) (*ColorPalette, error) {
	var out ColorPalette
	if err := c.Get(ctx, colorPalettePath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateColorPalette replaces a palette's name, type and colours.
func (c *Client) UpdateColorPalette(ctx context.Context, id string, in ColorPaletteInput) (*ColorPalette, error) {
	var out ColorPalette
	if err := c.Put(ctx, colorPalettePath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteColorPalette removes a palette.
func (c *Client) DeleteColorPalette(ctx context.Context, id string) error {
	return c.Delete(ctx, colorPalettePath(id), nil)
}

// ListColorPalettes returns every palette in the organization.
func (c *Client) ListColorPalettes(ctx context.Context) ([]ColorPalette, error) {
	var out listColorPalettesResponse
	if err := c.Get(ctx, "/v1/color-palettes", nil, &out); err != nil {
		return nil, err
	}
	return out.ColorPalettes, nil
}
