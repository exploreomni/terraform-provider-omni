# Terraform Provider for Omni Analytics

Manage an [Omni](https://omni.co) instance as code: connections, models, model YAML, folders,
users, groups, and role assignments, all through the [Omni REST API](https://docs.omni.co/api).

Built on the Terraform Plugin Framework, protocol version 6, so it works with Terraform 1.0+ and
OpenTofu.

## Why

Omni already keeps model YAML in git. Everything around the model - who has a `MODELER` role on
which connection, which folders exist, which service connection points at which warehouse - still
gets clicked into the UI and drifts between dev, stage, and prod instances. This provider puts
that layer in the same review process as the rest of your infrastructure.

## Resources

| Resource                     | Endpoint                                | Notes                                                                 |
| ---------------------------- | --------------------------------------- | --------------------------------------------------------------------- |
| `omni_connection`            | `/v1/connections`                       | Credentials and `base_role` update in place; other changes replace     |
| `omni_model`                 | `/v1/models`                            | `SHARED` and `SHARED_EXTENSION`; destroy archives to trash             |
| `omni_folder`                | `/v1/folders`                           | Nesting up to seven levels                                             |
| `omni_user`                  | `/scim/v2/users`                        | Organization API key required                                          |
| `omni_user_group`            | `/scim/v2/groups`                       | Authoritative membership; organization API key required                |
| `omni_user_model_role`       | `/v1/users/{id}/model-roles`            | Destroy downgrades to `NO_ACCESS` (no delete endpoint exists)          |
| `omni_user_group_model_role` | `/v1/user-groups/{id}/model-roles`      | Same behaviour on destroy                                              |

## Data sources

`omni_user`, `omni_user_group`, `omni_connection`, `omni_model` - each looks up by ID or by name.

## Quick start

```hcl
terraform {
  required_providers {
    omni = {
      source  = "ashima-omni/omni"
      version = "~> 0.1"
    }
  }
}

provider "omni" {
  base_url  = "https://blobsrus.omniapp.co" # or OMNI_BASE_URL
  api_token = var.omni_api_token            # or OMNI_API_TOKEN
}

resource "omni_folder" "finance" {
  name = "Finance"
  path = "finance"
}
```

More in [`examples/`](./examples), including an end-to-end setup in
[`examples/complete`](./examples/complete).

## Authentication

Generate a token under **Settings > API keys** in Omni.

- Organization API keys work for every resource, including users and groups.
- Personal access tokens work for connections, models, model YAML, folders, and role assignments,
  but the SCIM user and group endpoints reject them.

## Local development

```bash
go build ./...
go test ./internal/...
```

To try the provider against a real instance before it is published, add a dev override to
`~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "ashima-omni/omni" = "/path/to/your/gopath/bin"
  }
  direct {}
}
```

Then `go install .` and run Terraform as usual. `terraform init` is not needed with a dev
override, and Terraform will warn you that one is active.

Regenerate the registry docs after schema changes:

```bash
make docs
```

## Publishing to the Terraform Registry

1. Point the module path at the GitHub org that will own the repo: `make rename OWNER=your-org`.
   The repo itself must be named `terraform-provider-omni` and be public.
2. Create a GPG key, add `GPG_PRIVATE_KEY` and `PASSPHRASE` to the repo's Actions secrets, and
   upload the public key to your Terraform Registry account.
3. Tag a release: `git tag v0.1.0 && git push origin v0.1.0`. The release workflow builds every
   platform with GoReleaser and signs the checksums.
4. Sign in at [registry.terraform.io](https://registry.terraform.io) with GitHub and publish the
   repo.

## Known limitations

- The connection read endpoint does not return credentials or dialect-specific settings, so drift
  in those fields is not detected. Terraform keeps what you configured.
- The folders API has no get-by-id route, so reads page the list endpoint and match on ID.
- Role assignments cannot be deleted through the API. Destroying a role resource downgrades it to
  `role_on_destroy` (default `NO_ACCESS`) instead.
- `omni_user` tracks only the attribute keys declared in the config; attributes set elsewhere are
  left alone.

## Roadmap

Labels, dbt environments, schema refresh schedules, connection environments, model git
configuration, document and folder permissions, and schedules are all exposed by the API and are
the obvious next resources.

## License

MPL-2.0. This project is not an official Omni Analytics product.
