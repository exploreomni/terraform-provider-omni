# Omni API: coverage and gaps for infrastructure as code

A review of what the Omni REST API supports, what a Terraform provider can build
on it today, and what is missing. Written after building a working provider
against a live instance and finding twenty-two defects in the process.

**Method.** Every statement here was checked against `/openapi.json` on a live
instance, which lists 144 paths. Published documentation was not treated as
authoritative: three times during this work the documented shape and the actual
response disagreed, and each disagreement cost a debug cycle. Where behaviour is
quoted, it came from a real request.

**What was built.** A prototype Terraform provider covering 11 resources and 6
data sources, with 44 automated test cases running against a playground instance
under both Terraform and OpenTofu. It is not published and has not run against a
production instance. It exists to find out what is possible, and this document
is the result.

---

## 1. Summary

The API is in good shape for most infrastructure work. Connections, models,
folders, users, groups and permissions all support full CRUD and are already
managed as code.

The gaps cluster in one place: **embedding**. The unit that owns per-tenant
content, the embed entity, has no API at all. The schema that row-level security
depends on is read-only. Neither can be created from configuration, which caps
how much of an embed deployment can ever be automated, regardless of how much
provider work is done.

A second, quieter class of problem is shape rather than coverage. Write
responses return inconsistent subsets of the object, some documented schemas do
not match reality, and the same entity carries different identifiers on
different endpoints. These do not block anything, but they produced the majority
of the defects found while building.

---

## 2. Coverage by area

Nothing here is a shipped product. "Prototype" means built and tested against a
playground instance in the proof of concept described in section 7.
"Buildable" means the API supports it and no one has written it yet.
"Blocked" means no provider work can close the gap.

| Area | API support | Terraform | Notes |
| --- | --- | --- | --- |
| Users (SCIM) | Full CRUD | Prototype | Organization API key required |
| Groups (SCIM) | Full CRUD | Prototype | `PUT` is rejected, `PATCH` works |
| Model roles | Create, read | Prototype | No delete. See gap 6 |
| Connections | Full CRUD | Prototype | Most fields force replacement |
| Connection environments | Create, update, delete | Prototype | No read. See gap 5 |
| Schema refresh schedules | Full CRUD | Prototype | |
| Models | Full CRUD | Prototype | Delete archives |
| Folders | Full CRUD | Prototype | No get by id |
| Folder permissions | Full CRUD | Prototype | |
| Document permissions | Create, read, update | Prototype | No delete, set `NO_ACCESS` |
| User attributes | Read only | Prototype, read only |  See gap 2 |
| Embed users | List, read, delete | Blocked for create | See gap 3 |
| Embed entities | **None** | Blocked | See gap 1 |
| Embed secret | **None** | Blocked | See gap 4 |
| Labels | Full CRUD | Buildable | Provider work, not an API gap |
| Content schedules | Full CRUD plus actions | Buildable | Provider work |
| dbt configuration | Full CRUD plus key rotation | Buildable | Provider work |
| dbt environments | Full CRUD | Buildable | Provider work |
| Model git configuration | Full CRUD plus key rotation | Buildable | Provider work |
| Color palettes | Full CRUD | Buildable | Provider work. Relevant to embed branding |
| Email-only users | List, create, bulk create | Buildable | Provider work |
| API keys | List, read, update, delete | Blocked for create | No create. See gap 7 |
| Uploads | Full CRUD | Buildable | Provider work |
| AI skills and routines | Full CRUD | Buildable | Provider work |
| AI credit controls | Read, update | Buildable | Provider work. Governance as code |
| Documents and dashboards | Extensive | Out of scope | Deliberately out of scope for authoring |
| Model content: YAML, topics, views | Full CRUD | Out of scope | Deliberately out of scope, owned by git sync |

Roughly two thirds of the manageable surface could be managed as code. About
half of that is prototyped; the rest is buildable and simply has not been
written. The remainder is blocked by the API gaps in section 3, which no amount
of provider work can close.

---

## 3. Gaps that block, in order of coverage unlocked

### Gap 1: embed entities have no API

Searching the full spec for `embed`, `entity`, `sso` and `tenant` returns three
endpoints:

```
GET            /api/scim/v2/embed/Users
DELETE,GET     /api/scim/v2/embed/Users/{id}
POST           /api/v1/embed/sso/generate-session
```

There is no endpoint to create, read, update or delete an embed entity. Entities
create their own folders, which appear in the folder list as
`omni-system-sso-embed-entity-folder-*` owned by "Omni System SSO Embed Admin",
a system principal that an API key cannot act as.

