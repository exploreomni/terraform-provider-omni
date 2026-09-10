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
| TC-00 | `terraform validate` | Config is valid | | |
| TC-00b | Provider configures from env vars | No auth errors | | |

### Create

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-01 | `omni_folder` top level with explicit `path` | Created, `path` matches config, `url` populated | Created in 0s, id 9d16b92d | PASS |
| TC-02 | `omni_folder` nested via `parent_folder_id` | Created, `scope` inherited, `path` derived | Created in 1s, id 9b9f5ed2 | PASS |
| TC-03 | `omni_user` with display name | Created, `active = true` | Created in 0s, id e29daa7f | PASS |
| TC-03b | `omni_user` with `attributes` | Declared keys round-trip | | |
| TC-04 | `omni_user_group` with one member | Created, membership applied | Created in 0s, id ybJ3LeHP | PASS |
| TC-05 | `omni_model` SHARED_EXTENSION on a base model | Created, `model_kind` echoed back | Run 1: 400 base model does not belong to the specified connection. Test config bug, see finding 7 | RETEST |
| TC-07 | `omni_user_model_role` QUERIER on the model | Assigned, composite ID `<user>:<model>` | | |
| TC-08 | `omni_user_group_model_role` QUERIER | Assigned | | |
| TC-08b | `omni_connection` (opt in) | Created, ID returned | | |

### Read

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-09 | Plan immediately after create | No changes, no external-drift notice | | |
| TC-09b | Plan after an out-of-band UI edit | Drift detected and reported | | |
| TC-10 | `data.omni_user` by email | Resolves to the created user | Resolved to e29daa7f, matches | PASS |
| TC-10b | `data.omni_user_group` by name | Resolves to the created group | Resolved to ybJ3LeHP, matches | PASS |
| TC-10c | `data.omni_connection` by name | Resolves to an existing connection | Resolved to 26a20237 in 1s | PASS |
| TC-10d | `data.omni_model` by name and kind | Resolves to the base model | Resolved to c41f3e56 in 1s | PASS |

### Update

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-11 | Rename folder, name and path together | In place, ID unchanged, `url` follows the new path | | |
| TC-12 | Rename nested folder | In place, parent unchanged | | |
| TC-13 | Change user display name | In place via SCIM PUT | | |
| TC-13b | Change a user attribute value | In place, undeclared attributes untouched | | |
| TC-14 | Rename group | In place, membership preserved | | |
| TC-14b | Remove the only group member | Membership emptied, group survives | | |
| TC-15 | Rename model | In place via PATCH, ID unchanged | | |
| TC-17 | Change role QUERIER to VIEWER | In place | | |
| TC-18 | Change group role QUERIER to VIEWER | In place | | |
| TC-18b | Change `omni_connection.base_role` | In place, no replacement | | |
| TC-18c | Change `omni_connection.host` | Forces replacement (expected) | | |
| TC-19 | Plan after update | No changes | | |

### Import

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-20 | Import a folder by ID | State populated, following plan clean | | |
| TC-20b | Import a role by `<subject>:<model>` | State populated | | |

### Delete

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-21 | Destroy nested folder before parent | Both removed, correct ordering | | |
| TC-22 | Destroy non-empty folder without `delete_recursively` | Fails with a clear API error | | |
| TC-23 | Destroy user | Removed, gone from `GET /scim/v2/users` | | |
| TC-24 | Destroy group | Removed | | |
| TC-25 | Destroy model | Archived, `deletedAt` set, absent from active list | | |
| TC-27 | Destroy user role | Downgraded to `NO_ACCESS`, verified via the API | | |
| TC-28 | Destroy group role | Downgraded to `NO_ACCESS` | | |
| TC-28b | Destroy connection | Removed | | |

### Error handling

| ID | Case | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| TC-29 | Invalid API token | Clear auth error, no partial state | | |
| TC-30 | Role with neither `model_id` nor `connection_id` | Config validation error before any call | | |
| TC-31 | Invalid `dialect` | Validation error listing allowed values | | |
| TC-32 | API errors surfaced verbatim | API 400 shown with endpoint and message | Model 400 shown as `POST /v1/models returned 400: ...` | PASS |
| TC-33 | Delete an object outside Terraform, then plan | Removed from state, recreate proposed | | |

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

The shape of findings 3 and 4 is worth remembering: **different Omni endpoints
return different subsets of the same object.** Any new resource should assume a
write response is partial and read back rather than trusting it.

## Run history

| Run | Date | Result |
| --- | --- | --- |
| CI #1 | 2026-09-09 | 5 resources created, 4 data sources resolved. Stopped at TC-05 on a test config error (finding 7). No provider defect. |

## Not covered

- `omni_user_attribute` - the API exposes only `GET /v1/user-attributes`, so
  attribute definitions cannot be managed. Values on a user are covered by
  TC-03b.
- Model YAML. Out of scope for this provider: Omni versions model content
  through git sync, and a second writer would fight it.
- Concurrent applies against one instance. The API rate limit is 60 requests a
  minute; use `-parallelism=5` for large configurations.
- Embed users, schedules, dashboards and documents. Not implemented.
