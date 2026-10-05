# Try it: exercises for the pull request flow

Seven edits you can make in the GitHub web editor to see how the pipeline
behaves. Each one produces a different, recognisable plan.

Everything here changes `infra/main.tf`, which is applied to the playground
instance on merge. Do them one pull request at a time so each plan is readable.

**How to make an edit:** open `infra/main.tf` on GitHub, press `.` or click the
pencil, make the change, then **Commit changes** and choose *Create a new
branch and start a pull request*.

**What to expect:** the plan workflow comments the plan on the pull request
within about a minute. Merging queues the apply, which waits for your approval
in the Actions tab.

---

## 1. Add a resource

The simplest case. Append to `infra/main.tf`:

```hcl
resource "omni_folder" "marketing" {
  name = "Marketing"
  path = "marketing"
}
```

**Expect:** `Plan: 1 to add, 0 to change, 0 to destroy`, with `id`, `url`,
`scope` and `owner_id` as "known after apply".

---

## 2. Rename it

Change `name = "Marketing"` to `name = "Marketing and Growth"`.

**Expect:** `~ update in-place`, `id` unchanged, and `url` becoming "known
after apply" because it follows the path.

This is the case that broke three times during testing. If `id` appears under a
`-/+` replacement rather than `~`, something has regressed.

---

## 3. Watch a change cascade to children

Add a child, merge it, then rename the parent in a second pull request:

```hcl
resource "omni_folder" "campaigns" {
  name             = "Campaigns"
  parent_folder_id = omni_folder.marketing.id
}
```

**Expect on the parent rename:** both folders update in place, and the child's
`path` and `url` go to "known after apply" even though nothing about the child
changed. Renaming a parent rewrites every descendant path server side, so the
child's values genuinely cannot be predicted at plan time.

---

## 4. Force a replacement

Change the child's parent:

```hcl
  parent_folder_id = omni_folder.finance.id
```

**Expect:** `-/+ destroy and then create replacement`, with
`# forces replacement` next to `parent_folder_id`. The API has no way to move a
folder, so the provider replaces it.

This is the shape to watch for on `omni_connection`, where almost every field
forces replacement and a replacement takes every model on that connection with
it.

---

## 5. Delete a resource

Remove the `omni_folder.campaigns` block entirely.

**Expect:** `Plan: 0 to add, 0 to change, 1 to destroy`.

If the folder has documents in it by then, the apply fails with
`400: Only empty folders can be deleted`. That is the API protecting you.
Setting `delete_recursively = true` overrides it, which is worth doing
deliberately rather than by habit.

---

## 6. Break it on purpose

Add a role assignment with no target:

```hcl
resource "omni_user_model_role" "broken" {
  user_id   = "00000000-0000-0000-0000-000000000000"
  role_name = "QUERIER"
}
```

**Expect:** the plan job fails, and the pull request comment shows

```
At least one of these attributes must be configured: [model_id,connection_id]
```

No API call is made. This is the point of planning on a pull request: the
mistake is caught in review, not after merge. Close this one without merging.

---

## 7. Change something outside Terraform, then plan

Rename a managed folder in the Omni UI, then open a trivial pull request
against `infra/` (adding a comment line is enough).

**Expect:** an "Objects have changed outside of Terraform" section, followed by
a plan that puts the name back. Terraform treats the configuration as the
source of truth and reverts the UI edit on the next apply.

Worth doing once, because it is the behaviour people are most often surprised
by.

---

## Suggested order

1, 2, 3, 4, 5 in sequence tells a coherent story: create, edit, cascade,
replace, remove. 6 and 7 are standalone and can go any time.

## If something goes wrong

The apply workflow is gated on the `omni-playground` environment, so nothing
reaches the instance without your approval. If an apply half-completes, state
is in HCP Terraform and the next plan reconciles it.

Objects left behind by a failed run can be cleared with:

```bash
cd test && ./cleanup-orphans.sh          # list
./cleanup-orphans.sh --delete            # remove
```

That only matches the test suite's naming prefixes, so it will not touch
anything created by `infra/`.
