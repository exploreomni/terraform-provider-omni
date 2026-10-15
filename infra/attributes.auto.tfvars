# User attributes: the definitions, the value each person holds, and the values
# a group may switch between.

# Definitions. existing_id references one made elsewhere; without it the
# definition is created here.
#
# Definitions became creatable on 2026-09-21. Everything in this file that
# names an ID predates that and stays as it is: a definition an access filter
# or an embed URL already keys off should not be re-created under Terraform,
# because a destroy takes every user's value with it.
user_attributes = {
  # The attribute embed scoping keys off. Referenced, never created. It can be
  # renamed in the UI; the ID cannot.
  tenant = {
    existing_id = "98f49f77-2eb8-4a5c-8b97-b40690e87812"
  }

  # Created here, and nothing depends on it. Deliberately so: this is what the
  # create, update and delete paths get exercised on, and exercising them on
  # the tenant attribute above would mean practising destroys on the thing
  # row-level security keys off.
  region = {
    label       = "Region"
    description = "Sales region, for the selectable-values test. Nothing routes on this."
    type        = "String"

    # Starts single. Widening to true and then narrowing back is behaviour test
    # 6: the narrowing is a replacement, and a replacement here drops the value
    # every user holds.
    multiple_values = false
    default_value   = "anz"
  }
}

# A value is set on a person or it comes from the definition's default. There is
# no third source, and in particular no group-assigned value.
#
# A value can only be set on a user Terraform manages, which the root validation
# checks before the apply rather than after.
user_attribute_values = {
  "ashima@omni.co" = {
    tenant = "acme"

    # Also proves a created definition can be assigned in the same apply that
    # creates it. The SCIM payload is keyed by attribute name, and the name of
    # something created here is known at plan time, so this should not need two
    # applies. If it does, the module is resolving the name from the API rather
    # than from configuration.
    region = "apac"
  }
}

# Values a group may switch an attribute to.
#
# Not an assignment: this is the set a member can switch between from the File
# menu, and their actual value still comes from the assignment above or the
# definition's default.
#
# On "region" rather than "tenant" on purpose. Provisioning tenant values to a
# group is handing its members other tenants' rows, since the access filters
# route on exactly that attribute. Nothing routes on region, so getting this
# wrong costs nothing.
attribute_selectable_values = {
  "analysts-region" = {
    attribute = "region"
    group     = "analysts"
    values    = ["anz", "apac", "emea"]

    # The narrow option. True would let members switch to any value at all,
    # which on an attribute something routes on is an unbounded grant.
    allow_any_value = false
  }

  # No user entry yet. The endpoint takes the user's *membership* ID, which is
  # not the user ID, and it does not say which one it got: a wrong ID
  # provisions nothing and returns success. Whether omni_user.id is the right
  # handle is untested, so the group path goes first and a user entry waits
  # until there is something to compare against.
}

# The attribute embed scoping keys off, by name. Set tenant_attribute_id to
# reference it by ID instead, which is preferred: a rename in the UI cannot
# then break scoping in silence.
#
# This names the same definition as user_attributes["tenant"] above, by a
# different route. Worth collapsing: the embed existence check could key off
# that entry and this variable could go. Left alone here to keep this change to
# one subject.
tenant_attribute = "tenant_id"

