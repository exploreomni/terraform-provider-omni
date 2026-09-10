# Using the Omni Terraform provider

Manage an Omni instance as code: connections, models, folders, users, groups
and role assignments.

Two ways to install it, depending on whether it has been published to the
Terraform Registry. Section 1 covers the published path, section 2 covers
running it from source. Everything after that is the same either way.

---

## 1. Installing from the Terraform Registry

This is the normal path once the provider is published. Nothing to build, no
Go toolchain, no local configuration.

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
  base_url  = "https://blobsrus.omniapp.co"
  api_token = var.omni_api_token
}
```

```bash
terraform init
```

Terraform downloads the right binary for your platform and records its checksum
in `.terraform.lock.hcl`. Commit that lock file. It is what guarantees every
machine and every CI runner uses the same provider build.

Pin the version. `~> 0.1` accepts 0.1.x patches but not 0.2.0, which is what
you want while the provider is pre-1.0 and its schema can still change.

### Upgrading

```bash
terraform init -upgrade
```

Read the changelog before doing this in production. Below 1.0, a minor version
bump is allowed to break compatibility.

---

## 2. Running it from source

Use this when the provider has not been published yet, or when you are testing
a change before release. Two options: a dev override for local work, and a
filesystem mirror for CI.

### 2a. Dev override, for local work

Best for iterating. Terraform uses your locally built binary and skips
installation entirely.

```bash
git clone https://github.com/ashima-omni/terraform-provider-omni.git
cd terraform-provider-omni
go build ./... && go install .
go env GOPATH        # note the path
```

In `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "ashima-omni/omni" = "/Users/you/go/bin"
  }
  direct {}
}
```

Then run Terraform as usual, with two differences:

- **Do not run `terraform init`.** The override bypasses installation, and
  `init` will fail because the provider is not in the registry.
- Terraform prints a warning on every command saying an override is active.
  That is expected.

This file is global. While it exists, every Terraform project on the machine
resolves `ashima-omni/omni` to your local build. Delete it when you are done.

**A dev override cannot be used with a remote backend**, because a remote
backend requires `terraform init` and `init` will try to reach the registry.
For that combination use a filesystem mirror instead.

### 2b. Filesystem mirror, for CI or a remote backend

A mirror gives `init` a local source to install from, so `init` works normally
and remote state works with it.

Build the binary into the directory layout Terraform expects:

```bash
VERSION=0.1.0
MIRROR="$HOME/.terraform.d/plugins/registry.terraform.io/ashima-omni/omni/${VERSION}/linux_amd64"
mkdir -p "$MIRROR"
go build -o "${MIRROR}/terraform-provider-omni_v${VERSION}" .
```

Replace `linux_amd64` with your platform: `darwin_arm64` on Apple silicon,
`darwin_amd64` on Intel Macs, `windows_amd64` on Windows.

In `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/home/you/.terraform.d/plugins"
    include = ["registry.terraform.io/ashima-omni/omni"]
  }
  direct {
    exclude = ["registry.terraform.io/ashima-omni/omni"]
  }
}
```

The `direct` block with `exclude` matters. Without it, `init` still asks the
registry about this provider and fails.

In GitHub Actions, note that `hashicorp/setup-terraform` overwrites
`~/.terraformrc` when you pass it a credentials token, so write the mirror
config **after** that action runs, and append rather than overwrite. The
`.github/workflows/terraform-plan.yml` in this repo shows the pattern.

Terraform will warn that the lock file only has checksums for one platform.
That is expected for a mirror. Do not commit that lock file, or a colleague on
a different platform cannot install.

---

## 2c. OpenTofu

The provider serves protocol version 6, which OpenTofu implements, and the CRUD
suite runs under both CLIs in CI. Configuration is identical.

One difference matters: **OpenTofu resolves an unqualified source against
`registry.opentofu.org`**, not the Terraform registry. Until the provider is
published to both, either qualify the source:

```hcl
terraform {
  required_providers {
    omni = {
      source = "registry.terraform.io/ashima-omni/omni"
    }
  }
}
```

or mirror the binary under the OpenTofu hostname:

```
~/.terraform.d/plugins/registry.opentofu.org/ashima-omni/omni/0.1.0/<platform>/
```

OpenTofu reads `~/.tofurc` rather than `~/.terraformrc`, with the same syntax.
Dev overrides work the same way.

---

## 3. Authentication

Generate a token in Omni under **Settings > API keys**.

| Token type | Works for |
| --- | --- |
| Organization API key | Everything, including `omni_user` and `omni_user_group` |
| Personal access token | Connections, models, folders, role assignments |

The SCIM endpoints behind users and groups reject personal access tokens, so
if you manage people, use an organization key.

Supply credentials by environment variable rather than in configuration:

```bash
export OMNI_BASE_URL=https://blobsrus.omniapp.co
export OMNI_API_TOKEN=...
```

The provider also accepts `OMNI_API_KEY`, and `OMNI_TIMEOUT_SECONDS` to change
the per-request timeout from its default of 60.

`base_url` is the URL you log in with. The provider appends `/api` itself, so
both forms work.

---

## 4. Resources

### omni_connection

```hcl
resource "omni_connection" "warehouse" {
  name      = "Production Snowflake"
  dialect   = "snowflake"
  host      = "myaccount"          # account identifier only
  database  = "ANALYTICS"
  warehouse = "COMPUTE_WH"
  username  = "OMNI_SVC"
  password  = var.snowflake_password
  base_role = "NO_ACCESS"
}
```

Only `password`, `private_key`, `oauth_client_secret` and `base_role` can be
changed in place. **Every other change destroys and recreates the connection**,
which takes every model built on it with it. Treat a change to `host` or
`database` as a migration, not an edit.

Credentials are never returned by the API, so the provider cannot detect drift
in them. If someone rotates the password in the UI, Terraform will not notice.

`base_role` is the access floor for the whole connection. Set it to
`NO_ACCESS` if you intend to grant access per model.

### omni_model

```hcl
data "omni_model" "shared" {
  name       = "Production Snowflake"
  model_kind = "SHARED"
}