The consequence is structural. An embed tenant *is* an entity: it owns the
content, the folder, and the isolation boundary. Terraform can build the
connection, the model, the groups, the roles and the attribute values around a
tenant, but it cannot create the tenant. Onboarding will always include a manual
step, and the folder that `contentPath` points at cannot be created or addressed
from configuration.

**Needed:** `POST`, `GET`, `PATCH`, `DELETE /api/v1/embed/entities`, plus a way
to address an entity's folder so permissions can be granted on it.

**Unlocks:** per-tenant onboarding as a single `for_each` over a tenant map,
which is the entire value proposition for embed infrastructure as code.

### Gap 2: user attribute definitions are read-only

`GET /api/v1/user-attributes` is the only method. The response is complete and
useful, listing id, name, label, type, default value, whether multiple values
are allowed, and whether the attribute is system defined.

Attribute definitions are what row-level security keys off, and what every
signed embed URL passes values for. Because they can only be made in the UI, the
security-critical half of an embed deployment cannot be versioned, reviewed in a
pull request, or replicated to a new instance. Promoting a configuration between
environments silently produces an instance where scoping does nothing, because
the attribute the config references does not exist.

The provider mitigates this with a data source that fails at plan time when an
expected attribute is absent. That converts a silent failure into a loud one,
which is worth having, but it is a workaround.

**Needed:** `POST`, `PATCH`, `DELETE /api/v1/user-attributes`.

### Gap 3: embed users cannot be created or updated

`GET` on the collection, `GET` and `DELETE` on an individual user. Listing and
cleanup work. Creation and update do not.

This is defensible while every value arrives in the signed URL. It becomes a
limit as soon as you want to seed a tenant's users ahead of first login, correct
an attribute without waiting for the user to return, or reconcile embed users
against an external source of truth.

**Needed:** at minimum `PATCH /api/scim/v2/embed/Users/{id}` for attributes and
group membership.

### Gap 4: the embed secret cannot be managed

Nothing in the spec creates or rotates the embed signing secret. Standing up a
new instance therefore always requires a person in the UI, and rotating a secret
across environments is manual and error prone.

**Needed:** create and rotate endpoints for the embed secret. Rotation matters
more than creation: a rotation that has to be coordinated by hand across
environments tends not to happen.

### Gap 5: connection environments cannot be read

```
POST           /api/v1/connection-environments
DELETE,PUT     /api/v1/connection-environments/{id}
```

No `GET`. Connection environments implement physical tenant isolation, routing a
session to a different connection based on a user attribute value. Terraform can
create, update and delete them but cannot read them back, so it cannot detect
drift or notice one deleted in the UI.

The provider's read is a deliberate no-op and import is refused with an
explanatory error, because there is nothing to import from. For a resource that
implements an isolation boundary, being unable to verify its current state is a
meaningful weakness.

**Needed:** `GET /api/v1/connection-environments`, ideally filterable by base
connection.

### Gap 6: role assignments cannot be deleted

```
GET,POST       /api/v1/users/{id}/model-roles
GET,POST       /api/v1/user-groups/{id}/model-roles
```

No `DELETE`. Removing a role assignment is therefore impossible; the closest
approximation is assigning `NO_ACCESS`.

This has a consequence that surprises people. Omni resolves roles by priority. A
direct assignment sits at priority 0, and the connection base role sits at 150.
From a live instance after downgrading:

```json
{"roleName": "NO_ACCESS",    "from": {"type": "Connection Base Role"}, "priority": 150, "resolved": true}
{"roleName": "QUERY_TOPICS", "from": {"type": "User Role"},            "priority": 0,   "resolved": false}
```

The `NO_ACCESS` entry lost. The user retained `QUERY_TOPICS` through the base
role. In other words, **destroying a role assignment does not reliably revoke
access**. It only works if the connection's base role is already `NO_ACCESS`.

**Needed:** `DELETE` on both model-roles endpoints.

### Gap 7: API keys cannot be created

```
GET            /api/v1/api-keys
DELETE,GET,PUT /api/v1/api-keys/{id}
```

Read, update and delete, but no create. A new instance cannot mint the
credential that all other automation depends on, so bootstrapping always starts
with a manual step.

**Needed:** `POST /api/v1/api-keys`.

---

## 4. Problems of shape rather than coverage

None of these block a resource from being built. Collectively they produced
most of the defects found while building the provider, and most are cheap to
fix.

### 4.1 Documented schemas do not match responses

The folder permissions response is documented as an array of objects whose only
required field is `role`. The actual response:

