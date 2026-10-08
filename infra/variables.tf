# --------------------------------------------------------------------------
# Warehouse. Needed by both use cases.
# --------------------------------------------------------------------------

variable "connections" {
  description = "Connections to create, keyed by name. See modules/warehouse for the full shape."
  type        = any
  default     = {}
}

variable "connection_passwords" {
  description = "Password per connection. For BigQuery service accounts this is the full service account JSON."
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "connection_private_keys" {
  description = "RSA private key per connection, for Snowflake key pair authentication."
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "connection_oauth_secrets" {
  description = "OAuth client secret per connection."
  type        = map(string)
  sensitive   = true
  default     = {}
}

# --------------------------------------------------------------------------
# Models. Needed by both use cases.
# --------------------------------------------------------------------------

variable "models" {
  description = <<-DESC
    Models to create, keyed by a stable identifier. connection is a key from
    var.connections.

    The key is the model's identity and is what var.groups and
    var.groups reference. Set name to rename a model without re-keying:
    Omni renames in place, while changing the key destroys the model, discards
    its content and breaks everything pointing at the old key.

    kind, connection and base model all force replacement.
  DESC

  type    = any
  default = {}

  # Without this, removing a connection while a model still names it fails with
  # "Invalid index ... object with no attributes", which says nothing about
  # which model or which key. Destroying a connection means destroying what is
  # built on it, and this says so.
  validation {
    condition = alltrue([
      for k, v in var.models :
      try(v.connection, null) == null || contains(keys(var.connections), try(v.connection, "__unset__"))
    ])
    error_message = format(
      "These models are built on a connection that is not in var.connections: %s. A connection cannot be removed while models sit on it, in Terraform or in Omni. Remove the models in the same change, or restore the connection.",
      join("; ", [
        for k, v in var.models : "model \"${k}\" needs connection \"${try(v.connection, "")}\""
        if try(v.connection, null) != null && !contains(keys(var.connections), try(v.connection, "__unset__"))
      ])
    )
  }
}

variable "git_credentials" {
  description = "Git credentials per model, keyed the same as models."
  type        = any
  sensitive   = true
  default     = {}
}

# --------------------------------------------------------------------------
# Branding. Optional for both.
# --------------------------------------------------------------------------

variable "palettes" {
  type    = map(object({ type = string, colors = list(string) }))
  default = {}
}

variable "labels" {
  type = map(object({
    color       = optional(string)
    description = optional(string)
  }))
  default = {}
}

# --------------------------------------------------------------------------
# Internal access. Leave empty on an embed-only instance.
# --------------------------------------------------------------------------

variable "users" {
  description = "Internal users, keyed by email. Requires an organization API key."
  type = map(object({
    display_name = string
    attributes   = optional(map(string))
  }))
  default = {}
}

variable "user_attributes" {
  description = <<-DESC
    User attribute definitions to reference, as a local key to the definition's
    ID. Definitions are read-only in the API and created in the UI under
    Settings > User attributes. List them with:

      curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
        "$OMNI_BASE_URL/v1/user-attributes"

    IDs rather than names, because a definition can be renamed and a
    configuration naming it would stop matching in silence.
  DESC

  type    = map(string)
  default = {}
}

variable "user_attribute_values" {
  description = <<-DESC
    Attribute values per person, as an email to a map of keys from
    var.user_attributes to values.

    Per person because Omni has no concept of a group-assigned value. A user's
    value comes from a direct assignment or the definition's default, and
    nothing else: the attribute's Users page in the UI offers exactly those two
    sources.

    Groups do have "allowed user attribute values", which is a different thing
    - the set a member may switch an attribute to - and is not an assignment.
  DESC

  type    = map(map(string))
  default = {}

  validation {
    condition = alltrue([
      for email in keys(var.user_attribute_values) : contains(keys(var.users), email)
    ])
    error_message = format(
      "These attribute assignments name a user that is not in var.users: %s. Terraform can only set attributes on users it manages.",
      join("; ", [
        for email in keys(var.user_attribute_values) : email
        if !contains(keys(var.users), email)
      ])
    )
  }
}

variable "groups" {
  description = <<-DESC
    Internal groups, keyed by a stable name of your choosing. model and
    connection are keys from var.models and var.connections.

    The key is the group's identity. It becomes the visible name unless
    display_name overrides it, and re-keying replaces the group rather than
    renaming it, so set display_name to change only the label.
  DESC

  type = map(object({
    # Overrides the key as the visible name. Set this to rename a group without
    # re-keying it, which would replace it instead.
    display_name = optional(string)

    # Prepended to the key when display_name is unset, so a family of groups
    # stays recognisable without the prefix appearing in every key. Embed
    # groups conventionally use "tenant-".
    #
    # Defaults to "" rather than null: this gets interpolated, and coalesce
    # rejects empty strings as well as nulls, so a null would fail the call.
    name_prefix = optional(string, "")

    members = optional(list(string), [])

    # Whether Terraform owns the membership list.
    #
    # true, the default: members above are authoritative and anyone added
    # outside Terraform is removed on the next apply. Right for internal groups.
    #
    # false: Omni keeps whatever membership it has. Required for an embed group,
    # whose sessions create their own users - an authoritative list would delete
    # them on the next apply.
    manage_members = optional(bool, true)

    model      = optional(string)
    connection = optional(string)
    model_role = optional(string, "QUERIER")

    # A role on the whole connection rather than one model, usually
    # CONNECTION_ADMIN. Requires connection and ignores model.
    connection_role = optional(string)

    # Optional palette for this group, picked up by the branding module.
    colors = optional(list(string))
  }))
  default = {}

  validation {
    condition = alltrue([
      for k, v in var.groups : length(v.members) == 0 if !v.manage_members
    ])
    error_message = format(
      "These groups list members but set manage_members = false: %s. The two contradict each other: with manage_members false Terraform does not own the membership list and the members would be ignored. Drop the members, or let Terraform manage them.",
      join("; ", [
        for k, v in var.groups : "group \"${k}\""
        if !v.manage_members && length(v.members) > 0
      ])
    )
  }

  validation {
    condition     = alltrue([for k, v in var.groups : v.model == null || v.connection != null])
    error_message = "A group with a model also needs a connection: a model role is scoped to both."
  }

  validation {
    condition     = alltrue([for k, v in var.groups : v.connection_role == null || v.connection != null])
    error_message = "A group with a connection_role also needs a connection."
  }

  # "__unset__" rather than "": Terraform's coalesce skips empty strings as
  # well as nulls, so coalesce(x, "") errors with nothing left to return. The
  # sentinel is never a real key, and || does not short-circuit, so both sides
  # of every guard below have to be safe to evaluate.
  validation {
    condition = alltrue([
      for k, v in var.groups :
      v.connection == null || contains(keys(var.connections), coalesce(v.connection, "__unset__"))
    ])
    error_message = format(
      "These groups reference a connection that is not in var.connections: %s. Remove the reference or restore the connection.",
      join("; ", [
        for k, v in var.groups : "group \"${k}\" needs connection \"${v.connection}\""
        if v.connection != null && !contains(keys(var.connections), coalesce(v.connection, "__unset__"))
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.groups :
      v.model == null || contains(keys(var.models), coalesce(v.model, "__unset__"))
    ])
    error_message = format(
      "These groups hold a role on a model that is not in var.models: %s. A model cannot be removed or re-keyed while something references it. Update the reference in the same change.",
      join("; ", [
        for k, v in var.groups : "group \"${k}\" needs model \"${v.model}\""
        if v.model != null && !contains(keys(var.models), coalesce(v.model, "__unset__"))
      ])
    )
  }
}

variable "tenant_attribute_id" {
  description = <<-DESC
    ID of the user attribute that scopes tenants. Preferred over
    tenant_attribute below, for the same reason every other reference in this
    configuration moved to IDs: a definition can be renamed in the UI, and a
    name that no longer matches resolves to nothing while remaining a valid
    configuration. On an attribute driving an access filter that is row-level
    security quietly ceasing to apply, with nothing failing to say so.

    Read it from GET /v1/user-attributes. Definitions are made in the UI; the
    API has no create endpoint. Set this and tenant_attribute is ignored.
  DESC

  type    = string
  default = null
}

variable "tenant_attribute" {
  description = <<-DESC
    Name of the user attribute that scopes tenants. Must already exist.

    Kept for configurations that predate tenant_attribute_id and as a
    convenience when the ID is not to hand. Prefer the ID: a rename breaks this
    silently.
  DESC

  type    = string
  default = "tenant_id"
}

variable "tenant_base_connection" {
  description = "Key from var.connections that sessions query through. Only needed when tenant_routing is used."
  type        = string
  default     = null

  validation {
    condition = var.tenant_base_connection == null || contains(
      keys(var.connections), coalesce(var.tenant_base_connection, "__unset__")
    )
    # coalesce, not a bare interpolation: Terraform evaluates error_message
    # templates even when the condition passes, and a null in a template is an
    # error in its own right.
    error_message = "tenant_base_connection is \"${coalesce(var.tenant_base_connection, "unset")}\", which is not in var.connections. Embed routing queries through it, so set tenant_base_connection to null when removing that connection."
  }
}

variable "tenant_routing" {
  description = <<-DESC
    Tenants with a connection of their own, for physical isolation. Keyed by
    tenant; connection is a key from var.connections.

    Leave empty for the common case, where every tenant shares one connection
    and rows are isolated by access filters keyed off the user attribute.
  DESC

  type = map(object({
    connection            = string
    user_attribute_values = optional(list(string))
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.tenant_routing : contains(keys(var.connections), v.connection)
    ])
    error_message = format(
      "These tenant_routing entries name a connection that is not in var.connections: %s.",
      join("; ", [
        for k, v in var.tenant_routing : "tenant \"${k}\" routes to connection \"${v.connection}\""
        if !contains(keys(var.connections), v.connection)
      ])
    )
  }
}

# --------------------------------------------------------------------------
# Content access. Folders and who can see them, for tenants and internal
# groups alike.
# --------------------------------------------------------------------------

variable "folders" {
  description = <<-DESC
    Folders, keyed by a local name that the grants below reference.

    A folder is either looked up or created, and existing_path is what decides
    which. Set it and the folder is looked up by its path in Omni, so a missing
    one fails at plan time rather than being silently created alongside the real
    one. Leave it unset and the folder is created, which requires name.

    parent nests a created folder inside another key. One level only: a hub with
    subfolders is what this supports, and a deeper tree would need either a
    third resource or folders creating themselves by path.

    scope is required on a created top-level folder and must not be set on a
    nested one, which inherits its parent's:

      organization  org-wide shared content, with a grant adding a role
      restricted    reachable only through a grant, and needs owner_id when the
                    provider uses an organization API key

    No grants here. Who can see a folder is var.content_grants, so there is one
    place to look rather than two.
  DESC

  type = map(object({
    # Look this folder up instead of creating it. The path as Omni shows it.
    existing_path = optional(string)

    name               = optional(string)
    parent             = optional(string)
    scope              = optional(string)
    owner_id           = optional(string)
    delete_recursively = optional(bool, false)
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.folders : v.existing_path != null || v.name != null
    ])
    error_message = format(
      "These folders neither look one up nor create one: %s. Set existing_path to look a folder up by its path, or name to create it.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\""
        if v.existing_path == null && v.name == null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      v.existing_path == null || (v.name == null && v.parent == null && v.scope == null && v.owner_id == null)
    ])
    error_message = format(
      "These folders set existing_path alongside attributes that only apply to a folder being created: %s. A looked-up folder already has its name, parent and scope; Terraform does not change them.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\""
        if v.existing_path != null && !(v.name == null && v.parent == null && v.scope == null && v.owner_id == null)
      ])
    )
  }

  # Every check below resolves coalesce(v.parent, k) rather than v.parent, because
  # Terraform's || does not short-circuit: "v.parent == null || f(v.parent)" still
  # calls f with null and fails on the argument instead of reporting the condition.
  validation {
    condition = alltrue([
      for k, v in var.folders : contains(keys(var.folders), coalesce(v.parent, k))
    ])
    error_message = format(
      "These folders name a parent that is not another key in var.folders: %s.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\" names parent \"${v.parent}\""
        if v.parent != null && !contains(keys(var.folders), coalesce(v.parent, "__unset__"))
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      try(var.folders[coalesce(v.parent, k)].parent, null) == null
    ])
    error_message = "var.folders supports one level of nesting: a parent cannot itself have a parent."
  }

  # The sentinel rather than k: coalesce(v.parent, k) makes a parentless folder
  # look at itself, and a looked-up folder would then fail its own check on its
  # own existing_path. "__unset__" is never a real key, so the lookup misses and
  # try returns null, which is the pass.
  validation {
    condition = alltrue([
      for k, v in var.folders :
      try(var.folders[coalesce(v.parent, "__unset__")].existing_path, null) == null
    ])
    error_message = format(
      "These folders nest inside a looked-up folder: %s. A parent has to be one Terraform creates, because nesting is set at creation.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\" names parent \"${v.parent}\""
        if v.parent != null && try(var.folders[coalesce(v.parent, "__unset__")].existing_path, null) != null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      v.existing_path != null || v.parent != null || v.scope != null
    ])
    error_message = format(
      "These top-level folders do not state a scope: %s. Set organization or restricted. Left unset, the provider silently chooses organization, which is a decision about who can reach the folder before any grant applies.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\""
        if v.existing_path == null && v.parent == null && v.scope == null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      v.scope == null || contains(["organization", "restricted"], coalesce(v.scope, "organization"))
    ])
    error_message = "scope must be organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      v.parent == null || v.scope == null
    ])
    error_message = format(
      "These nested folders state a scope: %s. A nested folder inherits its parent's, and setting one here would be ignored.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\""
        if v.parent != null && v.scope != null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      coalesce(v.scope, "organization") != "restricted" || v.owner_id != null
    ])
    error_message = format(
      "These restricted folders have no owner_id: %s. A restricted folder needs one when the provider uses an organization API key.",
      join("; ", [
        for k, v in var.folders : "folder \"${k}\""
        if coalesce(v.scope, "organization") == "restricted" && v.owner_id == null
      ])
    )
  }
}

