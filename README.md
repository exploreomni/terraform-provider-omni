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

## Documentation

| | |
| --- | --- |
| [docs/USAGE.md](./docs/USAGE.md) | Installation and per-resource guide, for both registry and from-source use |
| [docs/TEST-REPORT.md](./docs/TEST-REPORT.md) | What has been verified against a live instance, and what has not |
| [docs/TEST-CASES.md](./docs/TEST-CASES.md) | The test case matrix |


## Setting up CI/CD

The workflows in this repo plan on pull request and apply on merge, behind an
approval gate. Four things to set up before that works.

### 1. Choose where state lives

CI runners are ephemeral. With local state, every run starts from nothing and
recreates every resource it has already created. A remote backend is not
optional for apply-on-merge.

Terraform state also stores `omni_connection.password` in plain text. Whatever
you pick, treat it as secret material.

**HCP Terraform** is free, needs no card, and stores state with locking and
versioning. Set the workspace to **Execution Mode: Local**, so GitHub Actions
runs Terraform and HCP only holds state. Remote execution would try to install
the provider from the registry, which fails while it is unpublished.

```hcl
terraform {
  cloud {
    organization = "your-org"
    workspaces { name = "omni-playground" }
  }
}
```

Then add a `TF_API_TOKEN` secret from **app.terraform.io > User settings >
Tokens**. The `setup-terraform` action picks it up automatically.

**GCS**, if you already have a GCP project. Turn on object versioning, because
state corruption is recoverable with prior versions and unrecoverable without.

```hcl
terraform {
  backend "gcs" {
    bucket = "omni-tf-state"
    prefix = "playground"
  }
}
```

```bash
gcloud storage buckets update gs://omni-tf-state --versioning
```

Authenticate CI with `google-github-actions/auth`. Workload Identity Federation
is preferable to a service account key, since the key is a long-lived
credential with write access to state.

**S3**, with `use_lockfile = true` for locking:

```hcl
terraform {
  backend "s3" {
    bucket       = "omni-tf-state"
    key          = "playground/terraform.tfstate"
    region       = "ap-southeast-2"
    use_lockfile = true
  }
}
```

**Cloudflare R2** works through the `s3` backend and has a free tier with no
card. It needs endpoint overrides:

```hcl
terraform {
  backend "s3" {
    bucket    = "omni-tf-state"
    key       = "playground/terraform.tfstate"
    region    = "auto"
    endpoints = { s3 = "https://<account-id>.r2.cloudflarestorage.com" }

    skip_credentials_validation = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    use_path_style              = true
  }
}
```

Do not commit state to the repo. This repo is public, and state contains
warehouse credentials in plain text.

### 2. Add repository secrets

**Settings > Secrets and variables > Actions**:

| Secret | Value |
| --- | --- |
| `OMNI_BASE_URL` | `https://yourinstance.omniapp.co` |
| `OMNI_API_TOKEN` | An Omni organization API key |
| `TF_API_TOKEN` | HCP Terraform token, if using the `cloud` backend |
| Backend credentials | Whatever your bucket needs, for example `GCP_SA_KEY` or `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` |

Use an **organization API key**, not a personal access token. The SCIM
endpoints behind `omni_user` and `omni_user_group` reject personal tokens.

Generate it in Omni under **Settings > API keys**. If a token is ever pasted
somewhere it should not be, rotate it there rather than trying to scrub it.

### 3. Create the approval environment

**Settings > Environments > New environment**, named `omni-playground` to match
the workflows.

Add yourself under **Required reviewers**. A merge then queues the apply and
waits for a human to approve it in the Actions tab before anything touches the
instance.

Two settings worth a look:

- *Allow administrators to bypass protection rules* is on by default. If you
  are an admin, you can skip your own gate. Turn it off if the gate should be
  real.
- *Prevent self-review* should stay off when you are the only reviewer, or
  nobody can ever approve.

### 4. Point the config at your instance

Edit `infra/main.tf`. As shipped it creates a couple of folders and a group as
a worked example. Replace that with what you actually want managed.

Then open a pull request. The plan workflow comments the plan on the PR; merging
queues the apply for approval.

### Local setup

For running Terraform from your own machine rather than CI:

```bash
export OMNI_BASE_URL=https://yourinstance.omniapp.co
export OMNI_API_TOKEN=...
```

See [docs/USAGE.md](./docs/USAGE.md) section 2 for installing the provider from
source, which is needed until it is published to the registry.

## Authentication

Generate a token under **Settings > API keys** in Omni.

- Organization API keys work for every resource, including users and groups.
- Personal access tokens work for connections, models, folders, and role
  assignments, but the SCIM user and group endpoints reject them.

Supply them as `OMNI_BASE_URL` and `OMNI_API_TOKEN` rather than putting them in
configuration.

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