```json
{"permits": [
  {"direct": {"role": "VIEWER", "accessBoost": false, "isOwner": false},
   "id": "0aea1b4c-ebfd-4f6a-bdf6-d7edfd798b34",
   "name": "tf-edge-perm-group",
   "type": "userGroup"}
]}
```

The role is nested under `direct`. The subject is a flat `id` with a `type`
discriminator. Neither appears in the schema. This cost two full debug cycles,
and was only resolved by printing the raw payload in CI.

Similarly, `GET /api/v1/models/{id}/yaml` returns `files` as an object keyed by
filename where the schema documents an array.

**This is the cheapest fix on the list.** It is a documentation correction, not
an API change, and it would have prevented several defects outright.

### 4.2 Write responses return inconsistent subsets

Creating a folder omits `url`. Updating one omits `url` and `scope`. Renaming a
model returns an object that is sometimes empty. Only the list endpoint returns
a complete folder.

Terraform requires that state after an apply matches what was planned, so a
partial response either forces an extra read per write, against a rate limit
that is already tight, or requires the provider to encode which endpoint omits
which field. The provider does the latter, and that knowledge exists only in
code comments.

Partial responses are a reasonable design. The expensive part is that the subset
differs between create and update on the same resource, so no general rule can
be written.

**Needed, in order of value:** consistency between create and update on a given
resource, then completeness.

### 4.3 The same entity has different identifiers

A user group created through SCIM returns `uoFHWGHz`. The same group in a
permissions permit is `0aea1b4c-ebfd-4f6a-bdf6-d7edfd798b34`. There is no field
correlating the two, so the provider resolves group permissions by display name,
which costs an extra API call per group on every read.

**Needed:** either consistent identifiers, or a `miniUuid` field on the permit.

### 4.4 No rate limit headers, and the documented limit does not match behaviour

No `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` or
`Retry-After` on any response, so every client guesses its backoff.

The documented limit is 60 requests a minute. The test suite received a 429 at a
request rate well below that, which suggests the real limit is lower, applied
per endpoint, or counted differently. Without headers there is no way for a
client to know.

This matters at scale. A multi-tenant embed deployment is roughly five resources
per tenant, several of which read back after writing. At a hundred tenants that
is a slow apply at best.

**Needed:** standard rate limit headers. Accurate documentation of the limit.

### 4.5 Sub-resources are not immediately addressable

Creating a connection and then immediately creating a schedule on it returns:

```
POST /v1/connections/{id}/schedules -> 404: Connection with id {id} does not exist
```

while `GET /v1/connections/{id}` serves the same connection. Anything scripting
against the API back to back will hit this. The provider retries the 404, but
the error message is actively misleading: the connection plainly exists.

**Needed:** either consistency, or an error that distinguishes "not ready" from
"not found".

### 4.6 No get by id for folders or models

Both are read by paging a list endpoint and matching on id. Acceptable at eight
folders, poor at eight thousand, and it was the direct cause of a defect where a
query parameter silently filtered a folder out of the list and Terraform
concluded it had been deleted.

**Needed:** `GET /api/v1/folders/{id}` and `GET /api/v1/models/{id}`.

---

## 5. Not API gaps: provider work still to do

For completeness, these are supported by the API and simply not built yet.
Listing them separately keeps the gap list honest.

Labels, content schedules with their pause, resume and recipient actions, dbt
configuration and environments, model git configuration with key and webhook
secret rotation, color palettes, email-only users, uploads, AI skills and
routines, and AI credit controls.

Color palettes are worth noting for embed specifically: white labelling an
embedded experience per tenant is fully supported by the API today.

---

## 6. Recommendations

**If one thing changes:** `POST /api/v1/embed/entities` and the ability to
address an entity's folder. It is the only gap that caps total coverage rather
than reducing it.

**If two:** add `POST /api/v1/user-attributes`. Together these make a complete
embed deployment expressible as code.

**Cheapest meaningful improvement:** correct the documented response schemas to
match reality, starting with folder and document permissions. No API change, and
it removes an entire class of client defect.

**Worth doing regardless:** rate limit headers, and `DELETE` on model roles so
that revoking access actually revokes it.

---

## 7. Status of the work this came from

The provider is a proof of concept. 11 resources and 6 data sources, 44 test
cases, verified against a playground instance under both Terraform and OpenTofu,
with plan on pull request and apply on merge behind an approval gate.

It has not been run against a production instance, and `omni_connection` has
only been exercised against an unreachable placeholder host, so its lifecycle is
proven while its usefulness is not.

Twenty-two defects were found and fixed during the work. Seven trace to the
response shape issues in section 4. That ratio is the clearest argument for
fixing them.
