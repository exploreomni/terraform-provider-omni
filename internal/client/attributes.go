package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AttributeDefault is a user attribute's default value.
//
// The API types this as a scalar or a list depending on multiple_values, and a
// Number attribute's value may arrive as a JSON number or as a string: Omni
// stores numbers as strings so that values beyond 2^53 - 1 keep their
// precision. Normalising all of it to []string on the way in keeps one shape in
// the provider, and the write path chooses scalar or list from the attribute's
// own multiple_values rather than guessing from what it was handed.
type AttributeDefault struct {
	Values []string

	// Set distinguishes "no default" from "a default that is an empty list".
	// The API treats null and a string that trims to empty as no default.
	Set bool
}

func (a *AttributeDefault) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		a.Values, a.Set = nil, false
		return nil
	}

	// UseNumber, not the default float64. Omni stores numbers as strings so
	// that values beyond 2^53 - 1 keep their precision, and decoding one into a
	// float64 would throw away exactly the precision that is being protected:
	// 9007199254740993 comes back as 9007199254740992. json.Number keeps the
	// literal text, so the value survives whichever way the API sends it.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	var one any
	if err := dec.Decode(&one); err != nil {
		return err
	}

	switch v := one.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			a.Values, a.Set = nil, false
			return nil
		}
		a.Values, a.Set = []string{v}, true
	case json.Number:
		a.Values, a.Set = []string{v.String()}, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch s := item.(type) {
			case string:
				out = append(out, s)
			case json.Number:
				out = append(out, s.String())
			default:
				return fmt.Errorf("default_value list holds %T, expected a string or a number", item)
			}
		}
		a.Values, a.Set = out, true
	default:
		return fmt.Errorf("default_value is %T, expected a string, a number or a list", one)
	}
	return nil
}

// UserAttributeInput is a definition to create or update.
type UserAttributeInput struct {
	Name           string
	Label          string
	Type           string
	Description    *string
	MultipleValues bool
	Default        AttributeDefault
}

// body builds the request. default_value is a list when the attribute takes
// multiple values and a scalar when it does not, which is what the API expects
// in each case. An unset default is sent as null, which clears it.
func (in UserAttributeInput) body(includeType bool) map[string]any {
	out := map[string]any{
		"name":            in.Name,
		"label":           in.Label,
		"multiple_values": in.MultipleValues,
	}
	if includeType {
		out["type"] = in.Type
	}
	if in.Description != nil {
		out["description"] = *in.Description
	}

	switch {
	case !in.Default.Set:
		out["default_value"] = nil
	case in.MultipleValues:
		out["default_value"] = in.Default.Values
	case len(in.Default.Values) > 0:
		out["default_value"] = in.Default.Values[0]
	default:
		out["default_value"] = nil
	}
	return out
}

// CreateUserAttribute creates a custom definition. Requires an organization API
// key: the endpoint is organization admin only.
func (c *Client) CreateUserAttribute(ctx context.Context, in UserAttributeInput) (*UserAttribute, error) {
	var out UserAttribute
	if err := c.Post(ctx, "/v1/user-attributes", in.body(true), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateUserAttribute updates a definition in place.
//
// The endpoint is a PUT that keeps omitted fields, so every mutable field is
// sent on every call and the request describes the whole desired state. type is
// not sent: it is immutable, and sending it on a no-op update would be a field
// the API has to ignore rather than a field the caller means.
func (c *Client) UpdateUserAttribute(ctx context.Context, id string, in UserAttributeInput) (*UserAttribute, error) {
	var out UserAttribute
	if err := c.Put(ctx, "/v1/user-attributes/"+url.PathEscape(id), in.body(false), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteUserAttribute removes a definition.
//
// This is destructive well beyond the definition: it removes the value every
// user holds for the attribute, breaks model SQL and embed SSO URLs that
// reference it, and makes any connection selecting an environment by its value
// fall back to the default environment.
func (c *Client) DeleteUserAttribute(ctx context.Context, id string) error {
	return c.Delete(ctx, "/v1/user-attributes/"+url.PathEscape(id), nil)
}

// SelectableValues is the set of values a user or group may switch an attribute
// to. It is not the value they are assigned.
type SelectableValues struct {
	UserAttributeID string   `json:"user_attribute_id"`
	Values          []string `json:"values"`
	AllowAnyValue   bool     `json:"allow_any_value"`
}

// SelectableValuesSubject is the user or group a provisioning applies to.
// Exactly one of the two is set; the API rejects both.
type SelectableValuesSubject struct {
	// UserID is the user's MEMBERSHIP id, not their user id. The two differ,
	// and the API gives no hint when the wrong one is passed.
	UserID  string
	GroupID string
}

func (s SelectableValuesSubject) query() url.Values {
	q := url.Values{}
	if s.UserID != "" {
		q.Set("userId", s.UserID)
	}
	if s.GroupID != "" {
		q.Set("groupId", s.GroupID)
	}
	return q
}

// GetSelectableValues reads a subject's own provisioning.
//
// It does not include values a user inherits from their groups, and it does not
// reflect an organization admin's ability to switch to any value. A subject
// with nothing provisioned answers 200 with an empty list rather than 404, so
// "never provisioned" and "provisioned with nothing" are indistinguishable.
func (c *Client) GetSelectableValues(ctx context.Context, attributeID string, subject SelectableValuesSubject) (*SelectableValues, error) {
	var out SelectableValues
	path := "/v1/user-attributes/" + url.PathEscape(attributeID) + "/selectable-values"
	if err := c.Get(ctx, path, subject.query(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetSelectableValues replaces a subject's provisioning.
//
// Sending an empty list with allow_any_value false removes the provisioning
// entirely, which is how a destroy is expressed: there is no DELETE.
func (c *Client) SetSelectableValues(ctx context.Context, attributeID string, subject SelectableValuesSubject, values []string, allowAny bool) (*SelectableValues, error) {
	body := map[string]any{
		"values":          values,
		"allow_any_value": allowAny,
	}
	if subject.UserID != "" {
		body["user_id"] = subject.UserID
	}
	if subject.GroupID != "" {
		body["group_id"] = subject.GroupID
	}
	if values == nil {
		body["values"] = []string{}
	}

	var out SelectableValues
	path := "/v1/user-attributes/" + url.PathEscape(attributeID) + "/selectable-values"
	if err := c.Put(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
