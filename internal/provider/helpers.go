package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

func pathRoot(name string) path.Path { return path.Root(name) }

// pathExpr builds a root path expression, which is what schema-level validators
// take (as opposed to the path.Path that diagnostics and state accessors use).
func pathExpr(name string) path.Expression { return path.MatchRoot(name) }

// clientFromResourceRequest pulls the configured API client out of a resource
// Configure request.
func clientFromResourceRequest(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}
	return c
}

// clientFromDataSourceRequest pulls the configured API client out of a data
// source Configure request.
func clientFromDataSourceRequest(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}
	return c
}

// notConfigured reports whether the client is missing, which happens during
// early framework lifecycle calls.
func notConfigured(c *client.Client, diags *diag.Diagnostics) bool {
	if c == nil {
		diags.AddError(
			"Provider not configured",
			"The Omni API client is not available. This usually means the provider block failed to configure.",
		)
		return true
	}
	return false
}

// stringPtr converts an optional framework string into a *string, returning nil
// for null or unknown values so the field is omitted from the request body.
func stringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// int64Ptr converts an optional framework int into a *int64.
func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

// boolPtr converts an optional framework bool into a *bool.
func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// stringOrNull returns a null framework string for empty API values, so that
// optional fields the API omits do not show up as "" diffs.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// ptrString dereferences a *string safely.
func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// diagnostics is a short alias used by helper functions that append diagnostics.
type diagnostics = diag.Diagnostics

// mapToAny converts a framework string map into the map[string]any shape the
// Omni API expects for user attributes.
func mapToAny(ctx context.Context, m types.Map) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}

	elements := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make(map[string]any, len(elements))
	for k, v := range elements {
		out[k] = v
	}
	return out, diags
}

// setToStrings converts a framework set of strings into a []string.
func setToStrings(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if s.IsNull() || s.IsUnknown() {
		return nil, diags
	}

	var out []string
	diags.Append(s.ElementsAs(ctx, &out, false)...)
	return out, diags
}
