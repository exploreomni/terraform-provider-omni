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
    var.tenant_groups reference. Set name to rename a model without re-keying:
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
        "$OMNI_BASE_URL/api/v1/user-attributes"

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

    Per person because Omni has no API for assigning values to a group: the
    SCIM group resource carries no attribute extension and the only attribute
    endpoint is a read.
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

    members    = optional(list(string), [])
    model      = optional(string)
    connection = optional(string)
    model_role = optional(string, "QUERIER")

    # A role on the whole connection rather than one model, usually
    # CONNECTION_ADMIN. Requires connection and ignores model.
    connection_role = optional(string)
  }))
  default = {}

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

variable "folders" {
  description = <<-DESC
    Internal folders, keyed by path.

    scope is required, not defaulted. It decides who can reach the folder
    before the groups below are granted anything:

      organization  org-wide shared content, with the grant adding a role
      restricted    reachable only through the grant, and needs owner_id when
                    the provider uses an organization API key

    Most internal folders are organization. Use restricted for content one
    team should hold on its own.
  DESC

  type = map(object({
    name     = string
    scope    = optional(string)
    owner_id = optional(string)
    groups   = optional(list(string), [])
    role     = optional(string, "EDITOR")
  }))
  default = {}

  validation {
    condition     = alltrue([for k, v in var.folders : v.scope != null])
    error_message = "Every folder must state a scope: organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      contains(["organization", "restricted"], coalesce(v.scope, "organization"))
    ])
    error_message = "scope must be organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      coalesce(v.scope, "organization") != "restricted" || v.owner_id != null
    ])
    error_message = "A restricted folder needs owner_id when the provider uses an organization API key."
  }
}

# --------------------------------------------------------------------------
# Embed. Three independent maps. Configure only what a deployment needs.
# --------------------------------------------------------------------------

variable "tenant_attribute" {
  description = "Reference of the user attribute that scopes tenants. Must already exist."
  type        = string
  default     = "tenant_id"
}

variable "tenant_group_prefix" {
  description = "Prefix for tenant group names."
  type        = string
  default     = "tenant-"
}

variable "tenant_groups" {
  description = <<-DESC
    Tenant groups, keyed by tenant. The key plus the prefix is the name a
    signed URL passes in "groups".

    Set model and connection to also grant a role on a model. Omit them and
    only the group is created.
  DESC

  type = map(object({
    # Overrides the generated prefix + key. Set this to rename a tenant group
    # without re-keying it, which would replace it.
    display_name = optional(string)

    model      = optional(string)
    connection = optional(string)
    model_role = optional(string, "QUERY_TOPICS")

    # Optional per-tenant palette, picked up by the branding module.
    colors = optional(list(string))
  }))

  default = {}

  validation {
    condition     = alltrue([for k, v in var.tenant_groups : v.model == null || v.connection != null])
    error_message = "A tenant group with a model also needs a connection: a model role is scoped to both."
  }

  validation {
    condition = alltrue([
      for k, v in var.tenant_groups :
      v.connection == null || contains(keys(var.connections), coalesce(v.connection, "__unset__"))
    ])
    error_message = format(
      "These tenant groups reference a connection that is not in var.connections: %s. Remove the reference or restore the connection.",
      join("; ", [
        for k, v in var.tenant_groups : "tenant \"${k}\" needs connection \"${v.connection}\""
        if v.connection != null && !contains(keys(var.connections), coalesce(v.connection, "__unset__"))
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.tenant_groups :
      v.model == null || contains(keys(var.models), coalesce(v.model, "__unset__"))
    ])
    error_message = format(
      "These tenant groups hold a role on a model that is not in var.models: %s. A model cannot be removed or re-keyed while a tenant references it. Rename with the name attribute instead of re-keying, or update the reference in the same change.",
      join("; ", [
        for k, v in var.tenant_groups : "tenant \"${k}\" needs model \"${v.model}\""
        if v.model != null && !contains(keys(var.models), coalesce(v.model, "__unset__"))
      ])
    )
  }
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

variable "existing_folders" {
  description = "Folders that already exist, keyed by a local name, valued by path. Looked up, never created."
  type        = map(string)
  default     = {}
}

variable "managed_folders" {
  description = <<-DESC
    Folders to create, keyed by local name.

    scope is required on a top-level folder: organization or restricted. A
    restricted folder also needs owner_id when the provider uses an
    organization API key. A nested folder inherits its parent's scope and must
    not set one.

    Every attribute the module accepts has to be declared here too. Terraform
    drops an attribute a variable's type does not mention, without warning, so
    an undeclared scope would be discarded on the way through and the folder
    would silently fall back to the provider default.
  DESC

  type = map(object({
    name               = string
    parent             = optional(string)
    scope              = optional(string)
    owner_id           = optional(string)
    delete_recursively = optional(bool, false)
  }))
  default = {}

  # Checked here as well as in the module so the message can name the folder
  # before the plan reaches the module at all.
  #
  # Every check resolves coalesce(v.parent, k), never v.parent directly:
  # Terraform's || does not short-circuit, so "v.parent == null || f(v.parent)"
  # still calls f with null and fails on the argument instead of reporting the
  # condition.
  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      contains(keys(var.managed_folders), coalesce(v.parent, k))
    ])
    error_message = format(
      "These folders name a parent that is not another key in managed_folders: %s.",
      join("; ", [
        for k, v in var.managed_folders : "folder \"${k}\" names parent \"${v.parent}\""
        if v.parent != null && !contains(keys(var.managed_folders), coalesce(v.parent, "__unset__"))
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      try(var.managed_folders[coalesce(v.parent, k)].parent, null) == null
    ])
    error_message = "managed_folders supports one level of nesting: a parent cannot itself have a parent."
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent != null || v.scope != null
    ])
    error_message = format(
      "These top-level folders do not state a scope: %s. Set organization or restricted. Left unset, the provider silently chooses organization, which is a decision about who can reach the folder before any grant applies.",
      join("; ", [for k, v in var.managed_folders : "folder \"${k}\"" if v.parent == null && v.scope == null])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      contains(["organization", "restricted"], coalesce(v.scope, "organization"))
    ])
    error_message = "scope must be organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent == null || v.scope == null
    ])
    error_message = "A nested folder inherits its parent's scope and must not set scope itself."
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      coalesce(v.scope, "organization") != "restricted" || v.owner_id != null
    ])
    error_message = format(
      "These restricted folders have no owner_id: %s. A restricted folder needs an owner when the provider uses an organization API key.",
      join("; ", [
        for k, v in var.managed_folders : "folder \"${k}\""
        if coalesce(v.scope, "organization") == "restricted" && v.owner_id == null
      ])
    )
  }
}

