# Test plan

End-to-end CRUD coverage for every resource and data source in the provider.

Run against a playground instance. The suite creates and deletes real users,
groups, models and folders.

## Running it

```bash
cd test
cp terraform.tfvars.example terraform.tfvars   # fill in for your instance
export OMNI_BASE_URL=https://<instance>.omniapp.co
export OMNI_API_TOKEN=<organization API key>
./run-tests.sh              # stops before destroy
./run-tests.sh --destroy    # full cycle including delete
```

An organization API key is required: the SCIM user and group endpoints reject
personal access tokens.

Results land in `results-<timestamp>.log`.

## What "pass" means

A resource passes CRUD when all four hold:

1. **Create** - apply succeeds and the ID is returned.
2. **Read** - an immediate `terraform plan` reports no changes. This is the one
   that catches most bugs: it proves the read path reconstructs exactly what
   create wrote.
3. **Update** - changing a mutable field produces `~ update in-place` with the
   ID unchanged, and a following plan is clean.
4. **Delete** - destroy succeeds and the object is gone from the instance, or
   reaches the documented end state where no delete endpoint exists.

## Coverage matrix

| Resource | C | R | U | D | Delete semantics |
| --- | --- | --- | --- | --- | --- |
| `omni_folder` | yes | yes | yes | yes | removed, `force` when non-empty |
| `omni_user` | yes | yes | yes | yes | removed |
| `omni_user_group` | yes | yes | yes | yes | removed |
| `omni_model` | yes | yes | rename only | yes | archived to trash, recoverable |
| `omni_user_model_role` | yes | yes | yes | partial | downgraded to `NO_ACCESS` |
| `omni_user_group_model_role` | yes | yes | yes | partial | downgraded to `NO_ACCESS` |
| `omni_connection` | yes | yes | credentials and `base_role` | yes | removed |

## Test cases

Fill in Actual and Status as you run. Leave failures in place: a documented
failure is worth more than a deleted row.

### Setup

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-00 | `terraform validate` | Config is valid | Init and validate passed in CI run 9 | PASS |
| TC-00b | Provider configures from env vars | No auth errors | Provider configured from OMNI_BASE_URL / OMNI_API_TOKEN, no auth errors | PASS |

### Create

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-01 | `omni_folder` top level with explicit `path` | Created, `path` matches config, `url` populated | Created, path matched config, url populated | PASS |
| TC-02 | `omni_folder` nested via `parent_folder_id` | Created, `scope` inherited, `path` derived | Path derived, scope inherited from parent | PASS |
| TC-03 | `omni_user` with display name | Created, `active = true` | Created, active = true | PASS |
| TC-03b | `omni_user` with `attributes` | Declared keys round-trip | Set via time_off_visibility, round-trips with no drift | PASS |
| TC-04 | `omni_user_group` with one member | Created, membership applied | Created with one member | PASS |
| TC-05 | `omni_model` SHARED_EXTENSION on a base model | Created, `model_kind` echoed back | Created, model_kind echoed back as SHARED_EXTENSION | PASS |
| TC-07 | `omni_user_model_role` QUERIER on the model | Assigned, composite ID `<user>:<model>` | Assigned, composite ID <user>:<model> | PASS |
| TC-08 | `omni_user_group_model_role` QUERIER | Assigned | Assigned, composite ID <group>:<model> | PASS |
| TC-08b | `omni_connection` (opt in) | Created, ID returned | Created against a placeholder host: the API does not validate connectivity on create | PASS |

### Read

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-09 | Plan immediately after create | No changes, no external-drift notice | No changes, no external-drift notice | PASS |
| TC-09b | Plan after an out-of-band UI edit | Drift detected and reported | Renamed via the API, drift detected and a correction proposed | PASS |
| TC-10 | `data.omni_user` by email | Resolves to the created user | Resolved to the created user, ids matched | PASS |
| TC-10b | `data.omni_user_group` by name | Resolves to the created group | Resolved to the created group, ids matched | PASS |
| TC-10c | `data.omni_connection` by name | Resolves to an existing connection | Resolved an existing connection by name | PASS |
| TC-10d | `data.omni_model` by name and kind | Resolves to the base model | Resolved the base model by name and kind | PASS |