resource "omni_model" "finance" {
  name          = "finance_metrics"
  model_kind    = "SHARED_EXTENSION"
  base_model_id = data.omni_model.shared.id
  connection_id = data.omni_model.shared.connection_id
}
```

Take `connection_id` from the base model as shown. Resolving the two
independently lets them disagree, and the API rejects a base model that does
not belong to the given connection.

Destroy archives the model to the trash rather than deleting it. It is
recoverable, but its content is not restored automatically.

Model YAML is deliberately not managed here. Omni versions model content
through git sync, and a second writer would fight it.

### omni_folder

```hcl
resource "omni_folder" "reporting" {
  name = "Reporting"
  path = "reporting"
}

resource "omni_folder" "finance" {
  name               = "Finance"
  parent_folder_id   = omni_folder.reporting.id
  delete_recursively = true
}
```

Nesting goes seven levels deep. A child inherits its parent's scope, and its
path is derived unless you set one.

Renaming a parent rewrites every descendant path. Terraform handles this, but
expect descendant `path` and `url` values to show as "known after apply" when
an ancestor changes.

`delete_recursively` defaults to `false`, so destroying a folder that still
contains documents fails. Set it to `true` only where you mean it.

### omni_user and omni_user_group

```hcl
resource "omni_user" "analyst" {
  user_name    = "blob@blobsrus.co"
  display_name = "Blob Ross"

  attributes = {
    region = "APAC"
  }
}