variable "content_grants" {
  description = <<-DESC
    Who can see which folder. folder is a key from existing_folders or
    managed_folders; tenant_groups and internal_groups are keys from
    var.tenant_groups and var.groups.
  DESC

  type = map(object({
    folder          = string
    role            = optional(string, "VIEWER")
    tenant_groups   = optional(list(string), [])
    internal_groups = optional(list(string), [])
    user_ids        = optional(list(string))
  }))

  default = {}

  validation {
    # A grant with no subjects reaches the API as an empty list and comes back
    # as "userIds or userGroupIds must be provided". Catch it here, where the
    # message can name the grant.
    condition = alltrue([
      for k, v in var.content_grants :
      length(v.tenant_groups) + length(v.internal_groups) + length(coalesce(v.user_ids, [])) > 0
    ])
    error_message = format(
      "These content grants name no groups and no users: %s. A grant needs at least one of tenant_groups, internal_groups or user_ids. Remove the grant rather than leaving it empty.",
      join("; ", [
        for k, v in var.content_grants : "grant \"${k}\" on folder \"${v.folder}\""
        if length(v.tenant_groups) + length(v.internal_groups) + length(coalesce(v.user_ids, [])) == 0
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.content_grants :
      contains(concat(keys(var.existing_folders), keys(var.managed_folders)), v.folder)
    ])
    error_message = format(
      "These content grants name a folder that is in neither existing_folders nor managed_folders: %s.",
      join("; ", [
        for k, v in var.content_grants : "grant \"${k}\" needs folder \"${v.folder}\""
        if !contains(concat(keys(var.existing_folders), keys(var.managed_folders)), v.folder)
      ])
    )
  }

  validation {
    condition = alltrue(flatten([
      for k, v in var.content_grants : [
        for g in v.tenant_groups : contains(keys(var.tenant_groups), g)
      ]
    ]))
    error_message = format(
      "These content grants name a tenant group that is not in var.tenant_groups: %s. A tenant group cannot be removed while a grant names it. Drop it from the grant in the same change.",
      join("; ", flatten([
        for k, v in var.content_grants : [
          for g in v.tenant_groups : "grant \"${k}\" names tenant group \"${g}\""
          if !contains(keys(var.tenant_groups), g)
        ]
      ]))
    )
  }

  validation {
    condition = alltrue(flatten([
      for k, v in var.content_grants : [
        for g in v.internal_groups : contains(keys(var.groups), g)
      ]
    ]))
    error_message = format(
      "These content grants name an internal group that is not in var.groups: %s. Drop it from the grant in the same change.",
      join("; ", flatten([
        for k, v in var.content_grants : [
          for g in v.internal_groups : "grant \"${k}\" names internal group \"${g}\""
          if !contains(keys(var.groups), g)
        ]
      ]))
    )
  }
}
