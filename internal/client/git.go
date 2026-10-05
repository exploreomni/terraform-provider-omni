package client

import (
	"context"
	"net/url"
)

// ModelGit is a model's git configuration.
//
// Secrets are never returned by the API, so the provider cannot detect drift in
// a deploy key or token.
type ModelGit struct {
	CloneURL                    string `json:"cloneUrl"`
	SSHURL                      string `json:"sshUrl"`
	WebURL                      string `json:"webUrl"`
	AuthMethod                  string `json:"authMethod"`
	BaseBranch                  string `json:"baseBranch"`
	BranchPerPullRequest        bool   `json:"branchPerPullRequest"`
	GitFollower                 bool   `json:"gitFollower"`
	GitServiceProvider          string `json:"gitServiceProvider"`
	GithubAppInstallationID     string `json:"githubAppInstallationId"`
	ModelPath                   string `json:"modelPath"`
	RequirePullRequest          string `json:"requirePullRequest"`
	CommitSigningCommitterName  string `json:"commitSigningCommitterName"`
	CommitSigningCommitterEmail string `json:"commitSigningCommitterEmail"`
}

// ModelGitInput is the create and update body. Pointers distinguish "leave
// unchanged" from "set", which the update endpoint relies on.
type ModelGitInput struct {
	CloneURL             *string `json:"cloneUrl,omitempty"`
	SSHURL               *string `json:"sshUrl,omitempty"`
	WebURL               *string `json:"webUrl,omitempty"`
	AuthMethod           *string `json:"authMethod,omitempty"`
	BaseBranch           *string `json:"baseBranch,omitempty"`
	BranchPerPullRequest *bool   `json:"branchPerPullRequest,omitempty"`
	GitFollower          *bool   `json:"gitFollower,omitempty"`
	GitServiceProvider   *string `json:"gitServiceProvider,omitempty"`
	ModelPath            *string `json:"modelPath,omitempty"`
	RequirePullRequest   *string `json:"requirePullRequest,omitempty"`

	// Secrets. Never returned by the API.
	DeployPrivateKey        *string `json:"deployPrivateKey,omitempty"`
	DeployKeyPassphrase     *string `json:"deployKeyPassphrase,omitempty"`
	Token                   *string `json:"token,omitempty"`
	GithubAppInstallationID *string `json:"githubAppInstallationId,omitempty"`

	CommitSigningCommitterName  *string `json:"commitSigningCommitterName,omitempty"`
	CommitSigningCommitterEmail *string `json:"commitSigningCommitterEmail,omitempty"`
}

func modelGitPath(modelID string) string {
	return "/v1/models/" + url.PathEscape(modelID) + "/git"
}

// CreateModelGit connects a model to a git repository.
func (c *Client) CreateModelGit(ctx context.Context, modelID string, in ModelGitInput) (*ModelGit, error) {
	var out ModelGit
	if err := c.Post(ctx, modelGitPath(modelID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetModelGit reads a model's git configuration.
func (c *Client) GetModelGit(ctx context.Context, modelID string) (*ModelGit, error) {
	var out ModelGit
	if err := c.Get(ctx, modelGitPath(modelID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateModelGit changes a model's git configuration. Omitted fields are left
// as they are.
func (c *Client) UpdateModelGit(ctx context.Context, modelID string, in ModelGitInput) (*ModelGit, error) {
	var out ModelGit
	if err := c.Patch(ctx, modelGitPath(modelID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteModelGit disconnects a model from git.
func (c *Client) DeleteModelGit(ctx context.Context, modelID string) error {
	return c.Delete(ctx, modelGitPath(modelID), nil)
}
