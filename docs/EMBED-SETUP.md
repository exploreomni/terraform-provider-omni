# Setting up embedding with Terraform

What a multi-tenant embed deployment looks like as code, in the order you build
it, and what still has to be done by hand.

The short version: Terraform owns everything the signed URL *refers to*. It does
not own identity. A magic URL carries `externalId`, `name`, `userAttributes`,
`groups` and `contentPath` at session time, and every one of those is a
reference to something that must already exist on the instance. Making those
referents identical across dev, stage and prod is the job.

---

## Before Terraform: three manual steps

These cannot be automated today. Do them once per instance.

### 1. Create the user attribute definitions

**Settings > User attributes.** Create the attribute your tenant scoping keys
off, for example `tenant_id`. Note its **Reference** value, which is what you
use everywhere else.

The API exposes only `GET /v1/user-attributes`. There is no create endpoint, so
the schema your entire row-level security depends on is the one part of the
deployment that cannot be versioned or reviewed. This is the single most
valuable API gap to close.

### 2. Create the embed secret

**Settings > Embed.** Generate the signing secret your application uses to build
signed URLs.

No endpoint for creating or rotating this appears in the API, so a new instance
cannot be stood up, and a secret cannot be rotated, without a person in the UI.

### 3. Have warehouse credentials ready

One set per tenant connection if you are isolating physically.

---

## The Terraform build, in order

### Step 1: connections

A base connection that sessions query through, and one per tenant to route to.

```hcl
variable "tenants" {
  type = map(object({
    database = string
    schema   = string
  }))
  default = {
    acme  = { database = "ANALYTICS", schema = "ACME" }
    globex = { database = "ANALYTICS", schema = "GLOBEX" }
  }
}

resource "omni_connection" "base" {
  name      = "embed-base"
  dialect   = "snowflake"
  host      = var.snowflake_account
  database  = "ANALYTICS"
  warehouse = "EMBED_WH"
  username  = "OMNI_EMBED"
  password  = var.snowflake_password

  # The access floor for everyone. Set it to NO_ACCESS and grant per model,
  # or destroying a role assignment will not actually revoke access.
  base_role = "NO_ACCESS"
}

resource "omni_connection" "tenant" {
  for_each = var.tenants

  name           = "embed-${each.key}"
  dialect        = "snowflake"
  host           = var.snowflake_account
  database       = each.value.database
  default_schema = each.value.schema
  warehouse      = "EMBED_WH"
  username       = "OMNI_EMBED"
  password       = var.snowflake_password
  base_role      = "NO_ACCESS"
}
```

Set the environment user attribute name on the base connection in the UI. The
provider does not expose that field yet.

### Step 2: connection environments

Route each tenant's attribute value to its connection.

```hcl
resource "omni_connection_environment" "tenant" {
  for_each = var.tenants

  base_connection_id        = omni_connection.base.id
  environment_connection_id = omni_connection.tenant[each.key].id
  user_attribute_values     = [each.key]
}
```

A session whose signed URL carries `userAttributes: {tenant_id: "acme"}` now
queries `embed-acme` instead of the base connection. This is physical isolation:
tenants are in different databases, not filtered rows.

**This resource cannot detect drift.** The API has no read endpoint for
connection environments, so Terraform keeps what it wrote and cannot tell you if
someone changed it in the UI.

### Step 3: the model

```hcl
data "omni_model" "shared" {
  name       = "embed-base"
  model_kind = "SHARED"
}

resource "omni_model" "embed" {
  name          = "embed_metrics"
  model_kind    = "SHARED_EXTENSION"
  base_model_id = data.omni_model.shared.id
  connection_id = data.omni_model.shared.connection_id
}

# Definitions cannot be created through the API, so reference the one the
# instance must already have. An instance missing it fails here rather than
# applying cleanly with scoping that does nothing.
data "omni_user_attribute" "tenant" {
  name = "tenant_id"
}
```

Take the connection from the base model rather than looking it up separately.
Resolving them independently lets them disagree, and the API rejects that.

Access grants and access filters, which do the row-level scoping, are model
content. They are declared in model YAML and versioned through git sync, not
here. The API does accept access grants when a model is created, but using that
would mean two systems writing one source of truth.

### Step 4: groups

One per tenant, matching what the signed URL will pass in `groups`.

```hcl
resource "omni_user_group" "tenant" {
  for_each = var.tenants

  display_name = "tenant-${each.key}"
}
```

