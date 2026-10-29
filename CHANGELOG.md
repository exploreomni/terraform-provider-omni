# Changelog

All notable changes to this provider are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-10-29

### Added

- `omni_dbt_configuration` - attaches a dbt project to a connection. One per
  connection: the API exposes a single `PUT` with no separate create, so
  applying it to a connection that already has dbt configured replaces that
  configuration rather than failing.
- `omni_dbt_environment` - a dbt environment routing queries to another target
  schema. Variables are given as two maps, `variables` and `secret_variables`,
  rather than a nested block; a name may appear in only one of them.
- `environment_user_attribute_name` and
  `environment_user_attribute_default_values` on `omni_connection` - the base
  connection's environment user attribute, which previously could only be set
  in the UI. A deployment using `omni_connection_environment` is now
  reproducible from configuration alone.
- A published support channel and response targets, in the README and
  `SECURITY.md`.

### Changed

- The documented API limitations were tested against a live instance rather
  than assumed, and four were wrong. Folders do have a get-by-id route and the
  provider already used it, so reads do not page a list except as a fallback.
  The base connection's environment user attribute is settable over the API, so
  it was never a UI-only field. The API reference's nested
  `environmentUserAttribute` body is the one that works; the worked guide's flat
  keys are rejected outright. And a permissions permit identifies a group by its
  eight-character id, not by a UUID.
- Two limitations were confirmed rather than corrected: there is no get-by-id
  route for models or user attributes, and no read endpoint for connection
  environments. Those paths answer a `GET` with a generic `400` because they are
  registered for other verbs, while a genuinely unknown path returns a clean
  `404`.

### Fixed

- The security contact in `SECURITY.md` was a placeholder, so the email
  fallback in a public security policy went nowhere.

### Known limitations

- **Setting a dbt environment on a model branch is not implemented.**
  `POST /v1/models/{id}/branch/{branch}/dbt` has no matching read, so a resource
  for it could not detect drift at all.
- **A default dbt environment cannot be destroyed on its own.** The first
  environment on a configuration becomes its default, and Omni refuses to delete
  it. Destroying one drops it from state with a warning; it is deleted along
  with its `omni_dbt_configuration`.
- **Secret dbt variable values are not read back.** The API returns `null` for a
  secret value, so adding or removing a key is detected but changing a value is
  not.
- **System user attributes have no ID.** `GET /v1/user-attributes` returns an
  empty `id` for every `system: true` attribute, so those can only be referenced
  by name.

## [0.1.1] - 2026-10-22

Held back from the Terraform Registry by an ingestion fault until 2026-10-29
([hashicorp/terraform-registry-support#73](https://github.com/hashicorp/terraform-registry-support/issues/73)),
when it was published.

### Fixed

- The provider `source` address in the published documentation, in every example
  and across the reference deployment was an unsubstituted `$(OWNER)`
  placeholder, so a user copying the install snippet got a configuration that
  could not resolve the provider. CI now fails on an unsubstituted placeholder.

### Added

- Examples for the ten resources that had none, and regenerated registry
  documentation carrying them.
- `CONTRIBUTING.md`, `SECURITY.md` and issue templates.

### Changed

- The README was rewritten as a user-facing front page, with the maintainer and
  deployment CI/CD material moved into `docs/`.

## [0.1.0] - 2026-10-19

First public release, published to the Terraform Registry as
`exploreomni/omni`.

### Added

Resources:

- `omni_connection`, `omni_connection_environment`, `omni_connection_schedule` -
  warehouse connections, their per-environment overrides, and schema refresh
  schedules
- `omni_model`, `omni_model_git` - shared, schema and shared extension models,
  and Git sync for model YAML
- `omni_folder`, `omni_folder_permission`, `omni_document_permission` - the
  content hierarchy and access to it
- `omni_user`, `omni_user_group` - identity, including attribute values on a
  managed user
- `omni_user_model_role`, `omni_user_group_model_role` - role assignments on a
  model or a connection
- `omni_user_attribute`, `omni_user_attribute_selectable_values` - attribute
  definitions, and the values a user or group may switch between
- `omni_label`, `omni_color_palette` - branding

Data sources:

- `omni_connection`, `omni_folder`, `omni_model`, `omni_user`,
  `omni_user_group`, `omni_user_attribute`, `omni_user_attributes`

### Known limitations

These follow from the Omni REST API rather than from the provider. Each has a
request open with Omni's product team.

- **Role assignments cannot be deleted.** The API has no delete, so destroying
  `omni_user_model_role` or `omni_user_group_model_role` downgrades the role to
  `NO_ACCESS` instead. Note that `NO_ACCESS` is not revocation: Omni resolves
  roles by priority, and a connection's base role outranks a destroyed direct
  assignment.
- **A group subject for selectable values needs a literal UUID.** That endpoint
  identifies a group by its UUID primary key while every other group-scoped
  endpoint uses the eight-character id that `omni_user_group.id` holds. The two
  are generated independently and no endpoint returns the UUID, so it cannot be
  a reference.
- **Connection environments cannot be read.** Create, update and delete work;
  without a read endpoint the provider cannot detect drift on them.
- **Custom roles are invisible to the API.** Only built-in roles can be
  assigned. A deployment that depends on a custom role needs that role created
  by hand, with a matching name, before any assignment referencing it applies.
- **Attribute values can only be set on a managed user.** Values are written
  through SCIM, so an account provisioned by an identity provider takes its
  values from that provider's attribute mapping. See
  [docs/SCIM-COEXISTENCE.md](docs/SCIM-COEXISTENCE.md).

[Unreleased]: https://github.com/exploreomni/terraform-provider-omni/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/exploreomni/terraform-provider-omni/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/exploreomni/terraform-provider-omni/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/exploreomni/terraform-provider-omni/releases/tag/v0.1.0