variable "document_grants" {
  description = <<-DESC
    Grants on a single document, keyed by a local name. groups are keys from
    var.groups.

    Prefer a folder grant. It covers everything in the folder and keeps access
    describable in one place; a document grant is for the case a folder grant
    cannot express, such as one dashboard shared more widely than the folder
    around it.

    document_id is a literal identifier rather than a reference. Documents are
    created in the UI, so there is no Terraform-side name to resolve, and this
    is the one place a raw ID in configuration is unavoidable. Take it from the
    document's URL.

    Destroying a grant does not revoke it: the documents API has no endpoint for
    that, so the role is downgraded to NO_ACCESS, and a permissive grant
    inherited from the enclosing folder still applies.
  DESC

  type = map(object({
    document_id = string
    role        = optional(string, "VIEWER")
    groups      = optional(list(string), [])
    user_ids    = optional(list(string))

    # Rejected with 403 when the organization has AccessBoost turned off.
    access_boost = optional(bool)
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.document_grants :
      length(v.groups) + length(coalesce(v.user_ids, [])) > 0
    ])
    error_message = format(
      "These document grants name no groups and no users: %s. A grant needs at least one of groups or user_ids; the provider rejects an empty one. Remove the grant rather than leaving it empty.",
      join("; ", [
        for k, v in var.document_grants : "grant \"${k}\" on document \"${v.document_id}\""
        if length(v.groups) + length(coalesce(v.user_ids, [])) == 0
      ])
    )
  }

  validation {
    condition = alltrue(flatten([
      for k, v in var.document_grants : [
        for g in v.groups : contains(keys(var.groups), g)
      ]
    ]))
    error_message = format(
      "These document grants name a group that is not in var.groups: %s. Drop it from the grant in the same change.",
      join("; ", flatten([
        for k, v in var.document_grants : [
          for g in v.groups : "grant \"${k}\" names group \"${g}\""
          if !contains(keys(var.groups), g)
        ]
      ]))
    )
  }
}

