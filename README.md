# Terraform Provider for Omni Analytics

Manage an [Omni](https://omni.co) instance as code: connections, models, model YAML, folders,
users, groups, role assignments, labels, palettes and permissions, all through the
[Omni REST API](https://docs.omni.co/api).

Built on the Terraform Plugin Framework, protocol version 6, so it works with Terraform 1.0+ and
OpenTofu. The reference deployment in [`infra/`](./infra) needs 1.9 or later, for cross-variable
validation.

## Why

Omni already keeps model YAML in git. Everything around the model - who has a `MODELER` role on
which connection, which folders exist, which service connection points at which warehouse - still
gets clicked into the UI and drifts between dev, stage, and prod instances. This provider puts
that layer in the same review process as the rest of your infrastructure.

## Resources

| Resource                     | Endpoint                                  | Notes                                                                        |
| ---------------------------- | ----------------------------------------- | ---------------------------------------------------------------------------- |
| `omni_connection`            | `/v1/connections`                         | Credentials and `base_role` update in place; other changes replace            |
| `omni_connection_environment`| `/v1/connection-environments`             | Routes a tenant's queries to its own database or schema                       |
| `omni_connection_schedule`   | `/v1/connections/{id}/schedules`          | Schema refresh. Fails until the connection has a schema model                 |
| `omni_model`                 | `/v1/models`                              | `SCHEMA`, `SHARED`, `SHARED_EXTENSION`. See the schema model note below       |
| `omni_model_git`             | `/v1/models/{id}/git`                     | Model version control                                                         |
| `omni_folder`                | `/v1/folders`                             | Nesting up to seven levels                                                    |
| `omni_folder_permission`     | `/v1/folders/{id}/permissions`            | Destroy downgrades rather than revokes                                        |
| `omni_document_permission`   | `/v1/documents/{id}/permissions`          | Same. `document_id` is a literal ID, since documents are made in the UI       |
| `omni_label`                 | `/v1/labels`                              |                                                                                |
| `omni_color_palette`         | `/v1/color-palettes`                      |                                                                                |
| `omni_user`                  | `/scim/v2/users`                          | Organization API key required. Create does not adopt an existing account      |
| `omni_user_group`            | `/scim/v2/groups`                         | Membership is authoritative unless `member_ids` is left unset                 |
| `omni_user_model_role`       | `/v1/users/{id}/model-roles`              | Destroy downgrades to `NO_ACCESS` (no delete endpoint exists)                 |
| `omni_user_group_model_role` | `/v1/user-groups/{id}/model-roles`        | Same behaviour on destroy                                                     |

## Data sources

`omni_connection`, `omni_model`, `omni_folder`, `omni_user`, `omni_user_group`,
`omni_user_attribute` and `omni_user_attributes`. Each looks up by ID or by name, except
`omni_user_attribute`, which takes exactly one of the two and prefers the ID.

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
  name  = "Finance"
  path  = "finance"
  scope = "organization"
}
```

More in [`examples/`](./examples), including an end-to-end setup in
[`examples/complete`](./examples/complete).

## The reference deployment

[`infra/`](./infra) is a working deployment that drives the modules in [`modules/`](./modules).
It doubles as the end-to-end test: every module is exercised against a live instance through the
plan and apply workflows.

Each module takes a map that defaults to empty, so a component you do not configure creates
nothing. An internal-only deployment leaves the embed variables unset, an embed-only deployment
leaves the user variables unset, and a deployment doing both sets everything.

| Module | What it manages |
| --- | --- |
| `warehouse` | Connections |
| `modeling` | Schema models and shared models, in that order |
| `schema-refresh` | Refresh schedules, after the schema models exist |
| `access` | Users and groups |
| `content-access` | Folders, folder grants and document grants |
| `user-attributes` | Attribute values per person, referenced by definition ID |
| `embed-routing` | Per-tenant connection environments |
| `branding` | Labels and colour palettes |

Configuration lives in one file per subject, so a variable is findable from the filename:

| File | Variables |
| --- | --- |
| `connections.auto.tfvars` | `connections` |
| `models.auto.tfvars` | `models`, `git_credentials` |
| `access.auto.tfvars` | `users`, `groups` |
| `folders.auto.tfvars` | `folders`, `content_grants`, `document_grants` |
| `attributes.auto.tfvars` | `user_attributes`, `user_attribute_values`, `tenant_attribute` |
| `embed.auto.tfvars` | `tenant_base_connection`, `tenant_routing` |
| `branding.auto.tfvars` | `palettes`, `labels` |

Two conventions run through all of it.

**The map key is the identity, not the label.** Re-keying an entry moves the Terraform address, so
the object is destroyed and recreated; `display_name` or `name` relabels it in place. This matters
most for groups, where a replacement takes the membership and every grant with it.

**Reference by ID what Terraform does not create.** Anything created here is referenced by its
configuration key and resolved to an ID internally. Anything made in the UI - user attribute
definitions above all - is referenced by ID, because a rename in the UI leaves a name-based
reference valid and matching nothing, which on an access filter means row-level security that
quietly stops applying.

Root variables carry validations that name the specific entry at fault rather than failing on an
index lookup. They run at plan time, before any API call. Note that `terraform validate` does not
evaluate variable validations; only `plan` does.

## Documentation

| | |
| --- | --- |
| [docs/USAGE.md](./docs/USAGE.md) | Installation and per-resource guide, for both registry and from-source use |
| [docs/TEST-REPORT.md](./docs/TEST-REPORT.md) | What has been verified against a live instance, and what has not |
| [docs/TEST-CASES.md](./docs/TEST-CASES.md) | The test case matrix, including every API behaviour that surprised us |
| [docs/BEHAVIOUR-TESTS.md](./docs/BEHAVIOUR-TESTS.md) | The lifecycle cases a plan cannot prove: rename, re-key, destroy, membership |
| [docs/API-COVERAGE.md](./docs/API-COVERAGE.md) | Which endpoints are covered and which are not |
| [docs/EMBED-SETUP.md](./docs/EMBED-SETUP.md) | Embed specifics |

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

Warehouse credentials are not in tfvars. They come from `TF_VAR_connection_passwords`,
`TF_VAR_connection_private_keys` and `TF_VAR_connection_oauth_secrets`, set from secrets in CI.

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

Edit the `*.auto.tfvars` files in [`infra/`](./infra), using the table above to find the right
one. As shipped they create a connection, a schema model, a shared model, a refresh schedule, two
groups with roles at both scopes, a few folders with grants, two labels and a palette.

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

These are properties of the Omni API, not things left undone. Each one is recorded with its
evidence in [docs/TEST-CASES.md](./docs/TEST-CASES.md).

**A schema model is the connection's schema, not an independent object.** It shares the
connection's ID, Omni names it after the connection whatever name you send, and it cannot be
deleted through the API. Nothing built on a connection works until it exists: schedules answer
"connection does not exist" and shared models answer "schema model does not exist", both pointing
at the wrong object. Creating it is `POST /v1/models` with the connection ID and `SCHEMA`, the API
equivalent of the UI's Build Schema button. Destroying one removes it from state with a warning.

**Creating a schema model does not populate it.** Introspection is a separate
`POST /v1/models/{id}/refresh`, and it is asynchronous, so a model can exist and see no tables.
The provider starts the refresh and waits, up to 90 seconds.

**Role assignments cannot be deleted.** Destroying a role resource downgrades it to
`role_on_destroy`, default `NO_ACCESS`. Terraform reports a destroy and the assignment is still
there in another form. Omni resolves roles by priority, so a permissive `base_role` on the
connection outranks the downgrade and access survives. Folder and document permissions behave the
same way.

**User attribute definitions are read-only.** `GET /v1/user-attributes` is the only endpoint;
definitions are created in the UI. Terraform references them and assigns values. There is also no
concept of a group-assigned value: a user's value comes from a direct assignment or the
definition's default, and nothing else. Groups do carry "allowed user attribute values", which is
the set a member may switch an attribute to, and is a different thing.

**Entity embed groups cannot be created.** The Groups settings page has Standard and Embed tabs.
The Embed tab holds one group per `entity` value, created by the embed session itself along with a
shared folder named after the entity. Creating one by name over SCIM produces a Standard group
that merely shares the name. What this provider creates is the non-entity groups that a signed
URL's `groups` claim resolves to.

**Custom roles are referenced by name only.** No role catalogue endpoint was found, so a custom
role renamed in the UI breaks a grant naming it, and nothing detects that.

**`omni_user` does not adopt an existing account.** Create calls SCIM directly, so an account that
already exists has to be imported first.

**Connection reads omit credentials and dialect-specific settings,** so drift in those fields is
not detected. Terraform keeps what you configured. `default_schema` on Snowflake is accepted and
stored as null, so it is deliberately not set.

**The folders API has no get-by-id route,** so reads page the list endpoint and match on ID.

**`omni_user` tracks only the attribute keys declared in the config;** attributes set elsewhere are
left alone.

## Roadmap

dbt configuration and dbt environments, email-only users, a `omni_user_attribute_group_allowed_values`
resource for the group allowed-values endpoint, and data-source-backed references so a deployment
can point at objects an existing instance already has.

## License

MPL-2.0. This project is not an official Omni Analytics product.