### Update

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-11 | Rename folder, name and path together | In place, ID unchanged, `url` follows the new path | In place, id unchanged, rename confirmed server side | PASS |
| TC-12 | Rename nested folder | In place, parent unchanged | In place. Parent rename cascaded the child path, handled correctly | PASS |
| TC-13 | Change user display name | In place via SCIM PUT | In place via SCIM PUT | PASS |
| TC-13b | Change a user attribute value | In place, undeclared attributes untouched | Attribute value changed in place | PASS |
| TC-14 | Rename group | In place, membership preserved | In place via SCIM PATCH. PUT is rejected by the API, see finding 13 | PASS |
| TC-14b | Remove the only group member | Membership emptied, group survives | Membership emptied, group survives with zero members | PASS |
| TC-15 | Rename model | In place via PATCH, ID unchanged | In place, id unchanged | PASS |
| TC-17 | Change role QUERIER to VIEWER | In place | QUERIER to VIEWER in place | PASS |
| TC-18 | Change group role QUERIER to VIEWER | In place | QUERIER to VIEWER in place | PASS |
| TC-18b | Change `omni_connection.base_role` | In place, no replacement | base_role changed in place, no replacement | PASS |
| TC-18c | Change `omni_connection.host` | Forces replacement (expected) | Changing host forces replacement, as designed | PASS |
| TC-19 | Plan after update | No changes | No changes after update | PASS |

### Import

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-20 | Import a folder by ID | State populated, following plan clean | Imported by ID, following plan clean | PASS |
| TC-20b | Import a role by `<subject>:<model>` | State populated | Not exercised in CI | NOT RUN |

### Delete

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-21 | Destroy nested folder before parent | Both removed, correct ordering | Child destroyed before parent, both clean | PASS |
| TC-22 | Destroy non-empty folder without `delete_recursively` | Fails with a clear API error | 400: Only empty folders can be deleted | PASS |
| TC-23 | Destroy user | Removed, gone from `GET /scim/v2/users` | GET user returned 404 | PASS |
| TC-24 | Destroy group | Removed | Destroyed | PASS |
| TC-25 | Destroy model | Archived, `deletedAt` set, absent from active list | Absent from the active model list | PASS |
| TC-27 | Destroy user role | Downgraded to `NO_ACCESS`, verified via the API | NO_ACCESS present with from.type User Role, but resolved false. See finding 11 | PASS with caveat |
| TC-28 | Destroy group role | Downgraded to `NO_ACCESS` | Downgraded to NO_ACCESS | PASS |
| TC-28b | Destroy connection | Removed | Connection destroyed | PASS |

### Error handling

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-29 | Invalid API token | Clear auth error, no partial state | 403 Invalid bearer token, nothing written to state | PASS |
| TC-30 | Role with neither `model_id` nor `connection_id` | Config validation error before any call | Rejected at validate: at least one of [model_id,connection_id] | PASS |
| TC-31 | Invalid `dialect` | Validation error listing allowed values | Rejected at validate, all 17 valid dialects listed | PASS |
| TC-32 | API errors surfaced verbatim | API 400 shown with endpoint and message | API 400s surfaced verbatim with method, path and message throughout testing | PASS |
| TC-33 | Delete an object outside Terraform, then plan | Removed from state, recreate proposed | Dropped from state, recreate proposed | PASS |

## Verifying the partial deletes

Terraform reporting success is not sufficient for the three resources with no
delete endpoint. Check the instance directly.

```bash
# TC-27 / TC-28 - roles should read NO_ACCESS
curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/v1/users/<user id>/model-roles" | python3 -m json.tool

# TC-26 - the YAML file should be empty or absent
curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/v1/models/<model id>/yaml" | python3 -m json.tool

# TC-25 - the model should carry a deletedAt
curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/v1/models?pageSize=100" | python3 -m json.tool
```

## Known findings

Issues found so far, all fixed. Kept as regression cases.