variable "content_grants" {
  description = <<-DESC
    Who can see which folder. folder is a key from var.folders; groups are keys
    from var.groups.

    The only place a folder grant is expressed. var.folders says which folders
    exist and this says who can see them, so there is one place to look for
    each question.
  DESC

  type = map(object({
    folder   = string
    role     = optional(string, "VIEWER")
    groups   = optional(list(string), [])
    user_ids = optional(list(string))
  }))

  default = {}

  validation {
    # A grant with no subjects reaches the API as an empty list and comes back
    # as "userIds or userGroupIds must be provided". Catch it here, where the
    # message can name the grant.
    condition = alltrue([
      for k, v in var.content_grants :
      length(v.groups) + length(coalesce(v.user_ids, [])) > 0
    ])
    error_message = format(
      "These content grants name no groups and no users: %s. A grant needs at least one of groups or user_ids. Remove the grant rather than leaving it empty.",
      join("; ", [
        for k, v in var.content_grants : "grant \"${k}\" on folder \"${v.folder}\""
        if length(v.groups) + length(coalesce(v.user_ids, [])) == 0
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.content_grants :
      contains(keys(var.folders), v.folder)
    ])
    error_message = format(
      "These content grants name a folder that is not in var.folders: %s.",
      join("; ", [
        for k, v in var.content_grants : "grant \"${k}\" needs folder \"${v.folder}\""
        if !contains(keys(var.folders), v.folder)
      ])
    )
  }

  validation {
    condition = alltrue(flatten([
      for k, v in var.content_grants : [
        for g in v.groups : contains(keys(var.groups), g)
      ]
    ]))
    error_message = format(
      "These content grants name a group that is not in var.groups: %s. A group cannot be removed while a grant names it. Drop it from the grant in the same change.",
      join("; ", flatten([
        for k, v in var.content_grants : [
          for g in v.groups : "grant \"${k}\" names group \"${g}\""
          if !contains(keys(var.groups), g)
        ]
      ]))
    )
  }
}
