package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientNormalisesBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://blobsrus.omniapp.co":      "https://blobsrus.omniapp.co/api",
		"https://blobsrus.omniapp.co/":     "https://blobsrus.omniapp.co/api",
		"https://blobsrus.omniapp.co/api":  "https://blobsrus.omniapp.co/api",
		"https://blobsrus.omniapp.co/api/": "https://blobsrus.omniapp.co/api",
		"blobsrus.omniapp.co":              "https://blobsrus.omniapp.co/api",
	}

	for in, want := range cases {
		c, err := NewClient(in, "token")
		if err != nil {
			t.Fatalf("NewClient(%q) returned an error: %v", in, err)
		}
		if got := c.BaseURL(); got != want {
			t.Errorf("NewClient(%q) base URL = %q, want %q", in, got, want)
		}
	}
}

func TestNewClientRejectsMissingValues(t *testing.T) {
	if _, err := NewClient("", "token"); err == nil {
		t.Error("expected an error for an empty base URL")
	}
	if _, err := NewClient("https://blobsrus.omniapp.co", ""); err == nil {
		t.Error("expected an error for an empty token")
	}
	if _, err := NewClient("http://blobsrus.omniapp.co", "token"); err == nil {
		t.Error("expected an error for a non-https base URL")
	}
}

func TestDoSendsBearerTokenAndDecodes(t *testing.T) {
	var gotAuth, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"connection":{"id":"abc","name":"Prod","dialect":"snowflake"}}`))
	}))
	defer server.Close()

	c := testClient(t, server.URL)

	connection, err := c.GetConnection(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetConnection returned an error: %v", err)
	}
	if connection.Name != "Prod" || connection.Dialect != "snowflake" {
		t.Errorf("unexpected connection: %+v", connection)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotPath != "/api/v1/connections/abc" {
		t.Errorf("request path = %q, want %q", gotPath, "/api/v1/connections/abc")
	}
}

func TestDoSurfacesNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"404","message":"Model not found"}`))
	}))
	defer server.Close()

	c := testClient(t, server.URL)

	_, err := c.GetConnection(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
}

func TestDoRetriesOnRateLimit(t *testing.T) {
	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"slow down"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"connection":{"id":"abc"}}`))
	}))
	defer server.Close()

	c := testClient(t, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if _, err := c.GetConnection(ctx, "abc"); err != nil {
		t.Fatalf("GetConnection returned an error: %v", err)
	}
	if calls != 2 {
		t.Errorf("server call count = %d, want 2", calls)
	}
}

func TestListModelRolesAcceptsBothShapes(t *testing.T) {
	bodies := []string{
		`[{"modelId":"m1","roleName":"QUERIER"}]`,
		`{"modelRoles":[{"modelId":"m1","roleName":"QUERIER"}]}`,
		`{"records":[{"modelId":"m1","roleName":"QUERIER"}]}`,
	}

	for _, body := range bodies {
		payload := body
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(payload))
		}))

		c := testClient(t, server.URL)
		roles, err := c.ListUserModelRoles(context.Background(), "user-1")
		server.Close()

		if err != nil {
			t.Fatalf("ListUserModelRoles(%s) returned an error: %v", payload, err)
		}
		if len(roles) != 1 || roles[0].RoleName != "QUERIER" {
			t.Errorf("ListUserModelRoles(%s) = %+v, want one QUERIER role", payload, roles)
		}
	}
}

func TestFindModelRoleTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") == "" {
			t.Errorf("expected a SCIM filter query parameter, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"totalResults":1,"Resources":[{"id":"u1","userName":"blob@blobsrus.co"}]}`))
	}))
	defer server.Close()

	c := testClient(t, server.URL)

	user, err := c.FindUserByUserName(context.Background(), "blob@blobsrus.co")
	if err != nil {
		t.Fatalf("FindUserByUserName returned an error: %v", err)
	}
	if user.ID != "u1" {
		t.Errorf("user ID = %q, want %q", user.ID, "u1")
	}
}

