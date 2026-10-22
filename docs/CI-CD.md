# CI/CD for the reference deployment

How to run the configuration in `infra/` and `modules/` from GitHub Actions:
where state lives, which secrets the workflows need, and how a plan is
reviewed before it is applied.

This is about operating a deployment with the provider. To work on the
provider itself, see [CONTRIBUTING.md](../CONTRIBUTING.md).

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

Warehouse credentials and git deploy keys are not in tfvars. The workflows pass them as
`TF_VAR_*` from these secrets, each a JSON object keyed the same as the connection or model:

| Secret | Variable |
| --- | --- |
| `CONNECTION_PASSWORDS` | `connection_passwords` |
| `CONNECTION_PRIVATE_KEYS` | `connection_private_keys` |
| `CONNECTION_OAUTH_SECRETS` | `connection_oauth_secrets` |
| `GIT_CREDENTIALS` | `git_credentials` |

Each is passed as `${{ secrets.NAME || '{}' }}`, and the fallback is load-bearing. An unset secret
arrives as an empty string, and Terraform parses a `TF_VAR_` value as HCL for any variable that is
not a string, so an empty one fails the run with "Missing expression" rather than falling back to
the variable's default. `{}` is a valid empty map, so you only set the secrets you actually use.

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

### Checking the configuration without the API

```bash
scripts/plan-harness.py
scripts/plan-harness.py --expect-error 'not in var.groups'
```

`terraform validate` does not evaluate variable validations, and it does not
evaluate module expressions against real values either. Both gaps have shipped
bugs from this repository, so the harness closes them: it rewrites `infra/` and
`modules/` into a configuration with no providers in it - every `omni_*` resource
becomes a local holding its evaluated body - and plans that against the real
tfvars.

So the `for_each` filters, the merges, the `coalesce` calls and all the
validations are evaluated, and `--expect-error` asserts that a bad tfvars fails
with the message it should. Cycles are caught too, since the module graph
survives the rewrite.

What it cannot tell you is anything the API decides. A plan here will happily
create something that already exists; only an apply knows.
