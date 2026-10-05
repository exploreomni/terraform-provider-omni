# infra

The configuration applied to one instance. One workspace, one instance.

```
infra/
  main.tf                   provider, backend, module calls
  variables.tf              every input, all defaulting to empty
  outputs.tf

  warehouse.auto.tfvars     connections
  models.auto.tfvars        models and their git connections
  tenants.auto.tfvars       embed tenants
  branding.auto.tfvars      palettes and labels
  access.auto.tfvars        internal users, groups, folders

  examples/
    internal.tfvars         an internal deployment
    embed.tfvars            an embed deployment
    both.tfvars             internal team building content tenants consume
    auth-methods.tfvars     one connection per authentication method
```

## Why one file per concern

Terraform loads every `*.auto.tfvars` automatically and merges them, so this is
organisational rather than functional. It earns its place at review time: a
pull request touching `tenants.auto.tfvars` is an onboarding, one touching
`warehouse.auto.tfvars` is a credential or warehouse change, and those often
want different reviewers.

Two things to know. A variable set in two files silently takes the value from
whichever loads later alphabetically, so keep each variable in one file. And
only the `.auto.tfvars` suffix auto-loads: a file named `models.tfvars` would
need `-var-file` on every command, including in CI.

## Getting started

The files as committed describe an embed deployment. For an internal one, move
the contents of `examples/internal.tfvars` into the matching files and empty
`tenants.auto.tfvars`.

Then:

```bash
export OMNI_BASE_URL=https://yourinstance.omniapp.co
export OMNI_API_TOKEN=...
terraform plan -var 'connection_passwords={"warehouse":"..."}'
```

Unused components create nothing, so starting from `internal.tfvars` and adding
tenants later is a supported path, not a rewrite.

## Adding a tenant

Two entries. A connection for the isolation, and the tenant itself:

```hcl
connections = {
  "embed-initech" = {
    dialect        = "snowflake"
    host           = "your-account"
    database       = "ANALYTICS"
    default_schema = "INITECH"
    username       = "OMNI_EMBED"
  }
}

tenants = {
  initech = {
    base_connection = "embed-base"
    connection      = "embed-initech"
  }
}
```

Commit, open a pull request, read the plan, merge, approve.

The embed entity still has to be created in the UI.

## Secrets

Never commit a password. Locally:

```bash
terraform apply -var 'connection_passwords={"warehouse":"..."}'
```

In CI, set `TF_VAR_connection_passwords` from a secret.

## Targeting an instance

`OMNI_BASE_URL` and `OMNI_API_TOKEN` decide which instance this applies to. The
`cloud` workspace decides where its state lives. Changing one without the other
applies one instance's configuration using another instance's state.
