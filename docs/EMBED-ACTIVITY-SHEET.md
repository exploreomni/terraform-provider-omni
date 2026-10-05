# Activity sheet: testing the embed setup through GitHub

Target instance: `https://terraform.playground.exploreomni.dev`

Work through in order. Each activity says where it happens, what to do, and
what proves it worked. Tick the result column as you go.

Allow about ninety minutes. Most of it is waiting for workflow runs.

---

## Part A: prerequisites

Nothing in Part B works without these. Three of the four are API gaps, not
choices: they cannot be automated.

| # | Activity | Where | Done |
| --- | --- | --- | --- |
| A1 | Create an **organization** API key. Settings > API keys. A personal access token will fail on users and groups. | Omni UI | |
| A2 | Create a user attribute. Reference `tenant_id`, type String, multiple values off. | Omni UI | |
| A3 | Create the embed secret. Settings > Embed. Copy it; your app signs URLs with it. | Omni UI | |
| A4 | Decide on warehouse credentials. A real connection is needed for C4 onwards; a placeholder is fine up to C3. | you | |

**Checkpoint A.** Confirm the attribute exists:

```bash
export OMNI_BASE_URL=https://terraform.playground.exploreomni.dev
export OMNI_API_TOKEN=<the key from A1>

curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/v1/user-attributes" \
| python3 -c "
import sys, json
names = [a['name'] for a in json.load(sys.stdin)['records']]
print('attributes:', names)
print('tenant_id present:', 'tenant_id' in names)
"
```

Must print `True`. If not, repeat A2.

---

## Part B: GitHub configuration

| # | Activity | Where | Done |
| --- | --- | --- | --- |
| B1 | Set repository secret `OMNI_BASE_URL` to `https://terraform.playground.exploreomni.dev` | Settings > Secrets and variables > Actions | |
| B2 | Set repository secret `OMNI_API_TOKEN` to the key from A1 | same | |
| B3 | Add repository secret `CONNECTION_PASSWORDS`, a JSON object as one string: `{"embed-base":"...","embed-acme":"...","embed-globex":"..."}` | same | |
| B4 | Confirm `TF_API_TOKEN` is still set. It is already there from the previous setup. | same | |
| B5 | Create environment `omni-terraform`, add yourself as a required reviewer | Settings > Environments | |
| B6 | Confirm the HCP workspace `omni-terraform` exists with Execution Mode **Local** | app.terraform.io | |

**Checkpoint B.** Both secrets and the environment are listed. The apply
workflow targets `omni-terraform`, so a missing environment means the job never
starts.

---

## Part C: the pull request flow

This is the part being tested. Each activity is one pull request.

### C1: a plan on an empty instance

| | |
| --- | --- |
| Where | GitHub web editor |
| Do | Edit `infra/warehouse.auto.tfvars` and change the host, database and username to real values. Check `infra/tenants.auto.tfvars` lists acme and globex. Commit to a new branch and open a pull request. |
| Expect | The `terraform plan` job comments on the pull request within about a minute. It should propose roughly 11 resources: three connections, one model, two connection environments, two groups, three folders, two permissions, one palette. |
| Proves | CI reaches the instance, reads HCP state, and the modules resolve. |

If the plan fails on `data.omni_user_attribute.tenant`, go back to A2. That
failure is the guard working.

### C2: apply it

| | |
| --- | --- |
| Where | GitHub |
| Do | Merge the pull request. Go to Actions, find the waiting `terraform apply` run, click **Review deployments**, approve. |
| Expect | Apply succeeds. The job summary shows a `tenants` output with group, content path, folder id and connection id for each tenant. |
| Proves | The approval gate works and the whole module tree applies. |

**Checkpoint C2.** Verify server side rather than trusting the exit code:

```bash
curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/v1/folders?pageSize=100" \
| python3 -c "
import sys, json
for f in json.load(sys.stdin)['records']:
    if f['path'].startswith('tenants'):
        print(f['path'], '|', f['name'])
"
```

Expect `tenants`, `tenants/acme`, `tenants/globex`.

### C3: add a tenant

