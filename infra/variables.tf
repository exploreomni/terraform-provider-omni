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
    Attribute definitions, keyed by a local name the rest of this configuration
    references. An attribute is what an access filter keys off, so this is
    usually the first thing a row-level security setup needs.

    existing_id references a definition made elsewhere instead of creating one.
    Use it for anything the UI created before Terraform arrived, and for
    anything another team owns. Without it the definition is created here and
    label is required; name defaults to the key.

    Definitions became creatable on 2026-09-21. Before that this variable held
    an ID per key and nothing else, because reference was all the API allowed.

    Destroying a definition removes the value every user holds for it, and
    breaks model SQL and embed SSO URLs that name it. Neither is reported.
  DESC

  type = map(object({
    # Reference rather than create. From GET /v1/user-attributes, or the UI.
    existing_id = optional(string)

    label           = optional(string)
    name            = optional(string)
    type            = optional(string, "String")
    description     = optional(string)
    multiple_values = optional(bool, false)

    # One default, or several for a multi-valued attribute. A Number attribute
    # takes its default as a string, which is how Omni stores numbers so that
    # values beyond 2^53 - 1 keep their precision.
    default_value  = optional(string)
    default_values = optional(list(string))
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.user_attributes : v.existing_id != null || v.label != null
    ])
    error_message = format(
      "These attributes neither reference one nor create one: %s. Set existing_id to reference a definition, or label to create it.",
      join("; ", [
        for k, v in var.user_attributes : "attribute \"${k}\""
        if v.existing_id == null && v.label == null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.user_attributes :
      v.existing_id == null || (v.label == null && v.name == null && v.description == null && v.default_value == null && v.default_values == null)
    ])
    error_message = format(
      "These attributes set existing_id alongside attributes that only apply to one being created: %s. A referenced definition already has its label, name and default, and Terraform does not change them.",
      join("; ", [
        for k, v in var.user_attributes : "attribute \"${k}\""
        if v.existing_id != null && !(v.label == null && v.name == null && v.description == null && v.default_value == null && v.default_values == null)
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.user_attributes :
      v.type == null || contains(["String", "Number"], coalesce(v.type, "String"))
    ])
    error_message = "type must be String or Number."
  }

  validation {
    condition = alltrue([
      for k, v in var.user_attributes : v.default_value == null || v.default_values == null
    ])
    error_message = format(
      "These attributes set both default_value and default_values: %s. The API takes one default whose shape follows multiple_values, so set the single one or the list, not both.",
      join("; ", [
        for k, v in var.user_attributes : "attribute \"${k}\""
        if v.default_value != null && v.default_values != null
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.user_attributes : v.default_values == null || v.multiple_values
    ])
    error_message = format(
      "These attributes give several defaults without multiple_values: %s. Only a multi-valued attribute holds more than one default; set multiple_values = true, or use default_value.",
      join("; ", [
        for k, v in var.user_attributes : "attribute \"${k}\""
        if v.default_values != null && !v.multiple_values
      ])
    )
  }
}

variable "attribute_selectable_values" {
  description = <<-DESC
    Values a user or group may switch an attribute to, keyed by a local name.

    This is not an assigned value. It is the set the subject can switch between
    from the File menu of a dashboard, workbook or model, and their actual value
    still comes from an assignment or the definition's default.

    It grants data access. Access filters and shared extensions commonly route
    on the same attribute, so letting someone switch to another tenant's value
    usually lets them read that tenant's rows. List only what the subject should
    see, and treat allow_any_value as the wide grant it is.

    attribute is a key from var.user_attributes. Exactly one subject:

      group_uuid  a group's UUID. Not a key from var.groups, and not
                  omni_user_group.id - see the note below.
      user        an email from var.users.

    group_uuid is a literal because this endpoint is the only one in the API
    that identifies a group by its UUID primary key. Every other one - model
    roles, connection roles, folder and document permissions, SCIM - uses the
    eight-character mini_uuid that omni_user_group.id holds, and no endpoint
    converts between them. Finding 43.

    Getting a UUID, in order of preference: query the database
    (select id, mini_uuid, name from user_groups), or open the group in Omni at
    /groups/<mini>/<name>/members and take the UUID that appears on that page
    and no other. Confirm it before using it, because the endpoint answers 200
    for any group in the organization and provisioning the wrong one grants the
    wrong people data access:

      curl -s -o /dev/null -w '%%{http_code}\n' \
        -H "Authorization: Bearer $OMNI_API_TOKEN" \
        "$OMNI_BASE_URL/api/v1/user-attributes/<attribute id>/selectable-values?groupId=<uuid>"

    A UUID belongs to the group row, so replacing a group makes it stale. That
    fails the apply with a 404 rather than provisioning quietly, which is the
    one good thing about this arrangement.
  DESC

  type = map(object({
    attribute  = string
    group_uuid = optional(string)
    user       = optional(string)
    values     = optional(list(string))

    # Whether the subject may switch to a value that is not listed.
    allow_any_value = optional(bool, false)
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.attribute_selectable_values :
      (v.group_uuid == null) != (v.user == null)
    ])
    error_message = format(
      "These entries do not name exactly one subject: %s. Set group_uuid or user, not both and not neither.",
      join("; ", [
        for k, v in var.attribute_selectable_values : "entry \"${k}\""
        if(v.group_uuid == null) == (v.user == null)
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.attribute_selectable_values :
      contains(keys(var.user_attributes), v.attribute)
    ])
    error_message = format(
      "These entries name an attribute that is not in var.user_attributes: %s.",
      join("; ", [
        for k, v in var.attribute_selectable_values : "entry \"${k}\" names \"${v.attribute}\""
        if !contains(keys(var.user_attributes), v.attribute)
      ])
    )
  }

  # A mini_uuid is eight characters and a UUID is thirty-six, so the common
  # mistake - passing omni_user_group.id, or a key from var.groups - is caught
  # here with an explanation rather than by the API with "Invalid UUID".
  validation {
    condition = alltrue([
      for k, v in var.attribute_selectable_values :
      v.group_uuid == null || can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", lower(coalesce(v.group_uuid, "__unset__"))))
    ])
    error_message = format(
      "These entries do not give a UUID for group_uuid: %s. This endpoint wants the group's UUID primary key, not the eight-character id that omni_user_group.id holds and that the rest of this configuration uses. See the variable description for how to find it.",
      join("; ", [
        for k, v in var.attribute_selectable_values : "entry \"${k}\" gives \"${v.group_uuid}\""
        if v.group_uuid != null && !can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", lower(coalesce(v.group_uuid, "__unset__"))))
      ])
    )
  }

  validation {
    condition = alltrue([
      for k, v in var.attribute_selectable_values :
      v.user == null || contains(keys(var.users), coalesce(v.user, "__unset__"))
    ])
    error_message = format(
      "These entries name a user that is not in var.users: %s. Terraform can only provision for a user it manages.",
      join("; ", [
        for k, v in var.attribute_selectable_values : "entry \"${k}\" names user \"${v.user}\""
        if v.user != null && !contains(keys(var.users), coalesce(v.user, "__unset__"))
      ])
    )
  }
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

    Read it from GET /v1/user-attributes, or from var.user_attributes if
    Terraform created it. Set this and tenant_attribute is ignored.
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