| # | Finding | Cause | Fix | Regression case |
| --- | --- | --- | --- | --- |
| 1 | `AtLeastOneOf` failed to compile | Passed `path.Path` where `path.Expression` was required | Added `pathExpr` | TC-30 |
| 2 | Folder read dropped the resource from state | `ownerId` was sent on organization-scope reads, filtering the folder out | Only send `ownerId` for restricted scope | TC-09 |
| 3 | `url` null after create | Create response omits `url`; only list returns it | Read back after create | TC-01 |
| 4 | Inconsistent result after update | Update response omits `url`, nulling a known value | Read back after update, never overwrite known values with empty ones | TC-11 |
| 5 | CI `terraform init` failed | `dev_overrides` does not stop init querying the registry | Filesystem mirror in CI | - |
| 6 | Mirror config ignored in CI | `setup-terraform` overwrites `~/.terraformrc` | Append after that action runs | - |
| 7 | Model create rejected with "base model does not belong to the specified connection" | Test config looked up the connection and the base model independently, and they disagreed | Take `connection_id` from `data.omni_model.base.connection_id` | TC-05 |
| 8 | Model YAML read failed: cannot unmarshal object into `[]client.YAMLFile` | `GET /v1/models/{id}/yaml` returns `files` as an object keyed by filename, not an array | Resource removed from the provider; model content belongs to git sync | - |
| 10 | Roles reported as deleted on every refresh | List response wraps entries in `results`, not `modelRoles`/`records`, and marks origin via `from.type` rather than a flat `source` | Parse `results`; match only entries whose `from.type` is the expected direct-assignment type | TC-09 |
| 21 | TC-P04 read the permit role from the wrong key | The CI assertion looked for a top-level `role`, the same mistake the provider itself had made. It reported `[None, None]` rather than a missing grant | Read `direct.role`, and also assert the superseded grant is gone | TC-P04 |
| 20 | Access grants were briefly added to `omni_model`, then removed | They are model content, declared in model YAML and versioned through git sync. Managing them from Terraform as well would mean two writers on one source of truth, the same reason `omni_model_yaml_file` was removed | Removed. User attribute data sources, added in the same change, were kept | TC-A01 |
| 19 | Folder permission read reported drift on every plan | Three mistakes at once: the role is nested under `direct`, the subject is a flat `id` with a `type` discriminator, and a group's id in a permit is its full UUID while the SCIM API returns a miniUuid. The documented schema showed only `role` | Decode the real shape; match users by id and groups by display name, resolved through SCIM | TC-P02 |
| 18 | The CRUD suite failed under OpenTofu with "terraform: command not found" | A command at end of line was never converted to the `$TF` variable, because the conversion required a trailing space. It worked under Terraform by coincidence | Convert any line whose first word is `terraform`. Found only because the suite runs under both CLIs | - |
| 17 | A test step crashed on empty JSON when two suites ran at once | Both suites hit the same instance concurrently and tripped the 60 requests a minute limit. A rate-limited `curl -sf` produces empty output, which the parser could not distinguish from a valid empty response | One shared concurrency group across every workflow that touches the instance, and the attribute lookup retries on a non-200 rather than parsing nothing | - |
| 15 | Declaring an Omni built-in user attribute drifts on every plan | The platform populates `omni_*` attributes and does not return them in the user attribute payload, so the read drops the key | Documented on the attribute. The edge suite now skips built-ins when discovering one to test | TC-03b |
| 16 | A failing check in the edge suite did not fail the step | The assertion printed FAIL without exiting non-zero | Audited every assertion in both suites: added the missing exits, `set -o pipefail` where an assertion sits behind a pipe, and tightened two checks that were written to pass either way. `actionlint` now runs in CI | - |
| 13 | Group update rejected with "members: Required" despite a valid body | The SCIM PUT route does not accept a well-formed replace body on this API | Switched group updates to SCIM PATCH with replace operations | TC-14 |
| 14 | Model rename returned an object with empty fields, producing an inconsistent result | The PATCH response shape is not dependable, and there is no get-by-id route for models | Try flat, then wrapped, then fall back to the list endpoint. Never map empty fields into state | TC-15 |
| 12 | Inconsistent result updating a child folder when its parent is renamed | Renaming a parent rewrites every descendant path server side, but `path` and `url` carried `UseStateForUnknown`, so Terraform planned them to stay put | Removed those modifiers: both are derived from ancestors and must be free to change | TC-12 |
| 11 | `NO_ACCESS` on destroy does not necessarily revoke access | Omni resolves roles by priority. A direct assignment is priority 0; the connection base role is 150 and wins, so the destroyed assignment shows `resolved: false` | Documented on `role_on_destroy`. Set the connection `base_role` to `NO_ACCESS` if destroy must revoke | TC-27 |
| 9 | CI build failed with "updates to go.mod needed" | Removing the yaml resource changed the dependency graph. CI builds with `-mod=readonly`; the local environment had `-mod=mod`, which hid it | `go mod tidy`, plus a tidiness gate in the test workflow | - |
| 34 | A renamed user attribute would silently stop matching | The attribute data source resolved definitions by name only, and the SCIM payload that carries values is keyed by name. Renaming a definition in the UI leaves a configuration naming it still valid and no longer matching, which on an access filter means row-level security that quietly stops applying | The data source now takes `id` or `name`, exactly one of, and `modules/user-attributes` references definitions by ID and resolves the name itself. A rename becomes invisible to Terraform; a deletion fails loudly | TC-A02 |
| 33 | A combined apply created the schema model, then failed on the shared model with a bare 404 | A model needs its connection's schema model to have been *refreshed*, not merely to exist, and the refresh is asynchronous. Staged applies had minutes between them and never showed it; one apply created the shared model a fraction of a second later and `POST /v1/models` answered 404 with no detail. The ordering was correct throughout: connection, schema model, shared model, in that order | `CreateModel` retries 404 and 429 against a 90-second budget. A 404 there means "not ready" far more often than "wrong id", and when it is a wrong id the budget expires with a message saying what to check | TC-M09 |
| 32 | A tainted schema model could never be replaced | `DELETE /v1/models/{id}` refuses a schema model: "Cannot delete a model with kind SCHEMA". The destroy half of any replacement failed, so the resource was stranded: it could not be replaced and could not be removed from state | Destroying a `SCHEMA` model now removes it from state with a warning rather than calling the API. It belongs to its connection, shares its ID, and goes when the connection goes | TC-M08 |
| 31 | Creating a schema model failed with "inconsistent result after apply" | Omni names a schema model after its connection, whatever name the create asks for. The provider sent the Terraform map key and got the connection name back, and a provider may not return a value contradicting configuration | `name` is now Optional and Computed, the modeling module sends none for a `SCHEMA` model, and `modelName` is omitted when empty. A schema model keeps its map key as the Terraform-side identity while Omni owns the visible name. A module validation rejects a named schema model with a message rather than letting the apply fail | TC-M07 |
| 30 | The refresh schedule was still created before the schema model | The schedule lived in the warehouse module, and modeling depends on warehouse because a model is built on a connection. So the schedule, which needs a schema model, was created first. Expressing the real order inside warehouse would have required warehouse to depend on modeling, which is a cycle | Moved the schedule into modules/schema-refresh, which runs after modeling. Configuration stays on the connection in tfvars; only the resource moved. Create order verified against Terraform's graph | TC-M06 |
| 29 | A schema model existed but had no tables | Creating a `SCHEMA` model registers it; introspecting the warehouse is a separate `POST /v1/models/{id}/refresh`. The UI's Build Schema button does both, the API create does only the first, so a model created by Terraform looked fine and saw nothing | The provider starts a refresh after creating a `SCHEMA` model, controlled by `refresh_on_create`. A failed refresh is a warning, not an error: the model exists, and failing the apply would taint it | TC-M05 |
| 25 | A connection was created, then everything built on it failed | Omni does not build a connection's schema model on its own. Until it exists, `POST /v1/connections/{id}/schedules` answers "Connection with id ... does not exist" while `GET` and `PATCH` on the same id return 200, and a shared model fails with "Schema model does not exist". Both errors point at the wrong object | Create the schema model first. It has no endpoint of its own: it is `POST /v1/models` with the connection id and `modelKind` `SCHEMA`, the API equivalent of the UI's Build Schema button. The modeling module now creates `SCHEMA` models ahead of every other kind | TC-M03 |
| 26 | `default_schema` silently did nothing on Snowflake | Omni accepts `defaultSchema` on create without error and stores null. The read-back then contradicted the configured value and Terraform refused the apply with "Provider produced inconsistent result after apply" | Two parts. `keepKnown` stops an absent field overwriting a known one, which is right for genuinely partial responses. And `default_schema` was removed from the Snowflake connection, because for a field the API ignores the guard would hide the problem rather than fix it | - |
| 27 | Renaming a model destroyed it | `omni_model.name` carries no `RequiresReplace`, so the API renames in place, but the module derived the name from the map key. Renaming meant re-keying, which moves the resource address, so Terraform destroyed the model and built a new one, losing its content and breaking every group pointing at the old key | `name = coalesce(each.value.name, each.key)`. The key is identity, `name` is the label. Same rule now applies to models, groups and tenant groups | TC-M04 |
| 28 | Dangling references reported as `Invalid index` | Removing a connection, model or group left other maps naming it. Terraform failed on the index lookup with "object with no attributes", which names neither the entry at fault nor what to do | Twelve cross-reference validations on the root variables, each naming the specific entries and the dependency. They run before any provider call and report every problem in one pass | - |