// testClient points a Client at an httptest server, bypassing the https check
// that NewClient enforces for real instances.
func testClient(t *testing.T, serverURL string) *Client {
	t.Helper()

	return &Client{
		baseURL:   serverURL + "/api",
		token:     "test-token",
		userAgent: "terraform-provider-omni/test",
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func TestListModelRolesParsesResultsShape(t *testing.T) {
	// The shape the API actually returns, captured from a live instance.
	body := `{
	  "membershipId": "u1",
	  "results": [
	    {"baseRole":"QUERY_TOPICS","from":{"type":"Connection Base Role"},"priority":150,
	     "roleName":"QUERY_TOPICS","connectionId":"c1","modelId":"m1","resolved":true},
	    {"baseRole":"QUERIER","from":{"type":"User Role"},"priority":0,
	     "roleName":"QUERIER","connectionId":"c1","modelId":"m1","resolved":false}
	  ]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	c := testClient(t, server.URL)

	roles, err := c.ListUserModelRoles(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ListUserModelRoles returned an error: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("got %d roles, want 2", len(roles))
	}
	if roles[0].SourceType() != "Connection Base Role" {
		t.Errorf("roles[0] source = %q, want %q", roles[0].SourceType(), "Connection Base Role")
	}
	if roles[1].SourceType() != "User Role" || roles[1].RoleName != "QUERIER" {
		t.Errorf("roles[1] = %+v, want a User Role QUERIER entry", roles[1])
	}
	if roles[0].Priority != 150 || !roles[0].Resolved {
		t.Errorf("priority/resolved not parsed: %+v", roles[0])
	}
}

func TestPermitDecodesTheRealShape(t *testing.T) {
	// Captured verbatim from a live instance. The role is nested under
	// "direct", the subject is a flat id with a type discriminator, and a
	// group's id here is its full UUID rather than the SCIM miniUuid.
	body := `{
	  "permits": [
	    {"direct":{"accessBoost":false,"role":"OWNER","isOwner":true},
	     "description":"someone@example.invalid","id":"faeb3931-f106-4e37-bb24-8748bf43bfa5",
	     "isEmbed":false,"name":"A Person","type":"user"},
	    {"direct":{"accessBoost":false,"role":"VIEWER","isOwner":false},
	     "description":"0 members","id":"0aea1b4c-ebfd-4f6a-bdf6-d7edfd798b34",
	     "name":"tf-edge-perm-group","type":"userGroup"}
	  ]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	permits, err := testClient(t, server.URL).ListFolderPermissions(context.Background(), "f1")
	if err != nil {
		t.Fatalf("ListFolderPermissions returned an error: %v", err)
	}
	if len(permits) != 2 {
		t.Fatalf("got %d permits, want 2", len(permits))
	}

	owner := permits[0]
	if owner.Role() != "OWNER" || owner.IsGroup() || owner.ID != "faeb3931-f106-4e37-bb24-8748bf43bfa5" {
		t.Errorf("user permit = %+v, want an OWNER user permit", owner)
	}
	if !owner.Direct.IsOwner {
		t.Error("expected isOwner true on the owner permit")
	}

	group := permits[1]
	if group.Role() != "VIEWER" || !group.IsGroup() || group.Name != "tf-edge-perm-group" {
		t.Errorf("group permit = %+v, want a VIEWER group permit", group)
	}
}

func TestPermitWithoutDirectGrantHasNoRole(t *testing.T) {
	// A permit with no direct block means inherited access, which Terraform
	// must not treat as a grant it made.
	body := `{"permits":[{"id":"x","name":"y","type":"userGroup"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	permits, err := testClient(t, server.URL).ListFolderPermissions(context.Background(), "f1")
	if err != nil {
		t.Fatalf("ListFolderPermissions returned an error: %v", err)
	}
	if permits[0].Role() != "" {
		t.Errorf("Role() = %q, want empty for a permit with no direct grant", permits[0].Role())
	}
}
