# Terraform alongside an identity provider

Most Omni deployments of any size get their users from somewhere else: Okta,
Entra, Google, or another IdP pushing accounts in over SCIM. This is how that
coexists with managing the same instance in Terraform.

The short answer: **the IdP owns identity, Terraform owns authorization.**
Everything below follows from that line.

## Why it matters

`omni_user` and `omni_user_group` are SCIM resources. So is the IdP's
provisioning. Two writers against the same objects do not merge, they take
turns, and the loser's changes disappear on the next sync.

Two specific ways that goes wrong:

- **Creating a user that already exists fails.** `omni_user` calls SCIM
  directly and does not adopt an existing account, so the apply stops with a
  409. Everything downstream of the access layer stops with it, which in the
  reference deployment is every folder and grant.
- **Destroying an `omni_user` deprovisions a person.** Not a Terraform-side
  record of a person. The person, with their content and their session.

Neither is a bug to be fixed. They are what it means for two systems to own the
same object.

## What each side owns

| Owned by the IdP | Owned by Terraform |
| --- | --- |
| Who exists | What exists for them to use |
| Group membership | The groups themselves, and their roles |
| Attribute values per person, through its own attribute mapping | Attribute definitions |
| Deprovisioning | Connections, models, folders, permissions, palettes, labels |

Terraform's half is the half that belongs in review: a role granted, a folder
shared, an access filter's attribute created. None of it is identity.

## Configuring it

### Leave `var.users` empty

An IdP-provisioned account does not go in `var.users`. Nothing in this
configuration needs it there: grants reference groups, and the groups come from
the IdP.

If you need to refer to a specific person, `data "omni_user"` resolves one by
email without owning it.

### Set `manage_members = false` on every IdP-managed group

```hcl
groups = {
  "analysts" = {
    display_name   = "Analysts"
    manage_members = false

    model      = "sales"
    connection = "warehouse"
    model_role = "QUERIER"
  }
}
```

With `manage_members = true`, which is the default, the members list in
configuration is authoritative and the next apply removes everyone the IdP
added. With it false, Terraform owns the group and its role and leaves
membership alone.

The group still has to exist on both sides with the same name. Whether
Terraform creates it and the IdP pushes members into it, or the IdP creates it
and Terraform adopts it with an `import` block, is a local decision. Creating
it in Terraform is simpler and makes the role reviewable in the same change as
the group.

### Attribute values come from the IdP

This is the one that surprises people. An attribute's value for a person is
stored on the user through SCIM, so Terraform can only set it on a user it
manages. The root configuration enforces that: `var.user_attribute_values`
rejects any email that is not in `var.users`, at plan time, rather than failing
the apply.

So for IdP-provisioned users, the value has to come from the IdP's attribute
mapping. In practice that is the right place for it anyway, because the value
is usually a fact about the person that the IdP already holds.

What Terraform still owns is the **definition** - the attribute's name, type and
default. That is the part an access filter keys off, the part that has to exist
before the IdP can map anything into it, and the part you want in version
control.

### Switchable values are the group-level lever

`omni_user_attribute_selectable_values` provisions what a group may switch an
attribute to, and it works on an IdP-managed group:

```hcl
attribute_selectable_values = {
  "analysts-region" = {
    attribute  = "region"
    group_uuid = "5a7b9d67-cd43-4724-a9cd-30bf1bbbd847"
    values     = ["anz", "apac", "emea"]
  }
}
```

Two cautions.

It is not an assignment. It is the set a member may switch *between*, from the
File menu. Their actual value still comes from the IdP mapping or the
definition's default. There is no group-assigned value in Omni.

And `group_uuid` is a literal UUID rather than a reference, because this
endpoint identifies a group by its UUID primary key while every other one -
including SCIM - uses an eight-character id. See finding 43 in
[TEST-CASES.md](./TEST-CASES.md) for how to obtain it and why it cannot be a
reference.

## The hybrid, and why to avoid it

You can import an IdP-provisioned user so Terraform can set attributes on them:

```hcl
import {
  to = module.access.omni_user.this["someone@example.com"]
  id = "<the id data.omni_user reports>"
}
```

It works. It also means a `terraform destroy`, or removing that entry from
configuration, deprovisions a real person, and that the IdP and Terraform are
now both writing their attributes.

Worth it for a handful of service or admin accounts the IdP does not manage.
Not worth it as a way around the attribute-mapping work.

## A note on ids

A person in Omni has two UUIDs, and knowing which is which saves an afternoon:

- The **Membership** id is what SCIM reports, what `omni_user.id` holds, and
  what `selectable-values` wants for a user subject.
- The **User** id is what a colliding create names in its 409 and what a
  folder's `owner_id` holds. The selectable-values endpoint rejects it.

Findings 38 and 39 in [TEST-CASES.md](./TEST-CASES.md) have the evidence.

## Checklist

For an instance whose users come from an IdP:

- [ ] `var.users` is empty, or holds only accounts the IdP does not manage
- [ ] Every group has `manage_members = false`
- [ ] Group names match on both sides
- [ ] `var.user_attribute_values` is empty, and the IdP maps attribute values
- [ ] Attribute definitions are in `var.user_attributes`, in version control
- [ ] The first apply is a plan someone has read, on an instance whose users
      already exist
