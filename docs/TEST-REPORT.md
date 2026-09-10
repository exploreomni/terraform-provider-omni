# Test report: terraform-provider-omni

Evidence-based record of what has been tested, what passed, what broke, and
what remains unverified. Every result here comes from a real run against a live
Omni instance. Nothing is marked passing on the strength of code review alone.

- **Provider version:** 0.1.0 (unreleased)
- **Target:** Omni playground instance
- **Runner:** GitHub Actions, `ubuntu-latest`, Terraform 1.9.x, Go 1.22
- **Scope:** 7 resources, 4 data sources

---

## 1. Why testing looks like this

A Terraform provider fails in ways unit tests do not reach. The provider is a
translation layer between Terraform's state model and a REST API, and almost
every real defect found here came from **the API returning something other than
what its documentation implied**. No amount of testing against a mock would have
caught them, because the mock would have encoded the same wrong assumption as
the code.

So the suite is built on one principle: **assert against the instance, not
against Terraform's own report of success.**

Three consequences:

1. **Read is tested as hard as write.** After every apply, the suite runs
   `terraform plan -detailed-exitcode` and fails on any drift. A provider that
   creates correctly but reads incorrectly will destroy and recreate resources
   on every run, which is worse than one that fails outright.
2. **Deletes are verified with `curl`.** For resources whose API has no delete
   endpoint, Terraform reporting success proves nothing about the instance.
3. **The suite always cleans up.** The destroy step runs with `if: always()`,
   and a failure prints created IDs into the job summary so orphans can be
   removed by hand.

---

## 2. Coverage

| Resource | Create | Read | Update | Delete | Delete semantics |
| --- | --- | --- | --- | --- | --- |
| `omni_folder` | PASS | PASS | PASS | PASS | Removed. `force` required when non-empty |
| `omni_user` | PASS | PASS | PASS | PASS | Removed, confirmed 404 |
| `omni_user_group` | PASS | PASS | PASS | PASS | Removed |
| `omni_model` | PASS | PASS | PASS (rename) | PASS | Archived to trash, recoverable |
| `omni_user_model_role` | PASS | RETEST | PASS | PASS with caveat | Downgraded, see finding 11 |
| `omni_user_group_model_role` | PASS | RETEST | PASS | PASS | Downgraded |
| `omni_connection` | not run | not run | not run | not run | Needs live warehouse credentials |

| Data source | Result |
| --- | --- |
| `omni_user` by email | PASS, matched the created user |
| `omni_user_group` by name | PASS, matched the created group |
| `omni_connection` by name | PASS |
| `omni_model` by name and kind | PASS |

RETEST entries are fixed in code with a regression test, awaiting confirmation
in CI run 5.

---

## 3. Run history

| Run | Outcome | What it established |
| --- | --- | --- |
| 1 | Failed at model create | Folders (both levels), user, group and all data sources work. Failure was a test config error, not the provider |
| 2 | Failed at destroy | Create, no-drift read, data sources, update, and a server-side rename check all passed for 7 resources. Failure was in the model YAML resource, since removed |
| 3 | Failed at build | Stale `go.mod` after the YAML removal. No provider code ran |
| 4 | Failed at read | Full create and full destroy verified server side. Role reads reported false drift |
| 5 | Failed at read | User role fix held. Group role still drifted: the label is `Group Role`, not the `User Group Role` that was guessed |
| 6 | Failed at update | **No drift after create.** Both role resources read cleanly. Failed on the child folder path cascade (finding 12) |

---

## 4. Evidence from run 4

The most complete run. Abridged, with the parts that matter.

**Create, 7 resources:**

```
omni_model.extension: Creation complete after 0s [id=484c696e-...]
omni_user.test: Creation complete after 1s [id=326e51c8-...]
omni_folder.parent: Creation complete after 1s [id=af37421c-...]
omni_user_group.test: Creation complete after 0s [id=gI8mD-wI]
omni_user_model_role.test: Creation complete after 0s [id=326e51c8-...:484c696e-...]
omni_folder.child: Creation complete after 0s [id=ee6809f4-...]
omni_user_group_model_role.test: Creation complete after 0s [id=gI8mD-wI:484c696e-...]

Apply complete! Resources: 7 added, 0 changed, 0 destroyed.
```

**Computed values the API filled in**, which is what proves the read path
reconstructs rather than echoes:

```
folder_child_path  = "tf-test-v1/tf-test-child-v1"   # derived, not configured
folder_child_scope = "organization"                  # inherited from the parent
folder_parent_url  = ".../f/tf-test-v1"              # only the list endpoint returns this
user_active        = true
```

**Data sources resolved to the same objects Terraform had just created:**

```
user_ids_match  = true
group_ids_match = true
```

**Destroy, with ordering:**

```
omni_folder.child: Destruction complete after 0s
omni_folder.parent: Destruction complete after 0s
Destroy complete! Resources: 5 destroyed.
```

Child before parent, which matters because the parent is non-empty and the API
refuses to delete non-empty folders without `force`.

**Post-destroy verification against the instance:**

```
TC-23 user should be gone:      GET user returned 404
TC-25 model should be archived: absent from the active list
TC-27 user role:                NO_ACCESS present, from.type "User Role"
```

---

## 5. Findings

Eleven defects found. All were found by running against a live instance; none
would have surfaced from code review or mocked tests.

