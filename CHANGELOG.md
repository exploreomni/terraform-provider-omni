# Changelog

All notable changes to this provider are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/exploreomni/terraform-provider-omni/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/exploreomni/terraform-provider-omni/releases/tag/v0.1.0