| | |
| --- | --- |
| Where | GitHub web editor |
| Do | Add a connection `embed-initech` to `infra/warehouse.auto.tfvars` and a tenant `initech` to `infra/tenants.auto.tfvars` referencing it. Add its password to the `CONNECTION_PASSWORDS` secret. Open a pull request. |
| Expect | Plan shows **5 to add, 0 to change, 0 to destroy**: connection, environment, group, folder, permission. Nothing existing is touched. |
| Proves | Onboarding is additive. This is the whole point of the module structure. |

Merge and approve.

### C4: prove the routing

| | |
| --- | --- |
| Where | Omni UI, then command line |
| Do | Create the embed entity for `acme` in the UI. Build a dashboard on the base connection and move it into `tenants/acme`. |
| Expect | Nothing yet; this is setup for C5. |
| Proves | Nothing. Noted because it cannot be automated: there is no API for embed entities. |

### C5: generate a signed session

```bash
curl -s -X POST -H "Authorization: Bearer $OMNI_API_TOKEN" \
  -H "Content-Type: application/json" \
  "$OMNI_BASE_URL/api/v1/embed/sso/generate-session" \
  -d '{
    "externalId": "test-user-1",
    "name": "Test User",
    "userAttributes": {"tenant_id": "acme"},
    "groups": ["tenant-acme"],
    "contentPath": "/tenants/acme"
  }' | python3 -m json.tool
```

| | |
| --- | --- |
| Expect | A URL. Open it in a private window and the dashboard loads. |
| Proves | Every value in that request resolves to something Terraform created: the attribute from A2, the group from the tenant module, the path from the folder. |

### C6: prove the isolation

| | |
| --- | --- |
| Do | Repeat C5 with `tenant_id` and `groups` changed to `globex`, and `contentPath` set to `/tenants/globex`. |
| Expect | Queries run against `embed-globex`, not `embed-acme`. |
| Proves | The connection environment is routing. If both tenants see the same data, check that the base connection has the environment user attribute name set, which is a UI setting the provider does not yet expose. |

### C7: a change that forces replacement

| | |
| --- | --- |
| Where | GitHub web editor |
| Do | Change the `host` on one tenant connection in `infra/warehouse.auto.tfvars`. Open a pull request. **Read the plan, then close it without merging.** |
| Expect | `-/+ destroy and then create replacement` with `# forces replacement` next to `host`. |
| Proves | The plan shows a destructive change before it happens. On a real connection this would take every model built on it. |

### C8: drift

| | |
| --- | --- |
| Where | Omni UI, then GitHub |
| Do | Rename the `acme` folder in the UI. Open a trivial pull request against `infra/`, a comment line is enough. |
| Expect | The plan shows "Objects have changed outside of Terraform" and proposes putting the name back. |
| Proves | Configuration is the source of truth and the UI edit is reverted on the next apply. |

---

## Part D: what to record

| # | Activity | Result |
| --- | --- | --- |
| C1 | Plan on empty instance | |
| C2 | Apply, resources created | |
| C3 | Add a tenant, 5 to add | |
| C5 | Signed session loads | |
| C6 | Routing isolates tenants | |
| C7 | Replacement shown in plan | |
| C8 | Drift detected | |

Anything that fails: capture the workflow run URL and the error. The provider
has twenty-two recorded findings so far and most surfaced exactly like this.

---

## Part E: teardown

```bash
cd infra
terraform destroy
```

Two things destroy will not do.

**Role assignments** cannot be deleted through the API, so they are downgraded
to `NO_ACCESS` rather than removed. That only revokes access because the
connections use `NO_ACCESS` as their base role.

**Embed users** created by sessions are not managed by Terraform:

```bash
curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
  "$OMNI_BASE_URL/api/scim/v2/embed/Users" | python3 -m json.tool
```

The embed entity has to be removed in the UI.

---

## Known limits of this test

**Placeholder connections do not query.** The API does not validate
connectivity on create, so C1 to C3 pass with a fake host. C5 and C6 need a
real warehouse.

**The embed entity is manual.** C4 exists only to note the gap.

**The environment attribute name is a UI setting.** If routing fails at C6,
this is the first thing to check. The provider does not expose it yet.
