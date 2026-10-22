# Contributing

Thanks for your interest in the Omni Terraform provider. This covers how to
build it, how to test a change, and what a reviewable pull request looks like.

## Requirements

- Go 1.25 or later (see `go.mod` for the exact version)
- Terraform 1.9 or later, or OpenTofu
- An Omni instance you are willing to have objects created and deleted in, if
  you want to run acceptance tests

## Building

```bash
make build      # compile
make install    # compile and install into $GOPATH/bin
make fmt        # gofmt and terraform fmt
make test       # unit tests, no network
```

## Running a local build

Point Terraform at your build with a dev override rather than publishing a
version. Create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "exploreomni/omni" = "/Users/you/go/bin"
  }
  direct {}
}
```

With an override in place Terraform skips `terraform init` for this provider
and warns on every command. Delete the block when you are done, or a stale
binary will quietly shadow the released one.

## Acceptance tests

These create, update and destroy real objects. Point them at a throwaway
instance and nothing else.

```bash
export OMNI_BASE_URL=https://<instance>.omniapp.co
export OMNI_API_TOKEN=<organization API key>
make testacc
```

An organization API key is required. Users and groups go through SCIM, and the
SCIM endpoints reject personal access tokens.

Four fixtures unlock the cases that need an object the suite cannot create for
itself. A case skips rather than fails when its fixture is unset:

| Variable | What it is |
| --- | --- |
| `OMNI_TEST_CONNECTION_ID` | A connection to assign connection-scoped roles on |
| `OMNI_TEST_MODEL_ID` | A model to assign model-scoped roles on |
| `OMNI_TEST_MEMBER_USER_ID` | A user's membership id, as SCIM reports it |
| `OMNI_TEST_GROUP_UUID` | A group's UUID primary key, which no endpoint returns |

Everything the suite creates is named `tfacc-<run token>-...`. If a run fails
hard it can leave objects behind, so there is a sweeper:

```bash
make sweep
```

The same suite runs nightly as a GitHub Action against a live instance. A
change that touches a resource should come with a case, or an explanation in
the pull request of why one is not practical.

## Documentation

Registry documentation under `docs/` is generated. Do not edit those files by
hand:

```bash
make docs
```

Attribute descriptions come from the schema in `internal/provider/`, and the
provider overview comes from `templates/index.md.tmpl`. Two things worth
knowing:

- A blank line inside an attribute description escapes its bullet in the
  rendered page unless the continuation is indented by two spaces.
- `make docs` passes `--provider-name omni` explicitly, because `tfplugindocs`
  otherwise infers the provider name from the directory name and fails in any
  checkout not called `terraform-provider-omni`.

Commit the regenerated files with your change.

## Pull requests

- One subject per pull request. A rename, a bug fix and a new resource are
  three pull requests.
- Explain what the API actually does, not just what the code does. Most of the
  difficult behaviour in this provider comes from endpoint quirks, and the next
  person needs to know which is which.
- If you find an API limitation, record it in `docs/TEST-CASES.md` with the
  evidence. That file is the reason the known limitations in `CHANGELOG.md` can
  be stated precisely.
- `make fmt`, `make test` and `make docs` should all be clean before you open
  it.

## Releasing

Releases are cut from a tag. The registry namespace publishes new versions
automatically, so pushing a tag is publishing. There is no separate publish
step, and no way to withdraw a version once it is out.

1. Update `CHANGELOG.md`: move everything under Unreleased into a new version
   heading with today's date.
2. Tag and push:

```bash
git tag -a v0.1.1 -m "v0.1.1"
git push origin v0.1.1
```

3. The release workflow imports the signing key, runs GoReleaser across every
   supported platform, and publishes 16 assets: 13 platform archives,
   `SHA256SUMS`, its detached signature, and the registry manifest.
4. Confirm the manifest is there. Without it the registry records the provider
   as protocol v5 only, and `terraform init` then fails for everyone.

```bash
gh release view v0.1.1 --json assets --jq '.assets[].name'
```

The signing key lives on the `exploreomni` namespace in HCP Terraform, and
`GPG_PRIVATE_KEY` and `PASSPHRASE` are repository secrets. A version number is
the one thing here that cannot be reused, so check the tag points where you
expect before pushing it.

## Reporting a problem

Open an issue using one of the templates. For anything security related, follow
[SECURITY.md](SECURITY.md) instead and do not open a public issue.