resource "omni_user_group" "finance" {
  display_name = "Finance"
  member_ids   = [omni_user.analyst.id]
}
```

Both need an organization API key.

Group membership is authoritative: a user removed from `member_ids` is removed
from the group. If people are added to groups through the UI as well,
Terraform will remove them on the next apply. Either manage a group fully in
Terraform or not at all.

`attributes` tracks only the keys you declare. Attributes set elsewhere are
left alone. Attribute **definitions** cannot be managed, because the API only
exposes a read endpoint for them; create them in the UI first and reference
them by their Reference value.

### omni_user_model_role and omni_user_group_model_role

```hcl
resource "omni_user_group_model_role" "finance_querier" {
  user_group_id = omni_user_group.finance.id
  model_id      = omni_model.finance.id
  role_name     = "QUERIER"
}
```

Roles: `VIEWER`, `QUERIER`, `QUERY_TOPICS` (Restricted Querier), `MODELER`,
`CONNECTION_ADMIN`, `NO_ACCESS`, or a custom role.

`model_id` is required for every role except `CONNECTION_ADMIN`, which can be
granted at the connection level instead.

**Destroy does not delete a role assignment.** The API has no endpoint for
that, so the provider downgrades the role to `role_on_destroy`, default
`NO_ACCESS`.

**And a downgrade to `NO_ACCESS` does not necessarily revoke access.** Omni
resolves roles by priority. A direct assignment sits at priority 0; the
connection's base role sits at 150 and wins. If the connection grants
`QUERIER` to everyone, destroying an assignment leaves the subject with
`QUERIER`. If you need destroy to actually remove access, set the connection's
`base_role` to `NO_ACCESS` and grant access explicitly per model.

---

## 5. Data sources

```hcl
data "omni_connection" "warehouse" { name = "Production Snowflake" }
data "omni_model" "shared"         { name = "Production Snowflake", model_kind = "SHARED" }
data "omni_user" "blob"            { user_name = "blob@blobsrus.co" }
data "omni_user_group" "finance"   { display_name = "Finance" }
```

Each takes either `id` or a name. Use these to reference things created outside
Terraform rather than hardcoding IDs, which differ between instances.

---

## 6. Common patterns

### One config, several instances

```hcl
provider "omni" {
  base_url  = var.omni_base_url
  api_token = var.omni_api_token
}
```

```bash
terraform workspace new stage
terraform apply -var-file=stage.tfvars
```

The value of this provider is mostly here: dev, stage and prod become the same
file with different variables, instead of three instances that drifted apart
because someone configured them by hand.

### Adopting an existing instance

Import rather than recreate:

```bash
terraform import omni_folder.reporting 21db26b3-466c-4791-90e7-b9ce9375426d
terraform import omni_user.blob 9e8719d9-276a-4964-9395-a493189a247c
terraform import omni_user_model_role.blob "<user id>:<model id>"
```

Role resources import with a composite `<subject id>:<model id>` ID.

Always run `terraform plan` afterwards. A clean plan means the import matched
your configuration. A plan proposing changes means it did not, and applying
would modify the real object.

### CI/CD

Plan on pull request, apply on merge, with an approval gate. The workflows in
`.github/workflows/terraform-plan.yml` and `terraform-apply.yml` implement
this. Two things they get right that are easy to miss:

- **Remote state.** CI runners are ephemeral. With local state every run starts
  from nothing and recreates everything.
- **Always destroy in test pipelines**, using `if: always()`, or a failed run
  leaves objects behind with no state to clean them from.

For large configurations use `terraform apply -parallelism=5`. The API allows
60 requests a minute and the provider retries on 429, but staying under the
limit is better than backing off.

---

## 7. Troubleshooting

**`Failed to query available provider packages`**
`terraform init` is asking the registry for an unpublished provider. Use a
filesystem mirror, not a dev override. See section 2b.

**`Inconsistent dependency lock file`**
The lock file references a provider version that is not installed. Run
`terraform init`, or delete `.terraform.lock.hcl` if you are using a mirror.

**`Provider produced inconsistent result after apply`**
A provider bug. Please open an issue with the resource type and the attribute
named in the message.

**Terraform wants to recreate something that clearly exists**
The read path is failing. Check that the token has permission on the object.
Organization keys and personal access tokens see different things, and folder
reads in particular behave differently between the two.

**`members: Required` on a group update**
Fixed in current versions, which use SCIM PATCH. Upgrade the provider.

**Group members disappearing**
Membership is authoritative. Something is adding members outside Terraform and
Terraform is removing them. Manage the group entirely in code, or drop
`member_ids` from the resource.

---

## 8. What this provider does not do

- **Dashboards and documents.** They belong in the UI, where analysts build
  them. Terraform is the wrong shape for interactively authored content.
- **Model YAML.** Managed through Omni's git sync.
- **User attribute definitions.** The API exposes only a read endpoint.
- **Schedules, embed users, labels.** Not implemented yet. All are exposed by
  the API and are reasonable additions.

The boundary is roughly: things a platform team owns and should review in a
pull request, as opposed to things analysts create day to day.