Do not set `member_ids`. Embed users are created by the session, not by
Terraform, and membership here is authoritative: anything Terraform manages it
will also remove.

### Step 5: model roles

What each tenant group can do with data.

```hcl
resource "omni_user_group_model_role" "tenant" {
  for_each = var.tenants

  user_group_id = omni_user_group.tenant[each.key].id
  model_id      = omni_model.embed.id
  connection_id = omni_connection.base.id
  role_name     = "QUERY_TOPICS"
}
```

`QUERY_TOPICS` is Restricted Querier: the group can query the topics it is given
and nothing else. For embed that is usually the right level.

### Step 6: folders

Where each tenant's content lives, which is what `contentPath` points at.

```hcl
resource "omni_folder" "tenants" {
  name = "Tenants"
  path = "tenants"
}

resource "omni_folder" "tenant" {
  for_each = var.tenants

  name               = each.key
  parent_folder_id   = omni_folder.tenants.id
  delete_recursively = true
}
```

### Step 7: folder permissions

The step that makes `contentPath` mean anything.

```hcl
resource "omni_folder_permission" "tenant" {
  for_each = var.tenants

  folder_id      = omni_folder.tenant[each.key].id
  role           = "VIEWER"
  user_group_ids = [omni_user_group.tenant[each.key].id]
}
```

One resource per role. `VIEWER` lets a tenant see dashboards without editing
them. Use `EXPLORER` if they should be able to drill and explore.

### Step 8: the dashboards

Build them in the UI, move them into the tenant folder. Not Terraform's job:
dashboards are authored interactively, and the provider deliberately does not
manage them.

### Step 9: the signed URL

Application code, not infrastructure:

```
externalId:     your user identifier
name:           display name
userAttributes: { tenant_id: "acme" }
groups:         ["tenant-acme"]
contentPath:    "/tenants/acme"
```

Every value here resolves to something the steps above created.

---

## Adding a tenant

```hcl
acme   = { database = "ANALYTICS", schema = "ACME" }
globex = { database = "ANALYTICS", schema = "GLOBEX" }
initech = { database = "ANALYTICS", schema = "INITECH" }   # new
```

One line, one pull request, a reviewable plan, and an apply behind an approval
gate. That is the point of the exercise.

---

## What you cannot do today

### Blocking, needs API work

| Gap | Consequence |
| --- | --- |
| **Create user attribute definitions** | `GET` only. The schema your tenant scoping depends on is created by hand and cannot be reviewed or replicated from code. |
| **Create or rotate the embed secret** | No endpoint found. A new instance always needs a person in the UI, and rotating across environments is manual. |
| **Read connection environments** | No `GET`. Terraform cannot detect drift on the resource that implements physical tenant isolation. |

### Missing from the provider, buildable

| Gap | Notes |
| --- | --- |
| Environment user attribute name on a connection | Set in the UI for now. |
| `omni_label`, schedules for content, dbt config, model git config | All have full CRUD in the API. |
| Document permissions beyond grants | The ability flags, `canDownload` and so on, are not yet a resource. |

### Out of scope by design

| | Why |
| --- | --- |
| Embed user provisioning | Sessions create them. The API is read and delete only, which is correct. |
| Dashboard authoring | Interactive work belongs in the UI. |
| Access grants and access filters | Model content. Declared in model YAML, versioned through git sync. |
| Generating signed URLs | Runtime, not infrastructure. |

### Practical limits

**Rate limit.** 60 requests a minute. Each tenant is roughly five resources, and
several read back after writing. A hundred tenants is a slow apply. Use
`-parallelism=5` and expect to split large configurations.

**Secrets in state.** Connection passwords are stored in plain text in Terraform
state. With per-tenant connections that is one credential per tenant. Treat
state as the most sensitive artifact in the deployment.

**Drift from session activity.** Embed sessions create users and set attribute
values continuously. Keep Terraform out of anything the session touches, or
every plan will show churn.

---

## Honest status

Steps 1, 3, 4, 5 and 6 are verified end to end against a live instance.

Step 2, connection environments, is verified for create, update and delete
against placeholder connections. It has never been exercised against a real
warehouse, and by construction its read path cannot be verified at all.

Step 7, folder permissions, was fixed after the permits response turned out to
be shaped differently from its documentation, and is awaiting a confirming run.

No part of this has been run against a production instance.
