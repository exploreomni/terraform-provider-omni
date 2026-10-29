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
| `omni_dbt_configuration`     | `/v1/connections/{id}/dbt`                | One per connection. A `PUT` with no create, so apply replaces any existing one |
| `omni_dbt_environment`       | `/v1/connections/{id}/dbt/environments`   | Needs `enable_virtual_schemas`. Secret variable values are not read back      |
| `omni_folder`                | `/v1/folders`                             | Nesting up to seven levels                                                    |
| `omni_folder_permission`     | `/v1/folders/{id}/permissions`            | Destroy downgrades rather than revokes                                        |
| `omni_document_permission`   | `/v1/documents/{id}/permissions`          | Same. `document_id` is a literal ID, since documents are made in the UI       |
| `omni_label`                 | `/v1/labels`                              |                                                                                |
| `omni_color_palette`         | `/v1/color-palettes`                      |                                                                                |
| `omni_user_attribute`        | `/v1/user-attributes`                     | Destroy removes the value every user holds, and breaks model SQL and embed URLs naming it     |
| `omni_user_attribute_selectable_values` | `/v1/user-attributes/{id}/selectable-values` | Values a user or group may switch to. Takes a membership ID, not a user ID       |
| `omni_user`                  | `/scim/v2/users`                          | Organization API key required. Create does not adopt an existing account      |
| `omni_user_group`            | `/scim/v2/groups`                         | Membership is authoritative unless `member_ids` is left unset                 |
| `omni_user_model_role`       | `/v1/users/{id}/model-roles`              | Destroy downgrades to `NO_ACCESS` (no delete endpoint exists)                 |
| `omni_user_group_model_role` | `/v1/user-groups/{id}/model-roles`        | Same behaviour on destroy                                                     |

## Data sources

`omni_connection`, `omni_model`, `omni_folder`, `omni_user`, `omni_user_group`,
`omni_user_attribute` and `omni_user_attributes`. Each looks up by ID or by name, except
`omni_user_attribute`, which takes exactly one of the two and prefers the ID. Use the data source
for an attribute someone made in the UI, and the resource of the same name for one Terraform owns.

## Installation

The provider is published to the Terraform Registry, so there is nothing to
download by hand. Declare it and run `terraform init`:

```hcl
terraform {
  required_providers {
    omni = {
      source  = "exploreomni/omni"
      version = "~> 0.1"
    }
  }
}
```

Terraform fetches the signed release and verifies it against the GPG key
registered on the `exploreomni` namespace. Works with Terraform 1.0 and later.

OpenTofu resolves against its own registry by default, so name this one:

```hcl
source = "registry.terraform.io/exploreomni/omni"
```

## Configuring the provider

```hcl
provider "omni" {
  base_url  = "https://yourinstance.omniapp.co"
  api_token = var.omni_api_token
}
```

Both arguments fall back to `OMNI_BASE_URL` and `OMNI_API_TOKEN`. Prefer the
environment in CI, so the token never reaches a `.tf` file:

```bash
export OMNI_BASE_URL=https://yourinstance.omniapp.co
export OMNI_API_TOKEN=...
terraform plan
```

## Your first configuration

```hcl
resource "omni_folder" "finance" {
  name  = "Finance"
  path  = "finance"
  scope = "organization"
}

resource "omni_user_group" "analysts" {
  display_name = "Analysts"
}

resource "omni_folder_permission" "analysts_finance" {
  folder_id      = omni_folder.finance.id
  role           = "VIEWER"
  user_group_ids = [omni_user_group.analysts.id]
}
```

A folder, a group, and the grant that connects them. Most Omni configuration
is that shape repeated.

## What people use this for

**Keeping instances identical.** One configuration and a tfvars file per
instance. The same folder tree, groups and model roles apply to dev, stage and
production, so the three stop drifting apart.

**Reviewing access as code.** A role grant becomes a diff. `terraform plan` in
a pull request shows who gains access to what, and merging is the approval.

**Rebuilding an instance from nothing.** Connections, models, folders, groups
and permissions all come from configuration, so standing up a new instance is
an apply rather than an afternoon of clicking.

**Running alongside an identity provider.** If Okta or Entra provisions users
over SCIM, let it own identity and let Terraform own authorization. See
[docs/SCIM-COEXISTENCE.md](./docs/SCIM-COEXISTENCE.md), which is the first
thing to read on any instance with more than a handful of people.

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
| [docs/SCIM-COEXISTENCE.md](./docs/SCIM-COEXISTENCE.md) | Running this alongside an identity provider that provisions users over SCIM |
| [docs/MODEL-GIT.md](./docs/MODEL-GIT.md) | Connecting a model to a git repository, including the deploy key |
| [docs/EMBED-SETUP.md](./docs/EMBED-SETUP.md) | Embed specifics |

## Running it in CI

