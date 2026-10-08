# Behaviour tests

Creating a resource is the easy half and CI already covers it. These are the
cases where the plan and the outcome can disagree, or where the verb does not
mean what it says. Each one is a tfvars edit, a plan to read, and in two cases
an apply, because a plan cannot prove them.

Run them while the groups and folders are still throwaway. Every one of these
destroys something real if it behaves the way we do not expect.

## 1. Renaming a group renames it in place

Edit `infra/access.auto.tfvars`:

    "analysts" = { display_name = "Data Analysts", ... }

Expect `~ update in-place` on `module.access.omni_user_group.this["analysts"]`,
with only `display_name` changing.

A `-/+ replace` instead means Omni cannot rename a group, and every label edit
silently costs that group its membership and its grants. The whole key-is-
identity convention in this repo rests on this being an update.

## 2. Re-keying a group replaces it, deliberately

Rename the key from `analysts` to `data-analysts`, leaving everything else.

Expect a destroy and a create, and a destroy of the `omni_user_group_model_role`
that names it.

This one is meant to be destructive. Confirming it is the point: it proves the
key is the identity and that `display_name` is the only safe way to relabel.
Revert afterwards.

## 3. Destroying a role assignment downgrades rather than removes it

Remove `model` and `connection` from a group so its
`omni_user_group_model_role` is no longer in the configuration.

Expect a destroy in the plan. Then check the group in the UI: the role is
`NO_ACCESS`, not absent.

There is no endpoint for removing a role assignment, so the provider writes
`NO_ACCESS` instead. Terraform reports a destroy and the grant is still there in
a different form. It is safe today only because the connection sets
`base_role = "NO_ACCESS"`; with a permissive base role the downgrade leaves
access intact, and the plan still says destroyed.

This is the one behaviour in the provider where the verb lies. Worth seeing once
on a group that does not matter.

## 4. An unmanaged group keeps its members through an unrelated change

Needs an apply, and a group with at least one member.

Add a group with `manage_members = false`, apply, add a member to it in the UI,
then change only its `display_name` and apply again.

Expect the member to still be there.

Before the membership fix this emptied the group: a null `member_ids` became
`"members": []` on what is a SCIM replace. On an embed group that is every user
its sessions had created. CI cannot prove this - it needs a real member and two
applies.

## 5. Changing a role updates rather than churns

Change a group's `model_role` from `QUERIER` to `VIEWER`.

Expect `~ update in-place`, not a replacement. A replacement would mean a window
with no grant at all, which on a live tenant is an outage.

## What these cannot cover

Membership authority on a managed group - add a user to a Terraform-managed
group in the UI, and the next apply removes them. Worth demonstrating once to
whoever will own this, because it surprises people, but it needs a spare seat.

Anything behind `git_credentials` or `tenant_routing`. Both are unset, so
`omni_model_git` and `omni_connection_environment` have never run. Model version
control needs a real repository and a credential; per-tenant routing needs a
second connection, since routing a tenant to the only connection proves nothing.