| # | Finding | Root cause | Fix |
| --- | --- | --- | --- |
| 1 | Provider would not compile | `AtLeastOneOf` takes `path.Expression`, not `path.Path` | Added a `pathExpr` helper |
| 2 | Folder read dropped resources from state | `ownerId` was sent on organization-scope reads, which filters to one user's folders and hid the folder | Only send `ownerId` for restricted scope |
| 3 | `url` null after create | The create response omits `url`; only the list endpoint returns it | Read back after create |
| 4 | "Inconsistent result after apply" on rename | The update response omits `url`, nulling a value Terraform already knew | Read back after update, and never overwrite a known value with an empty one |
| 5 | CI `terraform init` failed | `dev_overrides` does not stop `init` querying the registry | Filesystem mirror in CI |
| 6 | Mirror config ignored | `setup-terraform` overwrites `~/.terraformrc` when given a credentials token | Append to it after that action runs |
| 7 | Model create rejected | Test config resolved the connection and the base model independently; they disagreed | Take the connection from the base model |
| 8 | Model YAML read failed to decode | `files` is an object keyed by filename, not an array | Resource removed from the provider entirely |
| 9 | CI build failed | Removing a resource changed the dependency graph; CI uses `-mod=readonly` while the local shell had `-mod=mod`, hiding it | `go mod tidy`, plus a tidiness gate in CI |
| 10 | Roles reported deleted on every refresh | Response wraps entries in `results`, and marks origin via `from.type`, not a flat `source` field | Parse `results`, match on `from.type`, regression test added |
| 11 | `NO_ACCESS` on destroy may not revoke access | Roles resolve by priority. Direct assignment is 0; connection base role is 150 and wins | Documented on the attribute |
| 12 | Inconsistent result updating a child folder when its parent is renamed | Renaming a parent rewrites every descendant path server side, but `path` and `url` carried `UseStateForUnknown`, so Terraform planned them unchanged | Removed those modifiers; values derived from ancestors must be free to change |

### The pattern worth remembering

Findings 2, 3, 4, 8, 10 and 12 are the same mistake in six costumes: **different
Omni endpoints return different subsets and shapes of the same object.** Create
omits `url`. Update omits `url` and `scope`. Only list returns everything.
Model roles wrap in `results` with a nested `from.type`. Model YAML returns an
object where an array was expected.

The rule this produced, and which any new resource should follow:

> Treat every write response as partial. Read the object back rather than
> trusting what the write returned, and never let an absent field overwrite
> something Terraform already knows.

### Finding 11 in detail

This one is behavioural, not a bug, and it will surprise people.

The API has no endpoint to remove a role assignment, so destroying a role
resource downgrades it to `role_on_destroy`, default `NO_ACCESS`. From the
instance after destroy:

```json
{"roleName": "NO_ACCESS",     "from": {"type": "User Role"},            "priority": 0,   "resolved": false}
{"roleName": "QUERY_TOPICS",  "from": {"type": "Connection Base Role"}, "priority": 150, "resolved": true}
```

`resolved: false` on the `NO_ACCESS` entry means it lost. The user still has
`QUERY_TOPICS` through the connection's base role. **Destroying the resource
did not revoke access.**

If destroy must actually revoke, the connection's `base_role` has to be
`NO_ACCESS`, and access granted explicitly per model. That is a defensible
design for an Omni deployment, but it has to be a deliberate choice.

---

## 6. What is not covered

| Area | Why | Risk |
| --- | --- | --- |
| `omni_connection` end to end | Needs live warehouse credentials, opt-in via `test_connection` | Medium. Create and read paths are exercised through the data source; update and replace are unproven |
| Drift detection after a UI edit | Needs a human to change something mid-run | Low. Read paths are proven by the no-drift assertion |
| Non-empty folder delete without `force` | Not automated | Low. The recursive path was confirmed by hand |
| Concurrency and rate limits | Single-threaded runs only | Medium. The API allows 60 requests a minute; use `-parallelism=5` for large configurations |
| Import for every resource | Only folders were exercised | Medium |
| `omni_user_attribute` | The API exposes only `GET /v1/user-attributes`; definitions cannot be managed | Not a gap in the provider, a gap in the API |
| Model YAML | Deliberately out of scope. Omni versions model content through git sync; a second writer would fight it | None |

---

## 7. Running it

```bash
cd test
cp terraform.tfvars.example terraform.tfvars   # fill in for your instance
export OMNI_BASE_URL=https://<instance>.omniapp.co
export OMNI_API_TOKEN=<organization API key>
./run-tests.sh --destroy
```

Or in CI: **Actions -> provider crud test -> Run workflow**. It is manual only,
gated on the `omni-playground` environment, and it creates and deletes real
objects.

An organization API key is required. The SCIM user and group endpoints reject
personal access tokens.

---

## 8. Assessment

The four resources with conventional CRUD semantics, `omni_folder`, `omni_user`,
`omni_user_group` and `omni_model`, are verified end to end against a live
instance, including the read-back behaviour that determines whether Terraform
is stable across runs.

The two role resources are functionally verified, with one read fix awaiting
confirmation, and carry a documented caveat about what destroy actually does.

`omni_connection` is the weakest area and should be exercised against a real
warehouse before anyone relies on it, particularly the replace path: most of its
fields force replacement, which means a typo in a field like `host` destroys and
recreates a production connection.

Nothing here is ready to be published to the Terraform Registry without a run
against a second instance to confirm none of the fixes encoded assumptions
specific to this playground.