The shape of findings 3 and 4 is worth remembering: **different Omni endpoints
return different subsets of the same object.** Any new resource should assume a
write response is partial and read back rather than trusting it.

## Run history

| Run | Date | Result |
| --- | --- | --- |
| CI #1 | 2026-09-09 | 5 resources created, 4 data sources resolved. Stopped at TC-05 on a test config error (finding 7). No provider defect. |
| CI #2 | 2026-09-09 | Create, no-drift read, data sources, update and server-side rename all passed for 7 resources. Failed at destroy on the model YAML read (finding 8). Objects orphaned and removed by hand. |
| CI #3 | 2026-09-09 | Failed at the build step: stale `go.mod` (finding 9) and leftover `topic_description` vars in the workflow. No provider code exercised. |
| CI #4 | 2026-09-10 | 7 resources created, all 4 data sources matched, destroy removed 5 cleanly with correct ordering, and post-destroy state verified server side. One failure: drift on both role resources (finding 10). |
| CI #5 | 2026-09-10 | User role drift fixed. Group role still drifted: the label is `Group Role`, not `User Group Role`. |
| CI #6 | 2026-09-10 | **TC-09 passed: no drift after create.** Both role resources read cleanly, data sources matched. Failed in the update phase on the child folder path cascade (finding 12). |
| CI #7 | 2026-09-10 | Folder cascade fixed. Group PUT and model rename still failed (findings 13, 14). |
| CI #8 | 2026-09-10 | Diagnostics confirmed the group PUT body was valid but still rejected. Run-scoped names introduced so a failed run cannot block the next. |
| Edge #3 | 2026-09-10 | **Full pass of the edge suite.** All 14 negative, out-of-band, import and connection cases verified, with every assertion able to fail. |
| Edge #1 | 2026-09-10 | 13 cases run. One masked failure: an assertion printed FAIL without exiting (finding 16). |
| CI #9 | 2026-09-10 | **Full pass. 15 of 15 steps green in 51s:** create, no-drift read, data sources, update across all 7 resources, destroy with correct ordering, and post-destroy verification against the API. |

## Not covered

- `omni_user_attribute` - the API exposes only `GET /v1/user-attributes`, so
  attribute definitions cannot be managed. Values on a user are covered by
  TC-03b.
- Model YAML. Out of scope for this provider: Omni versions model content
  through git sync, and a second writer would fight it.
- Concurrent applies against one instance. The API rate limit is 60 requests a
  minute; use `-parallelism=5` for large configurations.
- Embed users, schedules, dashboards and documents. Not implemented.
