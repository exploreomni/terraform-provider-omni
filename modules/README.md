# Modules

Five modules covering both internal and embedded deployments. Every one is
driven by a map that defaults to empty, so a component you do not configure
creates nothing. Pick what you need.

| Module | Creates | Internal | Embed |
| --- | --- | --- | --- |
| `warehouse` | Connections and schema refresh schedules | Yes | Yes |
| `modeling` | Models and their git connections | Yes | Yes |
| `access` | Internal users, groups, folders, permissions, model roles | Yes | No |
| `embed-tenant` | Per-tenant routing, group, folder, permission, model role | No | Yes, once per tenant |
| `branding` | Colour palettes and the label taxonomy | Optional | Optional |

## Choosing

**Internal only.** `warehouse`, `modeling`, `access`. Leave `tenants` unset.
See `infra/examples/internal.tfvars`.

**Embed only.** `warehouse`, `modeling`, `embed-tenant`. Leave `users`,
`groups` and `folders` unset. See `infra/examples/embed.tfvars`.

**Both.** The common shape: an internal team builds the content tenants
consume. See `infra/examples/both.tfvars`.

## Why these boundaries

`warehouse` and `modeling` are shared because a connection is a connection
whether a person or a signed URL queries it.

`access` and `embed-tenant` are separate because the two kinds of identity work
differently. Internal users are provisioned through SCIM and hold seats.
Embed users are created by the session that signs them in, which is why
`embed-tenant` deliberately sets no group members: membership is authoritative
here, so Terraform would remove whoever the session added.

`branding` is separate because palettes and labels are instance-wide and
usually owned by someone other than whoever onboards tenants.

## What is not here, and why

**Model content.** Topics, views, fields, access grants and access filters are
versioned through git sync. A second writer would fight it. The `modeling`
module creates the model and connects the repository; what the repository
contains is git's business.

**Dashboards.** Authored interactively in the UI.

**Applying labels to content.** The taxonomy is configuration; tagging a
document is part of making the document.

**Embed entities and the embed secret.** No API exists for either, so every
embed deployment includes a manual step. See `docs/API-COVERAGE.md`.

## Conventions

**`base_role = "NO_ACCESS"` on connections.** Access is granted per model. This
is not stylistic: role assignments cannot be deleted through the API, so
destroying one only downgrades it to `NO_ACCESS`, which loses to a permissive
base role. With the floor at `NO_ACCESS`, revocation works.

**IDs are passed in, not looked up.** A module never reaches outside itself.
The root resolves `connection = "warehouse"` to an ID and hands it over, so
dependencies are visible in `infra/main.tf`.

**Secrets come from variables.** Passwords and deploy keys are never literals
in a module. They are also stored in plain text in Terraform state, which makes
the state backend the most sensitive thing in this repository.