Setting up GitHub Actions for the reference deployment - where state lives,
which secrets the workflows need, and how a plan is reviewed before it is
applied - is in [docs/CI-CD.md](docs/CI-CD.md).

The workflows this repository runs are in [`.github/workflows/`](./.github/workflows),
and the two that drive the reference deployment are a reasonable starting point for
your own:

| Workflow | What it does |
| --- | --- |
| [`terraform-plan.yml`](./.github/workflows/terraform-plan.yml) | Runs `fmt`, `validate` and `plan` on every pull request and posts the plan as a PR comment |
| [`terraform-apply.yml`](./.github/workflows/terraform-apply.yml) | Applies on merge to `main`, behind an optional approval environment |

Copy both into your repository and adjust:

- `working-directory` and the `paths` filters, which point at `infra/` and `modules/` here.
- Drop `internal/**` and `main.go` from the `paths` filters, and the `setup-go` step.
  Those only exist because this repository also builds the provider; yours installs it
  from the registry during `terraform init`.
- The `TF_VAR_*` secrets, to match the variables your configuration actually takes.
- The `environment:` name on the apply job, if you use an approval gate.

The rest of the workflows (`test`, `release`, the acceptance suites) are for developing
the provider itself and are not needed in a repository that only uses it.

## Authentication

Generate a token under **Settings > API keys** in Omni.

- Organization API keys work for every resource, including users and groups.
- Personal access tokens work for connections, models, folders, and role
  assignments, but the SCIM user and group endpoints reject them.

Supply them as `OMNI_BASE_URL` and `OMNI_API_TOKEN` rather than putting them in
configuration.

## Contributing

Building the provider, running the acceptance suite and regenerating the
registry documentation are covered in [CONTRIBUTING.md](CONTRIBUTING.md).

## Support

Open an issue for a bug or a feature request. For anything that needs a private
channel, including a security report, write to <ps@omni.co>, which is the
maintaining team's address rather than an individual's.

Response targets:

| Severity | First response |
| --- | --- |
| Critical, meaning the provider is unusable or a security issue | 2 business days |
| Everything else | 5 business days |

A fix for a confirmed critical issue goes out in the next patch release. See
[SECURITY.md](SECURITY.md) for how to report a vulnerability.

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

**There is no group-assigned attribute value.** A user's value comes from a direct assignment or
the definition's default, and nothing else. Groups carry "selectable values", which is the set a
member may switch an attribute to, and is a different thing: `omni_user_attribute_selectable_values`
manages it. Definitions themselves are no longer read-only; `omni_user_attribute` creates them.

**Entity embed groups cannot be created.** The Groups settings page has Standard and Embed tabs.
The Embed tab holds one group per `entity` value, created by the embed session itself along with a
shared folder named after the entity. Creating one by name over SCIM produces a Standard group
that merely shares the name. What this provider creates is the non-entity groups that a signed
URL's `groups` claim resolves to.

**Connection environment drift is undetectable.** `omni_connection_environment` routes queries on
values of the base connection's environment user attribute. That attribute is now set through
`omni_connection`, and it reads back, so drift on the base connection is detected. What is not
detected is the environment side: there is no read endpoint on `/v1/connection-environments`, so the
mapping of an environment connection to its attribute values cannot be read back at all.

**System user attributes have no ID.** `GET /v1/user-attributes` returns `"id": ""` for the
`system: true` attributes, so those can only be referenced by name.

**Custom roles are referenced by name only.** No role catalogue endpoint was found, so a custom
role renamed in the UI breaks a grant naming it, and nothing detects that.

**`omni_user` does not adopt an existing account.** Create calls SCIM directly, so an account that
already exists has to be imported first.

**Connection reads omit credentials and dialect-specific settings,** so drift in those fields is
not detected. Terraform keeps what you configured. `default_schema` on Snowflake is accepted and
stored as null, so it is deliberately not set.

**Folder reads use `GET /v1/folders/{id}`.** That response omits `scope` and `ownerId`, which live
on the permissions endpoint and are both `RequiresReplace`, so they are carried through from prior
state rather than being allowed to read as a change. Import has no prior state, so it pages the
list endpoint once to recover `ownerId`.

**`omni_user` tracks only the attribute keys declared in the config;** attributes set elsewhere are
left alone.

## Roadmap

Email-only users, for schedule recipients who hold no Omni seat, and
data-source-backed references so a deployment can point at objects an existing
instance already has.

dbt configuration and environments, and the base connection's environment user
attribute, are done and are described above.

A separate resource for group allowed values was planned and then dropped.
`omni_user_attribute_selectable_values` already covers both subjects through the
same endpoint, so a second resource would have shared it for no gain. What
blocks that path is the identifier, not the resource: see the limitation above.

## License

MPL-2.0. This project is not an official Omni Analytics product.
